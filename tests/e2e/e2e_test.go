//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eunanio/nori/pkg/deploy"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/packaging"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger creates a logger for tests
func testLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// findRuntime finds tofu or terraform in PATH
func findRuntime(t *testing.T) string {
	t.Helper()

	// Try tofu first
	if path, err := exec.LookPath("tofu"); err == nil {
		t.Logf("Found tofu at: %s", path)
		return path
	} else {
		t.Logf("tofu lookup failed: %v", err)
	}

	// Also try tofu.exe explicitly on Windows
	if path, err := exec.LookPath("tofu.exe"); err == nil {
		t.Logf("Found tofu.exe at: %s", path)
		return path
	}

	// Fall back to terraform
	if path, err := exec.LookPath("terraform"); err == nil {
		t.Logf("Found terraform at: %s", path)
		return path
	} else {
		t.Logf("terraform lookup failed: %v", err)
	}

	// Also try terraform.exe explicitly on Windows
	if path, err := exec.LookPath("terraform.exe"); err == nil {
		t.Logf("Found terraform.exe at: %s", path)
		return path
	}

	t.Skip("neither tofu nor terraform found in PATH")
	return ""
}

// getTestdataPath returns the absolute path to testdata directory
func getTestdataPath(t *testing.T) string {
	t.Helper()

	// Get the current working directory and navigate to testdata
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate up to project root from tests/e2e
	projectRoot := filepath.Join(wd, "..", "..")
	testdataPath := filepath.Join(projectRoot, "testdata")

	// Verify the path exists
	_, err = os.Stat(testdataPath)
	require.NoError(t, err, "testdata directory not found at %s", testdataPath)

	return testdataPath
}

// createModuleZip creates a zip archive of a module directory
func createModuleZip(t *testing.T, moduleDir string) string {
	t.Helper()

	// Create a temporary file for the zip
	zipFile, err := os.CreateTemp("", "module-*.zip")
	require.NoError(t, err)
	defer zipFile.Close()

	// Create a zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk the module directory and add files to the zip
	err = filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get the relative path
		relPath, err := filepath.Rel(moduleDir, path)
		if err != nil {
			return err
		}

		// Create file in zip
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
	require.NoError(t, err)

	// Close the zip writer to flush
	err = zipWriter.Close()
	require.NoError(t, err)

	return zipFile.Name()
}

// =============================================================================
// Nori Release Lifecycle Tests
// =============================================================================

