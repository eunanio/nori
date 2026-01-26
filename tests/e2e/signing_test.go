//go:build e2e

package e2e

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/packaging"
	"github.com/eunanio/nori/pkg/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Signing E2E Tests
// =============================================================================

func TestE2E_SigningWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()
	t.Logf("Registry available at: %s", registry.URL)

	testdataPath := getTestdataPath(t)
	moduleDir := filepath.Join(testdataPath, "random-resources")

	// Create a zip of the module
	zipPath := createModuleZip(t, moduleDir)
	defer os.Remove(zipPath)
	t.Logf("Created module zip: %s", zipPath)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create OCI client with insecure option for local registry
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)

	packager := packaging.NewPackager(client, logger)

	// Module reference in the test registry
	moduleRef := registryReference(registry.URL, "e2e/signed-module", "v1.0.0")

	// Generate test signing keys
	keyDir := t.TempDir()
	password := []byte("test-password-123")

	var keyResult *signing.KeyPairResult

	t.Run("generate_key_pair", func(t *testing.T) {
		var err error
		keyResult, err = signing.GenerateKeyPair(keyDir, password)
		require.NoError(t, err, "failed to generate key pair")

		assert.FileExists(t, keyResult.PrivateKeyPath)
		assert.FileExists(t, keyResult.PublicKeyPath)
		t.Logf("Generated keys in: %s", keyDir)
	})

	t.Run("package_module", func(t *testing.T) {
		t.Logf("Packaging module to: %s", moduleRef)

		result, err := packager.Package(ctx, moduleRef, zipPath, packaging.PackageOptions{
			Description: "E2E signing test module",
		})
		require.NoError(t, err, "package and push failed")
		assert.NotEmpty(t, result.Digest)
		t.Logf("Module pushed with digest: %s", result.Digest)
	})

	t.Run("sign_artifact", func(t *testing.T) {
		ref, err := oci.ParseReference(moduleRef)
		require.NoError(t, err)

		remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
		require.NoError(t, err)

		signer := signing.NewSigner(
			signing.WithKeyPath(keyResult.PrivateKeyPath),
			signing.WithPassword(password),
			signing.WithSignerLogger(logger),
			signing.WithSignerInsecure(true),
		)

		signResult, err := signer.Sign(ctx, ref, remoteOpts...)
		require.NoError(t, err, "signing failed")

		assert.NotEmpty(t, signResult.Digest)
		assert.NotEmpty(t, signResult.SignatureRef)
		t.Logf("Artifact signed: %s", signResult.SignatureRef)
	})

	t.Run("verify_signature_exists", func(t *testing.T) {
		ref, err := oci.ParseReference(moduleRef)
		require.NoError(t, err)

		remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
		require.NoError(t, err)

		signer := signing.NewSigner(
			signing.WithSignerLogger(logger),
			signing.WithSignerInsecure(true),
		)

		hasSignature, err := signer.HasSignature(ctx, ref, remoteOpts...)
		require.NoError(t, err)
		assert.True(t, hasSignature, "signature should exist")
	})

	t.Run("verify_signature_with_public_key", func(t *testing.T) {
		ref, err := oci.ParseReference(moduleRef)
		require.NoError(t, err)

		remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
		require.NoError(t, err)

		// Load public key
		publicKey, err := signing.LoadPublicKey(keyResult.PublicKeyPath)
		require.NoError(t, err)

		signer := signing.NewSigner(
			signing.WithSignerLogger(logger),
			signing.WithSignerInsecure(true),
		)

		verifyResult, err := signer.VerifyWithPublicKey(ctx, ref, publicKey, remoteOpts...)
		require.NoError(t, err)

		assert.True(t, verifyResult.Verified, "signature should be verified")
		t.Logf("Signature verified successfully")
	})

	t.Run("verify_fails_with_wrong_key", func(t *testing.T) {
		// Generate a different key pair
		wrongKeyDir := t.TempDir()
		wrongKeyResult, err := signing.GenerateKeyPair(wrongKeyDir, password)
		require.NoError(t, err)

		ref, err := oci.ParseReference(moduleRef)
		require.NoError(t, err)

		remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
		require.NoError(t, err)

		// Load wrong public key
		wrongPublicKey, err := signing.LoadPublicKey(wrongKeyResult.PublicKeyPath)
		require.NoError(t, err)

		signer := signing.NewSigner(
			signing.WithSignerLogger(logger),
			signing.WithSignerInsecure(true),
		)

		verifyResult, err := signer.VerifyWithPublicKey(ctx, ref, wrongPublicKey, remoteOpts...)
		require.NoError(t, err)

		assert.False(t, verifyResult.Verified, "signature should NOT be verified with wrong key")
		t.Logf("Correctly rejected wrong key")
	})
}

