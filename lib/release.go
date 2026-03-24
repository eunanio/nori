package nori

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eunanio/nori/pkg/codegen"
	"github.com/eunanio/nori/pkg/deploy"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/state"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gopkg.in/yaml.v3"
)

// CreateRelease creates a new release by deploying an OCI module.
// The release state is stored as an OCI artifact in the configured state repository.
func (c *Client) CreateRelease(ctx context.Context, releaseName, moduleRef string, opts CreateReleaseOptions) (*ReleaseResult, error) {
	log := c.logger

	stateRepo, err := c.stateRepo()
	if err != nil {
		return nil, err
	}

	stateStore := state.NewStateStore(c.ociClient, log)

	exists, _ := stateStore.ReleaseExists(ctx, stateRepo, releaseName)
	if exists {
		return nil, fmt.Errorf("release %q already exists; use UpgradeRelease to update it", releaseName)
	}

	log.Info("creating release", "name", releaseName, "module", moduleRef)

	// Merge values
	values, valuesYAML, err := c.mergeValues(opts.ValuesFile, opts.Values)
	if err != nil {
		return nil, err
	}

	annotations := opts.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	backendConfig := opts.BackendConfig
	if backendConfig == nil {
		backendConfig = make(map[string]string)
	}

	// Local release
	rel := release.NewRelease(releaseName, moduleRef)
	rel.Values = values
	rel.ValuesFile = opts.ValuesFile
	rel.BackendConfig = backendConfig
	rel.Annotations = annotations

	localStore := release.NewStore("")
	if err := localStore.Save(rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	// Codegen
	mainTF := codegen.GenerateMainTF(
		&codegen.ModuleConfig{Name: releaseName, Source: moduleRef, Values: values},
		nil,
	)

	moduleDir := localStore.GetModuleDir(releaseName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create module directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.tf"), mainTF, 0644); err != nil {
		return nil, fmt.Errorf("failed to write main.tf: %w", err)
	}

	deployer := deploy.NewDeployer(c.ociClient, "tofu", log)

	result, err := deployer.DeployRelease(ctx, rel, localStore, deploy.ReleaseDeployOptions{
		AutoApprove: opts.AutoApprove,
		Parallelism: opts.Parallelism,
		VarFiles:    opts.VarFiles,
		Targets:     opts.Targets,
		PlanOnly:    opts.PlanOnly,
		Upgrade:     opts.Upgrade,
	})

	if err != nil {
		rel.Status = release.StatusFailed
		localStore.Save(rel)

		if result != nil && result.ApplyAttempted && len(result.TFState) > 0 {
			stateRef, pushErr := c.pushReleaseState(ctx, stateStore, stateRepo, pushStateParams{
				ReleaseName: releaseName, ModuleRef: moduleRef, Version: rel.SemVer,
				Status: state.StatusFailed, MainTF: mainTF, TFState: result.TFState,
				Values: valuesYAML, Description: opts.Description, Annotations: annotations,
			})
			if pushErr != nil {
				log.Warn("failed to push failed release state", "error", pushErr)
			} else {
				rel.StateRef = stateRef
				localStore.Save(rel)
			}
		}

		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	if result.Applied {
		rel.Status = release.StatusDeployed
	}
	localStore.Save(rel)

	out := &ReleaseResult{
		Name:        releaseName,
		Version:     rel.SemVer,
		ModuleRef:   moduleRef,
		Status:      string(rel.Status),
		HasChanges:  result.HasChanges,
		Applied:     result.Applied,
		PlanOnly:    opts.PlanOnly,
		Outputs:     result.Outputs,
		Annotations: annotations,
	}

	if !opts.PlanOnly && result.Applied {
		stateRef, pushErr := c.pushReleaseState(ctx, stateStore, stateRepo, pushStateParams{
			ReleaseName: releaseName, ModuleRef: moduleRef, Version: rel.SemVer,
			Status: state.StatusDeployed, MainTF: mainTF, TFState: result.TFState,
			Values: valuesYAML, Description: opts.Description, Annotations: annotations,
		})
		if pushErr != nil {
			log.Warn("failed to push release state", "error", pushErr)
		} else {
			rel.StateRef = stateRef
			localStore.Save(rel)
			out.StateRef = stateRef
		}
	}

	return out, nil
}

// UpgradeRelease upgrades an existing release or runs a drift check.
//
// When called without ModuleRef, Tag, ValuesFile, Values, or ResetValues
// the operation acts as a drift check: it re-applies the current state and
// only bumps the version when changes are detected and applied.
func (c *Client) UpgradeRelease(ctx context.Context, releaseName string, opts UpgradeReleaseOptions) (*ReleaseResult, error) {
	log := c.logger

	stateRepo, err := c.stateRepo()
	if err != nil {
		return nil, err
	}

	stateStore := state.NewStateStore(c.ociClient, log)

	latestVersion, err := stateStore.GetLatestVersion(ctx, stateRepo, releaseName)
	if err != nil {
		return nil, fmt.Errorf("release %q not found in state repository: %w", releaseName, err)
	}

	stateRef := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, latestVersion))
	existingState, err := stateStore.PullState(ctx, stateRef)
	if err != nil {
		return nil, fmt.Errorf("failed to pull release state: %w", err)
	}

	isDriftCheck := opts.ModuleRef == "" && opts.Tag == "" &&
		opts.ValuesFile == "" && len(opts.Values) == 0 && !opts.ResetValues

	// Determine module reference
	moduleChanged := false
	moduleRef := existingState.Metadata.ModuleRef

	if opts.ModuleRef != "" {
		moduleRef = opts.ModuleRef
		moduleChanged = true
	} else if opts.Tag != "" {
		moduleRef = UpdateModuleTag(existingState.Metadata.ModuleRef, opts.Tag)
		moduleChanged = moduleRef != existingState.Metadata.ModuleRef
	}

	// Determine version
	var newVersion string
	if opts.Version != "" {
		newVersion = state.EnsureVPrefix(opts.Version)
	} else if isDriftCheck {
		newVersion = existingState.Metadata.Version
	} else {
		newVersion, err = state.NextVersion(existingState.Metadata.Version, moduleChanged)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate next version: %w", err)
		}
	}

	// Resolve values
	var baseValues map[string]interface{}
	if opts.ResetValues {
		baseValues = make(map[string]interface{})
	} else if opts.ReuseValues || opts.ValuesFile == "" {
		if len(existingState.Values) > 0 {
			if err := yaml.Unmarshal(existingState.Values, &baseValues); err != nil {
				log.Warn("failed to parse existing values", "error", err)
				baseValues = make(map[string]interface{})
			}
		} else {
			baseValues = make(map[string]interface{})
		}
	} else {
		baseValues = make(map[string]interface{})
	}

	var valuesYAML []byte
	if opts.ValuesFile != "" {
		data, err := os.ReadFile(opts.ValuesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read values file: %w", err)
		}
		fileValues := make(map[string]interface{})
		if err := yaml.Unmarshal(data, &fileValues); err != nil {
			return nil, fmt.Errorf("failed to parse values file: %w", err)
		}
		for k, v := range fileValues {
			baseValues[k] = v
		}
		valuesYAML = data
	}

	for k, v := range opts.Values {
		baseValues[k] = v
	}

	if len(opts.Values) > 0 || (opts.ReuseValues && opts.ValuesFile != "") {
		valuesYAML, _ = yaml.Marshal(baseValues)
	} else if valuesYAML == nil && len(existingState.Values) > 0 {
		valuesYAML = existingState.Values
	}

	// Merge annotations
	annotations := make(map[string]string)
	for k, v := range existingState.Metadata.Annotations {
		annotations[k] = v
	}
	for k, v := range opts.Annotations {
		annotations[k] = v
	}

	backendConfig := opts.BackendConfig
	if backendConfig == nil {
		backendConfig = make(map[string]string)
	}

	// Local release
	rel := release.NewRelease(releaseName, moduleRef)
	rel.Values = baseValues
	rel.ValuesFile = opts.ValuesFile
	rel.BackendConfig = backendConfig
	rel.Annotations = annotations
	rel.SemVer = newVersion

	localStore := release.NewStore("")

	// Restore existing terraform state
	if len(existingState.TFState) > 0 {
		moduleDir := localStore.GetModuleDir(releaseName)
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create module directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(moduleDir, "terraform.tfstate"), existingState.TFState, 0644); err != nil {
			log.Warn("failed to restore terraform state", "error", err)
		}
	}

	if err := localStore.Save(rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	mainTF := codegen.GenerateMainTF(
		&codegen.ModuleConfig{Name: releaseName, Source: moduleRef, Values: baseValues},
		nil,
	)

	moduleDir := localStore.GetModuleDir(releaseName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create module directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.tf"), mainTF, 0644); err != nil {
		return nil, fmt.Errorf("failed to write main.tf: %w", err)
	}

	deployer := deploy.NewDeployer(c.ociClient, "tofu", log)

	result, err := deployer.DeployRelease(ctx, rel, localStore, deploy.ReleaseDeployOptions{
		AutoApprove: opts.AutoApprove,
		Parallelism: opts.Parallelism,
		VarFiles:    opts.VarFiles,
		Targets:     opts.Targets,
		PlanOnly:    opts.PlanOnly,
		Upgrade:     opts.UpgradeInit,
		Reconfigure: true,
	})

	if err != nil {
		rel.Status = release.StatusFailed
		localStore.Save(rel)

		if opts.RollbackOnFailure {
			c.attemptRollback(ctx, stateStore, stateRepo, deployer, releaseName, latestVersion, existingState, localStore, opts.Parallelism, &result)
		}

		if result != nil && result.ApplyAttempted && len(result.TFState) > 0 {
			newStateRef, pushErr := c.pushReleaseState(ctx, stateStore, stateRepo, pushStateParams{
				ReleaseName: releaseName, ModuleRef: moduleRef, Version: newVersion,
				Status: state.StatusFailed, MainTF: mainTF, TFState: result.TFState,
				Values: valuesYAML, Description: opts.Description, Annotations: annotations,
			})
			if pushErr != nil {
				log.Warn("failed to push failed release state", "error", pushErr)
			} else {
				rel.StateRef = newStateRef
				localStore.Save(rel)
			}
		}

		return nil, fmt.Errorf("upgrade failed: %w", err)
	}

	if result.Applied {
		rel.Status = release.StatusDeployed
	}
	localStore.Save(rel)

	out := &ReleaseResult{
		Name:         releaseName,
		Version:      newVersion,
		ModuleRef:    moduleRef,
		Status:       string(rel.Status),
		HasChanges:   result.HasChanges,
		Applied:      result.Applied,
		PlanOnly:     opts.PlanOnly,
		IsDriftCheck: isDriftCheck,
		Outputs:      result.Outputs,
		Annotations:  annotations,
	}

	if !opts.PlanOnly && result.Applied {
		if isDriftCheck && opts.Version == "" {
			nv, calcErr := state.NextVersion(existingState.Metadata.Version, false)
			if calcErr != nil {
				log.Warn("failed to calculate next version for drift correction", "error", calcErr)
				nv = existingState.Metadata.Version
			}
			newVersion = nv
			rel.SemVer = newVersion
			out.Version = newVersion
		}

		newStateRef, pushErr := c.pushReleaseState(ctx, stateStore, stateRepo, pushStateParams{
			ReleaseName: releaseName, ModuleRef: moduleRef, Version: newVersion,
			Status: state.StatusDeployed, MainTF: mainTF, TFState: result.TFState,
			Values: valuesYAML, Description: opts.Description, Annotations: annotations,
		})
		if pushErr != nil {
			log.Warn("failed to push release state", "error", pushErr)
		} else {
			rel.StateRef = newStateRef
			localStore.Save(rel)
			out.StateRef = newStateRef
		}
	}

	return out, nil
}

