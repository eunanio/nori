package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetLastSuccessfulVersion_FromHistory(t *testing.T) {
	tests := []struct {
		name     string
		history  *ReleaseHistory
		expected string
	}{
		{
			name: "returns first deployed version",
			history: &ReleaseHistory{
				Name: "test-release",
				Versions: []ReleaseVersion{
					{Version: "v1.2.0", Status: StatusFailed},
					{Version: "v1.1.0", Status: StatusDeployed},
					{Version: "v1.0.0", Status: StatusSuperseded},
				},
			},
			expected: "v1.1.0",
		},
		{
			name: "returns latest when all deployed",
			history: &ReleaseHistory{
				Name: "test-release",
				Versions: []ReleaseVersion{
					{Version: "v1.2.0", Status: StatusDeployed},
					{Version: "v1.1.0", Status: StatusDeployed},
				},
			},
			expected: "v1.2.0",
		},
		{
			name: "returns empty when no deployed versions",
			history: &ReleaseHistory{
				Name: "test-release",
				Versions: []ReleaseVersion{
					{Version: "v1.2.0", Status: StatusFailed},
					{Version: "v1.1.0", Status: StatusFailed},
				},
			},
			expected: "",
		},
		{
			name: "returns empty for empty history",
			history: &ReleaseHistory{
				Name:     "test-release",
				Versions: []ReleaseVersion{},
			},
			expected: "",
		},
		{
			name: "skips pending versions",
			history: &ReleaseHistory{
				Name: "test-release",
				Versions: []ReleaseVersion{
					{Version: "v1.3.0", Status: StatusPending},
					{Version: "v1.2.0", Status: StatusFailed},
					{Version: "v1.1.0", Status: StatusDeployed},
				},
			},
			expected: "v1.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findLastSuccessfulVersion(tt.history)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func findLastSuccessfulVersion(history *ReleaseHistory) string {
	for _, v := range history.Versions {
		if v.Status == StatusDeployed {
			return v.Version
		}
	}
	return ""
}

func TestReleaseHistory_VersionOrdering(t *testing.T) {
	now := time.Now()
	history := &ReleaseHistory{
		Name: "test",
		Versions: []ReleaseVersion{
			{Version: "v1.2.0", CreatedAt: now, Status: StatusFailed},
			{Version: "v1.1.0", CreatedAt: now.Add(-1 * time.Hour), Status: StatusDeployed},
			{Version: "v1.0.0", CreatedAt: now.Add(-2 * time.Hour), Status: StatusSuperseded},
		},
	}

	assert.Len(t, history.Versions, 3)
	assert.Equal(t, "v1.2.0", history.Versions[0].Version)
	assert.Equal(t, "v1.1.0", history.Versions[1].Version)
	assert.Equal(t, "v1.0.0", history.Versions[2].Version)
}
