package provider

import (
	"fmt"
	"net/url"
	"strings"
)

// BitwardenSecretReference represents a parsed Bitwarden secret reference.
// Supported format: bw://<folder>/<item>[/<field>]
type BitwardenSecretReference struct {
	// Folder is the folder name
	Folder string
	// ItemName is the item name within the folder
	ItemName string
	// Field is the specific field to extract (optional)
	Field string
}

// String returns the secret reference as a string in bw:// format.
func (r *BitwardenSecretReference) String() string {
	if r.Field != "" {
		return fmt.Sprintf("bw://%s/%s/%s", r.Folder, r.ItemName, r.Field)
	}
	return fmt.Sprintf("bw://%s/%s", r.Folder, r.ItemName)
}

// HasField returns true if a specific field is requested.
func (r *BitwardenSecretReference) HasField() bool {
	return r.Field != ""
}

// Validate checks if the secret reference is valid.
func (r *BitwardenSecretReference) Validate() error {
	if r.Folder == "" {
		return fmt.Errorf("folder name is required")
	}
	if r.ItemName == "" {
		return fmt.Errorf("item name is required")
	}
	return nil
}

// ParseBitwardenReference parses a Bitwarden secret reference string.
// Supported format: bw://<folder>/<item>[/<field>]
func ParseBitwardenReference(ref string) (*BitwardenSecretReference, error) {
	if !strings.HasPrefix(ref, "bw://") {
		return nil, fmt.Errorf("invalid Bitwarden secret reference: must start with bw://")
	}

	// Remove the bw:// prefix
	path := strings.TrimPrefix(ref, "bw://")
	if path == "" {
		return nil, fmt.Errorf("invalid Bitwarden secret reference: empty path")
	}

	// Split by / and decode URL-encoded characters
	parts := strings.Split(path, "/")
	for i, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, fmt.Errorf("invalid Bitwarden secret reference: failed to decode %q: %w", part, err)
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

	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid Bitwarden secret reference: empty path after parsing")
	}

	switch len(parts) {
	case 2:
		// bw://<folder>/<item>
		return &BitwardenSecretReference{
			Folder:   parts[0],
			ItemName: parts[1],
		}, nil
	case 3:
		// bw://<folder>/<item>/<field>
		return &BitwardenSecretReference{
			Folder:   parts[0],
			ItemName: parts[1],
			Field:    parts[2],
		}, nil
	case 1:
		return nil, fmt.Errorf("invalid Bitwarden secret reference: missing item name (format: bw://folder/item)")
	default:
		return nil, fmt.Errorf("invalid Bitwarden secret reference: too many path components (format: bw://folder/item[/field])")
	}
}
