package components

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMetadataCacheInvalidationReleasesObsoleteRequest(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "orders", MetadataForeignKeys)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	first := cache.request(key, func() (any, error) {
		calls.Add(1)
		close(started)
		<-release
		return [][]string{{"old"}}, nil
	})
	<-started

	cache.invalidate(key)
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("invalidating an in-flight metadata request left waiters blocked")
	}

	second := cache.request(key, func() (any, error) {
		calls.Add(1)
		return [][]string{{"new"}}, nil
	})
	if second == nil {
		t.Fatal("expected invalidation to force a fresh request")
	}
	<-second
	close(release)

	status, value, err := cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("expected replacement request to be ready, got status=%v err=%v", status, err)
	}
	rows, ok := value.([][]string)
	if !ok || len(rows) != 1 || rows[0][0] != "new" {
		t.Fatalf("obsolete request replaced fresh value: %#v", value)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected old and replacement loads, got %d", got)
	}
}
