package state

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// InitialVersion is the default version for new releases.
const InitialVersion = "v1.0.0"

// ValidateVersion checks if a version string is a valid semantic version.
// The version must have a "v" prefix (e.g., "v1.0.0").
func ValidateVersion(version string) error {
	if !semver.IsValid(version) {
		return fmt.Errorf("invalid semver format: %s (must be vMAJOR.MINOR.PATCH)", version)
	}
	return nil
}

// BumpPatch increments the patch version (for value-only upgrades).
// Example: v1.0.0 -> v1.0.1
func BumpPatch(version string) (string, error) {
	if !semver.IsValid(version) {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	major, minor, patch, err := parseComponents(version)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1), nil
}

// BumpMinor increments the minor version and resets patch (for module version changes).
// Example: v1.0.5 -> v1.1.0
func BumpMinor(version string) (string, error) {
	if !semver.IsValid(version) {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	major, minor, _, err := parseComponents(version)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d.%d.0", major, minor+1), nil
}

// BumpMajor increments the major version and resets minor/patch.
// Example: v1.2.3 -> v2.0.0
func BumpMajor(version string) (string, error) {
	if !semver.IsValid(version) {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	major, _, _, err := parseComponents(version)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d.0.0", major+1), nil
}

// parseComponents extracts major, minor, patch from a semver string.
func parseComponents(version string) (major, minor, patch int, err error) {
	// Get canonical form and strip "v" prefix
	canonical := semver.Canonical(version)
	if canonical == "" {
		return 0, 0, 0, fmt.Errorf("invalid version: %s", version)
	}

	v := strings.TrimPrefix(canonical, "v")
	parts := strings.Split(v, ".")

	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %w", err)
	}

	return major, minor, patch, nil
}

// NextVersion determines the next version based on the type of upgrade.
// If moduleChanged is true, bumps major version; otherwise bumps minor.
func NextVersion(currentVersion string, moduleChanged bool) (string, error) {
	if moduleChanged {
		return BumpMajor(currentVersion)
	}
	return BumpMinor(currentVersion)
}

// SortVersionsDesc sorts a slice of version strings in descending order (newest first).
func SortVersionsDesc(versions []string) {
	semver.Sort(versions)
	// Reverse the slice (semver.Sort is ascending)
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}
}

// FindLatestVersion finds the latest version from a list of version strings.
func FindLatestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions provided")
	}

	var latest string
	for _, v := range versions {
		if !semver.IsValid(v) {
			continue
		}
		if latest == "" || semver.Compare(v, latest) > 0 {
			latest = v
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no valid semver versions found")
	}

	return latest, nil
}

// ExtractVersionFromTag extracts the version from a release tag.
// Expected format: <release-name>-v<version> or just v<version>
func ExtractVersionFromTag(tag, releaseName string) (string, error) {
	// Try to extract version from tag with release name prefix
	prefix := releaseName + "-"
	if strings.HasPrefix(tag, prefix) {
		version := strings.TrimPrefix(tag, prefix)
		if semver.IsValid(version) {
			return version, nil
		}
	}

	// Try to parse the tag directly as a version
	if semver.IsValid(tag) {
		return tag, nil
	}

	return "", fmt.Errorf("cannot extract version from tag: %s", tag)
}

// FormatReleaseTag formats a release name and version into an OCI tag.
// Format: <release-name>-<version>
func FormatReleaseTag(releaseName, version string) string {
	return fmt.Sprintf("%s-%s", releaseName, version)
}

// EnsureVPrefix ensures a version string has a "v" prefix.
func EnsureVPrefix(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}
