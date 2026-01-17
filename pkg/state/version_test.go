package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "valid semver",
			version: "v1.0.0",
			wantErr: false,
		},
		{
			name:    "valid with patch",
			version: "v1.2.3",
			wantErr: false,
		},
		{
			name:    "valid high numbers",
			version: "v10.20.30",
			wantErr: false,
		},
		{
			name:    "valid with prerelease",
			version: "v1.0.0-alpha",
			wantErr: false,
		},
		{
			name:    "valid with build metadata",
			version: "v1.0.0+build123",
			wantErr: false,
		},
		{
			name:    "missing v prefix",
			version: "1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid format",
			version: "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			version: "",
			wantErr: true,
		},
		{
			name:    "only v",
			version: "v",
			wantErr: true,
		},
		{
			name:    "short form (valid - normalizes to v1.0.0)",
			version: "v1.0",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err, "ValidateVersion(%q) should error", tt.version)
			} else {
				assert.NoError(t, err, "ValidateVersion(%q) should not error", tt.version)
			}
		})
	}
}

func TestBumpPatch(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "basic bump",
			version: "v1.0.0",
			want:    "v1.0.1",
		},
		{
			name:    "bump from higher patch",
			version: "v1.0.9",
			want:    "v1.0.10",
		},
		{
			name:    "preserves major and minor",
			version: "v2.5.3",
			want:    "v2.5.4",
		},
		{
			name:    "invalid version",
			version: "invalid",
			wantErr: true,
		},
		{
			name:    "missing v prefix",
			version: "1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BumpPatch(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBumpMinor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "basic bump",
			version: "v1.0.0",
			want:    "v1.1.0",
		},
		{
			name:    "resets patch",
			version: "v1.0.5",
			want:    "v1.1.0",
		},
		{
			name:    "bump from higher minor",
			version: "v1.9.3",
			want:    "v1.10.0",
		},
		{
			name:    "preserves major",
			version: "v3.2.1",
			want:    "v3.3.0",
		},
		{
			name:    "invalid version",
			version: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BumpMinor(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBumpMajor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "basic bump",
			version: "v1.0.0",
			want:    "v2.0.0",
		},
		{
			name:    "resets minor and patch",
			version: "v1.2.3",
			want:    "v2.0.0",
		},
		{
			name:    "bump from higher major",
			version: "v9.5.7",
			want:    "v10.0.0",
		},
		{
			name:    "invalid version",
			version: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BumpMajor(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNextVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		moduleChanged bool
		want          string
		wantErr       bool
	}{
		{
			name:          "module changed - bump major",
			version:       "v1.0.0",
			moduleChanged: true,
			want:          "v2.0.0",
		},
		{
			name:          "values only - bump minor",
			version:       "v1.0.0",
			moduleChanged: false,
			want:          "v1.1.0",
		},
		{
			name:          "module changed with existing minor and patch",
			version:       "v1.2.5",
			moduleChanged: true,
			want:          "v2.0.0",
		},
		{
			name:          "invalid version",
			version:       "invalid",
			moduleChanged: false,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NextVersion(tt.version, tt.moduleChanged)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSortVersionsDesc(t *testing.T) {
	versions := []string{"v1.0.0", "v2.0.0", "v1.5.0", "v1.0.1", "v3.0.0"}
	SortVersionsDesc(versions)

	expected := []string{"v3.0.0", "v2.0.0", "v1.5.0", "v1.0.1", "v1.0.0"}
	assert.Equal(t, expected, versions)
}

func TestSortVersions_Empty(t *testing.T) {
	versions := []string{}
	SortVersionsDesc(versions)
	assert.Empty(t, versions)
}

func TestFindLatestVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
		wantErr  bool
	}{
		{
			name:     "multiple versions",
			versions: []string{"v1.0.0", "v2.0.0", "v1.5.0"},
			want:     "v2.0.0",
		},
		{
			name:     "single version",
			versions: []string{"v1.0.0"},
			want:     "v1.0.0",
		},
		{
			name:     "with invalid versions mixed in",
			versions: []string{"v1.0.0", "invalid", "v2.0.0"},
			want:     "v2.0.0",
		},
		{
			name:     "empty list",
			versions: []string{},
			wantErr:  true,
		},
		{
			name:     "all invalid",
			versions: []string{"invalid", "also-invalid"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindLatestVersion(tt.versions)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractVersionFromTag(t *testing.T) {
	tests := []struct {
		name        string
		tag         string
		releaseName string
		want        string
		wantErr     bool
	}{
		{
			name:        "tag with release prefix",
			tag:         "my-release-v1.0.0",
			releaseName: "my-release",
			want:        "v1.0.0",
		},
		{
			name:        "tag is just version",
			tag:         "v1.0.0",
			releaseName: "my-release",
			want:        "v1.0.0",
		},
		{
			name:        "complex version",
			tag:         "app-v2.3.4",
			releaseName: "app",
			want:        "v2.3.4",
		},
		{
			name:        "invalid tag",
			tag:         "not-a-version",
			releaseName: "my-release",
			wantErr:     true,
		},
		{
			name:        "wrong release name",
			tag:         "other-release-v1.0.0",
			releaseName: "my-release",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractVersionFromTag(tt.tag, tt.releaseName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatReleaseTag(t *testing.T) {
	tests := []struct {
		name        string
		releaseName string
		version     string
		want        string
	}{
		{
			name:        "basic format",
			releaseName: "my-release",
			version:     "v1.0.0",
			want:        "my-release-v1.0.0",
		},
		{
			name:        "complex release name",
			releaseName: "my-app-prod",
			version:     "v2.3.4",
			want:        "my-app-prod-v2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatReleaseTag(tt.releaseName, tt.version))
		})
	}
}

func TestEnsureVPrefix(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "already has v prefix",
			version: "v1.0.0",
			want:    "v1.0.0",
		},
		{
			name:    "missing v prefix",
			version: "1.0.0",
			want:    "v1.0.0",
		},
		{
			name:    "empty string",
			version: "",
			want:    "v",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EnsureVPrefix(tt.version))
		})
	}
}

func TestInitialVersion(t *testing.T) {
	assert.Equal(t, "v1.0.0", InitialVersion)
}

func TestParseComponents(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{
			name:      "valid version",
			version:   "v1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "zeros",
			version:   "v0.0.0",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "large numbers",
			version:   "v10.20.30",
			wantMajor: 10,
			wantMinor: 20,
			wantPatch: 30,
		},
		{
			name:    "invalid version",
			version: "invalid",
			wantErr: true,
		},
		{
			name:      "short form normalized",
			version:   "v1.0",
			wantMajor: 1,
			wantMinor: 0,
			wantPatch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := parseComponents(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMajor, major, "major version mismatch")
			assert.Equal(t, tt.wantMinor, minor, "minor version mismatch")
			assert.Equal(t, tt.wantPatch, patch, "patch version mismatch")
		})
	}
}
