package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSigner(t *testing.T) {
	t.Run("creates signer with defaults", func(t *testing.T) {
		signer := NewSigner()
		assert.NotNil(t, signer)
		assert.NotNil(t, signer.logger)
		assert.False(t, signer.keyless)
		assert.Empty(t, signer.keyPath)
	})

	t.Run("creates signer with options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		password := []byte("test-password")

		signer := NewSigner(
			WithKeyPath("/path/to/key"),
			WithPassword(password),
			WithKeyless(true),
			WithSignerInsecure(true),
			WithSignerLogger(logger),
		)

		assert.NotNil(t, signer)
		assert.Equal(t, "/path/to/key", signer.keyPath)
		assert.Equal(t, password, signer.password)
		assert.True(t, signer.keyless)
		assert.True(t, signer.insecure)
		assert.Equal(t, logger, signer.logger)
	})
}

func TestDigestToSignatureTag(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "sha256 digest",
			digest:   "sha256:abc123def456",
			expected: "sha256-abc123def456.sig",
		},
		{
			name:     "sha512 digest",
			digest:   "sha512:abc123def456",
			expected: "sha512-abc123def456.sig",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := digestToSignatureTag(tc.digest)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSignPayload(t *testing.T) {
	t.Run("signs payload successfully", func(t *testing.T) {
		// Generate a test key
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)

		signature, err := signPayload(privateKey, payload)
		require.NoError(t, err)
		assert.NotNil(t, signature)
		assert.Len(t, signature, 64) // P-256 signature is 64 bytes (32 + 32)
	})
}

func TestVerifySignature(t *testing.T) {
	t.Run("verifies valid signature", func(t *testing.T) {
		// Generate a test key
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)

		// Sign the payload
		signature, err := signPayload(privateKey, payload)
		require.NoError(t, err)

		// Verify with public key
		verified := verifySignature(&privateKey.PublicKey, payload, signature)
		assert.True(t, verified)
	})

	t.Run("rejects invalid signature", func(t *testing.T) {
		// Generate a test key
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)

		// Create an invalid signature
		invalidSignature := make([]byte, 64)
		rand.Read(invalidSignature)

		// Verify should fail
		verified := verifySignature(&privateKey.PublicKey, payload, invalidSignature)
		assert.False(t, verified)
	})

	t.Run("rejects tampered payload", func(t *testing.T) {
		// Generate a test key
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)
		tamperedPayload := []byte(`{"test": "tampered"}`)

		// Sign the original payload
		signature, err := signPayload(privateKey, payload)
		require.NoError(t, err)

		// Verify with tampered payload should fail
		verified := verifySignature(&privateKey.PublicKey, tamperedPayload, signature)
		assert.False(t, verified)
	})

	t.Run("rejects wrong public key", func(t *testing.T) {
		// Generate two different keys
		privateKey1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		privateKey2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)

		// Sign with key 1
		signature, err := signPayload(privateKey1, payload)
		require.NoError(t, err)

		// Verify with key 2's public key should fail
		verified := verifySignature(&privateKey2.PublicKey, payload, signature)
		assert.False(t, verified)
	})

	t.Run("rejects wrong size signature", func(t *testing.T) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		payload := []byte(`{"test": "data"}`)
		wrongSizeSignature := make([]byte, 32) // Should be 64

		verified := verifySignature(&privateKey.PublicKey, payload, wrongSizeSignature)
		assert.False(t, verified)
	})
}

func TestSimpleSigningPayload(t *testing.T) {
	t.Run("marshals correctly", func(t *testing.T) {
		payload := SimpleSigningPayload{
			Critical: Critical{
				Identity: Identity{
					DockerReference: "ghcr.io/myorg/module",
				},
				Image: Image{
					DockerManifestDigest: "sha256:abc123",
				},
				Type: "cosign container image signature",
			},
			Optional: map[string]interface{}{
				"timestamp": 1234567890,
			},
		}

		data, err := json.Marshal(payload)
		require.NoError(t, err)

		// Verify JSON structure
		var parsed map[string]interface{}
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Contains(t, parsed, "critical")
		assert.Contains(t, parsed, "optional")

		critical := parsed["critical"].(map[string]interface{})
		assert.Contains(t, critical, "identity")
		assert.Contains(t, critical, "image")
		assert.Contains(t, critical, "type")
	})
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	t.Run("full signing and verification cycle", func(t *testing.T) {
		// Generate key pair
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		// Create payload
		payload := SimpleSigningPayload{
			Critical: Critical{
				Identity: Identity{
					DockerReference: "ghcr.io/myorg/module",
				},
				Image: Image{
					DockerManifestDigest: "sha256:abc123def456",
				},
				Type: "cosign container image signature",
			},
			Optional: map[string]interface{}{
				"timestamp": 1234567890,
			},
		}

		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		// Sign
		signature, err := signPayload(privateKey, payloadBytes)
		require.NoError(t, err)

		// Verify
		verified := verifySignature(&privateKey.PublicKey, payloadBytes, signature)
		assert.True(t, verified)

		// Manually verify the signature algorithm works correctly
		hash := sha256.Sum256(payloadBytes)
		r := new(big.Int).SetBytes(signature[:32])
		s := new(big.Int).SetBytes(signature[32:])
		manualVerify := ecdsa.Verify(&privateKey.PublicKey, hash[:], r, s)
		assert.True(t, manualVerify)
	})
}

func TestVerifyResult(t *testing.T) {
	t.Run("verify result structure", func(t *testing.T) {
		result := VerifyResult{
			Verified: true,
			SignerID: "test@example.com",
			Issuer:   "https://token.actions.githubusercontent.com",
		}

		assert.True(t, result.Verified)
		assert.Equal(t, "test@example.com", result.SignerID)
		assert.Equal(t, "https://token.actions.githubusercontent.com", result.Issuer)
	})
}

func TestSignResult(t *testing.T) {
	t.Run("sign result structure", func(t *testing.T) {
		result := SignResult{
			Digest:       "sha256:abc123",
			SignatureRef: "ghcr.io/myorg/module:sha256-abc123.sig",
		}

		assert.Equal(t, "sha256:abc123", result.Digest)
		assert.Equal(t, "ghcr.io/myorg/module:sha256-abc123.sig", result.SignatureRef)
	})
}

