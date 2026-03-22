package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/eunanio/nori/pkg/oci"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"gopkg.in/yaml.v3"
)

// StateStore provides OCI-based release state storage operations.
type StateStore struct {
	client *oci.Client
	logger *slog.Logger
}

// NewStateStore creates a new OCI-based state store.
func NewStateStore(client *oci.Client, logger *slog.Logger) *StateStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &StateStore{
		client: client,
		logger: logger,
	}
}

// stateArtifactImage wraps a v1.Image to set artifactType for release state.
type stateArtifactImage struct {
	v1.Image
	artifactType string
}

// ociManifest is an OCI image manifest with artifactType support.
type ociManifest struct {
	SchemaVersion int64             `json:"schemaVersion"`
	MediaType     types.MediaType   `json:"mediaType,omitempty"`
	ArtifactType  string            `json:"artifactType,omitempty"`
	Config        v1.Descriptor     `json:"config"`
	Layers        []v1.Descriptor   `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// RawManifest returns the serialized manifest with artifactType set.
func (i *stateArtifactImage) RawManifest() ([]byte, error) {
	m, err := i.Image.Manifest()
	if err != nil {
		return nil, err
	}

	manifest := ociManifest{
		SchemaVersion: m.SchemaVersion,
		MediaType:     m.MediaType,
		ArtifactType:  i.artifactType,
		Config:        m.Config,
		Layers:        m.Layers,
		Annotations:   m.Annotations,
	}

	return json.Marshal(manifest)
}

// Digest returns the digest of the manifest with artifactType.
func (i *stateArtifactImage) Digest() (v1.Hash, error) {
	raw, err := i.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	return h, err
}

// PushState pushes a release state to the OCI registry.
// The state is stored as a multi-layer artifact with each file in its own layer.
func (s *StateStore) PushState(ctx context.Context, ref string, state *ReleaseState) error {
	s.logger.Info("pushing release state", "reference", ref)

	parsedRef, err := s.client.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Serialize metadata to YAML
	metadataBytes, err := yaml.Marshal(state.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Build the image with multiple layers
	img := empty.Image

	// Set config
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{
		Created: v1.Time{Time: time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	// Layer 1: Release metadata (release.yaml)
	metadataLayer := oci.NewLayer(metadataBytes, MediaTypeReleaseMetadata)
	img, err = mutate.AppendLayers(img, metadataLayer)
	if err != nil {
		return fmt.Errorf("failed to add metadata layer: %w", err)
	}

	// Layer 2: Generated Terraform code (main.tf)
	if len(state.MainTF) > 0 {
		mainTFLayer := oci.NewLayer(state.MainTF, MediaTypeMainTF)
		img, err = mutate.AppendLayers(img, mainTFLayer)
		if err != nil {
			return fmt.Errorf("failed to add main.tf layer: %w", err)
		}
	}

	// Layer 3: Terraform state (terraform.tfstate)
	if len(state.TFState) > 0 {
		tfStateLayer := oci.NewLayer(state.TFState, MediaTypeTFState)
		img, err = mutate.AppendLayers(img, tfStateLayer)
		if err != nil {
			return fmt.Errorf("failed to add tfstate layer: %w", err)
		}
	}

	// Layer 4: Values (values.yaml)
	if len(state.Values) > 0 {
		valuesLayer := oci.NewLayer(state.Values, MediaTypeValues)
		img, err = mutate.AppendLayers(img, valuesLayer)
		if err != nil {
			return fmt.Errorf("failed to add values layer: %w", err)
		}
	}

	// Set manifest annotations
	annotations := map[string]string{
		oci.AnnotationCreated:     time.Now().UTC().Format(time.RFC3339),
		oci.AnnotationVersion:     state.Metadata.Version,
		oci.AnnotationNoriVersion: "1.2.0",
		"io.nori.release.name":    state.Metadata.Name,
		"io.nori.release.status":  string(state.Metadata.Status),
		"io.nori.module.ref":      state.Metadata.ModuleRef,
	}

	// Add user-defined annotations
	for k, v := range state.Metadata.Annotations {
		annotations[k] = v
	}

	img = mutate.Annotations(img, annotations).(v1.Image)

	// Convert to OCI manifest format
	img = mutate.MediaType(img, types.OCIManifestSchema1)

	// Wrap to add artifactType
	img = &stateArtifactImage{
		Image:        img,
		artifactType: ArtifactTypeReleaseState,
	}

	// Get remote options from the client
	registry := oci.RegistryFromRef(parsedRef)
	opts, err := s.getRemoteOptions(ctx, registry)
	if err != nil {
		return fmt.Errorf("failed to get remote options: %w", err)
	}

	// Push the image
	if err := remote.Write(parsedRef, img, opts...); err != nil {
		return fmt.Errorf("failed to push state: %w", err)
	}

	digest, _ := img.Digest()
	s.logger.Info("release state pushed successfully",
		"reference", ref,
		"digest", digest.String(),
		"version", state.Metadata.Version,
	)

	return nil
}

// PullState pulls a release state from the OCI registry.
func (s *StateStore) PullState(ctx context.Context, ref string) (*ReleaseState, error) {
	s.logger.Info("pulling release state", "reference", ref)

	parsedRef, err := s.client.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	registry := oci.RegistryFromRef(parsedRef)
	opts, err := s.getRemoteOptions(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	// Pull the image
	img, err := remote.Image(parsedRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull state: %w", err)
	}

	// Get layers
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("state artifact has no layers")
	}

	state := &ReleaseState{}

	// Extract each layer based on media type
	for _, layer := range layers {
		mediaType, err := layer.MediaType()
		if err != nil {
			s.logger.Warn("failed to get layer media type", "error", err)
			continue
		}

		content, err := extractLayerContent(layer)
		if err != nil {
			return nil, fmt.Errorf("failed to extract layer content: %w", err)
		}

		switch string(mediaType) {
		case MediaTypeReleaseMetadata:
			var metadata ReleaseMetadata
			if err := yaml.Unmarshal(content, &metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}
			state.Metadata = &metadata

		case MediaTypeMainTF:
			state.MainTF = content

		case MediaTypeTFState:
			state.TFState = content

		case MediaTypeValues:
			state.Values = content
		}
	}

	if state.Metadata == nil {
		return nil, fmt.Errorf("state artifact missing metadata layer")
	}

	s.logger.Info("release state pulled successfully",
		"reference", ref,
		"name", state.Metadata.Name,
		"version", state.Metadata.Version,
	)

	return state, nil
}

// ListReleases lists all releases in the state repository.
func (s *StateStore) ListReleases(ctx context.Context, stateRepository string) ([]string, error) {
	s.logger.Debug("listing releases", "repository", stateRepository)

	repo, err := s.client.NewRepository(stateRepository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	opts, err := s.getRemoteOptions(ctx, repo.RegistryStr())
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}

// GetReleaseHistory retrieves the version history for a release.
func (s *StateStore) GetReleaseHistory(ctx context.Context, stateRepository, releaseName string) (*ReleaseHistory, error) {
	s.logger.Debug("getting release history", "repository", stateRepository, "release", releaseName)

	// Use flat repository structure - all releases are tags on the base state repository
	repo, err := s.client.NewRepository(stateRepository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	opts, err := s.getRemoteOptions(ctx, repo.RegistryStr())
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	history := &ReleaseHistory{
		Name:     releaseName,
		Versions: make([]ReleaseVersion, 0),
	}

	// Filter tags by release name prefix and extract versions
	var versions []string
	for _, tag := range tags {
		version, err := ExtractVersionFromTag(tag, releaseName)
		if err == nil {
			versions = append(versions, version)
		}
	}

	SortVersionsDesc(versions)

	// Get metadata for each version
	for _, version := range versions {
		tag := FormatReleaseTag(releaseName, version)
		ref := fmt.Sprintf("%s:%s", stateRepository, tag)

		// Try to get metadata from annotations (faster than pulling full state)
		parsedRef, err := s.client.ParseReference(ref)
		if err != nil {
			continue
		}

		desc, err := remote.Get(parsedRef, opts...)
		if err != nil {
			continue
		}

		img, err := desc.Image()
		if err != nil {
			continue
		}

		manifest, err := img.Manifest()
		if err != nil {
			continue
		}

		rv := ReleaseVersion{
			Version:     version,
			Tag:         tag,
			Annotations: make(map[string]string),
		}

		// Extract info from annotations
		if manifest.Annotations != nil {
			if created, ok := manifest.Annotations[oci.AnnotationCreated]; ok {
				rv.CreatedAt, _ = time.Parse(time.RFC3339, created)
			}
			if status, ok := manifest.Annotations["io.nori.release.status"]; ok {
				rv.Status = ReleaseStatus(status)
			}
			if moduleRef, ok := manifest.Annotations["io.nori.module.ref"]; ok {
				rv.ModuleRef = moduleRef
			}

			// Copy user annotations (exclude system annotations)
			for k, v := range manifest.Annotations {
				if !isSystemAnnotation(k) {
					rv.Annotations[k] = v
				}
			}
		}

		history.Versions = append(history.Versions, rv)
	}

	return history, nil
}

// GetLatestVersion returns the latest version for a release.
func (s *StateStore) GetLatestVersion(ctx context.Context, stateRepository, releaseName string) (string, error) {
	history, err := s.GetReleaseHistory(ctx, stateRepository, releaseName)
	if err != nil {
		return "", err
	}

	if len(history.Versions) == 0 {
		return "", fmt.Errorf("no versions found for release %q", releaseName)
	}

	return history.Versions[0].Version, nil
}

// GetLastSuccessfulVersion returns the most recent version with StatusDeployed.
func (s *StateStore) GetLastSuccessfulVersion(ctx context.Context, stateRepository, releaseName string) (string, error) {
	history, err := s.GetReleaseHistory(ctx, stateRepository, releaseName)
	if err != nil {
		return "", err
	}

	for _, v := range history.Versions {
		if v.Status == StatusDeployed {
			return v.Version, nil
		}
	}
	return "", nil
}

// ReleaseExists checks if a release exists in the state repository.
func (s *StateStore) ReleaseExists(ctx context.Context, stateRepository, releaseName string) (bool, error) {
	_, err := s.GetLatestVersion(ctx, stateRepository, releaseName)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// DeleteRelease deletes all versions of a release from the OCI state repository.
func (s *StateStore) DeleteRelease(ctx context.Context, stateRepository, releaseName string) error {
	s.logger.Info("deleting release from OCI", "release", releaseName)

	// Get all versions
	history, err := s.GetReleaseHistory(ctx, stateRepository, releaseName)
	if err != nil {
		return fmt.Errorf("failed to get release history: %w", err)
	}

	if len(history.Versions) == 0 {
		s.logger.Debug("no versions found for release", "release", releaseName)
		return nil
	}

	// Delete each version (using flat repository structure)
	var deleteErrors []error
	for _, version := range history.Versions {
		ref := fmt.Sprintf("%s:%s", stateRepository, version.Tag)
		s.logger.Debug("deleting release version", "reference", ref)

		parsedRef, err := s.client.ParseReference(ref)
		if err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("invalid reference %s: %w", ref, err))
			continue
		}

		registry := oci.RegistryFromRef(parsedRef)
		opts, err := s.getRemoteOptions(ctx, registry)
		if err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to get remote options for %s: %w", ref, err))
			continue
		}

		if err := remote.Delete(parsedRef, opts...); err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete %s: %w", ref, err))
			continue
		}

		s.logger.Debug("deleted release version", "reference", ref)
	}

	if len(deleteErrors) > 0 {
		// Log all errors but return the first one
		for _, err := range deleteErrors {
			s.logger.Warn("delete error", "error", err)
		}
		return deleteErrors[0]
	}

	s.logger.Info("release deleted from OCI", "release", releaseName, "versions_deleted", len(history.Versions))
	return nil
}

// getRemoteOptions returns remote options for OCI operations.
func (s *StateStore) getRemoteOptions(ctx context.Context, registry string) ([]remote.Option, error) {
	return s.client.RemoteOptions(ctx, registry)
}

// extractLayerContent reads the content from a layer.
func extractLayerContent(layer v1.Layer) ([]byte, error) {
	rc, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// isSystemAnnotation returns true if the annotation key is a system annotation.
func isSystemAnnotation(key string) bool {
	systemPrefixes := []string{
		"org.opencontainers.",
		"io.nori.release.",
		"io.nori.module.",
		"io.nori.version",
	}

	for _, prefix := range systemPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
