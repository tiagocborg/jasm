package provider

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestNewOnePasswordProvider_NoToken(t *testing.T) {
	os.Unsetenv(opServiceAccountEnv)
	defer os.Unsetenv(opServiceAccountEnv)

	_, err := NewOnePasswordProvider(context.Background())
	if err == nil {
		t.Error("expected error when token not set")
	}
	if err.Error() != "1Password service account token not configured (set OP_SERVICE_ACCOUNT_TOKEN)" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestOnePasswordProvider_Name(t *testing.T) {
	// Skip if no token available (unit test)
	if os.Getenv(opServiceAccountEnv) == "" {
		t.Skip("skipping test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	p, err := NewOnePasswordProvider(context.Background())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if p.Name() != "1password" {
		t.Errorf("Name() = %q, want %q", p.Name(), "1password")
	}
}

func TestOnePasswordProvider_FetchSecret_InvalidReference(t *testing.T) {
	// Skip if no token available
	if os.Getenv(opServiceAccountEnv) == "" {
		t.Skip("skipping test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	p, err := NewOnePasswordProvider(context.Background())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "missing op:// prefix",
			path:    "vault/item/field",
			wantErr: "must start with op://",
		},
		{
			name:    "missing field",
			path:    "op://vault/item",
			wantErr: "missing field",
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: "must start with op://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.FetchSecret(context.Background(), tt.path)
			if err == nil {
				t.Error("expected error for invalid reference")
				return
			}
			if !containsStr(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "rate limit error",
			err:  errors.New("rate limit exceeded"),
			want: true,
		},
		{
			name: "429 error",
			err:  errors.New("HTTP 429 Too Many Requests"),
			want: true,
		},
		{
			name: "too many requests",
			err:  errors.New("too many requests"),
			want: true,
		},
		{
			name: "regular error",
			err:  errors.New("some other error"),
			want: false,
		},
		{
			name: "not found error",
			err:  errors.New("item not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRateLimited(tt.err); got != tt.want {
				t.Errorf("isRateLimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOnePasswordProvider_WrapError(t *testing.T) {
	// Skip if no token available
	if os.Getenv(opServiceAccountEnv) == "" {
		t.Skip("skipping test: OP_SERVICE_ACCOUNT_TOKEN not set")
	}

	p, err := NewOnePasswordProvider(context.Background())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ref := &SecretReference{
		Vault: "TestVault",
		Item:  "TestItem",
		Field: "TestField",
	}

	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "vault not found",
			err:     errors.New("vault not found"),
			wantMsg: "vault \"TestVault\" not found or access denied",
		},
		{
			name:    "item not found",
			err:     errors.New("item not found"),
			wantMsg: "item \"TestItem\" not found in vault \"TestVault\"",
		},
		{
			name:    "field not found",
			err:     errors.New("field not found"),
			wantMsg: "field \"TestField\" not found in item \"TestItem\"",
		},
		{
			name:    "authentication error",
			err:     errors.New("unauthorized access"),
			wantMsg: "authentication failed",
		},
		{
			name:    "network error",
			err:     errors.New("network error: connection refused"),
			wantMsg: "failed to connect to 1Password",
		},
		{
			name:    "generic error",
			err:     errors.New("some unknown error"),
			wantMsg: "failed to resolve secret from 1Password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := p.wrapError(tt.err, ref)
			if !containsStr(wrapped.Error(), tt.wantMsg) {
				t.Errorf("wrapError() = %q, want to contain %q", wrapped.Error(), tt.wantMsg)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
