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

func TestSQLEditorPasteInsertsAtCursor(t *testing.T) {
	editor := NewSQLEditor("")
	editor.SetText("SELECT  FROM t", false)
	editor.cx = len("SELECT ")

	editor.PasteHandler()("a,\r\nb", nil)

	if got, want := editor.GetText(), "SELECT a,\nb FROM t"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if editor.cy != 1 || editor.cx != 1 {
		t.Fatalf("cursor = (%d, %d), want (1, 1)", editor.cy, editor.cx)
	}

	editor.undo()
	if got, want := editor.GetText(), "SELECT  FROM t"; got != want {
		t.Fatalf("after undo text = %q, want %q", got, want)
	}
}
