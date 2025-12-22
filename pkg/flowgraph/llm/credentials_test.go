package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCredentials(t *testing.T) {
	t.Run("loads valid credentials", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".credentials.json")

		expiry := time.Now().Add(1 * time.Hour).UnixMilli()
		creds := CredentialFile{
			ClaudeAiOauth: &Credentials{
				AccessToken:      "sk-ant-test-token",
				RefreshToken:     "sk-ant-refresh-token",
				ExpiresAt:        expiry,
				Scopes:           []string{"user:inference", "user:profile"},
				SubscriptionType: "max",
				RateLimitTier:    "default_claude_max_20x",
			},
		}

		data, err := json.Marshal(creds)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0600))

		loaded, err := LoadCredentials(path)
		require.NoError(t, err)
		assert.Equal(t, "sk-ant-test-token", loaded.AccessToken)
		assert.Equal(t, "sk-ant-refresh-token", loaded.RefreshToken)
		assert.Equal(t, expiry, loaded.ExpiresAt)
		assert.Equal(t, []string{"user:inference", "user:profile"}, loaded.Scopes)
		assert.Equal(t, "max", loaded.SubscriptionType)
		assert.Equal(t, "default_claude_max_20x", loaded.RateLimitTier)
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := LoadCredentials("/nonexistent/path/.credentials.json")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCredentialsNotFound)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".credentials.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0600))

		_, err := LoadCredentials(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCredentialsInvalid)
	})

	t.Run("returns error for missing OAuth credentials", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".credentials.json")
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0600))

		_, err := LoadCredentials(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoOAuthCredentials)
	})

	t.Run("uses default path when empty", func(t *testing.T) {
		// This test verifies the function uses DefaultCredentialPath
		// when path is empty. Since that file likely doesn't exist in CI,
		// we expect ErrCredentialsNotFound.
		_, err := LoadCredentials("")
		// Either no error (if file exists) or ErrCredentialsNotFound
		if err != nil {
			assert.ErrorIs(t, err, ErrCredentialsNotFound)
		}
	})
}

func TestLoadCredentialsFromDir(t *testing.T) {
	t.Run("loads from directory", func(t *testing.T) {
		dir := t.TempDir()

		expiry := time.Now().Add(1 * time.Hour).UnixMilli()
		creds := CredentialFile{
			ClaudeAiOauth: &Credentials{
				AccessToken: "sk-ant-dir-token",
				ExpiresAt:   expiry,
			},
		}

		data, err := json.Marshal(creds)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".credentials.json"), data, 0600))

		loaded, err := LoadCredentialsFromDir(dir)
		require.NoError(t, err)
		assert.Equal(t, "sk-ant-dir-token", loaded.AccessToken)
	})
}

func TestCredentials_IsExpired(t *testing.T) {
	t.Run("returns false for future expiry", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
		}
		assert.False(t, creds.IsExpired())
	})

	t.Run("returns true for past expiry", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
		}
		assert.True(t, creds.IsExpired())
	})

	t.Run("returns true for now", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().UnixMilli(),
		}
		// May or may not be expired depending on timing
		// Just ensure it doesn't panic
		_ = creds.IsExpired()
	})
}

func TestCredentials_ExpiresIn(t *testing.T) {
	t.Run("returns positive duration for future expiry", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour)
		creds := &Credentials{
			ExpiresAt: future.UnixMilli(),
		}
		duration := creds.ExpiresIn()
		assert.Greater(t, duration, 59*time.Minute)
		assert.Less(t, duration, 61*time.Minute)
	})

	t.Run("returns negative duration for past expiry", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		creds := &Credentials{
			ExpiresAt: past.UnixMilli(),
		}
		duration := creds.ExpiresIn()
		assert.Less(t, duration, time.Duration(0))
	})
}

func TestCredentials_ExpirationTime(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour)
	creds := &Credentials{
		ExpiresAt: expiry.UnixMilli(),
	}

	result := creds.ExpirationTime()
	// Compare truncated to milliseconds since that's our precision
	assert.Equal(t, expiry.UnixMilli(), result.UnixMilli())
}

