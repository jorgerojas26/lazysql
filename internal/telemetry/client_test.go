package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	t.Setenv("DO_NOT_TRACK", "1")
	stop := Start(context.Background(), server.URL+"/v1/events", "1.2.3")
	stop()
	stop() // cleanup is safe more than once
	if calls.Load() != 0 {
		t.Fatal("disabled client sent data")
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
		run(ctx, client, server.URL+"/v1/events", Release("v0.5.8"), 20*time.Millisecond)
	}()
	for _, kind := range []string{"start", "heartbeat", "heartbeat"} {
		select {
		case got := <-received:
			want := map[string]any{"schema": float64(1), "event": kind, "version": "0.5.8"}
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/destination")
				w.WriteHeader(status)
			}))
			defer server.Close()
			client := newClient()
			defer client.CloseIdleConnections()
			send(context.Background(), client, server.URL, event{1, "start", "dev"})
			if calls.Load() != 1 {
				t.Fatalf("made %d requests", calls.Load())
			}
		})
	}
}

func TestCancellationInterruptsRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	go func() { defer close(done); run(ctx, client, server.URL, "dev", time.Minute) }()
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run(ctx, newClient(), server.URL, "dev", time.Minute)
	if calls.Load() != 0 {
		t.Fatal("canceled worker sent data")
	}
}
