package provider

import (
	"testing"
)

func TestParseSecretReference(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		want    *SecretReference
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid three-part reference",
			ref:  "op://Production/PostgreSQL/password",
			want: &SecretReference{
				Vault: "Production",
				Item:  "PostgreSQL",
				Field: "password",
			},
		},
		{
			name: "valid four-part reference with section",
			ref:  "op://Dev/API Keys/AWS/access_key",
			want: &SecretReference{
				Vault:   "Dev",
				Item:    "API Keys",
				Section: "AWS",
				Field:   "access_key",
			},
		},
		{
			name: "url encoded vault name",
			ref:  "op://My%20Vault/item/field",
			want: &SecretReference{
				Vault: "My Vault",
				Item:  "item",
				Field: "field",
			},
		},
		{
			name: "url encoded item name",
			ref:  "op://vault/My%20Item/field",
			want: &SecretReference{
				Vault: "vault",
				Item:  "My Item",
				Field: "field",
			},
		},
		{
			name: "url encoded field name",
			ref:  "op://vault/item/my%20field",
			want: &SecretReference{
				Vault: "vault",
				Item:  "item",
				Field: "my field",
			},
		},
		{
			name: "url encoded section",
			ref:  "op://vault/item/My%20Section/field",
			want: &SecretReference{
				Vault:   "vault",
				Item:    "item",
				Section: "My Section",
				Field:   "field",
			},
		},
		{
			name:    "missing op:// prefix",
			ref:     "Production/PostgreSQL/password",
			wantErr: true,
			errMsg:  "must start with op://",
		},
		{
			name:    "wrong prefix",
			ref:     "https://vault/item/field",
			wantErr: true,
			errMsg:  "must start with op://",
		},
		{
			name:    "missing vault, item, and field",
			ref:     "op://",
			wantErr: true,
			errMsg:  "empty path",
		},
		{
			name:    "missing item",
			ref:     "op://vault",
			wantErr: true,
			errMsg:  "missing item",
		},
		{
			name: "valid two-part reference (all fields)",
			ref:  "op://vault/item",
			want: &SecretReference{
				Vault: "vault",
				Item:  "item",
			},
		},
		{
			name:    "too many parts",
			ref:     "op://vault/item/section/field/extra",
			wantErr: true,
			errMsg:  "too many path components",
		},
		{
			name:    "empty string",
			ref:     "",
			wantErr: true,
			errMsg:  "must start with op://",
		},
		{
			name: "trailing slash handled",
			ref:  "op://vault/item/field/",
			want: &SecretReference{
				Vault: "vault",
				Item:  "item",
				Field: "field",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSecretReference(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSecretReference() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseSecretReference() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSecretReference() unexpected error = %v", err)
				return
			}
			if got.Vault != tt.want.Vault {
				t.Errorf("Vault = %q, want %q", got.Vault, tt.want.Vault)
			}
			if got.Item != tt.want.Item {
				t.Errorf("Item = %q, want %q", got.Item, tt.want.Item)
			}
			if got.Section != tt.want.Section {
				t.Errorf("Section = %q, want %q", got.Section, tt.want.Section)
			}
			if got.Field != tt.want.Field {
				t.Errorf("Field = %q, want %q", got.Field, tt.want.Field)
			}
		})
	}
}

func TestSecretReference_String(t *testing.T) {
	tests := []struct {
		name string
		ref  SecretReference
		want string
	}{
		{
			name: "without field (all fields)",
			ref: SecretReference{
				Vault: "Production",
				Item:  "PostgreSQL",
			},
			want: "op://Production/PostgreSQL",
		},
		{
			name: "without section",
			ref: SecretReference{
				Vault: "Production",
				Item:  "PostgreSQL",
				Field: "password",
			},
			want: "op://Production/PostgreSQL/password",
		},
		{
			name: "with section",
			ref: SecretReference{
				Vault:   "Dev",
				Item:    "API Keys",
				Section: "AWS",
				Field:   "access_key",
			},
			want: "op://Dev/API Keys/AWS/access_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("SecretReference.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSecretReference_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ref     SecretReference
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid reference without section",
			ref: SecretReference{
				Vault: "vault",
				Item:  "item",
				Field: "field",
			},
			wantErr: false,
		},
		{
			name: "valid reference with section",
			ref: SecretReference{
				Vault:   "vault",
				Item:    "item",
				Section: "section",
				Field:   "field",
			},
			wantErr: false,
		},
		{
			name: "missing vault",
			ref: SecretReference{
				Item:  "item",
				Field: "field",
			},
			wantErr: true,
			errMsg:  "vault name is required",
		},
		{
			name: "missing item",
			ref: SecretReference{
				Vault: "vault",
				Field: "field",
			},
			wantErr: true,
			errMsg:  "item name is required",
		},
		{
			name: "valid reference without field (all fields)",
			ref: SecretReference{
				Vault: "vault",
				Item:  "item",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestSecretReference_HasField(t *testing.T) {
	tests := []struct {
		name string
		ref  SecretReference
		want bool
	}{
		{
			name: "with field",
			ref: SecretReference{
				Vault: "vault",
				Item:  "item",
				Field: "field",
			},
			want: true,
		},
		{
			name: "without field",
			ref: SecretReference{
				Vault: "vault",
				Item:  "item",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.HasField(); got != tt.want {
				t.Errorf("HasField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