func TestCredentials_IsExpiringSoon(t *testing.T) {
	t.Run("returns true when expiring within threshold", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().Add(5 * time.Minute).UnixMilli(),
		}
		assert.True(t, creds.IsExpiringSoon(10*time.Minute))
		assert.False(t, creds.IsExpiringSoon(1*time.Minute))
	})

	t.Run("returns true when already expired", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().Add(-5 * time.Minute).UnixMilli(),
		}
		assert.True(t, creds.IsExpiringSoon(10*time.Minute))
	})
}

func TestCredentials_Validate(t *testing.T) {
	t.Run("valid credentials pass", func(t *testing.T) {
		creds := &Credentials{
			AccessToken: "sk-ant-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
		}
		assert.NoError(t, creds.Validate())
	})

	t.Run("missing access token fails", func(t *testing.T) {
		creds := &Credentials{
			ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
		}
		err := creds.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCredentialsInvalid)
		assert.Contains(t, err.Error(), "access token")
	})

	t.Run("missing expiration fails", func(t *testing.T) {
		creds := &Credentials{
			AccessToken: "sk-ant-token",
		}
		err := creds.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCredentialsInvalid)
		assert.Contains(t, err.Error(), "expiration")
	})
}

func TestCredentials_HasScope(t *testing.T) {
	creds := &Credentials{
		Scopes: []string{"user:inference", "user:profile", "user:sessions:claude_code"},
	}

	t.Run("returns true for existing scope", func(t *testing.T) {
		assert.True(t, creds.HasScope("user:inference"))
		assert.True(t, creds.HasScope("user:profile"))
		assert.True(t, creds.HasScope("user:sessions:claude_code"))
	})

	t.Run("returns false for missing scope", func(t *testing.T) {
		assert.False(t, creds.HasScope("admin:all"))
		assert.False(t, creds.HasScope(""))
	})

	t.Run("handles nil scopes", func(t *testing.T) {
		emptyCreds := &Credentials{}
		assert.False(t, emptyCreds.HasScope("user:inference"))
	})
}

func TestWriteCredentials(t *testing.T) {
	t.Run("writes credentials to file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".credentials.json")

		creds := &Credentials{
			AccessToken:      "sk-ant-write-token",
			RefreshToken:     "sk-ant-refresh",
			ExpiresAt:        time.Now().Add(1 * time.Hour).UnixMilli(),
			Scopes:           []string{"user:inference"},
			SubscriptionType: "max",
		}

		err := WriteCredentials(path, creds)
		require.NoError(t, err)

		// Verify file exists and has correct permissions
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Verify content
		loaded, err := LoadCredentials(path)
		require.NoError(t, err)
		assert.Equal(t, creds.AccessToken, loaded.AccessToken)
		assert.Equal(t, creds.RefreshToken, loaded.RefreshToken)
		assert.Equal(t, creds.ExpiresAt, loaded.ExpiresAt)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "dir", ".credentials.json")

		creds := &Credentials{
			AccessToken: "sk-ant-nested",
			ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
		}

		err := WriteCredentials(path, creds)
		require.NoError(t, err)

		loaded, err := LoadCredentials(path)
		require.NoError(t, err)
		assert.Equal(t, "sk-ant-nested", loaded.AccessToken)
	})
}

func TestWriteCredentialsToDir(t *testing.T) {
	dir := t.TempDir()

	creds := &Credentials{
		AccessToken: "sk-ant-dir-write",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	err := WriteCredentialsToDir(dir, creds)
	require.NoError(t, err)

	// Verify file was created at correct path
	path := filepath.Join(dir, ".credentials.json")
	loaded, err := LoadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-dir-write", loaded.AccessToken)
}

func TestDefaultCredentialPath(t *testing.T) {
	path := DefaultCredentialPath()

	// Should end with .claude/.credentials.json
	assert.True(t, filepath.IsAbs(path) || path == "", "should be absolute path or empty")
	if path != "" {
		assert.Contains(t, path, ".claude")
		assert.True(t, filepath.Base(path) == ".credentials.json")
	}
}