// DestroyRelease destroys a release's infrastructure and optionally removes
// its state from OCI and local storage.
func (c *Client) DestroyRelease(ctx context.Context, releaseName string, opts DestroyReleaseOptions) (*DestroyReleaseResult, error) {
	log := c.logger

	store := release.NewStore("")
	rel, err := store.Get(releaseName)
	if err != nil {
		return nil, fmt.Errorf("release %q not found: %w", releaseName, err)
	}

	log.Info("destroying release", "name", releaseName, "module", rel.ModuleRef)

	if opts.DryRun {
		return &DestroyReleaseResult{
			Name:      releaseName,
			ModuleRef: rel.ModuleRef,
			Revision:  rel.Version,
			WorkDir:   store.GetWorkDir(releaseName),
			DryRun:    true,
		}, nil
	}

	rel.Status = release.StatusDestroying
	store.Save(rel)

	deployer := deploy.NewDeployer(c.ociClient, "tofu", log)

	if err := deployer.DestroyRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
		AutoApprove: opts.AutoApprove,
		Parallelism: opts.Parallelism,
		Targets:     opts.Targets,
	}); err != nil {
		rel.Status = release.StatusFailed
		store.Save(rel)
		return nil, fmt.Errorf("destroy failed: %w", err)
	}

	if !opts.KeepHistory {
		if stateRepo, repoErr := c.stateRepo(); repoErr == nil {
			stateStore := state.NewStateStore(c.ociClient, log)
			if delErr := stateStore.DeleteRelease(ctx, stateRepo, releaseName); delErr != nil {
				log.Warn("failed to delete OCI state", "error", delErr)
			}
		}
		if err := store.Delete(releaseName); err != nil {
			return nil, fmt.Errorf("failed to delete release record: %w", err)
		}
	} else {
		rel.Status = release.StatusFailed
		store.Save(rel)
	}

	return &DestroyReleaseResult{
		Name:      releaseName,
		ModuleRef: rel.ModuleRef,
		Revision:  rel.Version,
		WorkDir:   store.GetWorkDir(releaseName),
	}, nil
}

