// Package runtime provides runtime management for OpenTofu/Terraform.
package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultRuntime is the default runtime to use.
	DefaultRuntime = "tofu"

	// OpenTofuGitHubOwner is the GitHub owner for OpenTofu releases.
	OpenTofuGitHubOwner = "opentofu"

	// OpenTofuGitHubRepo is the GitHub repository for OpenTofu releases.
	OpenTofuGitHubRepo = "opentofu"

	// GitHubAPIReleasesURL is the GitHub API URL for releases.
	GitHubAPIReleasesURL = "https://api.github.com/repos/%s/%s/releases/latest"

	// GitHubDownloadURL is the base URL for GitHub release downloads.
	GitHubDownloadURL = "https://github.com/%s/%s/releases/download/%s/%s"
)

// Manager handles runtime detection and installation.
type Manager struct {
	logger     *slog.Logger
	binDir     string
	httpClient *http.Client
}

// ManagerOption is a function that configures a Manager.
type ManagerOption func(*Manager)

// WithLogger sets the logger for the manager.
func WithLogger(logger *slog.Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = logger
	}
}

// WithBinDir sets the binary directory for the manager.
func WithBinDir(binDir string) ManagerOption {
	return func(m *Manager) {
		m.binDir = binDir
	}
}

// NewManager creates a new runtime manager.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		logger: slog.Default(),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.binDir == "" {
		m.binDir = DefaultBinDir()
	}

	return m
}

// DefaultBinDir returns the default binary directory for nori.
func DefaultBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nori", "bin")
}

// GetRuntime returns the path to the runtime binary.
// If the runtime is not found in PATH, it will attempt to install it.
func (m *Manager) GetRuntime(ctx context.Context, runtimeName string) (string, error) {
	if runtimeName == "" {
		runtimeName = DefaultRuntime
	}

	// First, check if the runtime is in PATH
	path, err := exec.LookPath(runtimeName)
	if err == nil {
		m.logger.Debug("found runtime in PATH", "runtime", runtimeName, "path", path)
		return path, nil
	}

	// Check if we have it installed in our bin directory
	installedPath := filepath.Join(m.binDir, runtimeName)
	if runtime.GOOS == "windows" {
		installedPath += ".exe"
	}

	if _, err := os.Stat(installedPath); err == nil {
		m.logger.Debug("found runtime in nori bin", "runtime", runtimeName, "path", installedPath)
		return installedPath, nil
	}

	// If it's tofu, try to install it
	if runtimeName == "tofu" {
		m.logger.Info("OpenTofu not found, attempting to install")
		if err := m.InstallOpenTofu(ctx); err != nil {
			return "", fmt.Errorf("failed to install OpenTofu: %w", err)
		}
		return installedPath, nil
	}

	return "", fmt.Errorf("runtime %q not found in PATH", runtimeName)
}

// InstallOpenTofu downloads and installs OpenTofu from GitHub.
func (m *Manager) InstallOpenTofu(ctx context.Context) error {
	m.logger.Info("installing OpenTofu")

	// Get the latest release info
	release, err := m.getLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	m.logger.Info("found latest OpenTofu release", "version", release.TagName)

	// Determine the asset to download
	assetName := m.getAssetName(release.TagName)
	downloadURL := fmt.Sprintf(GitHubDownloadURL, OpenTofuGitHubOwner, OpenTofuGitHubRepo, release.TagName, assetName)

	m.logger.Info("downloading OpenTofu", "url", downloadURL)

	// Download the asset
	data, err := m.downloadFile(ctx, downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Ensure bin directory exists
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Extract the binary
	binaryName := "tofu"
	if runtime.GOOS == "windows" {
		binaryName = "tofu.exe"
	}

	binaryPath := filepath.Join(m.binDir, binaryName)

	if err := m.extractBinary(data, binaryPath); err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	m.logger.Info("OpenTofu installed successfully", "path", binaryPath, "version", release.TagName)

	return nil
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

// getLatestRelease fetches the latest release info from GitHub.
func (m *Manager) getLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	url := fmt.Sprintf(GitHubAPIReleasesURL, OpenTofuGitHubOwner, OpenTofuGitHubRepo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "nori")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// getAssetName returns the asset name for the current platform.
func (m *Manager) getAssetName(version string) string {
	// Remove 'v' prefix if present for the filename
	ver := strings.TrimPrefix(version, "v")

	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch names to OpenTofu asset names
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "386"
	}

	// OpenTofu asset naming: tofu_<version>_<os>_<arch>.zip
	return fmt.Sprintf("tofu_%s_%s_%s.zip", ver, os, arch)
}

// downloadFile downloads a file from the given URL.
func (m *Manager) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nori")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// extractBinary extracts the tofu binary from a zip archive.
func (m *Manager) extractBinary(zipData []byte, destPath string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	binaryName := "tofu"
	if runtime.GOOS == "windows" {
		binaryName = "tofu.exe"
	}

	for _, file := range reader.File {
		if filepath.Base(file.Name) == binaryName {
			src, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer src.Close()

			dst, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("failed to copy binary: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("binary %q not found in archive", binaryName)
}

