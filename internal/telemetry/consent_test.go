package telemetry

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConsentLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazysql", "telemetry.toml")
	endpoint := "https://example.com/v1/events"
	if enabled, decided, err := LoadConsent(path, endpoint); enabled || decided || err != nil {
		t.Fatalf("new: %v %v %v", enabled, decided, err)
	}
	for _, choice := range []bool{true, false} {
		if err := SaveConsent(path, endpoint, choice); err != nil {
			t.Fatal(err)
		}
		if enabled, decided, err := LoadConsent(path, endpoint); enabled != choice || !decided || err != nil {
			t.Fatalf("saved: %v %v %v", enabled, decided, err)
		}
		if enabled, decided, err := LoadConsent(path, "https://different.example/v1/events"); enabled || decided != !choice || err != nil {
			t.Fatalf("new endpoint: %v %v %v", enabled, decided, err)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".telemetry-*"))
	if len(matches) != 0 {
		t.Fatalf("temporary files left: %v", matches)
	}
}

func TestMalformedConsentFailsClosed(t *testing.T) {
	for _, content := range []string{"invalid TOML!", "", "enabled = 'true'", "enabled = true"} {
		path := filepath.Join(t.TempDir(), "telemetry.toml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		enabled, _, _ := LoadConsent(path, "https://example.com/v1/events")
		if enabled {
			t.Errorf("enabled with malformed preference %q", content)
		}
	}
}

func TestConsentIOFailure(t *testing.T) {
	dir := t.TempDir()
	if enabled, decided, err := LoadConsent(dir, "endpoint"); enabled || !decided || err == nil {
		t.Fatal("read failure did not fail closed")
	}
	parent := filepath.Join(dir, "file")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveConsent(filepath.Join(parent, "telemetry.toml"), "endpoint", true); err == nil {
		t.Fatal("expected write failure")
	}
}