func TestE2E_DeployLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	runtimePath := findRuntime(t)
	t.Logf("Using runtime: %s", runtimePath)

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()
	t.Logf("Registry available at: %s", registry.URL)

	testdataPath := getTestdataPath(t)
	moduleDir := filepath.Join(testdataPath, "random-resources")

	// Create a zip of the module
	zipPath := createModuleZip(t, moduleDir)
	defer os.Remove(zipPath)
	t.Logf("Created module zip: %s", zipPath)

	ctx := context.Background()
	logger := testLogger(t)

	// Create OCI client with insecure option for local registry
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)

	packager := packaging.NewPackager(client, logger)

	// Module reference in the test registry
	moduleRef := registryReference(registry.URL, "e2e/random-module", "v1.0.0")

	// Create a temporary release store directory
	storeDir := t.TempDir()
	store := release.NewStore(storeDir)

	var rel *release.Release
	var deployer *deploy.Deployer

	t.Run("package_module", func(t *testing.T) {
		t.Logf("Packaging module and pushing to: %s", moduleRef)

		result, err := packager.Package(ctx, moduleRef, zipPath, packaging.PackageOptions{
			Description: "E2E lifecycle test module with random resources",
			ConfigPath:  moduleDir,
			Annotations: map[string]string{
				"io.nori.test": "e2e-lifecycle",
			},
		})
		require.NoError(t, err, "package and push failed")
		assert.NotEmpty(t, result.Digest)
		t.Logf("Module pushed with digest: %s", result.Digest)
	})

	t.Run("create_release", func(t *testing.T) {
		// Create a new release (equivalent to `nori release create`)
		rel = release.NewRelease("lifecycle-test", moduleRef)
		rel.Values = map[string]interface{}{
			"password_length":  20,
			"password_special": false,
			"string_length":    10,
			"pet_prefix":       "lifecycle",
			"resource_count":   2,
			"tags": map[string]interface{}{
				"Test":        "E2E",
				"Environment": "testing",
			},
		}
		rel.SetAnnotation("created-by", "e2e-test")

		// Save the release to the store
		err := store.Save(rel)
		require.NoError(t, err, "failed to save release")

		// Verify it exists
		assert.True(t, store.Exists("lifecycle-test"))
		assert.Equal(t, release.StatusPending, rel.Status)
		assert.Equal(t, 1, rel.Version)

		t.Logf("Release created: %s (version %d)", rel.Name, rel.Version)
	})

	t.Run("deploy_release", func(t *testing.T) {
		// Create deployer (equivalent to `nori deploy`)
		deployer = deploy.NewDeployer(client, filepath.Base(runtimePath), logger)

		// Deploy the release
		result, err := deployer.DeployRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
			AutoApprove: true,
		})
		require.NoError(t, err, "deploy release failed")

		assert.True(t, result.HasChanges, "expected changes to apply")
		assert.True(t, result.Applied, "expected changes to be applied")
		assert.NotEmpty(t, result.ModuleDir)
		t.Logf("Deploy result - Applied: %v, HasChanges: %v, ModuleDir: %s",
			result.Applied, result.HasChanges, result.ModuleDir)

		// Verify state file was created
		stateFile := filepath.Join(result.ModuleDir, "terraform.tfstate")
		_, err = os.Stat(stateFile)
		require.NoError(t, err, "state file not created")

		// Read and verify state has resources
		stateContent, err := os.ReadFile(stateFile)
		require.NoError(t, err)
		assert.Contains(t, string(stateContent), "random_password")
		assert.Contains(t, string(stateContent), "random_pet")
		assert.Contains(t, string(stateContent), "random_uuid")

		// Update release status to deployed
		rel.Status = release.StatusDeployed
		rel.UpdatedAt = time.Now()
		err = store.Save(rel)
		require.NoError(t, err)
	})

	t.Run("verify_outputs", func(t *testing.T) {
		// Get outputs from the deployed release
		outputs, err := deployer.GetReleaseOutputs(ctx, store, "lifecycle-test")
		require.NoError(t, err, "failed to get outputs")

		t.Logf("Release outputs: %v", outputs)

		// Verify expected outputs exist
		assert.Contains(t, outputs, "pet_names")
		assert.Contains(t, outputs, "uuids")
		assert.Contains(t, outputs, "strings")
		assert.Contains(t, outputs, "resource_count")
	})

	t.Run("list_releases", func(t *testing.T) {
		// List all releases
		releases, err := store.List()
		require.NoError(t, err)

		assert.Len(t, releases, 1)
		assert.Equal(t, "lifecycle-test", releases[0].Name)
		assert.Equal(t, release.StatusDeployed, releases[0].Status)
	})

	t.Run("upgrade_release", func(t *testing.T) {
		// Push a new version of the module
		moduleRefV2 := registryReference(registry.URL, "e2e/random-module", "v2.0.0")
		_, err := packager.Package(ctx, moduleRefV2, zipPath, packaging.PackageOptions{
			Description: "E2E lifecycle test module v2",
		})
		require.NoError(t, err)

		// Upgrade the release with new values
		newValues := map[string]interface{}{
			"password_length":  32,
			"password_special": true,
			"string_length":    16,
			"pet_prefix":       "upgraded",
			"resource_count":   3,
		}
		rel.UpdateForUpgrade(moduleRefV2, newValues)

		// Save the upgraded release
		err = store.Save(rel)
		require.NoError(t, err)

		assert.Equal(t, 2, rel.Version)
		assert.Equal(t, moduleRefV2, rel.ModuleRef)
		assert.Equal(t, release.StatusPending, rel.Status)

		// Deploy the upgrade
		result, err := deployer.DeployRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
			AutoApprove: true,
			Upgrade:     true,
		})
		require.NoError(t, err, "upgrade deploy failed")

		t.Logf("Upgrade result - Applied: %v, HasChanges: %v", result.Applied, result.HasChanges)

		// Update status
		rel.Status = release.StatusDeployed
		rel.UpdatedAt = time.Now()
		err = store.Save(rel)
		require.NoError(t, err)
	})

	t.Run("destroy_release", func(t *testing.T) {
		// Destroy the release (equivalent to `nori release destroy`)
		err := deployer.DestroyRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
			AutoApprove: true,
		})
		require.NoError(t, err, "destroy release failed")

		t.Logf("Release destroyed successfully")
	})

	t.Run("delete_release", func(t *testing.T) {
		// Delete the release from the store
		err := store.Delete("lifecycle-test")
		require.NoError(t, err, "delete release failed")

		// Verify it's gone
		assert.False(t, store.Exists("lifecycle-test"))

		// List should be empty
		releases, err := store.List()
		require.NoError(t, err)
		assert.Empty(t, releases)
	})
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestE2E_DeployBrokenModule tests that deploying a module with errors fails appropriately.
func TestE2E_DeployBrokenModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	runtimePath := findRuntime(t)
	t.Logf("Using runtime: %s", runtimePath)

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()
	t.Logf("Registry available at: %s", registry.URL)

	testdataPath := getTestdataPath(t)
	brokenModuleDir := filepath.Join(testdataPath, "broken-module")

	// Create a zip of the broken module
	zipPath := createModuleZip(t, brokenModuleDir)
	defer os.Remove(zipPath)
	t.Logf("Created broken module zip: %s", zipPath)

	ctx := context.Background()
	logger := testLogger(t)

	// Create OCI client
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)
	packager := packaging.NewPackager(client, logger)
	deployer := deploy.NewDeployer(client, filepath.Base(runtimePath), logger)

	releaseName := "broken-module-test"
	moduleRepo := "e2e/broken-module"
	moduleRef := registryReference(registry.URL, moduleRepo, "v1.0.0")

	t.Run("package_broken_module", func(t *testing.T) {
		t.Logf("Packaging broken module and pushing to: %s", moduleRef)
		result, err := packager.Package(ctx, moduleRef, zipPath, packaging.PackageOptions{
			Description: "E2E broken module test",
			ConfigPath:  brokenModuleDir,
		})
		// Packaging should succeed - the module is syntactically valid HCL
		require.NoError(t, err, "package and push failed")
		assert.NotEmpty(t, result.Digest)
		t.Logf("Broken module pushed with digest: %s", result.Digest)
	})

	// Create a temporary release store
	storeDir := t.TempDir()
	store := release.NewStore(storeDir)

	var rel *release.Release

	t.Run("create_release", func(t *testing.T) {
		rel = release.NewRelease(releaseName, moduleRef)
		// Intentionally NOT providing required 'name' variable
		rel.Values = map[string]interface{}{
			"count_value": 1,
		}
		err := store.Save(rel)
		require.NoError(t, err)
		t.Logf("Release created: %s", rel.Name)
	})

	t.Run("deploy_fails_with_validation_errors", func(t *testing.T) {
		result, err := deployer.DeployRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
			AutoApprove: true,
		})

		// The deployment should fail due to Terraform validation errors
		require.Error(t, err, "expected deployment to fail with broken module")
		t.Logf("Deploy failed as expected with error: %v", err)

		// Result may be nil or partially populated
		if result != nil {
			assert.False(t, result.Applied, "should not have applied changes")
		}
	})

	t.Run("release_status_not_deployed", func(t *testing.T) {
		// Reload release from store
		loadedRel, err := store.Get(releaseName)
		require.NoError(t, err)

		// Status should NOT be deployed since deployment failed
		assert.NotEqual(t, release.StatusDeployed, loadedRel.Status,
			"release should not be marked as deployed after failure")
		t.Logf("Release status after failed deploy: %s", loadedRel.Status)
	})

	t.Run("cleanup_failed_release", func(t *testing.T) {
		// Clean up the release from the store
		err := store.Delete(releaseName)
		require.NoError(t, err)
		assert.False(t, store.Exists(releaseName))
	})
}

