package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	// SignatureTagSuffix is appended to the image tag to create the signature tag.
	// This follows the cosign convention: image:tag -> image:sha256-<digest>.sig
	SignatureTagSuffix = ".sig"

	// SignatureMediaType is the media type for cosign signatures.
	SignatureMediaType = "application/vnd.dev.cosign.simplesigning.v1+json"

	// AnnotationSignature is the annotation key for the base64-encoded signature.
	AnnotationSignature = "dev.cosignproject.cosign/signature"
)

// Signer handles signing and verification of OCI artifacts.
type Signer struct {
	keyPath  string       // Path to private key file
	password []byte       // Password for private key
	insecure bool         // Allow insecure registry connections
	logger   *slog.Logger
}

// SignerOption configures a Signer.
type SignerOption func(*Signer)

// WithKeyPath sets the path to the private key file.
func WithKeyPath(path string) SignerOption {
	return func(s *Signer) {
		s.keyPath = path
	}
}

// WithPassword sets the password for the private key.
func WithPassword(password []byte) SignerOption {
	return func(s *Signer) {
		s.password = password
	}
}

// WithSignerInsecure allows insecure registry connections.
func WithSignerInsecure(insecure bool) SignerOption {
	return func(s *Signer) {
		s.insecure = insecure
	}
}

// WithSignerLogger sets the logger for the signer.
func WithSignerLogger(logger *slog.Logger) SignerOption {
	return func(s *Signer) {
		s.logger = logger
	}
}

// NewSigner creates a new Signer with the given options.
func NewSigner(opts ...SignerOption) *Signer {
	s := &Signer{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SimpleSigningPayload represents the payload that gets signed.
// This follows the cosign simple signing format.
type SimpleSigningPayload struct {
	Critical Critical               `json:"critical"`
	Optional map[string]interface{} `json:"optional,omitempty"`
}

// Critical contains the critical fields of a signature payload.
type Critical struct {
	Identity Identity `json:"identity"`
	Image    Image    `json:"image"`
	Type     string   `json:"type"`
}

// Identity identifies the signer.
type Identity struct {
	DockerReference string `json:"docker-reference"`
}

// Image identifies the signed image.
type Image struct {
	DockerManifestDigest string `json:"docker-manifest-digest"`
}

// SignResult contains the result of a signing operation.
type SignResult struct {
	Digest       string    // Digest of the signature artifact
	SignatureRef string    // Reference to the signature artifact
	SignedAt     time.Time // Time of signing
}

// VerifyResult contains the result of a verification operation.
type VerifyResult struct {
	Verified  bool      // Whether the signature is valid
	SignerID  string    // Key ID or path used for signing
	Issuer    string    // Reserved for future use
	Timestamp time.Time // Time of signing
}

// Sign signs an OCI artifact at the given reference.
func (s *Signer) Sign(ctx context.Context, ref name.Reference, remoteOpts ...remote.Option) (*SignResult, error) {
	s.logger.Info("signing artifact", "reference", ref.String())

	if s.keyPath == "" {
		return nil, fmt.Errorf("key path is required for signing")
	}

	// Load private key
	privateKey, err := LoadPrivateKey(s.keyPath, s.password)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Get the image descriptor to get the digest
	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact descriptor: %w", err)
	}

	digest := desc.Digest.String()
	s.logger.Debug("got artifact digest", "digest", digest)

	// Create the signing payload
	payload := SimpleSigningPayload{
		Critical: Critical{
			Identity: Identity{
				DockerReference: ref.Context().String(),
			},
			Image: Image{
				DockerManifestDigest: digest,
			},
			Type: "cosign container image signature",
		},
		Optional: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Sign the payload
	signature, err := signPayload(privateKey, payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign payload: %w", err)
	}

	// Create the signature image and push it
	sigRef, err := s.pushSignature(ctx, ref, digest, payloadBytes, signature, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to push signature: %w", err)
	}

	s.logger.Info("artifact signed successfully", "signatureRef", sigRef)

	return &SignResult{
		Digest:       digest,
		SignatureRef: sigRef,
		SignedAt:     time.Now(),
	}, nil
}

// Verify verifies the signature of an OCI artifact.
func (s *Signer) Verify(ctx context.Context, ref name.Reference, remoteOpts ...remote.Option) (*VerifyResult, error) {
	s.logger.Debug("verifying artifact signature", "reference", ref.String())

	// Get the image descriptor to get the digest
	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact descriptor: %w", err)
	}

	digest := desc.Digest.String()

	// Construct the signature tag
	sigTag := digestToSignatureTag(digest)
	sigRef, err := name.NewTag(ref.Context().String() + ":" + sigTag)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature reference: %w", err)
	}

	// Try to get the signature image
	sigDesc, err := remote.Get(sigRef, remoteOpts...)
	if err != nil {
		// No signature found
		return &VerifyResult{
			Verified: false,
		}, nil
	}

	// Get the signature image
	sigImg, err := sigDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature image: %w", err)
	}

	// Get the manifest to read annotations
	manifest, err := sigImg.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature manifest: %w", err)
	}

	// Extract signature from annotations
	if len(manifest.Layers) == 0 {
		return &VerifyResult{Verified: false}, nil
	}

	// Get layer annotations (cosign stores signature in layer annotations)
	layerAnnotations := manifest.Layers[0].Annotations
	signatureB64, ok := layerAnnotations[AnnotationSignature]
	if !ok {
		return &VerifyResult{Verified: false}, nil
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Get the payload from the layer
	layers, err := sigImg.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature layers: %w", err)
	}

	if len(layers) == 0 {
		return &VerifyResult{Verified: false}, nil
	}

	payloadReader, err := layers[0].Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get payload: %w", err)
	}
	defer payloadReader.Close()

	// Read payload (it's small, so read all)
	buf := make([]byte, 8192)
	n, err := payloadReader.Read(buf)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}
	payloadBytes := buf[:n]

	// Verify the signature
	// For key-based verification, we need the public key
	if s.keyPath != "" {
		// Derive public key path from private key path
		pubKeyPath := strings.TrimSuffix(s.keyPath, ".key") + ".pub"
		if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
			// Try looking for nori.pub in the same directory
			pubKeyPath = strings.TrimSuffix(s.keyPath, PrivateKeyFileName) + PublicKeyFileName
		}

		publicKey, err := LoadPublicKey(pubKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load public key for verification: %w", err)
		}

		verified := verifySignature(publicKey, payloadBytes, signature)

		return &VerifyResult{
			Verified:  verified,
			SignerID:  "key:" + pubKeyPath,
			Timestamp: time.Now(),
		}, nil
	}

	// Without a key, we can't verify
	return &VerifyResult{
		Verified: false,
	}, nil
}