func TestE2E_UnsignedArtifact(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()
	t.Logf("Registry available at: %s", registry.URL)

	testdataPath := getTestdataPath(t)
	moduleDir := filepath.Join(testdataPath, "random-resources")

	// Create a zip of the module
	zipPath := createModuleZip(t, moduleDir)
	defer os.Remove(zipPath)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create OCI client with insecure option for local registry
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)

	packager := packaging.NewPackager(client, logger)

	// Module reference in the test registry (NOT signed)
	moduleRef := registryReference(registry.URL, "e2e/unsigned-module", "v1.0.0")

	t.Run("package_unsigned_module", func(t *testing.T) {
		result, err := packager.Package(ctx, moduleRef, zipPath, packaging.PackageOptions{
			Description: "E2E unsigned test module",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.Digest)
	})

	t.Run("verify_no_signature", func(t *testing.T) {
		ref, err := oci.ParseReference(moduleRef)
		require.NoError(t, err)

		remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
		require.NoError(t, err)

		signer := signing.NewSigner(
			signing.WithSignerLogger(logger),
			signing.WithSignerInsecure(true),
		)

		hasSignature, err := signer.HasSignature(ctx, ref, remoteOpts...)
		require.NoError(t, err)
		assert.False(t, hasSignature, "unsigned artifact should not have signature")
	})
}

func TestE2E_KeyGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	t.Run("generate_and_use_keys", func(t *testing.T) {
		keyDir := t.TempDir()
		password := []byte("secure-password-456")

		// Generate keys
		keyResult, err := signing.GenerateKeyPair(keyDir, password)
		require.NoError(t, err)

		// Verify files exist with correct names
		assert.Equal(t, filepath.Join(keyDir, "nori.key"), keyResult.PrivateKeyPath)
		assert.Equal(t, filepath.Join(keyDir, "nori.pub"), keyResult.PublicKeyPath)

		// Load and verify keys work
		privateKey, err := signing.LoadPrivateKey(keyResult.PrivateKeyPath, password)
		require.NoError(t, err)
		assert.NotNil(t, privateKey)

		publicKey, err := signing.LoadPublicKey(keyResult.PublicKeyPath)
		require.NoError(t, err)
		assert.NotNil(t, publicKey)

		// Verify public keys match
		assert.Equal(t, privateKey.PublicKey.X, publicKey.X)
		assert.Equal(t, privateKey.PublicKey.Y, publicKey.Y)
	})

	t.Run("key_files_exist", func(t *testing.T) {
		keyDir := t.TempDir()
		password := []byte("test-password")

		keyResult, err := signing.GenerateKeyPair(keyDir, password)
		require.NoError(t, err)

		// Verify both key files exist
		_, err = os.Stat(keyResult.PrivateKeyPath)
		require.NoError(t, err, "private key should exist")

		_, err = os.Stat(keyResult.PublicKeyPath)
		require.NoError(t, err, "public key should exist")

		// Note: File permissions work differently on Windows vs Unix,
		// so we just verify the files exist and are readable
	})
}

