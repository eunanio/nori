// Package signing provides cryptographic signing and verification for OCI artifacts.
package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

const (
	// PrivateKeyFileName is the default filename for the private key.
	PrivateKeyFileName = "nori.key"
	// PublicKeyFileName is the default filename for the public key.
	PublicKeyFileName = "nori.pub"

	// PEM block types
	pemTypeEncryptedCosignPrivateKey = "ENCRYPTED COSIGN PRIVATE KEY"
	pemTypePublicKey                 = "PUBLIC KEY"

	// scrypt parameters for key derivation
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
)

// KeyPairResult contains the paths to the generated key files.
type KeyPairResult struct {
	PrivateKeyPath string
	PublicKeyPath  string
}

// GenerateKeyPair generates a new ECDSA P-256 key pair compatible with cosign.
// The private key is encrypted with the provided password.
// Keys are written to outputDir as nori.key and nori.pub.
func GenerateKeyPair(outputDir string, password []byte) (*KeyPairResult, error) {
	// Generate ECDSA P-256 key pair (same as cosign)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate paths
	privateKeyPath := filepath.Join(outputDir, PrivateKeyFileName)
	publicKeyPath := filepath.Join(outputDir, PublicKeyFileName)

	// Check if files already exist
	if _, err := os.Stat(privateKeyPath); err == nil {
		return nil, fmt.Errorf("private key already exists: %s", privateKeyPath)
	}
	if _, err := os.Stat(publicKeyPath); err == nil {
		return nil, fmt.Errorf("public key already exists: %s", publicKeyPath)
	}

	// Encrypt and write private key
	encryptedPEM, err := encryptPrivateKey(privateKey, password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	if err := os.WriteFile(privateKeyPath, encryptedPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	publicPEM, err := encodePublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	if err := os.WriteFile(publicKeyPath, publicPEM, 0644); err != nil {
		// Clean up private key on failure
		os.Remove(privateKeyPath)
		return nil, fmt.Errorf("failed to write public key: %w", err)
	}

	return &KeyPairResult{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
	}, nil
}

// LoadPrivateKey loads and decrypts a private key from the given path.
func LoadPrivateKey(path string, password []byte) (*ecdsa.PrivateKey, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	return decryptPrivateKey(pemData, password)
}

// LoadPublicKey loads a public key from the given path.
func LoadPublicKey(path string) (*ecdsa.PublicKey, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	return ecdsaPub, nil
}

// encryptPrivateKey encrypts an ECDSA private key using the cosign format.
// This uses scrypt for key derivation and NaCl secretbox for encryption.
func encryptPrivateKey(key *ecdsa.PrivateKey, password []byte) ([]byte, error) {
	// Marshal the private key to PKCS8 format
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Generate a random salt for scrypt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key using scrypt
	derivedKey, err := scrypt.Key(password, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// Generate a random nonce for secretbox
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt using NaCl secretbox
	var secretKey [32]byte
	copy(secretKey[:], derivedKey)
	encrypted := secretbox.Seal(nil, pkcs8, &nonce, &secretKey)

	// Build the encrypted payload: salt + nonce + ciphertext
	// This matches the cosign format
	payload := make([]byte, 0, len(salt)+len(nonce)+len(encrypted))
	payload = append(payload, salt...)
	payload = append(payload, nonce[:]...)
	payload = append(payload, encrypted...)

	// Encode as PEM
	block := &pem.Block{
		Type:  pemTypeEncryptedCosignPrivateKey,
		Bytes: payload,
	}

	return pem.EncodeToMemory(block), nil
}

// decryptPrivateKey decrypts a cosign-encrypted private key.
func decryptPrivateKey(pemData, password []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != pemTypeEncryptedCosignPrivateKey {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	payload := block.Bytes
	if len(payload) < 32+24 {
		return nil, fmt.Errorf("encrypted payload too short")
	}

	// Extract salt, nonce, and ciphertext
	salt := payload[:32]
	var nonce [24]byte
	copy(nonce[:], payload[32:56])
	ciphertext := payload[56:]

	// Derive encryption key using scrypt
	derivedKey, err := scrypt.Key(password, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// Decrypt using NaCl secretbox
	var secretKey [32]byte
	copy(secretKey[:], derivedKey)
	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &secretKey)
	if !ok {
		return nil, fmt.Errorf("failed to decrypt private key (wrong password?)")
	}

	// Parse the PKCS8 private key
	key, err := x509.ParsePKCS8PrivateKey(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA private key")
	}

	return ecdsaKey, nil
}

// encodePublicKey encodes an ECDSA public key to PEM format.
func encodePublicKey(key *ecdsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	block := &pem.Block{
		Type:  pemTypePublicKey,
		Bytes: der,
	}

	return pem.EncodeToMemory(block), nil
}

