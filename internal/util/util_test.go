package util

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
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

func TestExtractReadmeFromZip(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string // filename -> content
		wantReadme  string
		wantNil     bool
		wantErr     bool
	}{
		{
			name: "README at root",
			files: map[string]string{
				"main.tf":   "resource {}",
				"README.md": "# My Module\n\nThis is a test module.",
			},
			wantReadme: "# My Module\n\nThis is a test module.",
		},
		{
			name: "README in subdirectory",
			files: map[string]string{
				"module/main.tf":   "resource {}",
				"module/README.md": "# Subdir Module",
			},
			wantReadme: "# Subdir Module",
		},
		{
			name: "no README",
			files: map[string]string{
				"main.tf":      "resource {}",
				"variables.tf": "variable {}",
			},
			wantNil: true,
		},
		{
			name: "README.md case insensitive",
			files: map[string]string{
				"main.tf":    "resource {}",
				"readme.md":  "# Lowercase README",
			},
			wantReadme: "# Lowercase README",
		},
		{
			name: "README too deep - ignored",
			files: map[string]string{
				"main.tf":                 "resource {}",
				"a/b/README.md":           "# Too deep",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create zip in memory
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)

			for name, content := range tt.files {
				w, err := zw.Create(name)
				if err != nil {
					t.Fatalf("failed to create zip entry: %v", err)
				}
				if _, err := w.Write([]byte(content)); err != nil {
					t.Fatalf("failed to write zip content: %v", err)
				}
			}

			if err := zw.Close(); err != nil {
				t.Fatalf("failed to close zip: %v", err)
			}

			// Test extraction
			got, err := ExtractReadmeFromZip(buf.Bytes())
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractReadmeFromZip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ExtractReadmeFromZip() = %q, want nil", string(got))
				}
				return
			}

			if string(got) != tt.wantReadme {
				t.Errorf("ExtractReadmeFromZip() = %q, want %q", string(got), tt.wantReadme)
			}
		})
	}
}

func TestExtractReadmeFromTarGz(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string // filename -> content
		wantReadme string
		wantNil    bool
		wantErr    bool
	}{
		{
			name: "README at root",
			files: map[string]string{
				"main.tf":   "resource {}",
				"README.md": "# Tar Module",
			},
			wantReadme: "# Tar Module",
		},
		{
			name: "README in subdirectory",
			files: map[string]string{
				"module/main.tf":   "resource {}",
				"module/README.md": "# Subdir Tar Module",
			},
			wantReadme: "# Subdir Tar Module",
		},
		{
			name: "no README",
			files: map[string]string{
				"main.tf": "resource {}",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tar.gz file
			tempDir := t.TempDir()
			tarGzPath := filepath.Join(tempDir, "test.tar.gz")

			f, err := os.Create(tarGzPath)
			if err != nil {
				t.Fatalf("failed to create file: %v", err)
			}

			gw := gzip.NewWriter(f)
			tw := tar.NewWriter(gw)

			for name, content := range tt.files {
				header := &tar.Header{
					Name: name,
					Mode: 0644,
					Size: int64(len(content)),
				}
				if err := tw.WriteHeader(header); err != nil {
					t.Fatalf("failed to write tar header: %v", err)
				}
				if _, err := tw.Write([]byte(content)); err != nil {
					t.Fatalf("failed to write tar content: %v", err)
				}
			}

			tw.Close()
			gw.Close()
			f.Close()

			// Test extraction
			got, err := ExtractReadmeFromTarGz(tarGzPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractReadmeFromTarGz() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ExtractReadmeFromTarGz() = %q, want nil", string(got))
				}
				return
			}

			if string(got) != tt.wantReadme {
				t.Errorf("ExtractReadmeFromTarGz() = %q, want %q", string(got), tt.wantReadme)
			}
		})
	}
}

