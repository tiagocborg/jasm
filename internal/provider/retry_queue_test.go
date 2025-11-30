package provider

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRetryQueue(t *testing.T) {
	tests := []struct {
		name       string
		maxBackoff time.Duration
	}{
		{
			name:       "default max backoff",
			maxBackoff: 0,
		},
		{
			name:       "custom max backoff",
			maxBackoff: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewRetryQueue(tt.maxBackoff, nil)
			if q == nil {
				t.Error("NewRetryQueue returned nil")
			}
		})
	}
}

func TestRetryQueue_Add(t *testing.T) {
	var depthChanges []int64
	q := NewRetryQueue(time.Minute, func(depth int64) {
		depthChanges = append(depthChanges, depth)
	})

	ctx := context.Background()

	q.Add(ctx, RetryItem{Path: "test1"})
	q.Add(ctx, RetryItem{Path: "test2"})
	q.Add(ctx, RetryItem{Path: "test3"})

	if q.Depth() != 3 {
		t.Errorf("Depth = %d, want 3", q.Depth())
	}

	if len(depthChanges) != 3 {
		t.Errorf("depthChanges length = %d, want 3", len(depthChanges))
	}
	if depthChanges[0] != 1 || depthChanges[1] != 2 || depthChanges[2] != 3 {
		t.Errorf("depthChanges = %v, want [1, 2, 3]", depthChanges)
	}
}

func TestRetryQueue_ProcessSuccess(t *testing.T) {
	var completed atomic.Bool
	q := NewRetryQueue(time.Minute, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q.Add(ctx, RetryItem{
		Path: "test",
		RetryFunc: func(ctx context.Context) error {
			return nil // Success immediately
		},
		OnComplete: func(err error) {
			completed.Store(true)
		},
	})

	q.Start(ctx)
	defer q.Stop()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	if !completed.Load() {
		t.Error("OnComplete was not called")
	}

	if q.Depth() != 0 {
		t.Errorf("Depth = %d, want 0", q.Depth())
	}
}

func TestRetryQueue_ProcessRetry(t *testing.T) {
	var attempts atomic.Int32
	q := NewRetryQueue(time.Minute, nil)
	q.baseBackoff = 10 * time.Millisecond // Speed up test

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q.Add(ctx, RetryItem{
		Path: "test",
		RetryFunc: func(ctx context.Context) error {
			count := attempts.Add(1)
			if count < 3 {
				return errors.New("temporary error")
			}
			return nil // Success on third attempt
		},
	})

	q.Start(ctx)
	defer q.Stop()

	// Wait for processing with retries
	time.Sleep(time.Second)

	if attempts.Load() < 3 {
		t.Errorf("attempts = %d, want >= 3", attempts.Load())
	}

	if q.Depth() != 0 {
		t.Errorf("Depth = %d, want 0", q.Depth())
	}
}

func TestRetryQueue_CalculateBackoff(t *testing.T) {
	q := NewRetryQueue(5*time.Minute, nil)
	q.baseBackoff = time.Second

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 0},
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 4 * time.Second},
		{attempt: 4, want: 8 * time.Second},
		{attempt: 5, want: 16 * time.Second},
		{attempt: 6, want: 32 * time.Second},
		{attempt: 7, want: 64 * time.Second},
		{attempt: 8, want: 128 * time.Second},
		{attempt: 9, want: 256 * time.Second},
		{attempt: 10, want: 5 * time.Minute}, // Capped at max
		{attempt: 20, want: 5 * time.Minute}, // Still capped
	}

	for _, tt := range tests {
		got := q.calculateBackoff(tt.attempt)
		if got != tt.want {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestRetryQueue_Stop(t *testing.T) {
	q := NewRetryQueue(time.Minute, nil)

	ctx := context.Background()

	q.Start(ctx)

	// Should not block or panic
	q.Stop()
	q.Stop() // Calling stop twice should be safe
}

func TestRetryQueue_StartTwice(t *testing.T) {
	q := NewRetryQueue(time.Minute, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	q.Start(ctx)
	q.Start(ctx) // Should be a no-op

	q.Stop()
}

func TestRetryQueue_ConcurrentAccess(t *testing.T) {
	q := NewRetryQueue(time.Minute, nil)
	q.baseBackoff = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var completed atomic.Int32
	numItems := 20 // Reduced for faster tests

	// Add items concurrently
	for i := 0; i < numItems; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Add(ctx, RetryItem{
				Path: "test",
				RetryFunc: func(ctx context.Context) error {
					return nil
				},
				OnComplete: func(err error) {
					completed.Add(1)
				},
			})
		}()
	}

	wg.Wait()

	if q.Depth() != int64(numItems) {
		t.Errorf("Depth after adds = %d, want %d", q.Depth(), numItems)
	}

	q.Start(ctx)

	// Wait for all items to be processed
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && q.Depth() > 0 {
		time.Sleep(50 * time.Millisecond)
	}

	q.Stop()

	if q.Depth() != 0 {
		t.Errorf("Final Depth = %d, want 0", q.Depth())
	}

	if completed.Load() != int32(numItems) {
		t.Errorf("completed = %d, want %d", completed.Load(), numItems)
	}
}

func TestRetryQueue_ContextCancellation(t *testing.T) {
	q := NewRetryQueue(time.Minute, nil)
	q.baseBackoff = time.Second // Slow backoff to test cancellation

	ctx, cancel := context.WithCancel(context.Background())

	q.Add(ctx, RetryItem{
		Path:    "test",
		Attempt: 1, // Start with attempt 1 to trigger backoff
		RetryFunc: func(ctx context.Context) error {
			return nil
		},
	})

	q.Start(ctx)

	// Cancel context while waiting for backoff
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Queue should stop gracefully
	time.Sleep(200 * time.Millisecond)
}
