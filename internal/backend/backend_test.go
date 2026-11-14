package backend

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		bName   string
		url     string
		wantErr string
	}{
		{name: "valid http", bName: "a", url: "http://localhost:1111"},
		{name: "valid https", bName: "b", url: "https://example.com/path"},
		{name: "empty name", bName: "", url: "http://localhost:1111", wantErr: "name is empty"},
		{name: "bad scheme", bName: "a", url: "ftp://localhost:1111", wantErr: "scheme must be http or https"},
		{name: "empty host", bName: "a", url: "http://", wantErr: "host is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := New(tc.bName, tc.url)
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
			if b.Name != tc.bName {
				t.Errorf("Name = %q, want %q", b.Name, tc.bName)
			}
			if b.Target == nil || b.Target.String() != tc.url {
				t.Errorf("Target = %v, want %q", b.Target, tc.url)
			}
			if b.Proxy == nil {
				t.Error("Proxy is nil")
			}
			if !b.Alive() {
				t.Error("new Backend should start alive")
			}
		})
	}
}

func TestSetAlive(t *testing.T) {
	b, err := New("x", "http://localhost:1")
	if err != nil {
		t.Fatal(err)
	}
	b.SetAlive(false)
	if b.Alive() {
		t.Error("expected Alive=false after SetAlive(false)")
	}
	b.SetAlive(true)
	if !b.Alive() {
		t.Error("expected Alive=true after SetAlive(true)")
	}
}
