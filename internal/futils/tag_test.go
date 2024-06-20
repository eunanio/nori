package futils

import "testing"

func TestTag(t *testing.T) {
	t.Run("ParseImageTag", func(t *testing.T) {
		// test cases
		tests := []struct {
			tag    string
			host   string
			name   string
			version string
			err    bool
		}{
			{"", "", "", "", true},
			{"localhost:5000/test:v1", "localhost:5000", "test", "v1", false},
			{"test:latest", "", "test", "latest", false},
			{"name", "", "name", "latest", false},
			{"host/name", "host", "name", "latest", false},
			{"host/name:version:extra", "", "", "", true},
		}

		for _, tt := range tests {
			tag, err := ParseImageTag(tt.tag)
			if tt.err {
				if err == nil {
					t.Errorf("expected error for tag %s, got nil", tag)
				}
				continue
			}
		}
	})

}