package futils

import (
	"testing"

	"github.com/eunanio/nori/internal/spec"
)

func TestWriteBlob(t *testing.T) {
	t.Run("WriteBlob to fs", func(t *testing.T) {
		// test cases
		tests := []struct {
			data      []byte
			tag       *spec.Tag
			mediaType string
			err       bool
		}{
			{[]byte("test"), &spec.Tag{Host: "localhost:5000", Name: "test", Version: "v1"}, spec.MEDIA_TYPE_EMPTY, false},
		}

		for _, tt := range tests {
			digest, err := WriteBlob(tt.data, tt.mediaType)
			if tt.err {
				if err == nil {
					t.Errorf("expected error for tag %s, got nil", digest.Digest)
				}
				continue
			}
		}
	})
}

func TestLoadBlob(t *testing.T) {
	t.Run("LoadBlob from fs", func(t *testing.T) {
		// test cases
		// Test data file
		WriteBlob([]byte("test"), spec.MEDIA_TYPE_EMPTY)
		tests := []struct {
			sha string
			err bool
		}{
			{"sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", false},
			{"sha256:12345", true},
		}

		for _, tt := range tests {
			_, err := LoadBlob(tt.sha)
			if tt.err {
				if err == nil {
					t.Errorf("expected error for tag %s, got nil", tt.sha)
				}
				continue
			}
		}
	})
}
