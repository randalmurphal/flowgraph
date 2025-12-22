package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Credential errors.
var (
	// ErrCredentialsNotFound indicates no credentials file was found.
	ErrCredentialsNotFound = errors.New("credentials file not found")

	// ErrCredentialsInvalid indicates the credentials file is malformed.
	ErrCredentialsInvalid = errors.New("invalid credentials format")

	// ErrCredentialsExpired indicates the access token has expired.
	ErrCredentialsExpired = errors.New("credentials expired")

	// ErrNoOAuthCredentials indicates OAuth credentials are not present.
	ErrNoOAuthCredentials = errors.New("no OAuth credentials in file")
)

// Credentials represents Claude OAuth authentication tokens.
type Credentials struct {
	// AccessToken is the OAuth access token for API authentication.
	AccessToken string `json:"accessToken"`

	// RefreshToken is used to obtain new access tokens.
	RefreshToken string `json:"refreshToken"`

	// ExpiresAt is the Unix timestamp (milliseconds) when the access token expires.
	ExpiresAt int64 `json:"expiresAt"`

	// Scopes are the OAuth scopes granted to this token.
	Scopes []string `json:"scopes"`

	// SubscriptionType indicates the Claude subscription tier (e.g., "max").
	SubscriptionType string `json:"subscriptionType"`

	// RateLimitTier indicates the rate limit tier for this subscription.
	RateLimitTier string `json:"rateLimitTier"`
}

// CredentialFile represents the ~/.claude/.credentials.json file structure.
type CredentialFile struct {
	ClaudeAiOauth *Credentials `json:"claudeAiOauth"`
}

// DefaultCredentialPath returns the default path to Claude credentials.
// This is ~/.claude/.credentials.json on Unix-like systems.
func DefaultCredentialPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", ".credentials.json")
}

// LoadCredentials loads Claude OAuth credentials from the specified path.
// If path is empty, uses DefaultCredentialPath().
func LoadCredentials(path string) (*Credentials, error) {
	if path == "" {
		path = DefaultCredentialPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrCredentialsNotFound, path)
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var file CredentialFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCredentialsInvalid, err)
	}

	if file.ClaudeAiOauth == nil {
		return nil, ErrNoOAuthCredentials
	}

	return file.ClaudeAiOauth, nil
}

// LoadCredentialsFromDir loads credentials from a .credentials.json file
// in the specified directory. This is useful for container environments
// where the Claude config directory is mounted to a custom location.
func LoadCredentialsFromDir(dir string) (*Credentials, error) {
	path := filepath.Join(dir, ".credentials.json")
	return LoadCredentials(path)
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	return time.Now().UnixMilli() >= c.ExpiresAt
}

// ExpiresIn returns the duration until the access token expires.
// Returns a negative duration if already expired.
func (c *Credentials) ExpiresIn() time.Duration {
	expiresAt := time.UnixMilli(c.ExpiresAt)
	return time.Until(expiresAt)
}

// ExpiresAt returns the expiration time as a time.Time value.
func (c *Credentials) ExpirationTime() time.Time {
	return time.UnixMilli(c.ExpiresAt)
}

// IsExpiringSoon returns true if the token expires within the given duration.
// This is useful for proactive refresh or warning users.
func (c *Credentials) IsExpiringSoon(within time.Duration) bool {
	return c.ExpiresIn() <= within
}

// Validate checks that the credentials have required fields.
func (c *Credentials) Validate() error {
	if c.AccessToken == "" {
		return fmt.Errorf("%w: missing access token", ErrCredentialsInvalid)
	}
	if c.ExpiresAt == 0 {
		return fmt.Errorf("%w: missing expiration time", ErrCredentialsInvalid)
	}
	return nil
}

// HasScope returns true if the credentials include the specified scope.
func (c *Credentials) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// WriteCredentials writes credentials to the specified path.
// This is useful for containers that need to receive credentials.
func WriteCredentials(path string, creds *Credentials) error {
	file := CredentialFile{
		ClaudeAiOauth: creds,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// WriteCredentialsToDir writes credentials to .credentials.json in the specified directory.
func WriteCredentialsToDir(dir string, creds *Credentials) error {
	path := filepath.Join(dir, ".credentials.json")
	return WriteCredentials(path, creds)
}
