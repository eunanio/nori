package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// Status represents the state of a release.
type Status string

const (
	// StatusPending indicates the release is being created.
	StatusPending Status = "pending"
	// StatusDeployed indicates the release was successfully deployed.
	StatusDeployed Status = "deployed"
	// StatusFailed indicates the release deployment failed.
	StatusFailed Status = "failed"
	// StatusDestroying indicates the release is being destroyed.
	StatusDestroying Status = "destroying"
)

// Release represents a deployed module instance.
type Release struct {
	// Name is the unique identifier for this release.
	Name string `yaml:"name"`

	// ModuleRef is the OCI reference for the module.
	ModuleRef string `yaml:"module_ref"`

	// Version is the legacy integer version (deprecated, use SemVer).
	Version int `yaml:"version"`

	// SemVer is the semantic version of this release (e.g., "v1.0.0").
	SemVer string `yaml:"semver,omitempty"`

	// StateRef is the OCI reference to the release state artifact.
	StateRef string `yaml:"state_ref,omitempty"`

	// Values are the merged values used for deployment.
	Values map[string]interface{} `yaml:"values,omitempty"`

	// ValuesFile is the path to the original values file (for reference).
	ValuesFile string `yaml:"values_file,omitempty"`

	// Status is the current state of the release.
	Status Status `yaml:"status"`

	// CreatedAt is when the release was first created.
	CreatedAt time.Time `yaml:"created_at"`

	// UpdatedAt is when the release was last modified.
	UpdatedAt time.Time `yaml:"updated_at"`

	// Annotations are user-defined key-value pairs for the release.
	Annotations map[string]string `yaml:"annotations,omitempty"`

	// BackendType is the Terraform backend type (local, s3, gcs, azurerm, etc.).
	BackendType string `yaml:"backend_type,omitempty"`

	// BackendConfig contains backend-specific configuration.
	BackendConfig map[string]string `yaml:"backend_config,omitempty"`
}

// Store manages release persistence.
type Store struct {
	baseDir string
}

// DefaultReleasesDir returns the default releases directory.
func DefaultReleasesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nori", "releases")
}

// NewStore creates a new release store.
func NewStore(baseDir string) *Store {
	if baseDir == "" {
		baseDir = DefaultReleasesDir()
	}
	return &Store{baseDir: baseDir}
}

// GetWorkDir returns the working directory for a release.
func (s *Store) GetWorkDir(name string) string {
	return filepath.Join(s.baseDir, name)
}

// GetModuleDir returns the module directory for a release.
func (s *Store) GetModuleDir(name string) string {
	return filepath.Join(s.GetWorkDir(name), "module")
}

// releaseFilePath returns the path to the release metadata file.
func (s *Store) releaseFilePath(name string) string {
	return filepath.Join(s.GetWorkDir(name), "release.yaml")
}

// Exists checks if a release exists.
func (s *Store) Exists(name string) bool {
	_, err := os.Stat(s.releaseFilePath(name))
	return err == nil
}

// Get retrieves a release by name.
func (s *Store) Get(name string) (*Release, error) {
	path := s.releaseFilePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("release %q not found", name)
		}
		return nil, fmt.Errorf("failed to read release: %w", err)
	}

	var release Release
	if err := yaml.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// Save persists a release to disk.
func (s *Store) Save(r *Release) error {
	workDir := s.GetWorkDir(r.Name)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create release directory: %w", err)
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal release: %w", err)
	}

	path := s.releaseFilePath(r.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write release: %w", err)
	}

	return nil
}

// Delete removes a release and its working directory.
func (s *Store) Delete(name string) error {
	workDir := s.GetWorkDir(name)
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to delete release: %w", err)
	}
	return nil
}

// List returns all releases.
func (s *Store) List() ([]*Release, error) {
	// Ensure base directory exists
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return []*Release{}, nil
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read releases directory: %w", err)
	}

	var releases []*Release
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		release, err := s.Get(entry.Name())
		if err != nil {
			// Skip invalid releases
			continue
		}
		releases = append(releases, release)
	}

	// Sort by name
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Name < releases[j].Name
	})

	return releases, nil
}

// InitialSemVer is the default semantic version for new releases.
const InitialSemVer = "v1.0.0"

// NewRelease creates a new release with default values.
func NewRelease(name, moduleRef string) *Release {
	now := time.Now()
	return &Release{
		Name:        name,
		ModuleRef:   moduleRef,
		Version:     1,
		SemVer:      InitialSemVer,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		BackendType: "local",
		Annotations: make(map[string]string),
	}
}

// UpdateForUpgrade prepares the release for an upgrade.
func (r *Release) UpdateForUpgrade(moduleRef string, values map[string]interface{}) {
	r.Version++
	r.UpdatedAt = time.Now()
	r.Status = StatusPending
	if moduleRef != "" {
		r.ModuleRef = moduleRef
	}
	if values != nil {
		r.Values = values
	}
}

// SetAnnotation sets a single annotation on the release.
func (r *Release) SetAnnotation(key, value string) {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	r.Annotations[key] = value
}

// GetAnnotation retrieves an annotation value by key.
func (r *Release) GetAnnotation(key string) (string, bool) {
	if r.Annotations == nil {
		return "", false
	}
	v, ok := r.Annotations[key]
	return v, ok
}