// ListReleases returns all releases from the configured OCI state repository.
func (c *Client) ListReleases(ctx context.Context, opts ListReleasesOptions) ([]ReleaseInfo, error) {
	stateRepo, err := c.stateRepo()
	if err != nil {
		return nil, err
	}

	releases, err := c.listReleasesFromOCI(ctx, stateRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	if !opts.All {
		filtered := releases[:0]
		for _, r := range releases {
			if r.Status != string(state.StatusFailed) {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
	}

	return releases, nil
}

// ReleaseHistory returns the version history for a named release.
func (c *Client) ReleaseHistory(ctx context.Context, releaseName string, opts HistoryOptions) (*state.ReleaseHistory, error) {
	stateRepo, err := c.stateRepo()
	if err != nil {
		return nil, err
	}

	stateStore := state.NewStateStore(c.ociClient, c.logger)
	history, err := stateStore.GetReleaseHistory(ctx, stateRepo, releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release history: %w", err)
	}

	if opts.Limit > 0 && len(history.Versions) > opts.Limit {
		history.Versions = history.Versions[:opts.Limit]
	}

	return history, nil
}

// InspectRelease returns detailed state for a specific release version.
func (c *Client) InspectRelease(ctx context.Context, releaseName string, opts InspectReleaseOptions) (*state.ReleaseState, error) {
	stateRepo, err := c.stateRepo()
	if err != nil {
		return nil, err
	}

	stateStore := state.NewStateStore(c.ociClient, c.logger)

	version := opts.Version
	if version == "" {
		version, err = stateStore.GetLatestVersion(ctx, stateRepo, releaseName)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	ref := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, version))
	releaseState, err := stateStore.PullState(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to pull release state: %w", err)
	}

	return releaseState, nil
}

// ReleaseStatus returns the current status of a named release from local storage,
// optionally including Terraform outputs.
func (c *Client) ReleaseStatus(ctx context.Context, releaseName string, opts StatusOptions) (*ReleaseStatusResult, error) {
	store := release.NewStore("")
	rel, err := store.Get(releaseName)
	if err != nil {
		return nil, fmt.Errorf("release %q not found: %w", releaseName, err)
	}

	result := &ReleaseStatusResult{
		Name:          rel.Name,
		ModuleRef:     rel.ModuleRef,
		Revision:      rel.Version,
		SemVer:        rel.SemVer,
		Status:        rel.Status,
		CreatedAt:     rel.CreatedAt,
		UpdatedAt:     rel.UpdatedAt,
		ValuesFile:    rel.ValuesFile,
		BackendType:   rel.BackendType,
		BackendConfig: rel.BackendConfig,
		Values:        rel.Values,
		Annotations:   rel.Annotations,
	}

	if opts.ShowOutputs {
		deployer := deploy.NewDeployer(c.ociClient, "tofu", c.logger)
		outputs, err := deployer.GetReleaseOutputs(ctx, store, releaseName)
		if err != nil {
			c.logger.Warn("failed to get outputs", "error", err)
		} else {
			result.Outputs = outputs
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *Client) pushReleaseState(ctx context.Context, stateStore *state.StateStore, stateRepo string, p pushStateParams) (string, error) {
	rs := &state.ReleaseState{
		Metadata: state.NewReleaseMetadata(p.ReleaseName, p.ModuleRef, p.Version),
		MainTF:   p.MainTF,
		TFState:  p.TFState,
		Values:   p.Values,
	}
	rs.Metadata.Status = p.Status
	rs.Metadata.Description = p.Description
	for k, v := range p.Annotations {
		rs.Metadata.SetAnnotation(k, v)
	}

	ref := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(p.ReleaseName, p.Version))
	if err := stateStore.PushState(ctx, ref, rs); err != nil {
		return "", fmt.Errorf("failed to push release state: %w", err)
	}

	c.logger.Info("release state pushed", "reference", ref, "status", p.Status)
	return ref, nil
}

func (c *Client) mergeValues(valuesFile string, inlineValues map[string]interface{}) (map[string]interface{}, []byte, error) {
	values := make(map[string]interface{})
	var valuesYAML []byte

	if valuesFile != "" {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read values file: %w", err)
		}
		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, nil, fmt.Errorf("failed to parse values file: %w", err)
		}
		valuesYAML = data
	}

	for k, v := range inlineValues {
		values[k] = v
	}

	if len(inlineValues) > 0 {
		valuesYAML, _ = yaml.Marshal(values)
	}

	return values, valuesYAML, nil
}

func (c *Client) attemptRollback(
	ctx context.Context,
	stateStore *state.StateStore,
	stateRepo string,
	deployer *deploy.Deployer,
	releaseName, latestVersion string,
	existingState *state.ReleaseState,
	localStore *release.Store,
	parallelism int,
	result **deploy.DeployResult,
) {
	log := c.logger

	prevVersion, findErr := stateStore.GetLastSuccessfulVersion(ctx, stateRepo, releaseName)
	if findErr != nil {
		log.Warn("failed to find previous successful version for rollback", "error", findErr)
		return
	}
	if prevVersion == "" {
		log.Warn("no previous successful version found for rollback")
		return
	}
	if prevVersion == latestVersion && existingState.Metadata.Status == state.StatusDeployed {
		log.Debug("skipping rollback, already at last successful version")
		return
	}

	log.Info("attempting rollback", "target_version", prevVersion)
	prevStateRef := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, prevVersion))
	prevState, pullErr := stateStore.PullState(ctx, prevStateRef)
	if pullErr != nil {
		log.Warn("failed to pull previous state for rollback", "error", pullErr)
		return
	}

	rollbackResult, rollbackErr := deployer.RollbackRelease(ctx, releaseName, prevState, localStore, deploy.ReleaseDeployOptions{
		AutoApprove: true,
		Parallelism: parallelism,
		Reconfigure: true,
	})
	if rollbackErr != nil {
		log.Error("rollback failed", "error", rollbackErr)
		return
	}

	log.Info("rollback successful", "version", prevVersion)
	if rollbackResult != nil && len(rollbackResult.TFState) > 0 {
		*result = rollbackResult
	}
}

