// Package util provides internal utility functions for Nori.
package util

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ArchiveType represents the type of archive.
type ArchiveType string

const (
	ArchiveTypeZip     ArchiveType = "zip"
	ArchiveTypeTarGz   ArchiveType = "tar.gz"
	ArchiveTypeUnknown ArchiveType = "unknown"
)

// DetectArchiveType determines the archive type from the file extension.
func DetectArchiveType(filename string) ArchiveType {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ArchiveTypeZip
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return ArchiveTypeTarGz
	default:
		return ArchiveTypeUnknown
	}
}

// ValidateArchive checks if the file is a valid archive.
func ValidateArchive(path string) error {
	archiveType := DetectArchiveType(path)
	if archiveType == ArchiveTypeUnknown {
		return fmt.Errorf("unsupported archive format: must be .zip or .tar.gz")
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	switch archiveType {
	case ArchiveTypeZip:
		return validateZip(path)
	case ArchiveTypeTarGz:
		return validateTarGz(file)
	}

	return nil
}

func validateZip(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}
	defer reader.Close()

	if len(reader.File) == 0 {
		return fmt.Errorf("zip archive is empty")
	}

	return nil
}

func validateTarGz(file *os.File) error {
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	_, err = tarReader.Next()
	if err == io.EOF {
		return fmt.Errorf("tar archive is empty")
	}
	if err != nil {
		return fmt.Errorf("invalid tar archive: %w", err)
	}

	return nil
}

// ExtractArchive extracts an archive to the specified directory.
func ExtractArchive(archivePath, destDir string) error {
	archiveType := DetectArchiveType(archivePath)

	switch archiveType {
	case ArchiveTypeZip:
		return extractZip(archivePath, destDir)
	case ArchiveTypeTarGz:
		return extractTarGz(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		destPath := filepath.Join(destDir, file.Name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		if err := extractZipFile(file, destPath); err != nil {
			return err
		}
	}

	return nil
}

func extractZipFile(file *zip.File, destPath string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in archive: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	return nil
}

func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		destPath := filepath.Join(destDir, header.Name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			if err := extractTarFile(tarReader, destPath, header.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

func extractTarFile(reader *tar.Reader, destPath string, mode os.FileMode) error {
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ConvertTarGzToZip converts a tar.gz archive to zip format for OpenTofu compatibility.
func ConvertTarGzToZip(tarGzPath string) ([]byte, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "nori-convert-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract tar.gz to temp dir
	if err := ExtractArchive(tarGzPath, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract tar.gz: %w", err)
	}

	// Create zip in memory
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory
		if relPath == "." {
			return nil
		}

		// Use forward slashes for zip compatibility
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			// Add directory entry (with trailing slash)
			_, err := zipWriter.Create(relPath + "/")
			return err
		}

		// Create file entry
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Write file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(writer, file); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ExtractReadmeFromZip extracts README.md content from zip archive bytes.
// It looks for README.md at the root level or in a single top-level directory.
// Returns nil if no README is found.
func ExtractReadmeFromZip(content []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	// Look for README.md at root or in single top-level directory
	var readmeFile *zip.File
	for _, file := range reader.File {
		// Normalize to forward slashes for consistent handling
		name := filepath.ToSlash(file.Name)
		baseName := strings.ToLower(filepath.Base(name))

		// Check if it's a README.md file
		if baseName != "readme.md" {
			continue
		}

		// Check if it's at root level or one level deep
		// Count the number of path separators to determine depth
		dir := filepath.ToSlash(filepath.Dir(name))
		depth := strings.Count(dir, "/")
		if dir == "." || (depth == 0 && dir != "") {
			// Root level (dir == ".") or one level deep (e.g., "module" with no slashes)
			readmeFile = file
			break
		}
	}

	if readmeFile == nil {
		return nil, nil // No README found, not an error
	}

	rc, err := readmeFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open README: %w", err)
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// ExtractReadmeFromTarGz extracts README.md content from a tar.gz file.
// It looks for README.md at the root level or in a single top-level directory.
// Returns nil if no README is found.
func ExtractReadmeFromTarGz(archivePath string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Normalize to forward slashes for consistent handling
		name := filepath.ToSlash(header.Name)
		baseName := strings.ToLower(filepath.Base(name))

		// Check if it's a README.md file
		if baseName != "readme.md" {
			continue
		}

		// Check if it's at root level or one level deep
		// Count the number of path separators to determine depth
		dir := filepath.ToSlash(filepath.Dir(name))
		depth := strings.Count(dir, "/")
		if dir == "." || (depth == 0 && dir != "") {
			// Root level (dir == ".") or one level deep (e.g., "module" with no slashes)
			return io.ReadAll(tarReader)
		}
	}

	return nil, nil // No README found, not an error
}

// NormalizeModuleDir normalizes a module directory structure after extraction.
// If the extracted content has a single subdirectory containing .tf files (with no .tf files at root),
// it moves the contents up to the parent directory.
// This handles cases where modules are packaged with a wrapper directory like:
//
//	module.zip
//	└── s3/
//	    ├── main.tf
//	    └── variables.tf
func NormalizeModuleDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Check if there are any .tf files at the root level
	hasTFAtRoot := false
	var singleSubdir string
	subdirCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".tf") {
				hasTFAtRoot = true
			}
		} else {
			subdirCount++
			singleSubdir = entry.Name()
		}
	}

	// If there are .tf files at root, the structure is already normalized
	if hasTFAtRoot {
		return nil
	}

	// If there's not exactly one subdirectory, nothing to normalize
	if subdirCount != 1 {
		return nil
	}

	// Check if the single subdirectory contains .tf files
	subdirPath := filepath.Join(dir, singleSubdir)
	subdirEntries, err := os.ReadDir(subdirPath)
	if err != nil {
		return fmt.Errorf("failed to read subdirectory: %w", err)
	}

	hasTFInSubdir := false
	for _, entry := range subdirEntries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".tf") {
			hasTFInSubdir = true
			break
		}
	}

	// If no .tf files in subdirectory, nothing to normalize
	if !hasTFInSubdir {
		return nil
	}

	// Move all contents from subdirectory to parent
	for _, entry := range subdirEntries {
		srcPath := filepath.Join(subdirPath, entry.Name())
		dstPath := filepath.Join(dir, entry.Name())

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
	}

	// Remove the now-empty subdirectory
	if err := os.Remove(subdirPath); err != nil {
		return fmt.Errorf("failed to remove empty subdirectory: %w", err)
	}

	return nil
}