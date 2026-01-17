// Package deploy provides module deployment functionality.
package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eunanio/nori/internal/util"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/runtime"
	"github.com/google/go-containerregistry/pkg/name"
	"gopkg.in/yaml.v3"
)

// Deployer handles module deployment operations.
type Deployer struct {
	client         *oci.Client
	runtime        string
	runtimePath    string // Resolved path to runtime binary
	runtimeManager *runtime.Manager
	logger         *slog.Logger
}

// DeployOptions contains options for deploying a module.
type DeployOptions struct {
	// ValuesFile is the path to a values.yaml file.
	ValuesFile string

	// Values are inline values to use.
	Values map[string]interface{}

	// WorkDir is the working directory for deployment.
	WorkDir string

	// AutoApprove automatically approves apply operations.
	AutoApprove bool

	// Parallelism is the number of parallel operations.
	Parallelism int

	// VarFiles are additional var files to pass to Terraform.
	VarFiles []string

	// BackendConfig is additional backend configuration.
	BackendConfig map[string]string

	// Targets are specific resources to target.
	Targets []string

	// Destroy indicates this is a destroy operation.
	Destroy bool

	// PlanOnly only creates a plan, doesn't apply.
	PlanOnly bool

	// Upgrade upgrades the module before deployment.
	Upgrade bool

	// OutputFormat is the output format (text, json).
	OutputFormat string
}

// DeployResult contains the result of a deployment.
type DeployResult struct {
	// ModuleRef is the module reference.
	ModuleRef string

	// WorkDir is the working directory used.
	WorkDir string

	// ModuleDir is the module directory containing terraform files.
	ModuleDir string

	// PlanFile is the path to the generated plan file.
	PlanFile string

	// HasChanges indicates if the plan detected any changes to apply.
	HasChanges bool

	// Applied indicates if changes were applied successfully.
	Applied bool

	// ApplyAttempted indicates if tofu apply was started.
	// Used to determine if terraform state may contain resources even on failure.
	ApplyAttempted bool

	// Outputs contains the Terraform outputs.
	Outputs map[string]interface{}

	// TFState contains the terraform.tfstate content after apply.
	TFState []byte

	// TFStatePath is the path to the terraform.tfstate file.
	TFStatePath string
}

// DeployerOption is a function that configures a Deployer.
type DeployerOption func(*Deployer)

// WithRuntimeManager sets the runtime manager for auto-installation.
func WithRuntimeManager(mgr *runtime.Manager) DeployerOption {
	return func(d *Deployer) {
		d.runtimeManager = mgr
	}
}

// NewDeployer creates a new Deployer.
func NewDeployer(client *oci.Client, runtimeName string, logger *slog.Logger, opts ...DeployerOption) *Deployer {
	if runtimeName == "" {
		runtimeName = "tofu"
	}
	if logger == nil {
		logger = slog.Default()
	}
	d := &Deployer{
		client:  client,
		runtime: runtimeName,
		logger:  logger,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// ResolveRuntime resolves the runtime binary path, optionally auto-installing if missing.
func (d *Deployer) ResolveRuntime(ctx context.Context) (string, error) {
	// If already resolved, return cached path
	if d.runtimePath != "" {
		return d.runtimePath, nil
	}

	// If we have a runtime manager, use it to resolve (with auto-install)
	if d.runtimeManager != nil {
		path, err := d.runtimeManager.GetRuntime(ctx, d.runtime)
		if err != nil {
			return "", fmt.Errorf("failed to get runtime: %w", err)
		}
		d.runtimePath = path
		return path, nil
	}

	// Otherwise, just look in PATH
	path, err := exec.LookPath(d.runtime)
	if err != nil {
		return "", fmt.Errorf("runtime %q not found in PATH: %w", d.runtime, err)
	}
	d.runtimePath = path
	return path, nil
}

// Deploy deploys a module from an OCI artifact.
func (d *Deployer) Deploy(ctx context.Context, reference string, opts DeployOptions) (*DeployResult, error) {
	d.logger.Info("deploying module", "reference", reference, "runtime", d.runtime)

	// Parse reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	// Determine working directory
	workDir := opts.WorkDir
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "nori-deploy-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create work directory: %w", err)
		}
	}

	// Pull and extract the module
	moduleDir := filepath.Join(workDir, "module")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create module directory: %w", err)
	}

	if err := d.pullAndExtract(ctx, ref, moduleDir); err != nil {
		return nil, fmt.Errorf("failed to pull module: %w", err)
	}

	// Load and merge values
	values, err := d.loadValues(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load values: %w", err)
	}

	// Generate terraform.tfvars from values
	if len(values) > 0 {
		if err := d.writeVarFile(moduleDir, values); err != nil {
			return nil, fmt.Errorf("failed to write var file: %w", err)
		}
	}

	// Initialize Terraform
	if err := d.init(ctx, moduleDir, opts); err != nil {
		return nil, fmt.Errorf("terraform init failed: %w", err)
	}

	// Create plan
	planFile := filepath.Join(workDir, "tfplan")
	hasChanges, err := d.plan(ctx, moduleDir, planFile, opts)
	if err != nil {
		return nil, fmt.Errorf("terraform plan failed: %w", err)
	}

	tfStatePath := filepath.Join(moduleDir, "terraform.tfstate")
	result := &DeployResult{
		ModuleRef:   reference,
		WorkDir:     workDir,
		ModuleDir:   moduleDir,
		PlanFile:    planFile,
		TFStatePath: tfStatePath,
		HasChanges:  hasChanges,
		Applied:     false,
	}

	// Apply only if not plan-only AND there are changes to apply
	if !opts.PlanOnly && hasChanges {
		result.ApplyAttempted = true

		applyErr := d.apply(ctx, moduleDir, planFile, opts)

		// Always try to read terraform state after apply attempt (even on failure)
		tfState, readErr := os.ReadFile(tfStatePath)
		if readErr == nil {
			result.TFState = tfState
		}

		if applyErr != nil {
			return result, fmt.Errorf("terraform apply failed: %w", applyErr)
		}
		result.Applied = true

		// Get outputs (only on success)
		outputs, err := d.getOutputs(ctx, moduleDir)
		if err != nil {
			d.logger.Warn("failed to get outputs", "error", err)
		} else {
			result.Outputs = outputs
		}
	}

	return result, nil
}

