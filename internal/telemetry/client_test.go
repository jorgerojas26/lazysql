package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHeartbeatInterval(t *testing.T) {
	if HeartbeatInterval != 5*time.Minute {
		t.Fatalf("heartbeat interval = %s, want 5m", HeartbeatInterval)
	}
}

func TestRelease(t *testing.T) {
	for input, want := range map[string]string{
		"v0.5.8": "0.5.8", "1.20.300": "1.20.300", "dev": "dev",
		"1.2.3-jane-laptop": "dev", "1.2.3+abc123": "dev", "abc123": "dev",
		"v1.2.3-rc1": "dev", "01.2.3": "dev", "1000.1.1": "dev", "1.2.3\n": "dev",
	} {
		if got := Release(input); got != want {
			t.Errorf("Release(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "http://example.com/v1/events", "https://u:p@example.com/v1/events",
		"https://example.com/v1/events?token=secret", "https://example.com/v1/events?", "https://example.com/v1/events#id", "https://example.com/elsewhere", "https:///v1/events"} {
		if ValidEndpoint(endpoint) {
			t.Errorf("accepted %q", endpoint)
		}
	}
	if !ValidEndpoint("https://telemetry.example.com/v1/events") {
		t.Fatal("rejected valid endpoint")
	}
}

func TestDisabled(t *testing.T) {
	for _, key := range []string{"DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY", "CI"} {
		t.Setenv(key, "")
	}
	if Disabled() {
		t.Fatal("disabled without override")
	}
	for _, key := range []string{"DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY", "CI"} {
		t.Setenv(key, "1")
		if !Disabled() {
			t.Errorf("ignored %s", key)
		}
		t.Setenv(key, "")
	}
}

func TestStartDisabledDoesNotSend(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	defer server.Close()
	for _, key := range []string{"DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY", "CI"} {
		t.Setenv(key, "")
	}
	for _, key := range []string{"DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY", "CI"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "1")
			r := &Reporter{}
			stop := r.Start(context.Background(), server.URL+"/v1/events", "1.2.3", Picker, ReleaseBuild)
			r.Feature(QueryExecute)
			r.Connected("postgres", true)
			r.ConnectionFailed(Auth)
			stop()
			stop() // cleanup is safe more than once
			if r.ctx != nil || len(r.takeFeatures()) != 0 || calls.Load() != 0 {
				t.Fatal("disabled client started or retained/sent data")
			}
		})
	}
	for _, endpoint := range []string{"", "http://example.com/v1/events", server.URL + "/v1/events?secret=value"} {
		r := &Reporter{}
		stop := r.Start(context.Background(), endpoint, "1.2.3", Picker, ReleaseBuild)
		r.Connected("postgres", true)
		stop()
		if r.ctx != nil || calls.Load() != 0 {
			t.Fatal("unconfigured/invalid collector started")
		}
	}
}

func TestPayloadHeartbeatAndCancellation(t *testing.T) {
	received := make(chan map[string]any, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/v1/events" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("User-Agent") != "LazySQL-Telemetry/1" {
			t.Error("unexpected user agent")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing content type")
		}
		for _, key := range []string{"Cookie", "Authorization", "Referer", "X-Forwarded-For"} {
			if r.Header.Get(key) != "" {
				t.Errorf("unexpected header %s", key)
			}
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		received <- payload
		w.Header().Set("Set-Cookie", "id=tracking")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	client := newClient()
	defer client.CloseIdleConnections()
	go func() {
		defer close(done)
		(&Reporter{}).run(ctx, client, server.URL+"/v1/events", Release("v0.5.8"), Picker, ReleaseBuild, 20*time.Millisecond)
	}()
	for _, kind := range []string{"start", "heartbeat", "heartbeat"} {
		select {
		case got := <-received:
			want := map[string]any{"schema": float64(2), "event": kind, "version": "0.5.8"}
			if kind == "start" {
				want["startup_mode"] = "picker"
				want["distribution"] = "release"
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("payload = %#v, want %#v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("event not delivered")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestNoRedirectsOrRetries(t *testing.T) {
	for _, status := range []int{302, 307, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/destination")
				w.WriteHeader(status)
			}))
			defer server.Close()
			client := newClient()
			defer client.CloseIdleConnections()
			send(context.Background(), client, server.URL, event{Schema: 2, Event: "start", Version: "dev", StartupMode: Picker, Distribution: ReleaseBuild})
			if calls.Load() != 1 {
				t.Fatalf("made %d requests", calls.Load())
			}
		})
	}
}

func TestCancellationInterruptsRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := newClient()
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Reporter{}).run(ctx, client, server.URL, "dev", Picker, ReleaseBuild, time.Minute)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no request")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt request")
	}
}

func TestCanceledContextSendsNothing(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&Reporter{}).run(ctx, newClient(), server.URL, "dev", Picker, ReleaseBuild, time.Minute)
	if calls.Load() != 0 {
		t.Fatal("canceled worker sent data")
	}
}

func TestFeatureDeltasBoundedAndConcurrent(t *testing.T) {
	r := &Reporter{}
	r.Feature(QueryExecute) // before disclosure: not even retained
	if len(r.takeFeatures()) != 0 {
		t.Fatal("retained pre-disclosure features")
	}
	r.ctx = context.Background()
	r.features = make(map[Feature]int)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1200 {
				r.Feature(QueryExecute)
			}
		}()
	}
	wg.Wait()
	r.Feature(Feature("private-table-name"))
	r.Feature(RowInsert)
	got := r.takeFeatures()
	want := map[Feature]int{QueryExecute: MaxFeatureCount, RowInsert: 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delta = %v, want %v", got, want)
	}
	r.Feature(JSONViewer) // counted separately from the detached delta
	if !reflect.DeepEqual(r.takeFeatures(), map[Feature]int{JSONViewer: 1}) {
		t.Fatal("lost concurrent/new features or resent prior batch")
	}
	if len(r.takeFeatures()) != 0 {
		t.Fatal("resent features")
	}
}

func TestConnectionPayloadsAndInactiveReporter(t *testing.T) {
	received := make(chan map[string]any, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		received <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	r := &Reporter{}
	r.Connected("secret-host", true)
	r.ConnectionFailed(Unknown)
	if len(received) != 0 {
		t.Fatal("inactive reporter sent data")
	}
	// Supply a local test transport; production Start only accepts HTTPS.
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx, r.client, r.endpoint, r.version = ctx, newClient(), server.URL, "1.2.3"
	defer r.client.CloseIdleConnections()
	for _, tc := range []struct {
		record func()
		want   map[string]any
	}{
		{func() { r.Connected("postgres", false) }, map[string]any{"engine": "postgres", "read_only": false, "event": "connection"}},
		{func() { r.Connected("postgres://user:secret@private/db", true) }, map[string]any{"engine": "other", "read_only": true, "event": "connection"}},
		{func() { r.ConnectionFailed(FailureCategory("private-error")) }, map[string]any{"failure": "unknown", "event": "connection_failure"}},
	} {
		tc.record()
		tc.want["schema"], tc.want["version"] = float64(2), "1.2.3"
		select {
		case got := <-received:
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("payload = %v, want %v", got, tc.want)
			}
		case <-time.After(time.Second):
			t.Fatal("connection event not sent")
		}
	}
	cancel()
	r.workers.Wait()
}

func TestFailedHeartbeatDropsDelta(t *testing.T) {
	received := make(chan event, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var payload event
		_ = json.NewDecoder(req.Body).Decode(&payload)
		received <- payload
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	r := &Reporter{ctx: ctx, features: map[Feature]int{CSVExport: 3}}
	done := make(chan struct{})
	client := newClient()
	defer client.CloseIdleConnections()
	go func() {
		defer close(done)
		r.run(ctx, client, server.URL, "dev", Picker, ReleaseBuild, 20*time.Millisecond)
	}()
	defer func() { cancel(); <-done }()
	for _, expected := range []map[Feature]int{nil, {CSVExport: 3}, nil} {
		select {
		case payload := <-received:
			if !reflect.DeepEqual(payload.Features, expected) {
				t.Fatalf("features = %v, want %v", payload.Features, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("missing heartbeat")
		}
	}
}
