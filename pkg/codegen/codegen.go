// Package codegen provides HCL code generation for Terraform/OpenTofu configurations.
package codegen

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// ModuleConfig contains configuration for generating a module block.
type ModuleConfig struct {
	// Name is the local name for the module (e.g., "my_bucket").
	Name string

	// Source is the module source (e.g., OCI reference).
	Source string

	// Values are the input variables to pass to the module.
	Values map[string]interface{}
}

// BackendConfig contains configuration for the terraform backend.
type BackendConfig struct {
	// Type is the backend type (e.g., "local", "s3", "gcs").
	Type string

	// Config contains backend-specific configuration.
	Config map[string]string
}

// GenerateMainTF generates a main.tf file content with a module block.
func GenerateMainTF(module *ModuleConfig, backend *BackendConfig) []byte {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	// Add terraform block with backend if specified
	if backend != nil && backend.Type != "" {
		terraformBlock := rootBody.AppendNewBlock("terraform", nil)
		terraformBody := terraformBlock.Body()

		backendBlock := terraformBody.AppendNewBlock("backend", []string{backend.Type})
		backendBody := backendBlock.Body()

		for key, value := range backend.Config {
			backendBody.SetAttributeValue(key, cty.StringVal(value))
		}

		rootBody.AppendNewline()
	}

	// Add module block
	moduleBlock := rootBody.AppendNewBlock("module", []string{module.Name})
	moduleBody := moduleBlock.Body()

	// Set source
	// Ensure module.Source already includes potential path/subpath, but we add "oci://" and "?tag=" for tag/version as required by OpenTofu format.
	// Expect module.Source in the format: "<host>/<repo>:<tag>" or "<host>/<repo>@<digest>"
	var ociSource string
	if idx := len(module.Source); idx > 0 {
		// If the module.Source contains a colon (tag) or @ (digest) at the end, split out the tag/digest
		// to build ?tag=xxx for tag, or @sha256:... for digest.
		if at := indexOf(module.Source, "@"); at >= 0 {
			// Digest: use as is (OpenTofu supports @sha256:... after the repo)
			ociSource = "oci://" + module.Source
		} else if colon := lastIndexOf(module.Source, ":"); colon > -1 && colon > lastIndexOf(module.Source, "/") {
			// Tag: convert to ?tag=tag
			repo := module.Source[:colon]
			tag := module.Source[colon+1:]
			ociSource = "oci://" + repo + "?tag=" + tag
		} else {
			// No tag/digest
			ociSource = "oci://" + module.Source
		}
	}
	moduleBody.SetAttributeValue("source", cty.StringVal(ociSource))

	// Set values
	for key, value := range module.Values {
		ctyVal := toCtyValue(value)
		moduleBody.SetAttributeValue(key, ctyVal)
	}

	return f.Bytes()
}

func indexOf(s, sep string) int     { return strings.Index(s, sep) }
func lastIndexOf(s, sep string) int { return strings.LastIndex(s, sep) }

// toCtyValue converts a Go value to a cty.Value.
func toCtyValue(v interface{}) cty.Value {
	if v == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}

	switch val := v.(type) {
	case string:
		return cty.StringVal(val)
	case bool:
		return cty.BoolVal(val)
	case int:
		return cty.NumberIntVal(int64(val))
	case int64:
		return cty.NumberIntVal(val)
	case float64:
		return cty.NumberFloatVal(val)
	case []interface{}:
		return toCtySList(val)
	case []string:
		vals := make([]cty.Value, len(val))
		for i, s := range val {
			vals[i] = cty.StringVal(s)
		}
		if len(vals) == 0 {
			return cty.ListValEmpty(cty.String)
		}
		return cty.ListVal(vals)
	case map[string]interface{}:
		return toCtyMap(val)
	case map[string]string:
		vals := make(map[string]cty.Value)
		for k, v := range val {
			vals[k] = cty.StringVal(v)
		}
		if len(vals) == 0 {
			return cty.MapValEmpty(cty.String)
		}
		return cty.MapVal(vals)
	default:
		// Fallback to string representation
		return cty.StringVal(fmt.Sprintf("%v", v))
	}
}

// toCtySList converts a slice of interfaces to a cty list.
func toCtySList(vals []interface{}) cty.Value {
	if len(vals) == 0 {
		return cty.ListValEmpty(cty.DynamicPseudoType)
	}

	ctyVals := make([]cty.Value, len(vals))
	for i, v := range vals {
		ctyVals[i] = toCtyValue(v)
	}

	// Try to create a typed list if all elements have the same type
	return cty.TupleVal(ctyVals)
}

// toCtyMap converts a map to a cty object.
func toCtyMap(m map[string]interface{}) cty.Value {
	if len(m) == 0 {
		return cty.EmptyObjectVal
	}

	vals := make(map[string]cty.Value)
	for k, v := range m {
		vals[k] = toCtyValue(v)
	}

	return cty.ObjectVal(vals)
}