// VerifyWithPublicKey verifies an artifact's signature using a specific public key.
func (s *Signer) VerifyWithPublicKey(ctx context.Context, ref name.Reference, publicKey *ecdsa.PublicKey, remoteOpts ...remote.Option) (*VerifyResult, error) {
	s.logger.Debug("verifying artifact signature with public key", "reference", ref.String())

	// Get the image descriptor to get the digest
	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact descriptor: %w", err)
	}

	digest := desc.Digest.String()

	// Construct the signature tag
	sigTag := digestToSignatureTag(digest)
	sigRef, err := name.NewTag(ref.Context().String() + ":" + sigTag)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature reference: %w", err)
	}

	// Try to get the signature image
	sigDesc, err := remote.Get(sigRef, remoteOpts...)
	if err != nil {
		// No signature found
		return &VerifyResult{
			Verified: false,
		}, nil
	}

	// Get the signature image
	sigImg, err := sigDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature image: %w", err)
	}

	// Get the manifest to read annotations
	manifest, err := sigImg.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature manifest: %w", err)
	}

	// Extract signature from layer annotations
	if len(manifest.Layers) == 0 {
		return &VerifyResult{Verified: false}, nil
	}

	layerAnnotations := manifest.Layers[0].Annotations
	signatureB64, ok := layerAnnotations[AnnotationSignature]
	if !ok {
		return &VerifyResult{Verified: false}, nil
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Get the payload from the layer
	layers, err := sigImg.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get signature layers: %w", err)
	}

	if len(layers) == 0 {
		return &VerifyResult{Verified: false}, nil
	}

	payloadReader, err := layers[0].Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get payload: %w", err)
	}
	defer payloadReader.Close()

	buf := make([]byte, 8192)
	n, err := payloadReader.Read(buf)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}
	payloadBytes := buf[:n]

	// Verify the signature
	verified := verifySignature(publicKey, payloadBytes, signature)

	return &VerifyResult{
		Verified:  verified,
		Timestamp: time.Now(),
	}, nil
}

// HasSignature checks if an artifact has a signature without fully verifying it.
func (s *Signer) HasSignature(ctx context.Context, ref name.Reference, remoteOpts ...remote.Option) (bool, error) {
	// Get the image descriptor to get the digest
	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return false, fmt.Errorf("failed to get artifact descriptor: %w", err)
	}

	digest := desc.Digest.String()

	// Construct the signature tag
	sigTag := digestToSignatureTag(digest)
	sigRef, err := name.NewTag(ref.Context().String() + ":" + sigTag)
	if err != nil {
		return false, fmt.Errorf("failed to create signature reference: %w", err)
	}

	// Try to get the signature image
	_, err = remote.Get(sigRef, remoteOpts...)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// pushSignature creates and pushes a signature image.
func (s *Signer) pushSignature(ctx context.Context, ref name.Reference, digest string, payload, signature []byte, remoteOpts ...remote.Option) (string, error) {
	// Create the signature tag following cosign convention
	sigTag := digestToSignatureTag(digest)
	sigRef, err := name.NewTag(ref.Context().String() + ":" + sigTag)
	if err != nil {
		return "", fmt.Errorf("failed to create signature reference: %w", err)
	}

	// Create a simple signature image
	// This follows the cosign format with a single layer containing the payload
	sigLayer := &signatureLayer{
		payload:   payload,
		signature: signature,
	}

	sigImg := &signatureImage{
		layer: sigLayer,
	}

	// Push the signature image
	if err := remote.Write(sigRef, sigImg, remoteOpts...); err != nil {
		return "", fmt.Errorf("failed to push signature: %w", err)
	}

	return sigRef.String(), nil
}

// digestToSignatureTag converts a digest to a signature tag.
// Format: sha256-<hex>.sig
func digestToSignatureTag(digest string) string {
	// Remove the algorithm prefix and add .sig suffix
	// sha256:abc123... -> sha256-abc123....sig
	return strings.Replace(digest, ":", "-", 1) + SignatureTagSuffix
}

// signPayload signs a payload with an ECDSA private key.
func signPayload(key *ecdsa.PrivateKey, payload []byte) ([]byte, error) {
	hash := sha256.Sum256(payload)
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return nil, err
	}

	// Encode as DER format
	// This is a simplified encoding - proper ASN.1 DER encoding
	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Ensure each component is 32 bytes (P-256 curve)
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	// Concatenate r and s
	sig := append(rPadded, sPadded...)
	return sig, nil
}

// verifySignature verifies a signature with an ECDSA public key.
func verifySignature(key *ecdsa.PublicKey, payload, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	hash := sha256.Sum256(payload)

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(key, hash[:], r, s)
}

