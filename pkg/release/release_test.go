package release

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRelease(t *testing.T) {
	name := "test-release"
	moduleRef := "ghcr.io/myorg/module:v1.0.0"

	rel := NewRelease(name, moduleRef)

	assert.Equal(t, name, rel.Name)
	assert.Equal(t, moduleRef, rel.ModuleRef)
	assert.Equal(t, 1, rel.Version)
	assert.Equal(t, InitialSemVer, rel.SemVer)
	assert.Equal(t, StatusPending, rel.Status)
	assert.Equal(t, "local", rel.BackendType)
	assert.NotNil(t, rel.Annotations)
	assert.False(t, rel.CreatedAt.IsZero(), "CreatedAt should not be zero")
	assert.False(t, rel.UpdatedAt.IsZero(), "UpdatedAt should not be zero")
}

func TestRelease_UpdateForUpgrade(t *testing.T) {
	rel := NewRelease("test", "ghcr.io/myorg/module:v1.0.0")
	originalVersion := rel.Version
	originalUpdatedAt := rel.UpdatedAt

	// Small delay to ensure UpdatedAt changes
	time.Sleep(10 * time.Millisecond)

	newModuleRef := "ghcr.io/myorg/module:v2.0.0"
	newValues := map[string]interface{}{"key": "value"}

	rel.UpdateForUpgrade(newModuleRef, newValues)

	assert.Equal(t, originalVersion+1, rel.Version)
	assert.Equal(t, newModuleRef, rel.ModuleRef)
	assert.Equal(t, StatusPending, rel.Status)
	assert.Equal(t, "value", rel.Values["key"])
	assert.True(t, rel.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")
}

func TestRelease_UpdateForUpgrade_EmptyModuleRef(t *testing.T) {
	rel := NewRelease("test", "ghcr.io/myorg/module:v1.0.0")
	originalModuleRef := rel.ModuleRef

	rel.UpdateForUpgrade("", nil)

	assert.Equal(t, originalModuleRef, rel.ModuleRef, "should keep original moduleRef when empty")
}

func TestRelease_SetAnnotation(t *testing.T) {
	rel := NewRelease("test", "ghcr.io/myorg/module:v1.0.0")

	rel.SetAnnotation("key1", "value1")
	rel.SetAnnotation("key2", "value2")

	assert.Equal(t, "value1", rel.Annotations["key1"])
	assert.Equal(t, "value2", rel.Annotations["key2"])
}

func TestRelease_SetAnnotation_NilMap(t *testing.T) {
	rel := &Release{Name: "test"}
	rel.Annotations = nil

	rel.SetAnnotation("key", "value")

	require.NotNil(t, rel.Annotations, "Annotations map should be initialized")
	assert.Equal(t, "value", rel.Annotations["key"])
}

func TestRelease_GetAnnotation(t *testing.T) {
	rel := NewRelease("test", "ghcr.io/myorg/module:v1.0.0")
	rel.SetAnnotation("existing", "value")

	tests := []struct {
		name    string
		key     string
		wantVal string
		wantOk  bool
	}{
		{
			name:    "existing key",
			key:     "existing",
			wantVal: "value",
			wantOk:  true,
		},
		{
			name:    "non-existing key",
			key:     "nonexistent",
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := rel.GetAnnotation(tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

func TestRelease_GetAnnotation_NilMap(t *testing.T) {
	rel := &Release{Name: "test"}
	rel.Annotations = nil

	val, ok := rel.GetAnnotation("any")

	assert.Empty(t, val, "should return empty string for nil map")
	assert.False(t, ok, "should return false for nil map")
}

func TestStore_SaveAndGet(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	rel := NewRelease("my-release", "ghcr.io/myorg/module:v1.0.0")
	rel.Values = map[string]interface{}{"bucket_name": "test-bucket"}
	rel.SetAnnotation("env", "production")

	// Save
	err := store.Save(rel)
	require.NoError(t, err, "Save should not error")

	// Get
	loaded, err := store.Get("my-release")
	require.NoError(t, err, "Get should not error")

	// Verify fields
	assert.Equal(t, rel.Name, loaded.Name)
	assert.Equal(t, rel.ModuleRef, loaded.ModuleRef)
	assert.Equal(t, rel.SemVer, loaded.SemVer)
	assert.Equal(t, rel.Status, loaded.Status)
	assert.Equal(t, "test-bucket", loaded.Values["bucket_name"])
	assert.Equal(t, "production", loaded.Annotations["env"])
}

func TestStore_Get_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	_, err := store.Get("nonexistent")

	assert.Error(t, err, "Get on nonexistent release should error")
	assert.Contains(t, err.Error(), "not found")
}

func TestStore_Exists(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	rel := NewRelease("existing", "ghcr.io/myorg/module:v1.0.0")
	err := store.Save(rel)
	require.NoError(t, err)

	tests := []struct {
		name        string
		releaseName string
		want        bool
	}{
		{
			name:        "existing release",
			releaseName: "existing",
			want:        true,
		},
		{
			name:        "nonexistent release",
			releaseName: "nonexistent",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, store.Exists(tt.releaseName))
		})
	}
}

func TestStore_Delete(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	rel := NewRelease("to-delete", "ghcr.io/myorg/module:v1.0.0")
	err := store.Save(rel)
	require.NoError(t, err)

	// Verify it exists
	assert.True(t, store.Exists("to-delete"), "Release should exist before delete")

	// Delete
	err = store.Delete("to-delete")
	require.NoError(t, err, "Delete should not error")

	// Verify it no longer exists
	assert.False(t, store.Exists("to-delete"), "Release should not exist after delete")

	// Verify directory is removed
	workDir := store.GetWorkDir("to-delete")
	_, err = os.Stat(workDir)
	assert.True(t, os.IsNotExist(err), "Work directory should be removed")
}

func TestStore_Delete_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	// Deleting nonexistent release should not error
	err := store.Delete("nonexistent")
	assert.NoError(t, err, "Delete on nonexistent should not error")
}

func TestStore_List(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	// Create multiple releases
	releases := []string{"charlie", "alpha", "bravo"}
	for _, name := range releases {
		rel := NewRelease(name, "ghcr.io/myorg/module:v1.0.0")
		err := store.Save(rel)
		require.NoError(t, err)
	}

	// List
	list, err := store.List()
	require.NoError(t, err)

	assert.Len(t, list, 3, "should return 3 releases")

	// Verify sorted order
	expectedOrder := []string{"alpha", "bravo", "charlie"}
	for i, expected := range expectedOrder {
		assert.Equal(t, expected, list[i].Name, "releases should be sorted")
	}
}

func TestStore_List_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	list, err := store.List()
	require.NoError(t, err)

	assert.Empty(t, list, "empty store should return empty list")
}

