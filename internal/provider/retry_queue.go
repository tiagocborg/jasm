package provider

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultBaseBackoff = time.Second
	defaultMaxBackoff  = 5 * time.Minute
)

// RetryItem represents an item to be retried.
type RetryItem struct {
	Path       string
	Attempt    int
	RetryFunc  func(ctx context.Context) error
	OnComplete func(err error)
}

// RetryQueue manages retries with exponential backoff.
type RetryQueue struct {
	baseBackoff   time.Duration
	maxBackoff    time.Duration
	onDepthChange func(int64)

	mu      sync.Mutex
	items   []*RetryItem
	depth   atomic.Int64
	running atomic.Bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewRetryQueue creates a new retry queue.
func NewRetryQueue(maxBackoff time.Duration, onDepthChange func(int64)) *RetryQueue {
	if maxBackoff <= 0 {
		maxBackoff = defaultMaxBackoff
	}
	return &RetryQueue{
		baseBackoff:   defaultBaseBackoff,
		maxBackoff:    maxBackoff,
		onDepthChange: onDepthChange,
		stopCh:        make(chan struct{}),
	}
}

// Add adds an item to the retry queue.
func (q *RetryQueue) Add(ctx context.Context, item RetryItem) {
	q.mu.Lock()
	q.items = append(q.items, &item)
	q.mu.Unlock()

	newDepth := q.depth.Add(1)
	if q.onDepthChange != nil {
		q.onDepthChange(newDepth)
	}
}

// Start begins processing the retry queue.
func (q *RetryQueue) Start(ctx context.Context) {
	if q.running.Swap(true) {
		return // Already running
	}

	q.wg.Add(1)
	go q.process(ctx)
}

// Stop stops the retry queue processor.
func (q *RetryQueue) Stop() {
	if !q.running.Load() {
		return
	}
	close(q.stopCh)
	q.wg.Wait()
	q.running.Store(false)
	q.stopCh = make(chan struct{})
}

// Depth returns the current queue depth.
func (q *RetryQueue) Depth() int64 {
	return q.depth.Load()
}

func (q *RetryQueue) process(ctx context.Context) {
	defer q.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.stopCh:
			return
		case <-ticker.C:
			q.processNextItem(ctx)
		}
	}
}

func (q *RetryQueue) processNextItem(ctx context.Context) {
	q.mu.Lock()
	if len(q.items) == 0 {
		q.mu.Unlock()
		return
	}
	item := q.items[0]
	q.items = q.items[1:]
	q.mu.Unlock()

	// Calculate backoff duration
	backoff := q.calculateBackoff(item.Attempt)

	// Wait for backoff duration
	select {
	case <-ctx.Done():
		// Context cancelled, re-add item
		q.mu.Lock()
		q.items = append([]*RetryItem{item}, q.items...)
		q.mu.Unlock()
		return
	case <-q.stopCh:
		// Stop requested, re-add item
		q.mu.Lock()
		q.items = append([]*RetryItem{item}, q.items...)
		q.mu.Unlock()
		return
	case <-time.After(backoff):
	}

	// Execute the retry
	err := item.RetryFunc(ctx)
	if err != nil {
		// Re-queue for another retry
		item.Attempt++
		q.mu.Lock()
		q.items = append(q.items, item)
		q.mu.Unlock()
		return
	}

	// Success - update depth
	newDepth := q.depth.Add(-1)
	if q.onDepthChange != nil {
		q.onDepthChange(newDepth)
	}

	// Call completion callback
	if item.OnComplete != nil {
		item.OnComplete(nil)
	}
}

func (q *RetryQueue) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Exponential backoff: baseBackoff * 2^(attempt-1)
	backoff := q.baseBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > q.maxBackoff {
			return q.maxBackoff
		}
	}
	return backoff
}
