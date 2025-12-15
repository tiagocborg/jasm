package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultBitwardenURL = "https://vault.bitwarden.com"
	defaultCacheTTL     = 5 * time.Minute
	maxRetries          = 3
	initialBackoff      = 1 * time.Second
	maxBackoff          = 30 * time.Second
)

// VaultCache caches vault data to avoid repeated API calls.
type VaultCache struct {
	folders map[string]string // folder ID -> decrypted name
	items   []*DecryptedItem  // all decrypted items
	expiry  time.Time
	mu      sync.RWMutex
}

// IsValid returns true if the cache is still valid.
func (c *VaultCache) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Now().Before(c.expiry)
}

// Clear invalidates the cache.
func (c *VaultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.folders = nil
	c.items = nil
	c.expiry = time.Time{}
}

// BitwardenClient handles authentication and vault operations.
type BitwardenClient struct {
	serverURL  string
	email      string
	httpClient *http.Client

	// Derived keys
	masterKey    []byte
	stretchedEnc []byte
	stretchedMac []byte
	symKey       []byte // 32-byte encryption key for vault items
	symMac       []byte // 32-byte MAC key for vault items

	// Auth
	accessToken string
	tokenExpiry time.Time

	// Cache
	cache    *VaultCache
	cacheTTL time.Duration
}

// NewBitwardenClient creates a new Bitwarden client.
func NewBitwardenClient(serverURL, email string) *BitwardenClient {
	if serverURL == "" {
		serverURL = defaultBitwardenURL
	}
	return &BitwardenClient{
		serverURL:  strings.TrimSuffix(serverURL, "/"),
		email:      strings.ToLower(email),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cache:      &VaultCache{},
		cacheTTL:   defaultCacheTTL,
	}
}

// isRetryableError checks if an error or status code should be retried.
func isRetryableError(err error, statusCode int) bool {
	if err != nil {
		return true
	}
	return statusCode == 429 || statusCode >= 500
}

// backoffSleep waits with exponential backoff, respecting context cancellation.
func backoffSleep(ctx context.Context, attempt int) error {
	backoff := initialBackoff * time.Duration(1<<uint(attempt))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backoff):
		return nil
	}
}

// Prelogin retrieves KDF settings for the account.
func (c *BitwardenClient) Prelogin(ctx context.Context) (*PreloginResponse, error) {
	body, _ := json.Marshal(map[string]string{"email": c.email})

	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL+"/identity/accounts/prelogin", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create prelogin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prelogin request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prelogin failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var prelogin PreloginResponse
	if err := json.NewDecoder(resp.Body).Decode(&prelogin); err != nil {
		return nil, fmt.Errorf("decode prelogin: %w", err)
	}

	return &prelogin, nil
}

// DeriveKeys derives and stretches keys from the master password.
func (c *BitwardenClient) DeriveKeys(password string, kdfIterations int) error {
	// Step 1: Derive master key using PBKDF2
	c.masterKey = DeriveMasterKey(password, c.email, kdfIterations)

	// Step 2: Stretch master key using HKDF
	var err error
	c.stretchedEnc, c.stretchedMac, err = StretchMasterKey(c.masterKey)
	if err != nil {
		return err
	}

	return nil
}