// pullAndExtract pulls a module artifact and extracts it to a directory.
// The module is extracted to a subdirectory named after the repository for OpenTofu compatibility.
func (d *Deployer) pullAndExtract(ctx context.Context, ref name.Reference, destDir string) error {
	d.logger.Debug("pulling module", "reference", ref.String())

	// Pull the artifact
	_, content, err := d.client.PullArtifact(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to pull artifact: %w", err)
	}

	// Write to temporary file
	tempFile, err := os.CreateTemp("", "nori-module-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Extract to a subdirectory named after the repository for OpenTofu compatibility
	repoName := extractRepoName(ref)
	moduleDir := filepath.Join(destDir, repoName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}

	// Extract the archive
	if err := util.ExtractArchive(tempFile.Name(), moduleDir); err != nil {
		return fmt.Errorf("failed to extract module: %w", err)
	}

	// Normalize module directory structure (handles single subdirectory wrapper)
	if err := util.NormalizeModuleDir(moduleDir); err != nil {
		return fmt.Errorf("failed to normalize module directory: %w", err)
	}

	d.logger.Debug("module extracted", "path", moduleDir)
	return nil
}

// extractRepoName extracts the repository name from an OCI reference.
// e.g., "ghcr.io/org/s3-bucket:v1.0.0" -> "s3-bucket"
func extractRepoName(ref name.Reference) string {
	repo := ref.Context().RepositoryStr()
	parts := strings.Split(repo, "/")
	return parts[len(parts)-1]
}

// loadValues loads values from file and/or inline values.
func (d *Deployer) loadValues(opts DeployOptions) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	// Load from file if specified
	if opts.ValuesFile != "" {
		data, err := os.ReadFile(opts.ValuesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read values file: %w", err)
		}

		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("failed to parse values file: %w", err)
		}
	}

	// Merge inline values (inline values take precedence)
	for k, v := range opts.Values {
		values[k] = v
	}

	return values, nil
}

// writeVarFile writes a terraform.tfvars.json file with the given values.
func (d *Deployer) writeVarFile(moduleDir string, values map[string]interface{}) error {
	// Convert to HCL-compatible format
	var buf bytes.Buffer

	for key, value := range values {
		switch v := value.(type) {
		case string:
			fmt.Fprintf(&buf, "%s = %q\n", key, v)
		case bool:
			fmt.Fprintf(&buf, "%s = %t\n", key, v)
		case int, int64, float64:
			fmt.Fprintf(&buf, "%s = %v\n", key, v)
		case []interface{}:
			fmt.Fprintf(&buf, "%s = %s\n", key, formatHCLValue(v))
		case map[string]interface{}:
			fmt.Fprintf(&buf, "%s = %s\n", key, formatHCLValue(v))
		default:
			fmt.Fprintf(&buf, "%s = %q\n", key, fmt.Sprintf("%v", v))
		}
	}

	varFile := filepath.Join(moduleDir, "terraform.tfvars")
	if err := os.WriteFile(varFile, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write tfvars: %w", err)
	}

	d.logger.Debug("wrote terraform.tfvars", "path", varFile)
	return nil
}

