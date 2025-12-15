package provider

// PreloginResponse represents the response from /identity/accounts/prelogin.
type PreloginResponse struct {
	Kdf            int `json:"Kdf"`
	KdfIterations  int `json:"KdfIterations"`
	KdfMemory      int `json:"KdfMemory"`
	KdfParallelism int `json:"KdfParallelism"`
}

// TokenResponse represents the response from /identity/connect/token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Key          string `json:"Key"`
	PrivateKey   string `json:"PrivateKey"`
}

// SyncResponse represents the response from /api/sync.
type SyncResponse struct {
	Profile     Profile      `json:"profile"`
	Ciphers     []Cipher     `json:"ciphers"`
	Folders     []SyncFolder `json:"folders"`
	Collections []Collection `json:"collections"`
}

// Profile represents user profile data from sync.
type Profile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Key   string `json:"key"`
}

// SyncFolder represents a folder from the sync response.
type SyncFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Collection represents a collection from the sync response.
type Collection struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
}

// Cipher represents an encrypted vault item from the API.
type Cipher struct {
	ID             string        `json:"id"`
	OrganizationID *string       `json:"organizationId"`
	FolderID       *string       `json:"folderId"`
	Type           int           `json:"type"`
	Name           string        `json:"name"`
	Notes          *string       `json:"notes"`
	Login          *CipherLogin  `json:"login"`
	Card           *CipherCard   `json:"card"`
	Identity       *CipherID     `json:"identity"`
	SecureNote     *SecureNote   `json:"secureNote"`
	Fields         []CipherField `json:"fields"`
	CollectionIDs  []string      `json:"collectionIds"`
	RevisionDate   string        `json:"revisionDate"`
}

// CipherLogin contains encrypted login data.
type CipherLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     string `json:"totp"`
	URIs     []struct {
		URI string `json:"uri"`
	} `json:"uris"`
}

// CipherCard contains encrypted card data.
type CipherCard struct {
	CardholderName string `json:"cardholderName"`
	Brand          string `json:"brand"`
	Number         string `json:"number"`
	ExpMonth       string `json:"expMonth"`
	ExpYear        string `json:"expYear"`
	Code           string `json:"code"`
}

// CipherID contains encrypted identity data.
type CipherID struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// SecureNote contains secure note metadata.
type SecureNote struct {
	Type int `json:"type"`
}

// CipherField represents an encrypted custom field.
type CipherField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  int    `json:"type"`
}

// DecryptedItem represents a decrypted vault item.
type DecryptedItem struct {
	ID       string
	Name     string
	Type     int
	TypeName string
	Notes    string
	Login    *DecryptedLogin
	Card     *DecryptedCard
	Fields   []DecryptedField
	FolderID *string
}

// DecryptedLogin contains decrypted login credentials.
type DecryptedLogin struct {
	Username string
	Password string
	TOTP     string
	URIs     []string
}

// DecryptedCard contains decrypted card information.
type DecryptedCard struct {
	CardholderName string
	Brand          string
	Number         string
	ExpMonth       string
	ExpYear        string
	Code           string
}

// DecryptedField represents a decrypted custom field.
type DecryptedField struct {
	Name  string
	Value string
	Type  int
}

// ToSecretMap converts a DecryptedItem to a map suitable for Kubernetes secrets.
func (item *DecryptedItem) ToSecretMap() map[string]string {
	result := make(map[string]string)

	if item.Login != nil {
		if item.Login.Username != "" {
			result["username"] = item.Login.Username
		}
		if item.Login.Password != "" {
			result["password"] = item.Login.Password
		}
		if item.Login.TOTP != "" {
			result["totp"] = item.Login.TOTP
		}
	}

	if item.Card != nil {
		if item.Card.CardholderName != "" {
			result["cardholderName"] = item.Card.CardholderName
		}
		if item.Card.Brand != "" {
			result["brand"] = item.Card.Brand
		}
		if item.Card.Number != "" {
			result["number"] = item.Card.Number
		}
		if item.Card.ExpMonth != "" {
			result["expMonth"] = item.Card.ExpMonth
		}
		if item.Card.ExpYear != "" {
			result["expYear"] = item.Card.ExpYear
		}
		if item.Card.Code != "" {
			result["code"] = item.Card.Code
		}
	}

	for _, field := range item.Fields {
		if field.Name != "" && field.Value != "" {
			result[field.Name] = field.Value
		}
	}

	if item.Notes != "" {
		result["notes"] = item.Notes
	}

	return result
}

// Cipher type constants.
const (
	CipherTypeLogin      = 1
	CipherTypeSecureNote = 2
	CipherTypeCard       = 3
	CipherTypeIdentity   = 4
)

// KDF type constants.
const (
	KdfTypePBKDF2 = 0
	KdfTypeArgon2 = 1
)

// Encryption type constants.
const (
	EncTypeAES256CBC = 2
)
