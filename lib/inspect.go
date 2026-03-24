package nori

import (
	"context"
	"fmt"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/signing"
)

// Inspect retrieves metadata, layers, and annotations for an OCI artifact.
// If opts.IncludeReadme is true, the README layer content is also fetched.
func (c *Client) Inspect(ctx context.Context, reference string, opts InspectOptions) (*InspectResult, error) {
	ref, err := c.ociClient.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	artifact, err := c.ociClient.InspectArtifact(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect artifact: %w", err)
	}

	result := &InspectResult{
		Reference:   reference,
		Digest:      artifact.Digest,
		Layers:      artifact.Layers,
		Annotations: artifact.Annotations,
		Config:      artifact.Config,
	}

	// Optionally fetch README
	if opts.IncludeReadme {
		if artifact.Annotations[oci.AnnotationReadme] == "true" {
			readme, err := c.ociClient.PullReadme(ctx, ref)
			if err != nil {
				c.logger.Warn("failed to pull README", "error", err)
			} else {
				result.Readme = readme
			}
		}
	}

	// Check for signature (best-effort)
	sigInfo := c.checkSignature(ctx, ref, "")
	result.Signature = sigInfo

	return result, nil
}

// Sign signs an OCI artifact with a private key.
func (c *Client) Sign(ctx context.Context, reference string, opts SignOptions) (*SignResult, error) {
	keyPath := opts.KeyPath
	if keyPath == "" {
		if c.cfg.Signing.KeyPath != "" {
			keyPath = c.cfg.Signing.KeyPath
		} else {
			return nil, fmt.Errorf("key path required for signing")
		}
	}

	sigRef, err := c.signArtifact(ctx, reference, keyPath, opts.Password)
	if err != nil {
		return nil, err
	}

	return &SignResult{SignatureRef: sigRef}, nil
}

// Verify checks whether an OCI artifact has a valid signature.
func (c *Client) Verify(ctx context.Context, reference string, opts VerifyOptions) (*VerifyResult, error) {
	ref, err := c.ociClient.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	remoteOpts, err := c.ociClient.RemoteOptions(ctx, oci.RegistryFromRef(ref))
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	signerOpts := []signing.SignerOption{
		signing.WithSignerLogger(c.logger),
		signing.WithSignerInsecure(c.insecure),
	}
	if opts.KeyPath != "" {
		signerOpts = append(signerOpts, signing.WithKeyPath(opts.KeyPath))
	}

	signer := signing.NewSigner(signerOpts...)

	hasSignature, err := signer.HasSignature(ctx, ref, remoteOpts...)
	if err != nil {
		return &VerifyResult{HasSignature: false}, err
	}
	if !hasSignature {
		return &VerifyResult{HasSignature: false}, nil
	}

	if opts.KeyPath != "" {
		publicKey, err := signing.LoadPublicKey(opts.KeyPath)
		if err != nil {
			return &VerifyResult{HasSignature: true}, fmt.Errorf("failed to load public key: %w", err)
		}

		vr, err := signer.VerifyWithPublicKey(ctx, ref, publicKey, remoteOpts...)
		if err != nil {
			return &VerifyResult{HasSignature: true}, err
		}

		return &VerifyResult{
			HasSignature: true,
			Verified:     vr.Verified,
			SignerID:     vr.SignerID,
		}, nil
	}

	return &VerifyResult{HasSignature: true, Verified: false}, nil
}

// checkSignature is a best-effort helper used by Inspect.
func (c *Client) checkSignature(ctx context.Context, ref interface{ String() string }, keyPath string) *SignatureInfo {
	parsedRef, err := c.ociClient.ParseReference(ref.String())
	if err != nil {
		return &SignatureInfo{Error: err.Error()}
	}

	remoteOpts, err := c.ociClient.RemoteOptions(ctx, oci.RegistryFromRef(parsedRef))
	if err != nil {
		return &SignatureInfo{Error: err.Error()}
	}

	signerOpts := []signing.SignerOption{
		signing.WithSignerLogger(c.logger),
		signing.WithSignerInsecure(c.insecure),
	}
	if keyPath != "" {
		signerOpts = append(signerOpts, signing.WithKeyPath(keyPath))
	}

	signer := signing.NewSigner(signerOpts...)

	hasSignature, err := signer.HasSignature(ctx, parsedRef, remoteOpts...)
	if err != nil {
		return &SignatureInfo{Error: err.Error()}
	}
	if !hasSignature {
		return &SignatureInfo{HasSignature: false}
	}

	if keyPath != "" {
		publicKey, err := signing.LoadPublicKey(keyPath)
		if err != nil {
			return &SignatureInfo{HasSignature: true, Error: fmt.Sprintf("failed to load public key: %v", err)}
		}
		vr, err := signer.VerifyWithPublicKey(ctx, parsedRef, publicKey, remoteOpts...)
		if err != nil {
			return &SignatureInfo{HasSignature: true, Error: err.Error()}
		}
		return &SignatureInfo{HasSignature: true, Verified: vr.Verified, SignerID: vr.SignerID}
	}

	return &SignatureInfo{
		HasSignature: true,
		Verified:     false,
		Error:        "public key required for verification",
	}
}
