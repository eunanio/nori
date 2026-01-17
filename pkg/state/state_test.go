package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReleaseMetadata(t *testing.T) {
	name := "test-release"
	moduleRef := "ghcr.io/myorg/module:v1.0.0"
	version := "v1.0.0"

	metadata := NewReleaseMetadata(name, moduleRef, version)

	assert.Equal(t, name, metadata.Name)
	assert.Equal(t, moduleRef, metadata.ModuleRef)
	assert.Equal(t, version, metadata.Version)
	assert.Equal(t, StatusPending, metadata.Status)
	assert.NotNil(t, metadata.Annotations, "Annotations should be initialized")
	assert.False(t, metadata.CreatedAt.IsZero(), "CreatedAt should not be zero")
	assert.False(t, metadata.UpdatedAt.IsZero(), "UpdatedAt should not be zero")
}

func TestNewReleaseMetadata_UTCTime(t *testing.T) {
	metadata := NewReleaseMetadata("test", "ghcr.io/org/mod:v1", "v1.0.0")

	// Verify times are in UTC
	assert.Equal(t, time.UTC, metadata.CreatedAt.Location(), "CreatedAt should be in UTC")
	assert.Equal(t, time.UTC, metadata.UpdatedAt.Location(), "UpdatedAt should be in UTC")
}

func TestReleaseMetadata_SetAnnotation(t *testing.T) {
	metadata := NewReleaseMetadata("test", "ghcr.io/org/mod:v1", "v1.0.0")

	metadata.SetAnnotation("key1", "value1")
	metadata.SetAnnotation("key2", "value2")

	assert.Equal(t, "value1", metadata.Annotations["key1"])
	assert.Equal(t, "value2", metadata.Annotations["key2"])
}

func TestReleaseMetadata_SetAnnotation_Overwrite(t *testing.T) {
	metadata := NewReleaseMetadata("test", "ghcr.io/org/mod:v1", "v1.0.0")

	metadata.SetAnnotation("key", "value1")
	metadata.SetAnnotation("key", "value2")

	assert.Equal(t, "value2", metadata.Annotations["key"], "should overwrite existing value")
}

func TestReleaseMetadata_SetAnnotation_NilMap(t *testing.T) {
	metadata := &ReleaseMetadata{Name: "test"}
	metadata.Annotations = nil

	metadata.SetAnnotation("key", "value")

	require.NotNil(t, metadata.Annotations, "should initialize Annotations map")
	assert.Equal(t, "value", metadata.Annotations["key"])
}

func TestReleaseMetadata_GetAnnotation(t *testing.T) {
	metadata := NewReleaseMetadata("test", "ghcr.io/org/mod:v1", "v1.0.0")
	metadata.SetAnnotation("existing", "value")

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
		{
			name:    "empty key",
			key:     "",
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := metadata.GetAnnotation(tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

func TestReleaseMetadata_GetAnnotation_NilMap(t *testing.T) {
	metadata := &ReleaseMetadata{Name: "test"}
	metadata.Annotations = nil

	val, ok := metadata.GetAnnotation("any")

	assert.Empty(t, val, "should return empty string for nil map")
	assert.False(t, ok, "should return false for nil map")
}

func TestReleaseStatus_Constants(t *testing.T) {
	assert.Equal(t, ReleaseStatus("pending"), StatusPending)
	assert.Equal(t, ReleaseStatus("deployed"), StatusDeployed)
	assert.Equal(t, ReleaseStatus("failed"), StatusFailed)
	assert.Equal(t, ReleaseStatus("superseded"), StatusSuperseded)
}

func TestMediaType_Constants(t *testing.T) {
	assert.Equal(t, "application/vnd.nori.release.metadata+yaml", MediaTypeReleaseMetadata)
	assert.Equal(t, "application/vnd.nori.release.main-tf", MediaTypeMainTF)
	assert.Equal(t, "application/vnd.nori.release.tfstate+json", MediaTypeTFState)
	assert.Equal(t, "application/vnd.nori.release.values+yaml", MediaTypeValues)
	assert.Equal(t, "application/vnd.nori.release.state", ArtifactTypeReleaseState)
}

func TestReleaseState_Struct(t *testing.T) {
	metadata := NewReleaseMetadata("test", "ghcr.io/org/mod:v1", "v1.0.0")
	state := &ReleaseState{
		Metadata: metadata,
		MainTF:   []byte("# main.tf content"),
		TFState:  []byte(`{"version": 4}`),
		Values:   []byte("key: value"),
	}

	assert.Same(t, metadata, state.Metadata)
	assert.Equal(t, "# main.tf content", string(state.MainTF))
	assert.Equal(t, `{"version": 4}`, string(state.TFState))
	assert.Equal(t, "key: value", string(state.Values))
}

func TestReleaseHistory_Struct(t *testing.T) {
	now := time.Now()
	history := &ReleaseHistory{
		Name: "my-release",
		Versions: []ReleaseVersion{
			{
				Version:   "v1.1.0",
				Tag:       "my-release-v1.1.0",
				CreatedAt: now,
				Status:    StatusDeployed,
				ModuleRef: "ghcr.io/org/mod:v1.1.0",
			},
			{
				Version:   "v1.0.0",
				Tag:       "my-release-v1.0.0",
				CreatedAt: now.Add(-24 * time.Hour),
				Status:    StatusSuperseded,
				ModuleRef: "ghcr.io/org/mod:v1.0.0",
			},
		},
	}

	assert.Equal(t, "my-release", history.Name)
	assert.Len(t, history.Versions, 2)
	assert.Equal(t, "v1.1.0", history.Versions[0].Version)
	assert.Equal(t, StatusDeployed, history.Versions[0].Status)
	assert.Equal(t, StatusSuperseded, history.Versions[1].Status)
}

func TestReleaseVersion_Struct(t *testing.T) {
	now := time.Now()
	rv := ReleaseVersion{
		Version:     "v1.0.0",
		Tag:         "my-release-v1.0.0",
		CreatedAt:   now,
		Status:      StatusDeployed,
		ModuleRef:   "ghcr.io/org/mod:v1.0.0",
		Annotations: map[string]string{"env": "prod"},
	}

	assert.Equal(t, "v1.0.0", rv.Version)
	assert.Equal(t, "my-release-v1.0.0", rv.Tag)
	assert.Equal(t, StatusDeployed, rv.Status)
	assert.Equal(t, "ghcr.io/org/mod:v1.0.0", rv.ModuleRef)
	assert.Equal(t, "prod", rv.Annotations["env"])
	assert.WithinDuration(t, now, rv.CreatedAt, time.Second)
}
