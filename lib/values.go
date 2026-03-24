package nori

import (
	"fmt"
	"strings"
)

// ParseAnnotations parses a slice of "key=value" strings into a map.
func ParseAnnotations(raw []string) (map[string]string, error) {
	m := make(map[string]string, len(raw))
	for _, s := range raw {
		k, v, err := ParseAnnotation(s)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// ParseAnnotation splits a single "key=value" string.
func ParseAnnotation(s string) (string, string, error) {
	for i, c := range s {
		if c == '=' {
			return s[:i], s[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid annotation format %q, expected key=value", s)
}

// ParseSetValues parses a slice of "key=value" strings into a typed map
// where booleans are converted to bool and everything else stays as a string.
func ParseSetValues(raw []string) (map[string]interface{}, error) {
	m := make(map[string]interface{}, len(raw))
	for _, s := range raw {
		k, v, err := ParseValue(s)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// ParseValue splits a single "key=value" string and converts the value
// to a Go type: "true"/"false" become bool; everything else stays string.
func ParseValue(v string) (string, interface{}, error) {
	for i, c := range v {
		if c == '=' {
			key := v[:i]
			value := v[i+1:]

			if value == "true" {
				return key, true, nil
			}
			if value == "false" {
				return key, false, nil
			}
			return key, value, nil
		}
	}
	return "", "", fmt.Errorf("invalid value format %q, expected key=value", v)
}

// IsNumeric reports whether s looks like a number (digits, optional leading
// minus, optional single dot). Useful for deciding how to pass Terraform values.
func IsNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// UpdateModuleTag replaces the tag portion of an OCI module reference.
// Example: "ghcr.io/org/module:v1.0.0" with newTag "v2.0.0" → "ghcr.io/org/module:v2.0.0"
func UpdateModuleTag(moduleRef, newTag string) string {
	lastColon := -1
	for i := len(moduleRef) - 1; i >= 0; i-- {
		if moduleRef[i] == ':' {
			hasSlash := false
			for j := i + 1; j < len(moduleRef); j++ {
				if moduleRef[j] == '/' {
					hasSlash = true
					break
				}
			}
			if !hasSlash {
				lastColon = i
				break
			}
		}
	}

	if lastColon == -1 {
		return moduleRef + ":" + newTag
	}
	return moduleRef[:lastColon+1] + newTag
}

// DeriveOutputFilename builds a local filename from an OCI reference.
// Example: "ghcr.io/myorg/s3-bucket:v1.0.0" → "s3-bucket-v1.0.0.zip"
func DeriveOutputFilename(reference string) string {
	parts := strings.Split(reference, "/")
	nameWithTag := parts[len(parts)-1]

	if idx := strings.Index(nameWithTag, ":"); idx != -1 {
		name := nameWithTag[:idx]
		tag := nameWithTag[idx+1:]
		return fmt.Sprintf("%s-%s.zip", name, tag)
	}

	if idx := strings.Index(nameWithTag, "@"); idx != -1 {
		name := nameWithTag[:idx]
		return fmt.Sprintf("%s.zip", name)
	}

	return fmt.Sprintf("%s.zip", nameWithTag)
}

// IsSystemAnnotation reports whether an annotation key is a nori/OCI
// system annotation that should be hidden from user-facing output.
func IsSystemAnnotation(key string) bool {
	systemPrefixes := []string{
		"org.opencontainers.",
		"io.nori.release.",
		"io.nori.module.",
		"io.nori.version",
	}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
