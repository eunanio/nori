package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectArchiveType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     ArchiveType
	}{
		{
			name:     "zip file",
			filename: "module.zip",
			want:     ArchiveTypeZip,
		},
		{
			name:     "tar.gz file",
			filename: "module.tar.gz",
			want:     ArchiveTypeTarGz,
		},
		{
			name:     "tgz file",
			filename: "module.tgz",
			want:     ArchiveTypeTarGz,
		},
		{
			name:     "uppercase ZIP",
			filename: "module.ZIP",
			want:     ArchiveTypeZip,
		},
		{
			name:     "unknown extension",
			filename: "module.txt",
			want:     ArchiveTypeUnknown,
		},
		{
			name:     "no extension",
			filename: "module",
			want:     ArchiveTypeUnknown,
		},
		{
			name:     "path with zip",
			filename: "/path/to/module.zip",
			want:     ArchiveTypeZip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectArchiveType(tt.filename)
			if got != tt.want {
				t.Errorf("DetectArchiveType(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: tempFile,
			want: true,
		},
		{
			name: "non-existing file",
			path: filepath.Join(tempDir, "nonexistent.txt"),
			want: false,
		},
		{
			name: "existing directory",
			path: tempDir,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileExists(tt.path)
			if got != tt.want {
				t.Errorf("FileExists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

