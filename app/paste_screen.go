package app

import "github.com/gdamore/tcell/v2"

// pasteNewlineScreen reports line feeds inside a bracketed paste as Enter.
//
// While collecting a paste, the pinned tview keeps only KeyRune, KeyEnter
// and KeyTab, so the KeyCtrlJ that tcell reports for "\n" is dropped and a
// multi-line paste arrives as a single line. tview fixed this upstream in
// 2a2df46, but upgrading tview first needs lazysql's focus handling ported
// to tview's new focus notifications. Drop this wrapper once tview is
// upgraded.
type pasteNewlineScreen struct {
	tcell.Screen
	initErr error
	pasting bool
	afterCR bool
}

func (s *pasteNewlineScreen) Init() error {
	s.initErr = s.Screen.Init()
	return s.initErr
}

// PollEvent is only called from tview's event goroutine, so the paste state
// needs no locking.
func (s *pasteNewlineScreen) PollEvent() tcell.Event {
	for {
		ev := s.Screen.PollEvent()
		switch e := ev.(type) {
		case *tcell.EventPaste:
			s.pasting = e.Start()
			s.afterCR = false
		case *tcell.EventKey:
			if !s.pasting {
				break
			}
			if e.Key() == tcell.KeyCtrlJ {
				if s.afterCR {
					// "\r\n" is a single line break.
					s.afterCR = false
					continue
				}
				return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			}
			s.afterCR = e.Key() == tcell.KeyEnter
		}
		return ev
	}
}

// UseTerminal gives the app a terminal screen that keeps line breaks in
// pasted text. Call it right before Run; tests set simulation screens
// instead.
func (a *Application) UseTerminal() error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	s := &pasteNewlineScreen{Screen: screen}
	a.SetScreen(s) // Calls s.Init.
	if s.initErr != nil {
		return s.initErr
	}
	// tview only enables these on screens it creates itself.
	s.EnableMouse()
	s.EnablePaste()
	return nil
}