// =============================================================================
// OCI Package/Push/Pull Tests
// =============================================================================

func TestE2E_PackagePushPull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()

	t.Logf("Registry available at: %s", registry.URL)

	testdataPath := getTestdataPath(t)
	moduleDir := filepath.Join(testdataPath, "random-resources")

	// Create a zip of the module
	zipPath := createModuleZip(t, moduleDir)
	defer os.Remove(zipPath)
	t.Logf("Created module zip: %s", zipPath)

	ctx := context.Background()
	logger := testLogger(t)

	// Create OCI client with insecure option for local registry
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)

	packager := packaging.NewPackager(client, logger)

	t.Run("package_and_push", func(t *testing.T) {
		ref := registryReference(registry.URL, "test/random-module", "v1.0.0")
		t.Logf("Pushing to: %s", ref)

		result, err := packager.Package(ctx, ref, zipPath, packaging.PackageOptions{
			Description: "E2E test module with random resources",
			ConfigPath:  moduleDir,
			Annotations: map[string]string{
				"test.annotation": "e2e-test",
			},
		})
		require.NoError(t, err, "package and push failed")

		assert.NotEmpty(t, result.Digest)
		assert.Equal(t, ref, result.Reference)
		t.Logf("Pushed artifact with digest: %s", result.Digest)
	})

	t.Run("pull_and_verify", func(t *testing.T) {
		ref := registryReference(registry.URL, "test/random-module", "v1.0.0")

		parsedRef, err := oci.ParseReference(ref)
		require.NoError(t, err)

		artifact, content, err := client.PullArtifact(ctx, parsedRef)
		require.NoError(t, err, "pull failed")

		assert.NotEmpty(t, content)
		assert.NotEmpty(t, artifact.Digest)
		assert.Equal(t, 1, len(artifact.Layers))

		// Verify it's a valid zip
		zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		require.NoError(t, err, "pulled content is not a valid zip")

		// Check for expected files
		var foundMainTF, foundVariablesTF bool
		for _, f := range zipReader.File {
			t.Logf("Zip contains: %s", f.Name)
			if strings.HasSuffix(f.Name, "main.tf") {
				foundMainTF = true
			}
			if strings.HasSuffix(f.Name, "variables.tf") {
				foundVariablesTF = true
			}
		}
		assert.True(t, foundMainTF, "main.tf not found in pulled artifact")
		assert.True(t, foundVariablesTF, "variables.tf not found in pulled artifact")
	})

	t.Run("inspect_artifact", func(t *testing.T) {
		ref := registryReference(registry.URL, "test/random-module", "v1.0.0")

		parsedRef, err := oci.ParseReference(ref)
		require.NoError(t, err)

		artifact, err := client.InspectArtifact(ctx, parsedRef)
		require.NoError(t, err, "inspect failed")

		assert.NotEmpty(t, artifact.Digest)
		assert.NotEmpty(t, artifact.Annotations)
		assert.Contains(t, artifact.Annotations["test.annotation"], "e2e-test")
	})

	t.Run("list_tags", func(t *testing.T) {
		// Push another version
		ref := registryReference(registry.URL, "test/random-module", "v1.1.0")

		_, err := packager.Package(ctx, ref, zipPath, packaging.PackageOptions{
			Description: "E2E test module v1.1.0",
		})
		require.NoError(t, err)

		// List tags
		repo, err := oci.ParseReference(registryReference(registry.URL, "test/random-module", "latest"))
		require.NoError(t, err)

		tags, err := client.ListTags(ctx, repo.Context())
		require.NoError(t, err)

		t.Logf("Tags: %v", tags)
		assert.Contains(t, tags, "v1.0.0")
		assert.Contains(t, tags, "v1.1.0")
	})

	t.Run("delete_artifact", func(t *testing.T) {
		ref := registryReference(registry.URL, "test/random-module", "v1.1.0")

		parsedRef, err := oci.ParseReference(ref)
		require.NoError(t, err)

		// Docker registry requires deleting by digest, not by tag
		// First, get the digest of the artifact
		artifact, err := client.InspectArtifact(ctx, parsedRef)
		require.NoError(t, err, "failed to get artifact digest")

		// Create a digest reference for deletion
		digestRef, err := oci.ReferenceWithDigest(parsedRef, artifact.Digest)
		require.NoError(t, err, "failed to create digest reference")

		err = client.DeleteArtifact(ctx, digestRef)
		require.NoError(t, err, "delete failed")

		// Verify it's deleted by trying to inspect (should fail)
		_, err = client.InspectArtifact(ctx, parsedRef)
		assert.Error(t, err, "artifact should be deleted")
	})
}

