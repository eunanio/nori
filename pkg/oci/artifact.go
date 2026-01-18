package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/eunanio/nori/internal/util"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// OpenTofu compatible media types for module packages.
const (
	// ArtifactTypeModule is the artifact type for OpenTofu module packages.
	ArtifactTypeModule = "application/vnd.opentofu.modulepkg"

	// MediaTypeModuleZip is the media type for zip module layers (OpenTofu compatible).
	MediaTypeModuleZip = "archive/zip"

	// MediaTypeModuleLayerGzip is the media type for gzip-compressed module layers (legacy).
	MediaTypeModuleLayerGzip = "application/vnd.oci.image.layer.v1.tar+gzip"

	// MediaTypeConfig is the media type for the config layer.
	MediaTypeConfig = "application/vnd.oci.image.config.v1+json"

	// MediaTypeManifest is the media type for the manifest.
	MediaTypeManifest = "application/vnd.oci.image.manifest.v1+json"

	// AnnotationTitle is the ORAS annotation for artifact title.
	AnnotationTitle = "org.opencontainers.image.title"

	// AnnotationCreated is the annotation for creation time.
	AnnotationCreated = "org.opencontainers.image.created"

	// AnnotationDescription is the annotation for description.
	AnnotationDescription = "org.opencontainers.image.description"

	// AnnotationSource is the annotation for source.
	AnnotationSource = "org.opencontainers.image.source"

	// AnnotationVersion is the annotation for version.
	AnnotationVersion = "org.opencontainers.image.version"

	// AnnotationNoriVersion is the annotation for Nori version.
	AnnotationNoriVersion = "io.nori.version"

	// AnnotationModuleType is the annotation for module type.
	AnnotationModuleType = "io.nori.module.type"
)

// Artifact represents an OCI artifact for a Terraform module.
type Artifact struct {
	Reference   name.Reference
	Digest      string
	Annotations map[string]string
	Layers      []LayerInfo
	Config      []byte
}

// LayerInfo contains information about an artifact layer.
type LayerInfo struct {
	Digest    string
	Size      int64
	MediaType string
}