// Login authenticates with the server and retrieves the access token.
func (c *BitwardenClient) Login(ctx context.Context, password string) (*TokenResponse, error) {
	passwordHash := HashPassword(c.masterKey, password)

	data := url.Values{
		"grant_type":       {"password"},
		"username":         {c.email},
		"password":         {passwordHash},
		"scope":            {"api offline_access"},
		"client_id":        {"cli"},
		"deviceType":       {"10"},
		"deviceIdentifier": {"aabbccdd-1234-5678-9012-aabbccddeeff"},
		"deviceName":       {"jasm-controller"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL+"/identity/connect/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("login failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var token TokenResponse
	if err := json.Unmarshal(respBody, &token); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	c.accessToken = token.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	return &token, nil
}

// DecryptSymmetricKey decrypts the symmetric key from the login response.
func (c *BitwardenClient) DecryptSymmetricKey(encryptedKey string) error {
	plaintext, err := DecryptCipherString(encryptedKey, c.stretchedEnc, c.stretchedMac)
	if err != nil {
		return fmt.Errorf("decrypt symmetric key: %w", err)
	}

	if len(plaintext) != 64 {
		return fmt.Errorf("unexpected symmetric key length: %d (expected 64)", len(plaintext))
	}

	// First 32 bytes: encryption key, last 32 bytes: MAC key
	c.symKey = plaintext[:32]
	c.symMac = plaintext[32:]

	return nil
}

// SyncVault fetches and decrypts all vault items.
func (c *BitwardenClient) SyncVault(ctx context.Context) error {
	var sync SyncResponse
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := backoffSleep(ctx, attempt-1); err != nil {
				return err
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/api/sync", nil)
		if err != nil {
			return fmt.Errorf("create sync request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.accessToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sync request: %w", err)
			continue
		}

		if isRetryableError(nil, resp.StatusCode) {
			resp.Body.Close()
			lastErr = fmt.Errorf("sync failed: %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("sync failed: %d - %s", resp.StatusCode, string(respBody))
		}

		err = json.NewDecoder(resp.Body).Decode(&sync)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("decode sync: %w", err)
		}

		lastErr = nil
		break
	}

	if lastErr != nil {
		return fmt.Errorf("sync failed after %d retries: %w", maxRetries, lastErr)
	}

	// Decrypt folders
	folders := make(map[string]string)
	for _, folder := range sync.Folders {
		name, err := DecryptString(folder.Name, c.symKey, c.symMac)
		if err != nil {
			return fmt.Errorf("decrypt folder name: %w", err)
		}
		folders[folder.ID] = name
	}

	// Decrypt items
	var items []*DecryptedItem
	for _, cipher := range sync.Ciphers {
		item, err := c.decryptCipher(cipher)
		if err != nil {
			return fmt.Errorf("decrypt cipher: %w", err)
		}
		items = append(items, item)
	}

	// Update cache
	c.cache.mu.Lock()
	c.cache.folders = folders
	c.cache.items = items
	c.cache.expiry = time.Now().Add(c.cacheTTL)
	c.cache.mu.Unlock()

	return nil
}

// Authenticate performs the full authentication flow.
func (c *BitwardenClient) Authenticate(ctx context.Context, password string) error {
	// Step 1: Prelogin to get KDF settings
	prelogin, err := c.Prelogin(ctx)
	if err != nil {
		return fmt.Errorf("prelogin: %w", err)
	}

	// Check KDF type
	if prelogin.Kdf != KdfTypePBKDF2 {
		return fmt.Errorf("unsupported KDF type: %d (only PBKDF2 is supported)", prelogin.Kdf)
	}

	// Step 2: Derive keys
	if err := c.DeriveKeys(password, prelogin.KdfIterations); err != nil {
		return fmt.Errorf("derive keys: %w", err)
	}

	// Step 3: Login
	token, err := c.Login(ctx, password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	// Step 4: Decrypt symmetric key
	if err := c.DecryptSymmetricKey(token.Key); err != nil {
		return fmt.Errorf("decrypt symmetric key: %w", err)
	}

	// Step 5: Sync vault
	if err := c.SyncVault(ctx); err != nil {
		return fmt.Errorf("sync vault: %w", err)
	}

	return nil
}

// FindItem locates an item by folder name and item name.
func (c *BitwardenClient) FindItem(folderName, itemName string) (*DecryptedItem, error) {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()

	// Find folder ID by name
	var folderID string
	for id, name := range c.cache.folders {
		if name == folderName {
			folderID = id
			break
		}
	}
	if folderID == "" {
		return nil, fmt.Errorf("folder %q not found", folderName)
	}

	// Find item by name within folder
	for _, item := range c.cache.items {
		if item.FolderID != nil && *item.FolderID == folderID && item.Name == itemName {
			return item, nil
		}
	}

	return nil, fmt.Errorf("item %q not found in folder %q", itemName, folderName)
}

// IsCacheValid returns true if the vault cache is still valid.
func (c *BitwardenClient) IsCacheValid() bool {
	return c.cache.IsValid()
}

// IsAuthenticated returns true if the client has valid credentials.
func (c *BitwardenClient) IsAuthenticated() bool {
	return c.accessToken != "" && time.Now().Before(c.tokenExpiry)
}

// decryptCipher decrypts a cipher into a DecryptedItem.
func (c *BitwardenClient) decryptCipher(cipher Cipher) (*DecryptedItem, error) {
	item := &DecryptedItem{
		ID:       cipher.ID,
		Type:     cipher.Type,
		FolderID: cipher.FolderID,
	}

	// Type names
	switch cipher.Type {
	case CipherTypeLogin:
		item.TypeName = "Login"
	case CipherTypeSecureNote:
		item.TypeName = "SecureNote"
	case CipherTypeCard:
		item.TypeName = "Card"
	case CipherTypeIdentity:
		item.TypeName = "Identity"
	default:
		item.TypeName = fmt.Sprintf("Unknown(%d)", cipher.Type)
	}

	// Decrypt name
	name, err := DecryptString(cipher.Name, c.symKey, c.symMac)
	if err != nil {
		return nil, fmt.Errorf("decrypt name: %w", err)
	}
	item.Name = name

	// Decrypt notes
	if cipher.Notes != nil {
		notes, err := DecryptString(*cipher.Notes, c.symKey, c.symMac)
		if err != nil {
			return nil, fmt.Errorf("decrypt notes: %w", err)
		}
		item.Notes = notes
	}

	// Decrypt login
	if cipher.Login != nil {
		item.Login = &DecryptedLogin{}
		if cipher.Login.Username != "" {
			item.Login.Username, _ = DecryptString(cipher.Login.Username, c.symKey, c.symMac)
		}
		if cipher.Login.Password != "" {
			item.Login.Password, _ = DecryptString(cipher.Login.Password, c.symKey, c.symMac)
		}
		if cipher.Login.TOTP != "" {
			item.Login.TOTP, _ = DecryptString(cipher.Login.TOTP, c.symKey, c.symMac)
		}
		for _, u := range cipher.Login.URIs {
			if u.URI != "" {
				uri, _ := DecryptString(u.URI, c.symKey, c.symMac)
				item.Login.URIs = append(item.Login.URIs, uri)
			}
		}
	}

	// Decrypt card
	if cipher.Card != nil {
		item.Card = &DecryptedCard{}
		item.Card.CardholderName, _ = DecryptString(cipher.Card.CardholderName, c.symKey, c.symMac)
		item.Card.Brand, _ = DecryptString(cipher.Card.Brand, c.symKey, c.symMac)
		item.Card.Number, _ = DecryptString(cipher.Card.Number, c.symKey, c.symMac)
		item.Card.ExpMonth, _ = DecryptString(cipher.Card.ExpMonth, c.symKey, c.symMac)
		item.Card.ExpYear, _ = DecryptString(cipher.Card.ExpYear, c.symKey, c.symMac)
		item.Card.Code, _ = DecryptString(cipher.Card.Code, c.symKey, c.symMac)
	}

	// Decrypt custom fields
	for _, f := range cipher.Fields {
		df := DecryptedField{Type: f.Type}
		df.Name, _ = DecryptString(f.Name, c.symKey, c.symMac)
		df.Value, _ = DecryptString(f.Value, c.symKey, c.symMac)
		item.Fields = append(item.Fields, df)
	}

	return item, nil
}