func (c *Client) listReleasesFromOCI(ctx context.Context, stateRepo string) ([]ReleaseInfo, error) {
	repo, err := c.ociClient.NewRepository(stateRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	opts, err := c.ociClient.RemoteOptions(ctx, repo.RegistryStr())
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	releaseVersions := make(map[string][]string)
	for _, tag := range tags {
		parts := strings.Split(tag, "-v")
		if len(parts) >= 2 {
			releaseName := parts[0]
			version := "v" + parts[len(parts)-1]
			releaseVersions[releaseName] = append(releaseVersions[releaseName], version)
		}
	}

	stateStore := state.NewStateStore(c.ociClient, c.logger)
	var releases []ReleaseInfo

	for releaseName, versions := range releaseVersions {
		latestVersion, err := state.FindLatestVersion(versions)
		if err != nil {
			continue
		}

		ref := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, latestVersion))

		parsedRef, err := c.ociClient.ParseReference(ref)
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

		info := ReleaseInfo{
			Name:    releaseName,
			Version: latestVersion,
		}

		if manifest.Annotations != nil {
			if moduleRef, ok := manifest.Annotations["io.nori.module.ref"]; ok {
				info.ModuleRef = moduleRef
			}
			if s, ok := manifest.Annotations["io.nori.release.status"]; ok {
				info.Status = s
			}
			if created, ok := manifest.Annotations["org.opencontainers.image.created"]; ok {
				info.UpdatedAt, _ = time.Parse(time.RFC3339, created)
			}
		}

		info.Annotations = make(map[string]string)
		for k, v := range manifest.Annotations {
			if !IsSystemAnnotation(k) {
				info.Annotations[k] = v
			}
		}

		if info.ModuleRef == "" || info.Status == "" {
			fullState, err := stateStore.PullState(ctx, ref)
			if err == nil && fullState.Metadata != nil {
				info.ModuleRef = fullState.Metadata.ModuleRef
				info.Status = string(fullState.Metadata.Status)
				info.UpdatedAt = fullState.Metadata.UpdatedAt
			}
		}

		releases = append(releases, info)
	}

	return releases, nil
}