// ModuleConfig is the configuration stored in the artifact.
type ModuleConfig struct {
	Created     time.Time         `json:"created"`
	NoriVersion string            `json:"nori_version"`
	ModuleType  string            `json:"module_type"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// artifactLayer implements v1.Layer for arbitrary content.
type artifactLayer struct {
	content   []byte
	mediaType types.MediaType
}

func (l *artifactLayer) Digest() (v1.Hash, error) {
	r, err := l.Compressed()
	if err != nil {
		return v1.Hash{}, err
	}
	defer r.Close()
	h, _, err := v1.SHA256(r)
	return h, err
}

func (l *artifactLayer) DiffID() (v1.Hash, error) {
	r, err := l.Uncompressed()
	if err != nil {
		return v1.Hash{}, err
	}
	defer r.Close()
	h, _, err := v1.SHA256(r)
	return h, err
}

func (l *artifactLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.content)), nil
}

func (l *artifactLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.content)), nil
}

func (l *artifactLayer) Size() (int64, error) {
	return int64(len(l.content)), nil
}

func (l *artifactLayer) MediaType() (types.MediaType, error) {
	return l.mediaType, nil
}

// NewLayer creates a new layer from content.
func NewLayer(content []byte, mediaType string) v1.Layer {
	return &artifactLayer{
		content:   content,
		mediaType: types.MediaType(mediaType),
	}
}

// ociManifest is an OCI image manifest with artifactType support.
// This extends the go-containerregistry v1.Manifest to include artifactType.
type ociManifest struct {
	SchemaVersion int64             `json:"schemaVersion"`
	MediaType     types.MediaType   `json:"mediaType,omitempty"`
	ArtifactType  string            `json:"artifactType,omitempty"`
	Config        v1.Descriptor     `json:"config"`
	Layers        []v1.Descriptor   `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// artifactTypeImage wraps a v1.Image to add artifactType to the manifest.
// This is needed for OpenTofu compatibility which requires artifactType to be set.
type artifactTypeImage struct {
	v1.Image
	artifactType string
}

// RawManifest returns the serialized manifest with artifactType set.
func (i *artifactTypeImage) RawManifest() ([]byte, error) {
	m, err := i.Image.Manifest()
	if err != nil {
		return nil, err
	}

	// Create a manifest with artifactType
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
func (i *artifactTypeImage) Digest() (v1.Hash, error) {
	raw, err := i.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	return h, err
}

// PushArtifact pushes an artifact to a registry.
// The artifact is pushed in OpenTofu-compatible format with artifactType and archive/zip media type.
func (c *Client) PushArtifact(ctx context.Context, ref name.Reference, content []byte, annotations map[string]string) (*Artifact, error) {
	c.logger.Info("pushing artifact", "reference", ref.String())

	registry := RegistryFromRef(ref)
	opts, err := c.RemoteOptions(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	// Create the module layer with OpenTofu-compatible media type (archive/zip)
	moduleLayer := NewLayer(content, MediaTypeModuleZip)

	// Create config
	config := ModuleConfig{
		Created:     time.Now().UTC(),
		NoriVersion: "1.0.1",
		ModuleType:  "terraform",
		Annotations: annotations,
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Build the image
	img := empty.Image

	// Add config
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{
		Created: v1.Time{Time: config.Created},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set config: %w", err)
	}

	// Add the module layer
	img, err = mutate.AppendLayers(img, moduleLayer)
	if err != nil {
		return nil, fmt.Errorf("failed to add layer: %w", err)
	}

	// Set annotations
	allAnnotations := make(map[string]string)
	allAnnotations[AnnotationCreated] = config.Created.Format(time.RFC3339)
	allAnnotations[AnnotationNoriVersion] = config.NoriVersion
	allAnnotations[AnnotationModuleType] = config.ModuleType

	for k, v := range annotations {
		allAnnotations[k] = v
	}

	img = mutate.Annotations(img, allAnnotations).(v1.Image)

	// Convert to OCI manifest format (required by OpenTofu)
	img = mutate.MediaType(img, types.OCIManifestSchema1)

	// Wrap the image to set artifactType for OpenTofu compatibility
	img = &artifactTypeImage{
		Image:        img,
		artifactType: ArtifactTypeModule,
	}

	// Push the image
	if err := remote.Write(ref, img, opts...); err != nil {
		return nil, fmt.Errorf("failed to push artifact: %w", err)
	}

	// Get the digest
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get digest: %w", err)
	}

	moduleDigest, err := moduleLayer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer digest: %w", err)
	}

	moduleSize, err := moduleLayer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer size: %w", err)
	}

	c.logger.Info("artifact pushed successfully",
		"reference", ref.String(),
		"digest", digest.String(),
	)

	return &Artifact{
		Reference:   ref,
		Digest:      digest.String(),
		Annotations: allAnnotations,
		Layers: []LayerInfo{
			{
				Digest:    moduleDigest.String(),
				Size:      moduleSize,
				MediaType: MediaTypeModuleZip,
			},
		},
		Config: configBytes,
	}, nil
}

// PullArtifact pulls an artifact from a registry.
func (c *Client) PullArtifact(ctx context.Context, ref name.Reference) (*Artifact, []byte, error) {
	c.logger.Info("pulling artifact", "reference", ref.String())

	registry := RegistryFromRef(ref)
	opts, err := c.RemoteOptions(ctx, registry)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	// Pull the image
	img, err := remote.Image(ref, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pull artifact: %w", err)
	}

	// Get manifest
	manifest, err := img.Manifest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// Get the digest
	digest, err := img.Digest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get digest: %w", err)
	}

	// Get layers
	layers, err := img.Layers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, nil, fmt.Errorf("artifact has no layers")
	}

	// Get the module content from the first layer
	moduleLayer := layers[0]
	content, err := extractLayerContent(moduleLayer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract layer content: %w", err)
	}

	// Build layer info
	var layerInfos []LayerInfo
	for _, layer := range layers {
		layerDigest, _ := layer.Digest()
		layerSize, _ := layer.Size()
		layerMediaType, _ := layer.MediaType()
		layerInfos = append(layerInfos, LayerInfo{
			Digest:    layerDigest.String(),
			Size:      layerSize,
			MediaType: string(layerMediaType),
		})
	}

	// Get config
	configBytes, err := img.RawConfigFile()
	if err != nil {
		c.logger.Debug("failed to get config file", "error", err)
	}

	artifact := &Artifact{
		Reference:   ref,
		Digest:      digest.String(),
		Annotations: manifest.Annotations,
		Layers:      layerInfos,
		Config:      configBytes,
	}

	c.logger.Info("artifact pulled successfully",
		"reference", ref.String(),
		"digest", digest.String(),
		"size", len(content),
	)

	return artifact, content, nil
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

