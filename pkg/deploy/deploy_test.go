package deploy

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "simple reference",
			ref:      "ghcr.io/myorg/s3-bucket:v1.0.0",
			expected: "s3-bucket",
		},
		{
			name:     "nested path",
			ref:      "ghcr.io/myorg/infra/modules/s3-bucket:v1.0.0",
			expected: "s3-bucket",
		},
		{
			name:     "with port",
			ref:      "localhost:5000/mymodule:latest",
			expected: "mymodule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := name.ParseReference(tt.ref)
			require.NoError(t, err)
			result := extractRepoName(ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatHCLValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string value",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "float",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "string slice",
			input:    []interface{}{"a", "b"},
			expected: `["a", "b"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatHCLValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeployResult_Fields(t *testing.T) {
	result := &DeployResult{
		ModuleRef:      "ghcr.io/org/mod:v1.0.0",
		WorkDir:        "/tmp/work",
		ModuleDir:      "/tmp/work/module",
		PlanFile:       "/tmp/work/tfplan",
		HasChanges:     true,
		Applied:        true,
		ApplyAttempted: true,
		TFState:        []byte(`{"version": 4}`),
		TFStatePath:    "/tmp/work/module/terraform.tfstate",
		Outputs:        map[string]interface{}{"id": "abc123"},
	}

	assert.Equal(t, "ghcr.io/org/mod:v1.0.0", result.ModuleRef)
	assert.True(t, result.HasChanges)
	assert.True(t, result.Applied)
	assert.True(t, result.ApplyAttempted)
	assert.NotEmpty(t, result.TFState)
	assert.Equal(t, "abc123", result.Outputs["id"])
}

func TestReleaseDeployOptions_Fields(t *testing.T) {
	opts := ReleaseDeployOptions{
		AutoApprove: true,
		Parallelism: 5,
		VarFiles:    []string{"vars.tfvars"},
		Targets:     []string{"module.foo"},
		PlanOnly:    false,
		Upgrade:     true,
		Reconfigure: true,
	}

	assert.True(t, opts.AutoApprove)
	assert.Equal(t, 5, opts.Parallelism)
	assert.Len(t, opts.VarFiles, 1)
	assert.True(t, opts.Reconfigure)
}
