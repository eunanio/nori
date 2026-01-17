// Package state provides OCI-based release state management.
package state

import (
	"time"
)

// Media types for release state artifact layers.
const (
	// MediaTypeReleaseMetadata is the media type for the release metadata layer.
	MediaTypeReleaseMetadata = "application/vnd.nori.release.metadata+yaml"

	// MediaTypeMainTF is the media type for the generated main.tf layer.
	MediaTypeMainTF = "application/vnd.nori.release.main-tf"

	// MediaTypeTFState is the media type for the terraform.tfstate layer.
	MediaTypeTFState = "application/vnd.nori.release.tfstate+json"

	// MediaTypeValues is the media type for the values.yaml layer.
	MediaTypeValues = "application/vnd.nori.release.values+yaml"

	// ArtifactTypeReleaseState is the artifact type for release state packages.
	ArtifactTypeReleaseState = "application/vnd.nori.release.state"
)

// ReleaseStatus represents the state of a release.
type ReleaseStatus string

const (
	// StatusPending indicates the release is being created.
	StatusPending ReleaseStatus = "pending"
	// StatusDeployed indicates the release was successfully deployed.
	StatusDeployed ReleaseStatus = "deployed"
	// StatusFailed indicates the release deployment failed.
	StatusFailed ReleaseStatus = "failed"
	// StatusSuperseded indicates this release version has been upgraded.
	StatusSuperseded ReleaseStatus = "superseded"
)

// ReleaseMetadata contains the metadata stored in the release state artifact.
type ReleaseMetadata struct {
	// Name is the unique identifier for this release.
	Name string `yaml:"name" json:"name"`

	// Version is the semantic version of this release state.
	Version string `yaml:"version" json:"version"`

	// ModuleRef is the OCI reference for the deployed module.
	ModuleRef string `yaml:"module_ref" json:"module_ref"`

	// Status is the current state of the release.
	Status ReleaseStatus `yaml:"status" json:"status"`

	// CreatedAt is when this release version was created.
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// UpdatedAt is when this release version was last modified.
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// Annotations are user-defined key-value pairs.
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`

	// Description is an optional description of this release version.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// ReleaseState represents a complete release state stored as an OCI artifact.
// Each field corresponds to a separate layer in the OCI manifest.
type ReleaseState struct {
	// Metadata contains the release metadata (release.yaml layer).
	Metadata *ReleaseMetadata

	// MainTF contains the generated Terraform code (main.tf layer).
	MainTF []byte

	// TFState contains the Terraform state file (terraform.tfstate layer).
	TFState []byte

	// Values contains the deployment values (values.yaml layer).
	Values []byte
}

// ReleaseHistory represents the version history of a release.
type ReleaseHistory struct {
	// Name is the release name.
	Name string

	// Versions is a list of versions for this release, sorted newest first.
	Versions []ReleaseVersion
}

// ReleaseVersion represents a single version entry in the release history.
type ReleaseVersion struct {
	// Version is the semantic version string.
	Version string

	// Tag is the full OCI tag for this version.
	Tag string

	// CreatedAt is when this version was created.
	CreatedAt time.Time

	// Status is the status of this version.
	Status ReleaseStatus

	// ModuleRef is the module reference used in this version.
	ModuleRef string

	// Annotations are user-defined key-value pairs.
	Annotations map[string]string
}

// NewReleaseMetadata creates a new release metadata with default values.
func NewReleaseMetadata(name, moduleRef, version string) *ReleaseMetadata {
	now := time.Now().UTC()
	return &ReleaseMetadata{
		Name:        name,
		Version:     version,
		ModuleRef:   moduleRef,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		Annotations: make(map[string]string),
	}
}

// SetAnnotation sets a single annotation on the metadata.
func (m *ReleaseMetadata) SetAnnotation(key, value string) {
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
	m.Annotations[key] = value
}

// GetAnnotation retrieves an annotation value by key.
func (m *ReleaseMetadata) GetAnnotation(key string) (string, bool) {
	if m.Annotations == nil {
		return "", false
	}
	v, ok := m.Annotations[key]
	return v, ok
}
