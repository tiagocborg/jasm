package provider

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestParseBitwardenReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		errMsg   string
		folder   string
		itemName string
		field    string
	}{
		{
			name:     "valid folder/item",
			input:    "bw://my-folder/my-item",
			folder:   "my-folder",
			itemName: "my-item",
		},
		{
			name:     "valid folder/item/field",
			input:    "bw://my-folder/my-item/password",
			folder:   "my-folder",
			itemName: "my-item",
			field:    "password",
		},
		{
			name:     "url encoded characters",
			input:    "bw://my%20folder/my%20item",
			folder:   "my folder",
			itemName: "my item",
		},
		{
			name:     "trailing slash ignored",
			input:    "bw://folder/item/",
			folder:   "folder",
			itemName: "item",
		},
		{
			name:    "missing prefix",
			input:   "folder/item",
			wantErr: true,
			errMsg:  "must start with bw://",
		},
		{
			name:    "empty path",
			input:   "bw://",
			wantErr: true,
			errMsg:  "empty path",
		},
		{
			name:    "missing item name",
			input:   "bw://folder",
			wantErr: true,
			errMsg:  "missing item name",
		},
		{
			name:    "too many components",
			input:   "bw://a/b/c/d",
			wantErr: true,
			errMsg:  "too many path components",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseBitwardenReference(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ref.Folder != tt.folder {
				t.Errorf("Folder = %q, want %q", ref.Folder, tt.folder)
			}
			if ref.ItemName != tt.itemName {
				t.Errorf("ItemName = %q, want %q", ref.ItemName, tt.itemName)
			}
			if ref.Field != tt.field {
				t.Errorf("Field = %q, want %q", ref.Field, tt.field)
			}
		})
	}
}

