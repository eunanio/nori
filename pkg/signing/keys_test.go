package signing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
	t.Run("generates valid key pair", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify paths are correct
		assert.Equal(t, filepath.Join(tmpDir, PrivateKeyFileName), result.PrivateKeyPath)
		assert.Equal(t, filepath.Join(tmpDir, PublicKeyFileName), result.PublicKeyPath)

		// Verify files exist
		_, err = os.Stat(result.PrivateKeyPath)
		assert.NoError(t, err, "private key file should exist")

		_, err = os.Stat(result.PublicKeyPath)
		assert.NoError(t, err, "public key file should exist")

		// Note: File permissions work differently on Windows, so we skip the permission check
		// On Unix-like systems, we would check for 0600 permissions
	})

	t.Run("fails if keys already exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		// Create keys first time
		_, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		// Second attempt should fail
		_, err = GenerateKeyPair(tmpDir, password)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("creates output directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "nested", "keys")
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(nestedDir, password)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify directory was created
		_, err = os.Stat(nestedDir)
		assert.NoError(t, err, "nested directory should be created")
	})
}

func TestLoadPrivateKey(t *testing.T) {
	t.Run("loads private key with correct password", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		key, err := LoadPrivateKey(result.PrivateKeyPath, password)
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.NotNil(t, key.PublicKey)
	})

	t.Run("fails with wrong password", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("correct-password")
		wrongPassword := []byte("wrong-password")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		_, err = LoadPrivateKey(result.PrivateKeyPath, wrongPassword)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wrong password")
	})

	t.Run("fails for non-existent file", func(t *testing.T) {
		_, err := LoadPrivateKey("/non/existent/path", []byte("password"))
		assert.Error(t, err)
	})
}

func TestLoadPublicKey(t *testing.T) {
	t.Run("loads public key successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		key, err := LoadPublicKey(result.PublicKeyPath)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("fails for non-existent file", func(t *testing.T) {
		_, err := LoadPublicKey("/non/existent/path")
		assert.Error(t, err)
	})

	t.Run("fails for invalid PEM content", func(t *testing.T) {
		tmpDir := t.TempDir()
		invalidPath := filepath.Join(tmpDir, "invalid.pub")
		err := os.WriteFile(invalidPath, []byte("not a valid PEM"), 0644)
		require.NoError(t, err)

		_, err = LoadPublicKey(invalidPath)
		assert.Error(t, err)
	})
}

func TestKeyPairConsistency(t *testing.T) {
	t.Run("public key matches private key", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		privateKey, err := LoadPrivateKey(result.PrivateKeyPath, password)
		require.NoError(t, err)

		publicKey, err := LoadPublicKey(result.PublicKeyPath)
		require.NoError(t, err)

		// Verify the public keys match
		assert.Equal(t, privateKey.PublicKey.X, publicKey.X, "X coordinates should match")
		assert.Equal(t, privateKey.PublicKey.Y, publicKey.Y, "Y coordinates should match")
		assert.Equal(t, privateKey.PublicKey.Curve, publicKey.Curve, "curves should match")
	})
}

func TestPrivateKeyEncryption(t *testing.T) {
	t.Run("private key file is encrypted", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		// Read the private key file content
		content, err := os.ReadFile(result.PrivateKeyPath)
		require.NoError(t, err)

		// Verify it contains PEM header for encrypted key
		assert.Contains(t, string(content), "-----BEGIN ENCRYPTED COSIGN PRIVATE KEY-----")
		assert.Contains(t, string(content), "-----END ENCRYPTED COSIGN PRIVATE KEY-----")
	})

	t.Run("public key file is not encrypted", func(t *testing.T) {
		tmpDir := t.TempDir()
		password := []byte("test-password-123")

		result, err := GenerateKeyPair(tmpDir, password)
		require.NoError(t, err)

		// Read the public key file content
		content, err := os.ReadFile(result.PublicKeyPath)
		require.NoError(t, err)

		// Verify it contains standard public key PEM header
		assert.Contains(t, string(content), "-----BEGIN PUBLIC KEY-----")
		assert.Contains(t, string(content), "-----END PUBLIC KEY-----")
	})
}

