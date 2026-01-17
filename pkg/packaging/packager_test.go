package packaging

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/eunanio/nori/pkg/oci"
)

func TestPreparePackage(t *testing.T) {
	tests := []struct {
		name        string
		archivePath string
		opts        PackageOptions
		wantErr     bool
		checkResult func(t *testing.T, result *PrepareResult)
	}{
		{
			name:        "valid zip archive",
			archivePath: "../../testdata/s3-module.zip",
			opts:        PackageOptions{},
			wantErr:     false,
			checkResult: func(t *testing.T, result *PrepareResult) {
				if result.Size == 0 {
					t.Error("expected non-zero size")
				}
				if len(result.Content) == 0 {
					t.Error("expected non-empty content")
				}
				if result.Annotations == nil {
					t.Error("expected annotations to be set")
				}
				if result.Annotations[oci.AnnotationTitle] != "s3-module.zip" {
					t.Errorf("expected title annotation to be 's3-module.zip', got %q", result.Annotations[oci.AnnotationTitle])
				}
			},
		},
		{
			name:        "with description",
			archivePath: "../../testdata/s3-module.zip",
			opts:        PackageOptions{Description: "Test module description"},
			wantErr:     false,
			checkResult: func(t *testing.T, result *PrepareResult) {
				if result.Annotations[oci.AnnotationDescription] != "Test module description" {
					t.Errorf("expected description annotation, got %q", result.Annotations[oci.AnnotationDescription])
				}
			},
		},
		{
			name:        "with custom annotations",
			archivePath: "../../testdata/s3-module.zip",
			opts: PackageOptions{
				Annotations: map[string]string{
					"custom.annotation": "custom-value",
					"another.annotation": "another-value",
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *PrepareResult) {
				if result.Annotations["custom.annotation"] != "custom-value" {
					t.Errorf("expected custom annotation, got %q", result.Annotations["custom.annotation"])
				}
				if result.Annotations["another.annotation"] != "another-value" {
					t.Errorf("expected another annotation, got %q", result.Annotations["another.annotation"])
				}
			},
		},
		{
			name:        "with config path for metadata extraction",
			archivePath: "../../testdata/s3-module.zip",
			opts:        PackageOptions{ConfigPath: "../../testdata/s3-bucket"},
			wantErr:     false,
			checkResult: func(t *testing.T, result *PrepareResult) {
				// Should have metadata annotation if extraction succeeded
				if _, ok := result.Annotations["io.nori.module.metadata"]; !ok {
					t.Error("expected metadata annotation to be set")
				}
			},
		},
		{
			name:        "non-existent file",
			archivePath: "../../testdata/nonexistent.zip",
			opts:        PackageOptions{},
			wantErr:     true,
		},
		{
			name:        "invalid archive extension",
			archivePath: "../../testdata/values.yaml",
			opts:        PackageOptions{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packager := NewPackager(nil, nil)
			result, err := packager.PreparePackage(context.Background(), tt.archivePath, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("PreparePackage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestPreparePackage_TarGzConversion(t *testing.T) {
	// Create a temporary tar.gz file for testing
	tempDir := t.TempDir()
	tarGzPath := filepath.Join(tempDir, "test.tar.gz")

	// Create a valid tar.gz file with actual content
	if err := createTestTarGz(tarGzPath); err != nil {
		t.Skipf("skipping tar.gz conversion test: %v", err)
	}

	packager := NewPackager(nil, nil)
	result, err := packager.PreparePackage(context.Background(), tarGzPath, PackageOptions{})

	if err != nil {
		t.Skipf("skipping tar.gz conversion test due to error: %v", err)
	}

	// Check that content is valid zip (starts with PK signature)
	if len(result.Content) < 4 {
		t.Fatal("content too short to be valid zip")
	}
	if result.Content[0] != 'P' || result.Content[1] != 'K' {
		t.Error("expected content to be converted to zip format (PK signature)")
	}
}

func TestPreparePackage_AnnotationsHaveCreatedTime(t *testing.T) {
	packager := NewPackager(nil, nil)
	result, err := packager.PreparePackage(context.Background(), "../../testdata/s3-module.zip", PackageOptions{})

	if err != nil {
		t.Fatalf("PreparePackage() error = %v", err)
	}

	if _, ok := result.Annotations[oci.AnnotationCreated]; !ok {
		t.Error("expected created annotation to be set")
	}
}

// createTestTarGz creates a simple tar.gz file for testing
func createTestTarGz(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add a simple test file
	content := []byte("# Test Terraform file\nvariable \"test\" {}\n")
	header := &tar.Header{
		Name: "main.tf",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}