func TestBitwardenSecretReferenceValidate(t *testing.T) {
	tests := []struct {
		name    string
		ref     BitwardenSecretReference
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid reference",
			ref:     BitwardenSecretReference{Folder: "folder", ItemName: "item"},
			wantErr: false,
		},
		{
			name:    "valid with field",
			ref:     BitwardenSecretReference{Folder: "folder", ItemName: "item", Field: "password"},
			wantErr: false,
		},
		{
			name:    "missing folder",
			ref:     BitwardenSecretReference{ItemName: "item"},
			wantErr: true,
			errMsg:  "folder name is required",
		},
		{
			name:    "missing item name",
			ref:     BitwardenSecretReference{Folder: "folder"},
			wantErr: true,
			errMsg:  "item name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBitwardenSecretReferenceString(t *testing.T) {
	tests := []struct {
		name string
		ref  BitwardenSecretReference
		want string
	}{
		{
			name: "folder and item",
			ref:  BitwardenSecretReference{Folder: "folder", ItemName: "item"},
			want: "bw://folder/item",
		},
		{
			name: "folder, item, and field",
			ref:  BitwardenSecretReference{Folder: "folder", ItemName: "item", Field: "password"},
			want: "bw://folder/item/password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBitwardenSecretReferenceHasField(t *testing.T) {
	t.Run("has field", func(t *testing.T) {
		ref := BitwardenSecretReference{Folder: "f", ItemName: "i", Field: "password"}
		if !ref.HasField() {
			t.Error("HasField() = false, want true")
		}
	})

	t.Run("no field", func(t *testing.T) {
		ref := BitwardenSecretReference{Folder: "f", ItemName: "i"}
		if ref.HasField() {
			t.Error("HasField() = true, want false")
		}
	})
}

func TestNewBitwardenClient_DefaultURL(t *testing.T) {
	client := NewBitwardenClient("", "test@example.com")

	if client.serverURL != defaultBitwardenURL {
		t.Errorf("serverURL = %q, want %q", client.serverURL, defaultBitwardenURL)
	}
	if client.email != "test@example.com" {
		t.Errorf("email = %q, want %q", client.email, "test@example.com")
	}
}

func TestNewBitwardenClient_CustomURL(t *testing.T) {
	customURL := "https://vaultwarden.example.com"
	client := NewBitwardenClient(customURL, "test@example.com")

	if client.serverURL != customURL {
		t.Errorf("serverURL = %q, want %q", client.serverURL, customURL)
	}
}

func TestNewBitwardenClient_URLNormalization(t *testing.T) {
	client := NewBitwardenClient("https://vault.example.com/", "TEST@EXAMPLE.COM")

	if client.serverURL != "https://vault.example.com" {
		t.Errorf("serverURL = %q, want trailing slash removed", client.serverURL)
	}
	if client.email != "test@example.com" {
		t.Errorf("email = %q, want lowercase", client.email)
	}
}

func TestDecryptedItemToSecretMap(t *testing.T) {
	t.Run("login item with all fields", func(t *testing.T) {
		item := &DecryptedItem{
			Name:  "test-item",
			Notes: "test notes",
			Login: &DecryptedLogin{
				Username: "testuser",
				Password: "testpass",
				TOTP:     "totp-secret",
				URIs:     []string{"https://example.com"},
			},
			Fields: []DecryptedField{
				{Name: "api_key", Value: "secret-key"},
			},
		}

		secrets := item.ToSecretMap()

		if secrets["username"] != "testuser" {
			t.Errorf("username = %q, want %q", secrets["username"], "testuser")
		}
		if secrets["password"] != "testpass" {
			t.Errorf("password = %q, want %q", secrets["password"], "testpass")
		}
		if secrets["totp"] != "totp-secret" {
			t.Errorf("totp = %q, want %q", secrets["totp"], "totp-secret")
		}
		if secrets["notes"] != "test notes" {
			t.Errorf("notes = %q, want %q", secrets["notes"], "test notes")
		}
		if secrets["api_key"] != "secret-key" {
			t.Errorf("api_key = %q, want %q", secrets["api_key"], "secret-key")
		}
	})

	t.Run("secure note with fields", func(t *testing.T) {
		item := &DecryptedItem{
			Name:  "secure-note",
			Notes: "secret content",
			Fields: []DecryptedField{
				{Name: "db_password", Value: "pg-secret"},
				{Name: "db_host", Value: "localhost"},
			},
		}

		secrets := item.ToSecretMap()

		if secrets["notes"] != "secret content" {
			t.Errorf("notes = %q, want %q", secrets["notes"], "secret content")
		}
		if secrets["db_password"] != "pg-secret" {
			t.Errorf("db_password = %q, want %q", secrets["db_password"], "pg-secret")
		}
		if secrets["db_host"] != "localhost" {
			t.Errorf("db_host = %q, want %q", secrets["db_host"], "localhost")
		}
	})

	t.Run("card item", func(t *testing.T) {
		item := &DecryptedItem{
			Name: "my-card",
			Card: &DecryptedCard{
				CardholderName: "John Doe",
				Number:         "4111111111111111",
				ExpMonth:       "12",
				ExpYear:        "2025",
				Code:           "123",
				Brand:          "Visa",
			},
		}

		secrets := item.ToSecretMap()

		if secrets["cardholderName"] != "John Doe" {
			t.Errorf("cardholderName = %q, want %q", secrets["cardholderName"], "John Doe")
		}
		if secrets["number"] != "4111111111111111" {
			t.Errorf("number = %q, want %q", secrets["number"], "4111111111111111")
		}
		if secrets["expMonth"] != "12" {
			t.Errorf("expMonth = %q, want %q", secrets["expMonth"], "12")
		}
		if secrets["expYear"] != "2025" {
			t.Errorf("expYear = %q, want %q", secrets["expYear"], "2025")
		}
		if secrets["code"] != "123" {
			t.Errorf("code = %q, want %q", secrets["code"], "123")
		}
	})

	t.Run("empty item", func(t *testing.T) {
		item := &DecryptedItem{Name: "empty"}
		secrets := item.ToSecretMap()

		if len(secrets) != 0 {
			t.Errorf("expected empty map, got %d entries", len(secrets))
		}
	})
}

func TestBitwardenProvider_Name(t *testing.T) {
	p := &BitwardenProvider{}
	if p.Name() != "bitwarden" {
		t.Errorf("Name() = %q, want %q", p.Name(), "bitwarden")
	}
}

func TestNewBitwardenProvider_Validation(t *testing.T) {
	t.Run("not enabled", func(t *testing.T) {
		os.Unsetenv("ENABLE_BITWARDEN")

		_, err := NewBitwardenProvider(context.Background())
		if err == nil {
			t.Error("expected error when not enabled")
		}
		if !strings.Contains(err.Error(), "ENABLE_BITWARDEN") {
			t.Errorf("error should mention ENABLE_BITWARDEN: %v", err)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		os.Setenv("ENABLE_BITWARDEN", "true")
		os.Unsetenv("BW_EMAIL")
		defer os.Unsetenv("ENABLE_BITWARDEN")

		_, err := NewBitwardenProvider(context.Background())
		if err == nil {
			t.Error("expected error when email is missing")
		}
		if !strings.Contains(err.Error(), "BW_EMAIL") {
			t.Errorf("error should mention BW_EMAIL: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		os.Setenv("ENABLE_BITWARDEN", "true")
		os.Setenv("BW_EMAIL", "test@example.com")
		os.Unsetenv("BW_PASSWORD")
		defer func() {
			os.Unsetenv("ENABLE_BITWARDEN")
			os.Unsetenv("BW_EMAIL")
		}()

		_, err := NewBitwardenProvider(context.Background())
		if err == nil {
			t.Error("expected error when password is missing")
		}
		if !strings.Contains(err.Error(), "BW_PASSWORD") {
			t.Errorf("error should mention BW_PASSWORD: %v", err)
		}
	})
}

func TestIsBWRateLimited(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "rate limited error", err: ErrBWRateLimited, want: true},
		{name: "429 in message", err: testErr("HTTP 429"), want: true},
		{name: "rate limit in message", err: testErr("rate limit exceeded"), want: true},
		{name: "too many requests", err: testErr("too many requests"), want: true},
		{name: "regular error", err: testErr("some other error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBWRateLimited(tt.err); got != tt.want {
				t.Errorf("isBWRateLimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBitwardenProvider_WrapError(t *testing.T) {
	p := &BitwardenProvider{}

	t.Run("folder not found", func(t *testing.T) {
		ref := &BitwardenSecretReference{Folder: "test-folder", ItemName: "test-item"}
		err := p.wrapError(testErr("folder not found"), ref)
		if !strings.Contains(err.Error(), "test-folder") {
			t.Errorf("error = %q, want containing folder name", err.Error())
		}
	})

	t.Run("item not found", func(t *testing.T) {
		ref := &BitwardenSecretReference{Folder: "test-folder", ItemName: "test-item"}
		err := p.wrapError(testErr("item not found"), ref)
		if !strings.Contains(err.Error(), "test-item") {
			t.Errorf("error = %q, want containing item name", err.Error())
		}
	})

	t.Run("field not found", func(t *testing.T) {
		ref := &BitwardenSecretReference{Folder: "f", ItemName: "i", Field: "test-field"}
		err := p.wrapError(testErr("field not found"), ref)
		if !strings.Contains(err.Error(), "test-field") {
			t.Errorf("error = %q, want containing field name", err.Error())
		}
	})

	t.Run("authentication error", func(t *testing.T) {
		ref := &BitwardenSecretReference{Folder: "f", ItemName: "i"}
		err := p.wrapError(testErr("authentication failed"), ref)
		if !strings.Contains(err.Error(), "authentication") {
			t.Errorf("error = %q, want containing 'authentication'", err.Error())
		}
	})

	t.Run("network error", func(t *testing.T) {
		ref := &BitwardenSecretReference{Folder: "f", ItemName: "i"}
		err := p.wrapError(testErr("network connection failed"), ref)
		if !strings.Contains(err.Error(), "connect") {
			t.Errorf("error = %q, want containing 'connect'", err.Error())
		}
	})
}

func TestVaultCache_IsValid(t *testing.T) {
	t.Run("valid cache", func(t *testing.T) {
		cache := &VaultCache{}
		// Manually set expiry (normally done by SyncVault)
		cache.mu.Lock()
		cache.expiry = cache.expiry.Add(5 * 60 * 1e9) // 5 minutes from zero time is in future
		cache.mu.Unlock()

		// Fresh cache should be invalid (zero time)
		fresh := &VaultCache{}
		if fresh.IsValid() {
			t.Error("fresh cache should be invalid")
		}
	})

	t.Run("cleared cache", func(t *testing.T) {
		cache := &VaultCache{
			folders: map[string]string{"id": "name"},
			items:   []*DecryptedItem{{Name: "test"}},
		}
		cache.Clear()

		if cache.IsValid() {
			t.Error("cleared cache should be invalid")
		}
		if cache.folders != nil {
			t.Error("folders should be nil after clear")
		}
		if cache.items != nil {
			t.Error("items should be nil after clear")
		}
	})
}

type bwTestError struct {
	msg string
}

func (e *bwTestError) Error() string {
	return e.msg
}

func testErr(msg string) error {
	return &bwTestError{msg: msg}
}