// formatHCLValue formats a value for HCL.
func formatHCLValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int64, float64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, formatHCLValue(item))
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	case map[string]interface{}:
		var parts []string
		for key, val := range v {
			parts = append(parts, fmt.Sprintf("%s = %s", key, formatHCLValue(val)))
		}
		return fmt.Sprintf("{\n  %s\n}", strings.Join(parts, "\n  "))
	default:
		return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
	}
}

// init runs terraform init.
func (d *Deployer) init(ctx context.Context, moduleDir string, opts DeployOptions) error {
	d.logger.Info("initializing terraform")

	args := []string{"init", "-input=false"}

	// Add backend config
	for key, value := range opts.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", key, value))
	}

	if opts.Upgrade {
		args = append(args, "-upgrade")
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// plan runs terraform plan.
// Returns (hasChanges, error) where hasChanges is true if the plan detected changes.
// Uses -detailed-exitcode: exit 0 = no changes, exit 2 = changes present.
func (d *Deployer) plan(ctx context.Context, moduleDir, planFile string, opts DeployOptions) (bool, error) {
	d.logger.Info("creating terraform plan")

	args := []string{"plan", "-input=false", "-detailed-exitcode", "-out=" + planFile}

	if opts.Destroy {
		args = append(args, "-destroy")
	}

	// Add var files
	for _, varFile := range opts.VarFiles {
		args = append(args, "-var-file="+varFile)
	}

	// Add targets
	for _, target := range opts.Targets {
		args = append(args, "-target="+target)
	}

	// Add parallelism
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}

	err := d.runTerraformWithExitCode(ctx, moduleDir, args, os.Stdout, os.Stderr)
	if err != nil {
		// Check if it's an exit code 2 (changes present)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				d.logger.Info("terraform plan detected changes")
				return true, nil // has changes, no error
			}
		}
		return false, err
	}

	// Exit code 0 means no changes
	d.logger.Info("terraform plan: no changes detected")
	return false, nil
}

// runTerraformWithExitCode runs terraform and returns the raw error (including exit codes).
func (d *Deployer) runTerraformWithExitCode(ctx context.Context, workDir string, args []string, stdout, stderr io.Writer) error {
	runtimePath, err := d.ResolveRuntime(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve runtime: %w", err)
	}

	cmd := exec.CommandContext(ctx, runtimePath, args...)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)

	d.logger.Debug("running terraform", "command", runtimePath, "args", args, "dir", workDir)

	return cmd.Run()
}

// apply runs terraform apply.
func (d *Deployer) apply(ctx context.Context, moduleDir, planFile string, opts DeployOptions) error {
	d.logger.Info("applying terraform plan")

	args := []string{"apply", "-input=false"}

	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}

	// Add parallelism
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}

	// Use plan file if it exists
	if planFile != "" && util.FileExists(planFile) {
		args = append(args, planFile)
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// getOutputs retrieves Terraform outputs.
func (d *Deployer) getOutputs(ctx context.Context, moduleDir string) (map[string]interface{}, error) {
	var stdout bytes.Buffer

	args := []string{"output", "-json"}

	if err := d.runTerraform(ctx, moduleDir, args, &stdout, io.Discard); err != nil {
		return nil, err
	}

	var outputs map[string]interface{}
	if err := yaml.Unmarshal(stdout.Bytes(), &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse outputs: %w", err)
	}

	return outputs, nil
}

// runTerraform runs a terraform command.
func (d *Deployer) runTerraform(ctx context.Context, workDir string, args []string, stdout, stderr io.Writer) error {
	// Resolve runtime path (may auto-install if using runtime manager)
	runtimePath, err := d.ResolveRuntime(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve runtime: %w", err)
	}

	cmd := exec.CommandContext(ctx, runtimePath, args...)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)

	d.logger.Debug("running terraform", "command", runtimePath, "args", args, "dir", workDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", d.runtime, args[0], err)
	}

	return nil
}

