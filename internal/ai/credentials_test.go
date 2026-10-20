package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestOpenCodeCredentialLifecycleAndPermissions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	key, err := LoadOpenCodeKey()
	if err != nil || key != "" {
		t.Fatalf("empty credential = %q %v", key, err)
	}
	if err := SaveOpenCodeKey("  subscription-secret\n"); err != nil {
		t.Fatal(err)
	}
	path, err := OpenCodeCredentialPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		t.Fatal("credential was not saved in the user directory")
	}
	if runtime.GOOS != "windows" {
		file, _ := os.Stat(path)
		directory, _ := os.Stat(filepath.Dir(path))
		if file.Mode().Perm() != 0600 || directory.Mode().Perm() != 0700 {
			t.Fatalf("credential permissions = %v directory = %v", file.Mode(), directory.Mode())
		}
	}
	key, err = LoadOpenCodeKey()
	if err != nil || key != "subscription-secret" {
		t.Fatalf("saved credential = %q %v", key, err)
	}
	if err := SaveOpenCodeKey("replacement-key"); err != nil {
		t.Fatal(err)
	}
	key, err = LoadOpenCodeKey()
	if err != nil || key != "replacement-key" {
		t.Fatalf("replacement = %q %v", key, err)
	}
	if err := DeleteOpenCodeKey(); err != nil {
		t.Fatal(err)
	}
	if err := DeleteOpenCodeKey(); err != nil {
		t.Fatal("logout should be idempotent:", err)
	}
	if key, err := LoadOpenCodeKey(); err != nil || key != "" {
		t.Fatal("credential still present after logout")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary credential files remain: %v %v", entries, err)
	}
}

func TestOpenCodeCredentialValidationDoesNotLeakSecrets(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, key := range []string{"", "private secret", "secret\nother", strings.Repeat("s", 8193)} {
		err := SaveOpenCodeKey(key)
		if err == nil || strings.Contains(err.Error(), "private secret") || strings.Contains(err.Error(), "secret\nother") {
			t.Fatalf("invalid key not rejected safely: %v", err)
		}
	}
	path, _ := OpenCodeCredentialPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid credential created a file")
	}
}

func TestOpenCodeCredentialSourcePrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("CUSTOM_GO_KEY", "")
	if err := SaveOpenCodeKey("saved-key"); err != nil {
		t.Fatal(err)
	}
	client, err := newOpenCode(models.AIConfig{Model: "model"})
	if err != nil || client.key != "saved-key" {
		t.Fatalf("saved key not used: %v %v", client, err)
	}
	t.Setenv("OPENCODE_API_KEY", "environment-key")
	client, err = newOpenCode(models.AIConfig{Model: "model"})
	if err != nil || client.key != "environment-key" {
		t.Fatal("environment key should take precedence")
	}
	if _, err := newOpenCode(models.AIConfig{Model: "model", APIKeyEnv: "CUSTOM_GO_KEY"}); err == nil {
		t.Fatal("missing explicit custom env key must not use a different account")
	}
	t.Setenv("CUSTOM_GO_KEY", "custom-key")
	client, err = newOpenCode(models.AIConfig{Model: "model", APIKeyEnv: "CUSTOM_GO_KEY"})
	if err != nil || client.key != "custom-key" {
		t.Fatal("explicit custom environment key ignored")
	}
}

func TestOpenCodeCredentialsRejectSymlinksAndInsecureFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions and symlinks")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SaveOpenCodeKey("key"); err != nil {
		t.Fatal(err)
	}
	path, _ := OpenCodeCredentialPath()
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOpenCodeKey(); err == nil {
		t.Fatal("world-readable credential accepted")
	}
	if err := SaveOpenCodeKey("key"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "unrelated")
	if err := os.WriteFile(target, []byte("unrelated-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOpenCodeKey(); err == nil {
		t.Fatal("credential symlink followed")
	}
	if err := SaveOpenCodeKey("new-key"); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(target)
	if string(original) != "unrelated-secret" {
		t.Fatal("saving credential followed a symlink")
	}
	if err := DeleteOpenCodeKey(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(target), filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := SaveOpenCodeKey("key"); err == nil {
		t.Fatal("symlinked credential directory accepted")
	}
	if err := DeleteOpenCodeKey(); err == nil {
		t.Fatal("logout followed a symlinked credential directory")
	}
}
