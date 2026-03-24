package nori

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eunanio/nori/internal/util"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/packaging"
	"github.com/eunanio/nori/pkg/signing"
)

// Package packages a module archive and pushes it to an OCI registry.
//
// When opts.PackageOnly is true the artifact is written to a local file
// (controlled by opts.OutputPath) instead of being pushed.
//
// When opts.Sign is true the artifact is signed after pushing.
func (c *Client) Package(ctx context.Context, reference, archivePath string, opts PackageOptions) (*PackageResult, error) {
	log := c.logger

	if err := oci.ValidateReference(reference, c.ociClient.NameOptions()...); err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	if opts.Sign && opts.PackageOnly {
		return nil, fmt.Errorf("sign cannot be used with package-only mode")
	}

	keyPath := opts.SignKeyPath
	if opts.Sign && keyPath == "" {
		if c.cfg.Signing.KeyPath != "" {
			keyPath = c.cfg.Signing.KeyPath
		} else {
			return nil, fmt.Errorf("sign requires a key path or signing.key_path in config")
		}
	}

	annotations := opts.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	packager := packaging.NewPackager(c.ociClient, log)
	pkgOpts := packaging.PackageOptions{
		Description: opts.Description,
		ConfigPath:  opts.ConfigPath,
		Annotations: annotations,
		Insecure:    c.insecure,
	}

	// Package-only: write to local file
	if opts.PackageOnly {
		return c.packageOnly(ctx, packager, reference, archivePath, pkgOpts, opts.OutputPath)
	}

	result, err := packager.Package(ctx, reference, archivePath, pkgOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to package module: %w", err)
	}

	out := &PackageResult{
		Reference:   result.Reference,
		Digest:      result.Digest,
		Size:        result.Size,
		Annotations: result.Annotations,
	}

	if opts.Sign {
		sigRef, err := c.signArtifact(ctx, reference, keyPath, opts.SignPassword)
		if err != nil {
			return out, fmt.Errorf("artifact pushed but signing failed: %w", err)
		}
		out.Signed = true
		out.SignatureRef = sigRef
	}

	return out, nil
}

func (c *Client) packageOnly(ctx context.Context, packager *packaging.Packager, reference, archivePath string, opts packaging.PackageOptions, outputPath string) (*PackageResult, error) {
	result, err := packager.PreparePackage(ctx, archivePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare package: %w", err)
	}

	if outputPath == "" {
		outputPath = DeriveOutputFilename(reference)
	}

	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	if err := os.WriteFile(outputPath, result.Content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write package file: %w", err)
	}

	return &PackageResult{
		Reference: reference,
		Size:      result.Size,
		LocalPath: outputPath,
	}, nil
}

// signArtifact signs an already-pushed OCI artifact and returns the signature reference.
func (c *Client) signArtifact(ctx context.Context, reference, keyPath string, password []byte) (string, error) {
	ref, err := c.ociClient.ParseReference(reference)
	if err != nil {
		return "", fmt.Errorf("invalid reference: %w", err)
	}

	remoteOpts, err := c.ociClient.RemoteOptions(ctx, oci.RegistryFromRef(ref))
	if err != nil {
		return "", fmt.Errorf("failed to get remote options: %w", err)
	}

	if password == nil && c.cfg.Signing.PasswordEnv != "" {
		if envPw := os.Getenv(c.cfg.Signing.PasswordEnv); envPw != "" {
			password = []byte(envPw)
		}
	}

	if password == nil {
		return "", fmt.Errorf("signing password required: set SignPassword or configure signing.password_env")
	}

	signer := signing.NewSigner(
		signing.WithSignerLogger(c.logger),
		signing.WithSignerInsecure(c.insecure),
		signing.WithKeyPath(keyPath),
		signing.WithPassword(password),
	)

	signResult, err := signer.Sign(ctx, ref, remoteOpts...)
	if err != nil {
		return "", err
	}

	return signResult.SignatureRef, nil
}

// Push pushes a local archive file to an OCI registry.
func (c *Client) Push(ctx context.Context, reference, filePath string, opts PushOptions) (*PushResult, error) {
	log := c.logger
	log.Info("pushing artifact", "reference", reference, "file", filePath)

	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if err := util.ValidateArchive(filePath); err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	ref, err := c.ociClient.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	annotations := opts.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	artifact, err := c.ociClient.LoadAndPushArtifact(ctx, ref, filePath, annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to push artifact: %w", err)
	}

	return &PushResult{
		Reference: reference,
		Digest:    artifact.Digest,
	}, nil
}
