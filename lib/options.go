package nori

import (
	"time"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/state"
)

// ---------------------------------------------------------------------------
// Package / Push
// ---------------------------------------------------------------------------

// PackageOptions configures a Package operation.
type PackageOptions struct {
	// Description is the module description annotation.
	Description string
	// ConfigPath is an optional path to Terraform config for metadata extraction.
	ConfigPath string
	// Annotations are custom OCI annotations (key→value).
	Annotations map[string]string
	// PackageOnly prepares the artifact without pushing to a registry.
	// When true, OutputPath controls where the local file is written.
	PackageOnly bool
	// OutputPath is the local file path for --package-only mode.
	OutputPath string
	// Sign signs the artifact after pushing.
	Sign bool
	// SignKeyPath is the path to the private signing key.
	// Falls back to config signing.key_path when empty.
	SignKeyPath string
	// SignPassword is the password for the signing key.
	// When nil, the library checks the config password env var.
	SignPassword []byte
}

// PackageResult is returned by Package.
type PackageResult struct {
	Reference   string
	Digest      string
	Size        int64
	Annotations map[string]string
	// LocalPath is set when PackageOnly is true.
	LocalPath string
	// Signed indicates whether the artifact was signed.
	Signed bool
	// SignatureRef is set when the artifact was signed.
	SignatureRef string
}

// PushOptions configures a Push operation.
type PushOptions struct {
	Annotations map[string]string
}

// PushResult is returned by Push.
type PushResult struct {
	Reference string
	Digest    string
}

// ---------------------------------------------------------------------------
// Pull
// ---------------------------------------------------------------------------

// PullOptions configures a Pull operation.
type PullOptions struct {
	// OutputPath overrides the default output file name.
	OutputPath string
}

// PullResult is returned by Pull.
type PullResult struct {
	Reference  string
	OutputPath string
}

// ---------------------------------------------------------------------------
// Inspect / Verify
// ---------------------------------------------------------------------------

// InspectOptions configures an Inspect operation.
type InspectOptions struct {
	// IncludeReadme fetches the README layer content.
	IncludeReadme bool
}

// InspectResult is returned by Inspect.
type InspectResult struct {
	Reference   string
	Digest      string
	Layers      []oci.LayerInfo
	Annotations map[string]string
	Config      []byte
	Readme      []byte
	Signature   *SignatureInfo
}

// SignatureInfo describes an artifact's signature status.
type SignatureInfo struct {
	HasSignature bool   `json:"has_signature"`
	Verified     bool   `json:"verified"`
	SignerID     string `json:"signer_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// SignOptions configures a Sign operation.
type SignOptions struct {
	KeyPath  string
	Password []byte
}

// SignResult is returned by Sign.
type SignResult struct {
	SignatureRef string
}

// VerifyOptions configures a Verify operation.
type VerifyOptions struct {
	// KeyPath is the path to the public key for verification.
	// If empty, only checks for signature presence.
	KeyPath string
}

// VerifyResult is returned by Verify.
type VerifyResult struct {
	HasSignature bool
	Verified     bool
	SignerID     string
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// Credentials represents registry authentication credentials.
type Credentials struct {
	Username string
	Password string
}

// ---------------------------------------------------------------------------
// Deploy (non-release one-off)
// ---------------------------------------------------------------------------

// DeployOptions configures a one-off Deploy operation.
type DeployOptions struct {
	ValuesFile    string
	Values        map[string]interface{}
	WorkDir       string
	AutoApprove   bool
	Parallelism   int
	VarFiles      []string
	BackendConfig map[string]string
	Targets       []string
	Destroy       bool
	PlanOnly      bool
	Upgrade       bool
}

// DeployResult is returned by Deploy.
type DeployResult struct {
	ModuleRef  string
	WorkDir    string
	PlanFile   string
	HasChanges bool
	Applied    bool
	Outputs    map[string]interface{}
}

// DestroyOptions configures a one-off Destroy operation.
type DestroyOptions struct {
	AutoApprove bool
	Parallelism int
	Targets     []string
}

// ---------------------------------------------------------------------------
// Release lifecycle
// ---------------------------------------------------------------------------

// CreateReleaseOptions configures CreateRelease.
type CreateReleaseOptions struct {
	ValuesFile    string
	Values        map[string]interface{}
	Annotations   map[string]string
	AutoApprove   bool
	Parallelism   int
	VarFiles      []string
	BackendConfig map[string]string
	Targets       []string
	PlanOnly      bool
	Upgrade       bool
	Description   string
}

// UpgradeReleaseOptions configures UpgradeRelease.
type UpgradeReleaseOptions struct {
	// ModuleRef overrides the module reference (full OCI ref).
	ModuleRef string
	// Tag updates only the tag on the existing module reference.
	Tag               string
	ValuesFile        string
	Values            map[string]interface{}
	Annotations       map[string]string
	AutoApprove       bool
	Parallelism       int
	VarFiles          []string
	BackendConfig     map[string]string
	Targets           []string
	PlanOnly          bool
	UpgradeInit       bool
	ReuseValues       bool
	ResetValues       bool
	Description       string
	Version           string
	RollbackOnFailure bool
}

// DestroyReleaseOptions configures DestroyRelease.
type DestroyReleaseOptions struct {
	AutoApprove bool
	Parallelism int
	Targets     []string
	KeepHistory bool
	DryRun      bool
}

// ReleaseResult is returned by CreateRelease and UpgradeRelease.
type ReleaseResult struct {
	Name        string
	Version     string
	ModuleRef   string
	Status      string
	StateRef    string
	HasChanges  bool
	Applied     bool
	PlanOnly    bool
	IsDriftCheck bool
	Outputs     map[string]interface{}
	Annotations map[string]string
}

// ListReleasesOptions configures ListReleases.
type ListReleasesOptions struct {
	// All includes failed releases in the output.
	All bool
}

// ReleaseInfo describes a release for listing purposes.
type ReleaseInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	ModuleRef   string            `json:"module_ref"`
	Status      string            `json:"status"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// HistoryOptions configures ReleaseHistory.
type HistoryOptions struct {
	Limit int
}

// InspectReleaseOptions configures InspectRelease.
type InspectReleaseOptions struct {
	// Version selects a specific release version. Empty means latest.
	Version string
}

// StatusOptions configures ReleaseStatus.
type StatusOptions struct {
	ShowOutputs bool
}

// ReleaseStatusResult is returned by ReleaseStatus.
type ReleaseStatusResult struct {
	Name          string                 `json:"name"`
	ModuleRef     string                 `json:"module_ref"`
	Revision      int                    `json:"revision"`
	SemVer        string                 `json:"semver,omitempty"`
	Status        release.Status         `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	ValuesFile    string                 `json:"values_file,omitempty"`
	BackendType   string                 `json:"backend_type,omitempty"`
	BackendConfig map[string]string      `json:"backend_config,omitempty"`
	Values        map[string]interface{} `json:"values,omitempty"`
	Outputs       map[string]interface{} `json:"outputs,omitempty"`
	Annotations   map[string]string      `json:"annotations,omitempty"`
}

// DestroyReleaseResult is returned by DestroyRelease when DryRun is true.
type DestroyReleaseResult struct {
	Name      string
	ModuleRef string
	Revision  int
	WorkDir   string
	DryRun    bool
}

// pushStateParams contains internal parameters for pushing release state.
type pushStateParams struct {
	ReleaseName string
	ModuleRef   string
	Version     string
	Status      state.ReleaseStatus
	MainTF      []byte
	TFState     []byte
	Values      []byte
	Description string
	Annotations map[string]string
}