// Destroy destroys the deployed infrastructure.
func (d *Deployer) Destroy(ctx context.Context, workDir string, opts DeployOptions) error {
	d.logger.Info("destroying infrastructure", "workDir", workDir)

	moduleDir := filepath.Join(workDir, "module")

	args := []string{"destroy", "-input=false"}

	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}

	// Add targets
	for _, target := range opts.Targets {
		args = append(args, "-target="+target)
	}

	// Add parallelism
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// ReleaseDeployOptions contains options for deploying a release.
type ReleaseDeployOptions struct {
	// AutoApprove automatically approves apply operations.
	AutoApprove bool

	// Parallelism is the number of parallel operations.
	Parallelism int

	// VarFiles are additional var files to pass to Terraform.
	VarFiles []string

	// Targets are specific resources to target.
	Targets []string

	// PlanOnly only creates a plan, doesn't apply.
	PlanOnly bool

	// Upgrade upgrades providers during init.
	Upgrade bool

	// Reconfigure forces reconfiguration of the backend.
	Reconfigure bool
}

// DeployRelease deploys or upgrades a release.
func (d *Deployer) DeployRelease(ctx context.Context, rel *release.Release, store *release.Store, opts ReleaseDeployOptions) (*DeployResult, error) {
	d.logger.Info("deploying release", "name", rel.Name, "module", rel.ModuleRef, "version", rel.Version)

	// Parse reference
	ref, err := name.ParseReference(rel.ModuleRef)
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	// Get working directories
	workDir := store.GetWorkDir(rel.Name)
	moduleBaseDir := store.GetModuleDir(rel.Name)

	// Create directories
	if err := os.MkdirAll(moduleBaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create module directory: %w", err)
	}

	// Pull and extract the module
	// Note: pullAndExtract extracts to a subdirectory named after the repository
	if err := d.pullAndExtract(ctx, ref, moduleBaseDir); err != nil {
		return nil, fmt.Errorf("failed to pull module: %w", err)
	}

	// The actual module directory is a subdirectory named after the repository
	repoName := extractRepoName(ref)
	moduleDir := filepath.Join(moduleBaseDir, repoName)

	// Write backend configuration for local state
	if err := d.writeBackendConfig(moduleDir, rel); err != nil {
		return nil, fmt.Errorf("failed to write backend config: %w", err)
	}

	// Generate terraform.tfvars from values
	if len(rel.Values) > 0 {
		if err := d.writeVarFile(moduleDir, rel.Values); err != nil {
			return nil, fmt.Errorf("failed to write var file: %w", err)
		}
	}

	// Initialize Terraform
	if err := d.initRelease(ctx, moduleDir, rel, opts); err != nil {
		return nil, fmt.Errorf("terraform init failed: %w", err)
	}

	// Create plan
	planFile := filepath.Join(workDir, "tfplan")
	planOpts := DeployOptions{
		VarFiles:    opts.VarFiles,
		Targets:     opts.Targets,
		Parallelism: opts.Parallelism,
	}
	hasChanges, err := d.plan(ctx, moduleDir, planFile, planOpts)
	if err != nil {
		return nil, fmt.Errorf("terraform plan failed: %w", err)
	}

	tfStatePath := filepath.Join(moduleDir, "terraform.tfstate")
	result := &DeployResult{
		ModuleRef:   rel.ModuleRef,
		WorkDir:     workDir,
		ModuleDir:   moduleDir,
		PlanFile:    planFile,
		TFStatePath: tfStatePath,
		HasChanges:  hasChanges,
		Applied:     false,
	}

	// Apply only if not plan-only AND there are changes to apply
	if !opts.PlanOnly && hasChanges {
		result.ApplyAttempted = true

		applyOpts := DeployOptions{
			AutoApprove: opts.AutoApprove,
			Parallelism: opts.Parallelism,
		}
		applyErr := d.apply(ctx, moduleDir, planFile, applyOpts)

		// Always try to read terraform state after apply attempt (even on failure)
		// This captures partial state when apply fails mid-way through
		tfState, readErr := os.ReadFile(tfStatePath)
		if readErr == nil {
			result.TFState = tfState
		} else {
			d.logger.Debug("no terraform state file found after apply", "error", readErr)
		}

		if applyErr != nil {
			return result, fmt.Errorf("terraform apply failed: %w", applyErr)
		}
		result.Applied = true

		// Get outputs (only on success)
		outputs, err := d.getOutputs(ctx, moduleDir)
		if err != nil {
			d.logger.Warn("failed to get outputs", "error", err)
		} else {
			result.Outputs = outputs
		}
	}

	return result, nil
}

