// Package updater checks official stable releases and installs standalone binaries.
package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const releasesURL = "https://api.github.com/repos/jorgerojas26/lazysql/releases/latest"
const maxArchive = 100 << 20

// Release contains only the metadata needed to install an official release.
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Client struct {
	HTTP      *http.Client
	Endpoint  string
	CacheFile string
}

func New() *Client {
	c := &Client{HTTP: &http.Client{Timeout: 2 * time.Minute}, Endpoint: releasesURL}
	if dir, err := os.UserCacheDir(); err == nil {
		c.CacheFile = filepath.Join(dir, "lazysql", "release.json")
	}
	return c
}

func version(s string) string    { return "v" + strings.TrimPrefix(s, "v") }
func ValidVersion(s string) bool { return semver.IsValid(version(s)) }
func Newer(current, latest string) bool {
	return ValidVersion(current) && ValidVersion(latest) && semver.Prerelease(version(latest)) == "" && semver.Compare(version(latest), version(current)) > 0
}

// Check uses a daily cache for automatic checks. Manual checks always retry.
// Development builds deliberately do not contact GitHub.
func (c *Client) Check(ctx context.Context, current string, manual bool) (*Release, error) {
	if !ValidVersion(current) {
		return nil, errors.New("update checks are unavailable for development builds")
	}
	var cached struct {
		Checked time.Time
		Release Release
	}
	if !manual && c.CacheFile != "" {
		if data, err := os.ReadFile(c.CacheFile); err == nil && json.Unmarshal(data, &cached) == nil {
			age := time.Since(cached.Checked)
			if age >= 0 && age < 24*time.Hour && ValidVersion(cached.Release.Tag) {
				return available(current, cached.Release), nil
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := c.get(ctx, c.Endpoint, 2<<20)
	if err != nil {
		return nil, err
	}
	var release Release
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("invalid release response: %w", err)
	}
	if !ValidVersion(release.Tag) || release.Draft || release.Prerelease || semver.Prerelease(version(release.Tag)) != "" {
		return nil, errors.New("GitHub did not return a stable release")
	}
	cached.Checked, cached.Release = time.Now(), release
	if c.CacheFile != "" {
		if data, err := json.Marshal(cached); err == nil {
			// Cache failure must never prevent starting the application.
			if os.MkdirAll(filepath.Dir(c.CacheFile), 0700) == nil {
				_ = os.WriteFile(c.CacheFile, data, 0600)
			}
		}
	}
	return available(current, release), nil
}

func available(current string, r Release) *Release {
	if r.Draft || r.Prerelease || !Newer(current, r.Tag) {
		return nil
	}
	return &r
}

func (c *Client) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lazysql-updater")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release request failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("release download exceeds size limit")
	}
	return data, nil
}

func archiveName(goos, arch string) string {
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}
	osName := map[string]string{"darwin": "Darwin", "linux": "Linux", "windows": "Windows"}[goos]
	if osName == "" || (arch != "x86_64" && arch != "i386" && arch != "arm64") {
		return ""
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return "lazysql_" + osName + "_" + arch + ext
}

func (r Release) asset(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name != name {
			continue
		}
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.Path != "/jorgerojas26/lazysql/releases/download/"+r.Tag+"/"+name {
			return "", errors.New("release asset is not hosted in the official repository")
		}
		return a.URL, nil
	}
	return "", fmt.Errorf("release asset %q is unavailable", name)
}

// InstallGuidance conservatively excludes common managed installation paths.
// Never invoke a package manager or ask for elevated permissions implicitly.
func InstallGuidance(path, goos string) string {
	p := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(p, "/cellar/") || strings.Contains(p, "/homebrew/") || strings.Contains(p, "/linuxbrew/"):
		return "This installation is managed by Homebrew. Run: brew upgrade lazysql"
	case strings.Contains(p, "/scoop/"):
		return "This installation is managed by Scoop. Run: scoop update lazysql"
	case strings.Contains(p, "/nix/store/") || strings.HasPrefix(p, "/usr/bin/") || strings.HasPrefix(p, "/usr/sbin/") || strings.HasPrefix(p, "/snap/") || strings.Contains(p, "/chocolatey/"):
		return "This installation is managed by your system package manager. Upgrade lazysql with that package manager."
	case goos == "windows":
		return "On Windows, upgrade with your package manager or replace lazysql.exe using the latest GitHub release after closing the app."
	}
	return ""
}

func Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// Install verifies the archive checksum, stages the executable next to the old
// one, and replaces it with one rename. The running session is not restarted.
func (c *Client) Install(ctx context.Context, r Release, target string) error {
	if hint := InstallGuidance(target, runtime.GOOS); hint != "" {
		return errors.New(hint)
	}
	if !ValidVersion(r.Tag) || r.Draft || r.Prerelease || semver.Prerelease(version(r.Tag)) != "" {
		return errors.New("not a stable release")
	}
	name := archiveName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		return errors.New("self-update is not supported on this platform; use your installation method")
	}
	archiveURL, err := r.asset(name)
	if err != nil {
		return err
	}
	checksumURL, err := r.asset("lazysql_" + strings.TrimPrefix(r.Tag, "v") + "_checksums.txt")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	checksums, err := c.get(ctx, checksumURL, 1<<20)
	if err != nil {
		return err
	}
	archive, err := c.get(ctx, archiveURL, maxArchive)
	if err != nil {
		return err
	}
	if err := verifyChecksum(archive, checksums, name); err != nil {
		return err
	}
	return replace(ctx, target, archive)
}

func verifyChecksum(data, checksums []byte, name string) error {
	sum := sha256.Sum256(data)
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if strings.EqualFold(fields[0], hex.EncodeToString(sum[:])) {
				return nil
			}
			return errors.New("release checksum mismatch; executable was not changed")
		}
	}
	return errors.New("release checksum is missing; executable was not changed")
}

func replace(ctx context.Context, target string, archive []byte) error {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("executable is not a regular file")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return errors.New("archive does not contain lazysql")
		}
		if err != nil {
			return err
		}
		if header.Name != "lazysql" {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxArchive {
			return errors.New("invalid release executable")
		}
		tmp, err := os.CreateTemp(filepath.Dir(target), ".lazysql-update-*")
		if err != nil {
			return fmt.Errorf("cannot write installation directory; upgrade using your original installation method: %w", err)
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		if _, err := io.CopyN(tmp, tr, header.Size); err != nil {
			return err
		}
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			return err
		}
		if err := tmp.Sync(); err != nil {
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), target)
	}
}
