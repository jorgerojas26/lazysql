package app

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestPasteNewlineScreenReportsPastedLineFeedsAsEnter(t *testing.T) {
	sim := tcell.NewSimulationScreen("")
	screen := &pasteNewlineScreen{Screen: sim}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()

	lineFeed := func() tcell.Event { return tcell.NewEventKey(tcell.KeyRune, '\n', tcell.ModNone) }
	key := func(ch rune) tcell.Event { return tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone) }
	for _, ev := range []tcell.Event{
		lineFeed(), // Outside a paste Ctrl+J stays Ctrl+J.
		tcell.NewEventPaste(true),
		key('a'), lineFeed(),
		key('b'), key('\r'), lineFeed(),
		key('c'),
		tcell.NewEventPaste(false),
	} {
		if err := sim.PostEvent(ev); err != nil {
			t.Fatal(err)
		}
	}

	var got []string
	for range 8 {
		switch e := screen.PollEvent().(type) {
		case *tcell.EventPaste:
			got = append(got, map[bool]string{true: "paste-start", false: "paste-end"}[e.Start()])
		case *tcell.EventKey:
			got = append(got, e.Name())
		}
	}
	want := []string{"Ctrl+J", "paste-start", "Rune[a]", "Enter", "Rune[b]", "Enter", "Rune[c]", "paste-end"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q, want %q", got, want)
	}
}
