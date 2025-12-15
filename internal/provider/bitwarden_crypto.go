package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

// DeriveMasterKey derives the master key from password using PBKDF2.
// Salt is the email address (lowercase).
func DeriveMasterKey(password, email string, iterations int) []byte {
	salt := []byte(strings.ToLower(email))
	return pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
}

// StretchMasterKey stretches the master key using HKDF-Expand.
// Returns encryption key (32 bytes) and MAC key (32 bytes).
func StretchMasterKey(masterKey []byte) (encKey, macKey []byte, err error) {
	// Encryption key
	hkdfEnc := hkdf.Expand(sha256.New, masterKey, []byte("enc"))
	encKey = make([]byte, 32)
	if _, err := io.ReadFull(hkdfEnc, encKey); err != nil {
		return nil, nil, fmt.Errorf("hkdf enc: %w", err)
	}

	// MAC key
	hkdfMac := hkdf.Expand(sha256.New, masterKey, []byte("mac"))
	macKey = make([]byte, 32)
	if _, err := io.ReadFull(hkdfMac, macKey); err != nil {
		return nil, nil, fmt.Errorf("hkdf mac: %w", err)
	}

	return encKey, macKey, nil
}

// HashPassword creates the password hash for server authentication.
// masterPasswordHash = PBKDF2(masterKey, password, 1, 32)
func HashPassword(masterKey []byte, password string) string {
	hash := pbkdf2.Key(masterKey, []byte(password), 1, 32, sha256.New)
	return base64.StdEncoding.EncodeToString(hash)
}

// CipherString represents a parsed encrypted string.
type CipherString struct {
	EncType    string
	IV         []byte
	Ciphertext []byte
	MAC        []byte
}

// ParseCipherString parses an encrypted string in format: encType.iv|ciphertext|mac
func ParseCipherString(s string) (*CipherString, error) {
	if s == "" {
		return nil, nil
	}

	// Parse format: encType.iv|ciphertext|mac
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cipher string format: missing type separator")
	}

	encType := parts[0]
	if encType != "2" {
		return nil, fmt.Errorf("unsupported encryption type: %s (expected 2 for AES-256-CBC)", encType)
	}

	dataParts := strings.Split(parts[1], "|")
	if len(dataParts) < 2 {
		return nil, fmt.Errorf("invalid cipher data format: expected at least iv|ciphertext")
	}

	iv, err := base64.StdEncoding.DecodeString(dataParts[0])
	if err != nil {
		return nil, fmt.Errorf("decode iv: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(dataParts[1])
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	var mac []byte
	if len(dataParts) >= 3 {
		mac, err = base64.StdEncoding.DecodeString(dataParts[2])
		if err != nil {
			return nil, fmt.Errorf("decode mac: %w", err)
		}
	}

	return &CipherString{
		EncType:    encType,
		IV:         iv,
		Ciphertext: ciphertext,
		MAC:        mac,
	}, nil
}

// DecryptCipherString decrypts an encrypted string using AES-256-CBC with HMAC verification.
func DecryptCipherString(cipherStr string, encKey, macKey []byte) ([]byte, error) {
	if cipherStr == "" {
		return nil, nil
	}

	cs, err := ParseCipherString(cipherStr)
	if err != nil {
		return nil, err
	}
	if cs == nil {
		return nil, nil
	}

	// Verify MAC if present
	if len(cs.MAC) > 0 && macKey != nil {
		h := hmac.New(sha256.New, macKey)
		h.Write(cs.IV)
		h.Write(cs.Ciphertext)
		expectedMAC := h.Sum(nil)

		if !hmac.Equal(cs.MAC, expectedMAC) {
			return nil, fmt.Errorf("MAC verification failed")
		}
	}

	// Decrypt AES-256-CBC
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	if len(cs.Ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext not multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, cs.IV)
	plaintext := make([]byte, len(cs.Ciphertext))
	mode.CryptBlocks(plaintext, cs.Ciphertext)

	// Remove PKCS#7 padding
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	// Validate padding bytes
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if int(plaintext[i]) != padding {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	plaintext = plaintext[:len(plaintext)-padding]

	return plaintext, nil
}

// DecryptString decrypts an encrypted string and returns it as a string.
func DecryptString(cipherStr string, encKey, macKey []byte) (string, error) {
	plaintext, err := DecryptCipherString(cipherStr, encKey, macKey)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
