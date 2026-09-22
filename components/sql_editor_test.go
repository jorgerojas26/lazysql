package components

import (
	"sync"
	"testing"
)

func TestSQLEditorSubscribeAndPublishConcurrently(t *testing.T) {
	editor := NewSQLEditor("")
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for range 1000 {
			editor.Subscribe()
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for range 1000 {
			editor.Publish("key", "value")
		}
	}()

	close(start)
	wg.Wait()

	subscriber := editor.Subscribe()
	editor.Publish("key", "value")

	change := <-subscriber
	if change.Key != "key" || change.Value != "value" {
		t.Fatalf("unexpected state change: %#v", change)
	}
}