// =============================================================================
// State Management Tests
// =============================================================================

func TestE2E_StateManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Start the registry container
	registry, cleanup := setupRegistry(t)
	defer cleanup()

	t.Logf("Registry available at: %s", registry.URL)

	ctx := context.Background()
	logger := testLogger(t)

	// Create OCI client
	client := oci.NewClient(
		oci.WithInsecure(true),
		oci.WithLogger(logger),
	)

	stateStore := state.NewStateStore(client, logger)

	t.Run("push_state", func(t *testing.T) {
		releaseName := "test-release"
		moduleRef := registryReference(registry.URL, "test/random-module", "v1.0.0")
		stateRef := registryReference(registry.URL, "test/state", fmt.Sprintf("%s-v1.0.0", releaseName))

		// Create release metadata
		metadata := state.NewReleaseMetadata(releaseName, moduleRef, "v1.0.0")
		metadata.Status = state.StatusDeployed
		metadata.SetAnnotation("test.key", "test.value")

		// Create sample terraform state
		tfState := []byte(`{
			"version": 4,
			"terraform_version": "1.6.0",
			"serial": 1,
			"lineage": "test-lineage",
			"resources": []
		}`)

		// Create sample values
		values := []byte(`password_length: 20
resource_count: 2
`)

		releaseState := &state.ReleaseState{
			Metadata: metadata,
			MainTF:   []byte(`module "test" { source = "oci://test" }`),
			TFState:  tfState,
			Values:   values,
		}

		err := stateStore.PushState(ctx, stateRef, releaseState)
		require.NoError(t, err, "push state failed")
	})

	t.Run("pull_state", func(t *testing.T) {
		stateRef := registryReference(registry.URL, "test/state", "test-release-v1.0.0")

		pulledState, err := stateStore.PullState(ctx, stateRef)
		require.NoError(t, err, "pull state failed")

		assert.Equal(t, "test-release", pulledState.Metadata.Name)
		assert.Equal(t, "v1.0.0", pulledState.Metadata.Version)
		assert.Equal(t, state.StatusDeployed, pulledState.Metadata.Status)

		val, ok := pulledState.Metadata.GetAnnotation("test.key")
		assert.True(t, ok)
		assert.Equal(t, "test.value", val)

		assert.NotEmpty(t, pulledState.TFState)
		assert.Contains(t, string(pulledState.TFState), "terraform_version")

		assert.NotEmpty(t, pulledState.Values)
		assert.Contains(t, string(pulledState.Values), "password_length")
	})

	t.Run("push_multiple_versions", func(t *testing.T) {
		releaseName := "versioned-release"
		moduleRef := registryReference(registry.URL, "test/random-module", "v1.0.0")

		// Push v1.0.0
		metadata1 := state.NewReleaseMetadata(releaseName, moduleRef, "v1.0.0")
		metadata1.Status = state.StatusSuperseded

		err := stateStore.PushState(ctx, registryReference(registry.URL, "test/state", fmt.Sprintf("%s-v1.0.0", releaseName)), &state.ReleaseState{
			Metadata: metadata1,
			TFState:  []byte(`{"version": 4, "serial": 1}`),
		})
		require.NoError(t, err)

		// Push v1.1.0
		metadata2 := state.NewReleaseMetadata(releaseName, moduleRef, "v1.1.0")
		metadata2.Status = state.StatusDeployed

		err = stateStore.PushState(ctx, registryReference(registry.URL, "test/state", fmt.Sprintf("%s-v1.1.0", releaseName)), &state.ReleaseState{
			Metadata: metadata2,
			TFState:  []byte(`{"version": 4, "serial": 2}`),
		})
		require.NoError(t, err)
	})

	t.Run("list_releases", func(t *testing.T) {
		stateRepo := fmt.Sprintf("%s/test/state", registry.URL)

		tags, err := stateStore.ListReleases(ctx, stateRepo)
		require.NoError(t, err)

		t.Logf("State tags: %v", tags)
		assert.GreaterOrEqual(t, len(tags), 2, "expected at least 2 release versions")
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		dstFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
