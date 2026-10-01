// Package telemetry sends disclosed, identifier-free aggregate usage counts.
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
	"sync"
	"time"
)

const HeartbeatInterval = 5 * time.Minute

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
	Schema       int             `json:"schema"`
	Event        string          `json:"event"`
	Version      string          `json:"version"`
	StartupMode  StartupMode     `json:"startup_mode,omitempty"`
	Distribution Distribution    `json:"distribution,omitempty"`
	Engine       string          `json:"engine,omitempty"`
	ReadOnly     *bool           `json:"read_only,omitempty"`
	Failure      FailureCategory `json:"failure,omitempty"`
	Features     map[Feature]int `json:"features,omitempty"`
}

// Reporter holds only in-memory feature counters and the active sender. Its
// zero value drops all metrics until Start is called after the disclosure.
type Reporter struct {
	mu       sync.Mutex
	features map[Feature]int
	ctx      context.Context
	client   *http.Client
	endpoint string
	version  string
	workers  sync.WaitGroup
}

// Start must be called only after the disclosure/preference gate. It never
// blocks the UI. Cleanup cancels I/O and joins workers without a shutdown flush.
// No retry queue, disk spool, cookies, redirects or application logging.
func (r *Reporter) Start(parent context.Context, endpoint, version string, mode StartupMode, distribution Distribution) func() {
	if Disabled() || !ValidEndpoint(endpoint) {
		return func() {}
	}
	if mode != ConnectionArg {
		mode = Picker
	}
	switch distribution {
	case Homebrew, ReleaseBuild, Source:
	default:
		distribution = UnknownDistribution
	}
	client := newClient()
	ctx, cancel := context.WithCancel(parent)
	r.mu.Lock()
	r.ctx, r.client, r.endpoint, r.version = ctx, client, endpoint, Release(version)
	r.features = make(map[Feature]int)
	r.workers.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.workers.Done()
		r.run(ctx, client, endpoint, Release(version), mode, distribution, HeartbeatInterval)
	}()
	return func() {
		r.mu.Lock()
		r.ctx = nil
		r.features = nil
		cancel()
		r.mu.Unlock()
		r.workers.Wait()
		client.CloseIdleConnections()
	}
}

func (r *Reporter) Feature(feature Feature) {
	if !validFeature(feature) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx != nil && r.features[feature] < MaxFeatureCount {
		r.features[feature]++
	}
}

// Take the delta before I/O so actions during a send belong to the next batch.
// Failed sends deliberately lose that delta too: no queue or retry.
func (r *Reporter) takeFeatures() map[Feature]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	features := r.features
	if r.ctx != nil {
		r.features = make(map[Feature]int)
	}
	return features
}

func (r *Reporter) Connected(provider string, readOnly bool) {
	r.connection(event{Event: "connection", Engine: Engine(provider), ReadOnly: &readOnly})
}

func (r *Reporter) ConnectionFailed(category FailureCategory) {
	r.connection(event{Event: "connection_failure", Failure: boundedFailure(category)})
}

// Connection attempts are infrequent; dispatch once and drop failures. Unlike
// features, there is no buffered event queue (online or offline).
func (r *Reporter) connection(payload event) {
	r.mu.Lock()
	if r.ctx == nil {
		r.mu.Unlock()
		return
	}
	ctx, client, endpoint := r.ctx, r.client, r.endpoint
	payload.Schema, payload.Version = 2, r.version
	r.workers.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.workers.Done()
		send(ctx, client, endpoint, payload)
	}()
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

func (r *Reporter) run(ctx context.Context, client *http.Client, endpoint, version string, mode StartupMode, distribution Distribution, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	send(ctx, client, endpoint, event{Schema: 2, Event: "start", Version: version, StartupMode: mode, Distribution: distribution})
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send(ctx, client, endpoint, event{Schema: 2, Event: "heartbeat", Version: version, Features: r.takeFeatures()})
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
	// Do not let Transport replay even a request that failed on a reused connection.
	request.GetBody = nil
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "LazySQL-Telemetry/1")
	response, err := client.Do(request)
	if err == nil {
		// Never consume or interpret the response body; even a hostile collector
		// cannot ask for additional information or allocate an unbounded buffer.
		_ = response.Body.Close()
	}
}
