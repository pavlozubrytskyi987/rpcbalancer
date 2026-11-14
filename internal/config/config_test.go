package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, c *Config)
	}{
		{
			name: "valid",
			yaml: `
listen: ":9000"
backends:
  - name: a
    url: http://localhost:1111
  - name: b
    url: https://example.com
`,
			check: func(t *testing.T, c *Config) {
				if c.Listen != ":9000" {
					t.Errorf("listen = %q, want :9000", c.Listen)
				}
				if len(c.Backends) != 2 {
					t.Fatalf("backends len = %d, want 2", len(c.Backends))
				}
				if c.Backends[0].Name != "a" || c.Backends[1].Name != "b" {
					t.Errorf("backend names = %q,%q", c.Backends[0].Name, c.Backends[1].Name)
				}
			},
		},
		{
			name: "default listen",
			yaml: `
backends:
  - name: a
    url: http://localhost:1111
`,
			check: func(t *testing.T, c *Config) {
				if c.Listen != ":8080" {
					t.Errorf("listen = %q, want default :8080", c.Listen)
				}
			},
		},
		{
			name:    "no backends",
			yaml:    `listen: ":8080"`,
			wantErr: "at least one backend",
		},
		{
			name: "empty name",
			yaml: `
backends:
  - name: ""
    url: http://localhost:1111
`,
			wantErr: "name is empty",
		},
		{
			name: "duplicate name",
			yaml: `
backends:
  - name: a
    url: http://localhost:1111
  - name: a
    url: http://localhost:2222
`,
			wantErr: "duplicate name",
		},
		{
			name: "bad scheme",
			yaml: `
backends:
  - name: a
    url: ftp://localhost:1111
`,
			wantErr: "scheme must be http or https",
		},
		{
			name: "empty host",
			yaml: `
backends:
  - name: a
    url: http://
`,
			wantErr: "host is empty",
		},
		{
			name:    "malformed yaml",
			yaml:    "backends: [this is: broken",
			wantErr: "parse yaml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(path)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("got nil err, want error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %q, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read config") {
		t.Errorf("err = %q, want it to mention read config", err)
	}
}

