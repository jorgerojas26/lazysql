package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"0.9.0", "v0.10.0", true}, {"v1.0.0", "1.0.0", false},
		{"2.0.0", "1.9.0", false}, {"dev", "1.0.0", false},
		{"1.0.0", "garbage", false}, {"1.0.0", "1.1.0-rc.1", false},
		{"1.1.0-rc.1", "1.1.0", true}, {"1.0.0+abc", "1.0.0+def", false},
	} {
		if got := Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v", tt.current, tt.latest, got)
		}
	}
}

func TestCheckCache(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("User-Agent") != "lazysql-updater" {
			t.Error("missing user agent")
		}
		_ = json.NewEncoder(w).Encode(Release{Tag: "v1.2.0"})
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client(), Endpoint: server.URL, CacheFile: filepath.Join(t.TempDir(), "release.json")}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		release, err := client.Check(ctx, "1.0.0", false)
		if err != nil || release == nil || release.Tag != "v1.2.0" {
			t.Fatalf("check: %v, %v", release, err)
		}
	}
	if calls != 1 {
		t.Fatalf("cache missed: %d calls", calls)
	}
	if r, err := client.Check(ctx, "1.2.0", false); err != nil || r != nil {
		t.Fatalf("cached release comparison: %v, %v", r, err)
	}
	if _, err := client.Check(ctx, "1.0.0", true); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("manual check must bypass cache")
	}
	if _, err := client.Check(ctx, "dev", true); err == nil || calls != 2 {
		t.Fatal("development build contacted server")
	}
	data := []byte(`{"Checked":"2000-01-01T00:00:00Z","Release":{"tag_name":"v1.1.0"}}`)
	if err := os.WriteFile(client.CacheFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(ctx, "1.0.0", false); err != nil || calls != 3 {
		t.Fatalf("expired cache: %v", err)
	}
}

func TestCheckFailures(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"rate limit", "", 403}, {"offline server", "", 503},
		{"malformed", "not json", 200}, {"empty", `{}`, 200},
		{"prerelease", `{"tag_name":"v2.0.0","prerelease":true}`, 200},
		{"draft", `{"tag_name":"v2.0.0","draft":true}`, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			c := &Client{HTTP: server.Client(), Endpoint: server.URL}
			if _, err := c.Check(context.Background(), "1.0.0", true); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDownloadBoundsAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("too long")) }))
	defer server.Close()
	c := &Client{HTTP: server.Client()}
	if _, err := c.get(context.Background(), server.URL, 3); err == nil {
		t.Fatal("unbounded response")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.get(ctx, server.URL, 30); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestArchiveName(t *testing.T) {
	for _, tt := range []struct{ os, arch, name string }{
		{"darwin", "arm64", "lazysql_Darwin_arm64.tar.gz"},
		{"linux", "amd64", "lazysql_Linux_x86_64.tar.gz"},
		{"linux", "386", "lazysql_Linux_i386.tar.gz"},
		{"windows", "amd64", "lazysql_Windows_x86_64.zip"},
		{"linux", "arm", ""}, {"freebsd", "amd64", ""},
	} {
		if got := archiveName(tt.os, tt.arch); got != tt.name {
			t.Errorf("got %q, want %q", got, tt.name)
		}
	}
}

func TestAssetOrigin(t *testing.T) {
	for _, address := range []string{
		"http://github.com/jorgerojas26/lazysql/releases/download/v1.0.0/file",
		"https://evil.example/file",
		"https://github.com/another/repo/releases/download/v1.0.0/file",
		"https://github.com/jorgerojas26/lazysql/releases/download/v0.9.0/file",
	} {
		r := Release{Tag: "v1.0.0", Assets: []Asset{{Name: "file", URL: address}}}
		if _, err := r.asset("file"); err == nil {
			t.Errorf("accepted %s", address)
		}
	}
}

func TestGuidance(t *testing.T) {
	for _, p := range []string{"/opt/homebrew/Cellar/lazysql/1/bin/lazysql", "/usr/bin/lazysql", "/nix/store/pkg/bin/lazysql", "/snap/lazysql/bin/lazysql", "C:/Users/me/scoop/apps/lazysql/current/lazysql.exe"} {
		if InstallGuidance(p, "linux") == "" {
			t.Errorf("managed path accepted: %s", p)
		}
	}
	if InstallGuidance("/home/me/.local/bin/lazysql", "linux") != "" {
		t.Fatal("standalone path rejected")
	}
	if InstallGuidance("C:/tools/lazysql.exe", "windows") == "" {
		t.Fatal("Windows must get manual instructions")
	}
}

func testArchive(t *testing.T, name string, kind byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	body := []byte("new executable")
	size := int64(len(body))
	if kind != tar.TypeReg {
		size = 0
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Typeflag: kind, Size: size}); err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses package-manager guidance")
	}
	for _, tt := range []struct {
		name              string
		archive           []byte
		canceled, success bool
	}{
		{"valid", testArchive(t, "lazysql", tar.TypeReg), false, true},
		{"traversal", testArchive(t, "../lazysql", tar.TypeReg), false, false},
		{"symlink", testArchive(t, "lazysql", tar.TypeSymlink), false, false},
		{"corrupt", []byte("invalid gzip"), false, false},
		{"canceled", testArchive(t, "lazysql", tar.TypeReg), true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "lazysql")
			if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			err := replace(ctx, target, tt.archive)
			if (err == nil) != tt.success {
				t.Fatalf("replace: %v", err)
			}
			got, _ := os.ReadFile(target)
			want := "old"
			if tt.success {
				want = "new executable"
			}
			if string(got) != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			info, _ := os.Stat(target)
			if info.Mode().Perm() != 0755 {
				t.Fatal("permissions lost")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Fatal("temporary files left behind")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstall(t *testing.T) {
	if runtime.GOOS == "windows" || archiveName(runtime.GOOS, runtime.GOARCH) == "" {
		t.Skip("unsupported self-update platform")
	}
	archive := testArchive(t, "lazysql", tar.TypeReg)
	name := archiveName(runtime.GOOS, runtime.GOARCH)
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "lazysql")
			if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
				t.Fatal(err)
			}
			sum := fmt.Sprintf("%x", sha256.Sum256(archive))
			if !valid {
				sum = strings.Repeat("0", 64)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "checksums.txt") {
					_, _ = fmt.Fprintf(w, "%s  %s\n", sum, name)
				} else {
					_, _ = w.Write(archive)
				}
			}))
			defer server.Close()
			transport := server.Client().Transport
			client := &Client{HTTP: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				copy := req.Clone(req.Context())
				copy.URL.Scheme = "http"
				copy.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return transport.RoundTrip(copy)
			})}}
			base := "https://github.com/jorgerojas26/lazysql/releases/download/v1.2.0/"
			r := Release{Tag: "v1.2.0", Assets: []Asset{{Name: name, URL: base + name}, {Name: "lazysql_1.2.0_checksums.txt", URL: base + "lazysql_1.2.0_checksums.txt"}}}
			err := client.Install(context.Background(), r, target)
			if (err == nil) != valid {
				t.Fatalf("install error: %v", err)
			}
			got, _ := os.ReadFile(target)
			want := "old"
			if valid {
				want = "new executable"
			}
			if string(got) != want {
				t.Fatalf("binary = %q", got)
			}
		})
	}
}

func TestMissingChecksum(t *testing.T) {
	if err := verifyChecksum([]byte("binary"), []byte(""), "archive"); err == nil {
		t.Fatal("missing checksum accepted")
	}
}
