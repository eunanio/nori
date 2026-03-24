package nori

import (
	"testing"
)

func TestParseAnnotation(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
		wantErr bool
	}{
		{"key=value", "key", "value", false},
		{"org.opencontainers.image.authors=team@example.com", "org.opencontainers.image.authors", "team@example.com", false},
		{"key=val=ue", "key", "val=ue", false},
		{"key=", "key", "", false},
		{"noequals", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			k, v, err := ParseAnnotation(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAnnotation(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if k != tt.wantKey {
				t.Errorf("key = %q, want %q", k, tt.wantKey)
			}
			if v != tt.wantVal {
				t.Errorf("value = %q, want %q", v, tt.wantVal)
			}
		})
	}
}

func TestParseAnnotations(t *testing.T) {
	m, err := ParseAnnotations([]string{"a=1", "b=2"})
	if err != nil {
		t.Fatal(err)
	}
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("got %v", m)
	}

	_, err = ParseAnnotations([]string{"good=ok", "bad"})
	if err == nil {
		t.Error("expected error for invalid annotation")
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal interface{}
		wantErr bool
	}{
		{"key=value", "key", "value", false},
		{"enabled=true", "enabled", true, false},
		{"disabled=false", "disabled", false, false},
		{"count=42", "count", "42", false},
		{"noequals", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			k, v, err := ParseValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseValue(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if k != tt.wantKey {
				t.Errorf("key = %q, want %q", k, tt.wantKey)
			}
			if v != tt.wantVal {
				t.Errorf("value = %v (%T), want %v (%T)", v, v, tt.wantVal, tt.wantVal)
			}
		})
	}
}

func TestParseSetValues(t *testing.T) {
	m, err := ParseSetValues([]string{"name=my-bucket", "versioning=true"})
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "my-bucket" {
		t.Errorf("name = %v", m["name"])
	}
	if m["versioning"] != true {
		t.Errorf("versioning = %v", m["versioning"])
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"42", true},
		{"-42", true},
		{"3.14", true},
		{"-3.14", true},
		{"", false},
		{"abc", false},
		{"12abc", false},
		{"0", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsNumeric(tt.input)
			if got != tt.want {
				t.Errorf("IsNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUpdateModuleTag(t *testing.T) {
	tests := []struct {
		moduleRef string
		newTag    string
		want      string
	}{
		{"ghcr.io/org/module:v1.0.0", "v2.0.0", "ghcr.io/org/module:v2.0.0"},
		{"ghcr.io/org/module", "v1.0.0", "ghcr.io/org/module:v1.0.0"},
		{"localhost:5000/module:v1", "v2", "localhost:5000/module:v2"},
		{"registry.example.com:8080/ns/mod:latest", "v1.0.0", "registry.example.com:8080/ns/mod:v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.moduleRef+"->"+tt.newTag, func(t *testing.T) {
			got := UpdateModuleTag(tt.moduleRef, tt.newTag)
			if got != tt.want {
				t.Errorf("UpdateModuleTag(%q, %q) = %q, want %q", tt.moduleRef, tt.newTag, got, tt.want)
			}
		})
	}
}

func TestDeriveOutputFilename(t *testing.T) {
	tests := []struct {
		reference string
		want      string
	}{
		{"ghcr.io/myorg/s3-bucket:v1.0.0", "s3-bucket-v1.0.0.zip"},
		{"ghcr.io/myorg/vpc", "vpc.zip"},
		{"ghcr.io/myorg/mod@sha256:abc123", "mod@sha256-abc123.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.reference, func(t *testing.T) {
			got := DeriveOutputFilename(tt.reference)
			if got != tt.want {
				t.Errorf("DeriveOutputFilename(%q) = %q, want %q", tt.reference, got, tt.want)
			}
		})
	}
}

func TestIsSystemAnnotation(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"org.opencontainers.image.created", true},
		{"io.nori.release.status", true},
		{"io.nori.module.ref", true},
		{"io.nori.version", true},
		{"team", false},
		{"environment", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := IsSystemAnnotation(tt.key)
			if got != tt.want {
				t.Errorf("IsSystemAnnotation(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
