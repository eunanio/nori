package nori

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/eunanio/nori/pkg/oci"
)

// Pull downloads a module artifact from an OCI registry and saves it locally.
func (c *Client) Pull(ctx context.Context, reference string, opts PullOptions) (*PullResult, error) {
	log := c.logger
	log.Info("pulling module", "reference", reference)

	ref, err := c.ociClient.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	outputPath := opts.OutputPath
	if outputPath == "" {
		repo := oci.RepositoryFromRef(ref)
		tag := oci.TagFromRef(ref)
		if tag == "" {
			tag = "latest"
		}
		repoName := filepath.Base(repo)
		outputPath = fmt.Sprintf("%s-%s.tar.gz", repoName, tag)
	}

	if err := c.ociClient.SaveArtifact(ctx, ref, outputPath); err != nil {
		return nil, fmt.Errorf("failed to pull module: %w", err)
	}

	return &PullResult{
		Reference:  reference,
		OutputPath: outputPath,
	}, nil
}
