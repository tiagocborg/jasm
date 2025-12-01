package provider

import (
	"fmt"
	"net/url"
	"strings"
)

// SecretReference represents a parsed 1Password secret reference.
type SecretReference struct {
	Vault   string
	Item    string
	Section string
	Field   string
}

// String returns the secret reference as a string in op:// format.
func (r *SecretReference) String() string {
	if r.Section != "" {
		return fmt.Sprintf("op://%s/%s/%s/%s", r.Vault, r.Item, r.Section, r.Field)
	}
	if r.Field != "" {
		return fmt.Sprintf("op://%s/%s/%s", r.Vault, r.Item, r.Field)
	}
	return fmt.Sprintf("op://%s/%s", r.Vault, r.Item)
}

// HasField returns true if a specific field is requested.
func (r *SecretReference) HasField() bool {
	return r.Field != ""
}

// Validate checks if the secret reference is valid.
func (r *SecretReference) Validate() error {
	if r.Vault == "" {
		return fmt.Errorf("vault name is required")
	}
	if r.Item == "" {
		return fmt.Errorf("item name is required")
	}
	// Field is optional - when not specified, all fields are fetched
	return nil
}

// ParseSecretReference parses a 1Password secret reference string.
// Supported formats:
//   - op://vault/item (fetches all fields)
//   - op://vault/item/field
//   - op://vault/item/section/field
func ParseSecretReference(ref string) (*SecretReference, error) {
	if !strings.HasPrefix(ref, "op://") {
		return nil, fmt.Errorf("invalid 1Password secret reference: must start with op://")
	}

	// Remove the op:// prefix
	path := strings.TrimPrefix(ref, "op://")
	if path == "" {
		return nil, fmt.Errorf("invalid 1Password secret reference: empty path")
	}

	// Split by / and decode URL-encoded characters
	parts := strings.Split(path, "/")
	for i, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, fmt.Errorf("invalid 1Password secret reference: failed to decode %q: %w", part, err)
		}
		parts[i] = decoded
	}

	// Filter out empty parts (handles trailing slashes)
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	parts = nonEmpty

	switch len(parts) {
	case 2:
		// op://vault/item (fetch all fields)
		return &SecretReference{
			Vault: parts[0],
			Item:  parts[1],
		}, nil
	case 3:
		// op://vault/item/field
		return &SecretReference{
			Vault: parts[0],
			Item:  parts[1],
			Field: parts[2],
		}, nil
	case 4:
		// op://vault/item/section/field
		return &SecretReference{
			Vault:   parts[0],
			Item:    parts[1],
			Section: parts[2],
			Field:   parts[3],
		}, nil
	case 0:
		return nil, fmt.Errorf("invalid 1Password secret reference: missing vault and item")
	case 1:
		return nil, fmt.Errorf("invalid 1Password secret reference: missing item")
	default:
		return nil, fmt.Errorf("invalid 1Password secret reference: too many path components")
	}
}
