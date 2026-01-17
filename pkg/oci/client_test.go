package oci

import (
	"testing"
)

func TestValidateReference(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{
			name:    "valid reference with tag",
			ref:     "ghcr.io/myorg/module:v1.0.0",
			wantErr: false,
		},
		{
			name:    "valid reference with digest",
			ref:     "ghcr.io/myorg/module@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: false,
		},
		{
			name:    "valid reference with latest tag",
			ref:     "ghcr.io/myorg/module:latest",
			wantErr: false,
		},
		{
			name:    "valid reference with port",
			ref:     "registry.example.com:5000/myorg/module:v1.0.0",
			wantErr: false,
		},
		{
			name:    "empty reference",
			ref:     "",
			wantErr: true,
		},
		{
			name:    "no slash",
			ref:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReference(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReference() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantRepo   string
		wantTag    string
		wantDigest string
		wantErr    bool
	}{
		{
			name:     "reference with tag",
			ref:      "ghcr.io/myorg/module:v1.0.0",
			wantRepo: "myorg/module",
			wantTag:  "v1.0.0",
			wantErr:  false,
		},
		{
			name:     "reference without tag",
			ref:      "ghcr.io/myorg/module",
			wantRepo: "myorg/module",
			wantTag:  "latest",
			wantErr:  false,
		},
		{
			name:       "reference with digest",
			ref:        "ghcr.io/myorg/module@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantRepo:   "myorg/module",
			wantDigest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			gotRepo := RepositoryFromRef(ref)
			if gotRepo != tt.wantRepo {
				t.Errorf("RepositoryFromRef() = %v, want %v", gotRepo, tt.wantRepo)
			}

			if tt.wantTag != "" {
				gotTag := TagFromRef(ref)
				if gotTag != tt.wantTag {
					t.Errorf("TagFromRef() = %v, want %v", gotTag, tt.wantTag)
				}
			}

			if tt.wantDigest != "" {
				gotDigest := DigestFromRef(ref)
				if gotDigest != tt.wantDigest {
					t.Errorf("DigestFromRef() = %v, want %v", gotDigest, tt.wantDigest)
				}
			}
		})
	}
}