func TestStore_List_NonExistentDir(t *testing.T) {
	store := NewStore("/nonexistent/path/that/does/not/exist")

	list, err := store.List()
	require.NoError(t, err)

	assert.Empty(t, list, "nonexistent dir should return empty list")
}

func TestStore_GetWorkDir(t *testing.T) {
	store := NewStore("/base/dir")

	got := store.GetWorkDir("my-release")
	want := filepath.Join("/base/dir", "my-release")

	assert.Equal(t, want, got)
}

func TestStore_GetModuleDir(t *testing.T) {
	store := NewStore("/base/dir")

	got := store.GetModuleDir("my-release")
	want := filepath.Join("/base/dir", "my-release", "module")

	assert.Equal(t, want, got)
}

func TestNewStore_DefaultDir(t *testing.T) {
	store := NewStore("")

	// Should use default releases dir
	assert.NotEmpty(t, store.baseDir, "should use default dir when empty")
}

func TestDefaultReleasesDir(t *testing.T) {
	dir := DefaultReleasesDir()

	if dir == "" {
		t.Skip("Could not determine home directory")
	}

	assert.True(t, filepath.IsAbs(dir), "should be absolute path")
	assert.Contains(t, dir, ".nori", "should contain .nori")
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("pending"), StatusPending)
	assert.Equal(t, Status("deployed"), StatusDeployed)
	assert.Equal(t, Status("failed"), StatusFailed)
	assert.Equal(t, Status("destroying"), StatusDestroying)
}

func TestInitialSemVer(t *testing.T) {
	assert.Equal(t, "v1.0.0", InitialSemVer)
}