// InspectArtifact retrieves metadata about an artifact without downloading it.
func (c *Client) InspectArtifact(ctx context.Context, ref name.Reference) (*Artifact, error) {
	c.logger.Debug("inspecting artifact", "reference", ref.String())

	registry := RegistryFromRef(ref)
	opts, err := c.RemoteOptions(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	// Get the image descriptor
	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact descriptor: %w", err)
	}

	// Parse the manifest
	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to parse image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// Get layers info
	var layerInfos []LayerInfo
	for _, layer := range manifest.Layers {
		layerInfos = append(layerInfos, LayerInfo{
			Digest:    layer.Digest.String(),
			Size:      layer.Size,
			MediaType: string(layer.MediaType),
		})
	}

	// Get config
	configBytes, err := img.RawConfigFile()
	if err != nil {
		c.logger.Debug("failed to get config file", "error", err)
	}

	return &Artifact{
		Reference:   ref,
		Digest:      desc.Digest.String(),
		Annotations: manifest.Annotations,
		Layers:      layerInfos,
		Config:      configBytes,
	}, nil
}

// ListTags lists all tags for a repository.
func (c *Client) ListTags(ctx context.Context, repo name.Repository) ([]string, error) {
	c.logger.Debug("listing tags", "repository", repo.String())

	registry := repo.RegistryStr()
	opts, err := c.RemoteOptions(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return tags, nil
}

// DeleteArtifact deletes an artifact from a registry.
func (c *Client) DeleteArtifact(ctx context.Context, ref name.Reference) error {
	c.logger.Info("deleting artifact", "reference", ref.String())

	registry := RegistryFromRef(ref)
	opts, err := c.RemoteOptions(ctx, registry)
	if err != nil {
		return fmt.Errorf("failed to get remote options: %w", err)
	}

	if err := remote.Delete(ref, opts...); err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	c.logger.Info("artifact deleted successfully", "reference", ref.String())
	return nil
}

// SaveArtifact saves an artifact to a local file.
func (c *Client) SaveArtifact(ctx context.Context, ref name.Reference, path string) error {
	c.logger.Info("saving artifact to file", "reference", ref.String(), "path", path)

	_, content, err := c.PullArtifact(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to pull artifact: %w", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	c.logger.Info("artifact saved successfully", "path", path)
	return nil
}

// LoadAndPushArtifact loads a file and pushes it as an artifact.
// OpenTofu requires modules in zip format, so tar.gz files are converted automatically.
func (c *Client) LoadAndPushArtifact(ctx context.Context, ref name.Reference, path string, annotations map[string]string) (*Artifact, error) {
	c.logger.Info("loading and pushing artifact", "path", path, "reference", ref.String())

	var content []byte
	var err error

	// OpenTofu requires zip format - convert tar.gz if needed
	archiveType := util.DetectArchiveType(path)
	if archiveType == util.ArchiveTypeTarGz {
		c.logger.Debug("converting tar.gz to zip for OpenTofu compatibility")
		content, err = util.ConvertTarGzToZip(path)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tar.gz to zip: %w", err)
		}
	} else {
		// Zip files are used as-is (OpenTofu native format)
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	}

	return c.PushArtifact(ctx, ref, content, annotations)
}
