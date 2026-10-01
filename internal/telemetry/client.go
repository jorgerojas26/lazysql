// Package telemetry sends only opt-in, identifier-free usage counters.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const HeartbeatInterval = time.Minute

// Disabled is a one-way override: environment variables cannot grant consent.
func Disabled() bool {
	for _, key := range []string{"DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY", "CI"} {
		if value := os.Getenv(key); value != "" && value != "0" && value != "false" {
			return true
		}
	}
	return false
}

// ValidEndpoint deliberately disallows credentials, query parameters, redirects,
// and plaintext HTTP. An unconfigured build has no telemetry capability.
func ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" &&
		u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" &&
		u.Path == "/v1/events" && u.RawPath == ""
}

var releasePattern = regexp.MustCompile(`^v?(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})$`)

// Release excludes commit hashes, prerelease labels and arbitrary build strings.
func Release(version string) string {
	if !releasePattern.MatchString(version) {
		return "dev"
	}
	return strings.TrimPrefix(version, "v")
}

type event struct {
	Schema  int    `json:"schema"`
	Event   string `json:"event"`
	Version string `json:"version"`
}

// Start requires the caller to have obtained consent. It never blocks the UI.
// The returned function cancels in-flight I/O and joins the worker. There is no
// retry queue, disk spool, shutdown event, cookie jar, or application logging.
func Start(parent context.Context, endpoint, version string) func() {
	if Disabled() || !ValidEndpoint(endpoint) {
		return func() {}
	}
	client := newClient()
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer client.CloseIdleConnections()
		run(ctx, client, endpoint, Release(version), HeartbeatInterval)
	}()
	return func() {
		cancel()
		<-done
	}
}

func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 3 * time.Second,
			IdleConnTimeout:     30 * time.Second,
			MaxConnsPerHost:     1,
			DisableCompression:  true,
		},
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func run(ctx context.Context, client *http.Client, endpoint, version string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	send(ctx, client, endpoint, event{1, "start", version})
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send(ctx, client, endpoint, event{1, "heartbeat", version})
		}
	}
}

func send(ctx context.Context, client *http.Client, endpoint string, payload event) {
	if ctx.Err() != nil {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "LazySQL-Telemetry/1")
	response, err := client.Do(request)
	if err == nil {
		// Never consume or interpret the response body; even a hostile collector
		// cannot ask for additional information or allocate an unbounded buffer.
		_ = response.Body.Close()
	}
}