// writeBackendConfig writes a backend.tf file for local state storage.
func (d *Deployer) writeBackendConfig(moduleDir string, rel *release.Release) error {
	// For local backend, configure state path
	if rel.BackendType == "" || rel.BackendType == "local" {
		backendTF := fmt.Sprintf(`terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}
`)
		backendFile := filepath.Join(moduleDir, "backend_override.tf")
		if err := os.WriteFile(backendFile, []byte(backendTF), 0644); err != nil {
			return fmt.Errorf("failed to write backend config: %w", err)
		}
		d.logger.Debug("wrote backend config", "path", backendFile, "type", "local")
	}
	// For remote backends, the user provides -backend-config flags
	return nil
}

// initRelease runs terraform init for a release.
func (d *Deployer) initRelease(ctx context.Context, moduleDir string, rel *release.Release, opts ReleaseDeployOptions) error {
	d.logger.Info("initializing terraform for release", "name", rel.Name)

	args := []string{"init", "-input=false"}

	// Add backend config from release
	for key, value := range rel.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", key, value))
	}

	if opts.Upgrade {
		args = append(args, "-upgrade")
	}

	if opts.Reconfigure {
		args = append(args, "-reconfigure")
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// initForDestroy runs terraform init before destroy operation.
// This is required when the configuration uses OCI module sources that need to be fetched.
func (d *Deployer) initForDestroy(ctx context.Context, moduleDir string, rel *release.Release, opts ReleaseDeployOptions) error {
	d.logger.Info("initializing terraform for destroy", "name", rel.Name)

	args := []string{"init", "-input=false"}

	// Add backend config from release
	for key, value := range rel.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", key, value))
	}

	if opts.Upgrade {
		args = append(args, "-upgrade")
	}

	if opts.Reconfigure {
		args = append(args, "-reconfigure")
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// DestroyRelease destroys a release's infrastructure.
func (d *Deployer) DestroyRelease(ctx context.Context, rel *release.Release, store *release.Store, opts ReleaseDeployOptions) error {
	d.logger.Info("destroying release", "name", rel.Name)

	// Parse reference to get repo name (same as DeployRelease)
	ref, err := name.ParseReference(rel.ModuleRef)
	if err != nil {
		return fmt.Errorf("invalid module reference: %w", err)
	}

	moduleBaseDir := store.GetModuleDir(rel.Name)

	// Calculate module directory explicitly (same as DeployRelease)
	repoName := extractRepoName(ref)
	moduleDir := filepath.Join(moduleBaseDir, repoName)

	// Verify the directory exists and has terraform files
	if !hasTerraformFiles(moduleDir) {
		return fmt.Errorf("release working directory not found: %s", moduleDir)
	}

	// Initialize terraform before destroy (required for OCI module sources)
	if err := d.initForDestroy(ctx, moduleDir, rel, opts); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	args := []string{"destroy", "-input=false"}

	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}

	// Add targets
	for _, target := range opts.Targets {
		args = append(args, "-target="+target)
	}

	// Add parallelism
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}

	return d.runTerraform(ctx, moduleDir, args, os.Stdout, os.Stderr)
}

// GetReleaseOutputs retrieves outputs for a release.
func (d *Deployer) GetReleaseOutputs(ctx context.Context, store *release.Store, releaseName string) (map[string]interface{}, error) {
	moduleBaseDir := store.GetModuleDir(releaseName)

	// Find the actual module directory (subdirectory with .tf files)
	moduleDir, err := findModuleDir(moduleBaseDir)
	if err != nil {
		return nil, fmt.Errorf("module directory not found: %w", err)
	}

	return d.getOutputs(ctx, moduleDir)
}

// findModuleDir finds the actual module directory containing .tf files.
// The module may be in a subdirectory created during extraction.
func findModuleDir(baseDir string) (string, error) {
	// First check if .tf files exist directly in baseDir
	if hasTerraformFiles(baseDir) {
		return baseDir, nil
	}

	// Look for a subdirectory containing .tf files
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(baseDir, entry.Name())
			if hasTerraformFiles(subDir) {
				return subDir, nil
			}
		}
	}

	return "", fmt.Errorf("no terraform files found in %s", baseDir)
}

// hasTerraformFiles checks if a directory contains .tf files.
func hasTerraformFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tf") {
			return true
		}
	}

	return false
}

