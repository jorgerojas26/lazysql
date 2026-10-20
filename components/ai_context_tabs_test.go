package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIContextTabsSupportKeyboardMouseAndComposer(t *testing.T) {
	session := newAILayoutSession(t)
	session.toggleContext()
	session.capture(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	if name, _ := session.contextTabs.GetFrontPage(); name != "conversation" || app.App.GetFocus() != session.requestDetails {
		t.Fatal("conversation shortcut did not select and focus request inspection")
	}
	for _, primitive := range []tview.Primitive{session.contextTabs.GetTabByReference("usage").Header, session.contextTabs.GetTabByReference("conversation").Header, session.requestDetails} {
		if !slices.Contains(session.focusable, primitive) {
			t.Fatal("context tabs are not keyboard accessible")
		}
	}
	session.toggleContext()
	session.toggleContext()
	if app.App.GetFocus() != session.requestDetails {
		t.Fatal("reopening context did not restore the selected tab")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	mainPages.SetRect(0, 0, 100, 30)
	mainPages.Draw(screen)
	x, y, found := findAIChatText(screen, "Usage")
	if !found {
		t.Fatalf("usage tab is not visible:\n%s", simulationText(screen))
	}
	mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if name, _ := session.contextTabs.GetFrontPage(); name != "usage" || app.App.GetFocus() != session.contextDetails {
		t.Fatal("clicking usage tab did not switch the view")
	}
	session.capture(tcell.NewEventKey(tcell.KeyRune, ']', tcell.ModNone))
	if session.contextTabs.GetCurrentTab().Reference != "conversation" || app.App.GetFocus() != session.requestDetails {
		t.Fatal("standard next-tab key did not switch context tabs")
	}
	session.capture(tcell.NewEventKey(tcell.KeyRune, '[', tcell.ModNone))
	if session.contextTabs.GetCurrentTab().Reference != "usage" {
		t.Fatal("standard previous-tab key did not switch context tabs")
	}
	header := session.contextTabs.GetTabByReference("conversation").Header
	session.focus(header)
	header.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if session.contextTabs.GetCurrentTab().Reference != "conversation" || app.App.GetFocus() != session.requestDetails {
		t.Fatal("focused tab header could not be activated with Enter")
	}
	session.toggleContext()
	session.focus(session.input)
	event := tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone)
	if session.capture(event) != event {
		t.Fatal("context shortcut stole digits typed in the composer")
	}
}

func TestAIRequestInspectionIsLiteralAndClearedForNewChat(t *testing.T) {
	session := newAILayoutSession(t)
	if !strings.Contains(session.requestDetails.GetText(false), "No model request yet") {
		t.Fatal("empty request inspection is misleading")
	}
	session.requestBody = `{"instructions":"[red]untrusted text","input":[{"role":"user","content":"SELECT 1;"}],"tools":[]}`
	session.requestRound = 3
	session.refreshRequest()
	session.toggleContext()
	session.switchContextTab("conversation")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	mainPages.SetRect(0, 0, 100, 30)
	mainPages.Draw(screen)
	if _, _, found := findAIChatText(screen, "[red]untrusted text"); !found {
		t.Fatal("request text was interpreted as UI markup")
	}
	if !strings.Contains(session.requestDetails.GetText(false), "round 3") {
		t.Fatal("request inspection does not identify the sent round")
	}
	session.newChat()
	if session.requestBody != "" || session.requestRound != 0 || !strings.Contains(session.requestDetails.GetText(false), "No model request yet") {
		t.Fatal("new chat retained the previous conversation payload")
	}
	if name, _ := session.contextTabs.GetFrontPage(); name != "usage" || session.contextTabs.GetCurrentTab().Reference != "usage" {
		t.Fatal("new chat did not reset the context tab")
	}
}

func TestAIUsageSummaryDoesNotInventUnreportedTokens(t *testing.T) {
	session := newAILayoutSession(t)
	session.metricTotals = aiMetricTotals{Calls: 2, Reports: 1, Output: 50, OutputKnown: true}
	session.lastMetrics = ai.RequestMetrics{Usage: ai.Usage{Output: 50, OutputKnown: true}}
	session.refreshContext()
	text := session.contextDetails.GetText(false)
	if strings.Count(text, "Input: not reported") != 2 || strings.Count(text, "Cached input: not reported") != 2 || strings.Count(text, "Output: 50 tokens") != 2 {
		t.Fatalf("unreported token counts were shown as zero:\n%s", text)
	}
}

func TestAIContextTabsUseSharedHeaderStyling(t *testing.T) {
	session := newAILayoutSession(t)
	session.toggleContext()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, width := range []int{100, 48} {
		screen.SetSize(width, 30)
		mainPages.SetRect(0, 0, width, 30)
		for _, selected := range []string{"usage", "conversation"} {
			session.switchContextTab(selected)
			mainPages.Draw(screen)
			for tab := session.contextTabs.state.FirstTab; tab != nil; tab = tab.NextTab {
				x, y, found := findAIChatText(screen, tab.Name)
				if !found {
					t.Fatalf("tab %q is missing at width %d", tab.Name, width)
				}
				_, _, style, _ := screen.GetContent(x, y)
				_, background, _ := style.Decompose()
				expected := app.Styles.PrimitiveBackgroundColor
				if tab.Reference == selected {
					expected = app.Styles.SecondaryTextColor
				}
				if background != expected {
					t.Fatalf("tab %q does not use shared header styling", tab.Name)
				}
			}
		}
	}
}

func TestAIContextHidesComposerAndRestoresDraft(t *testing.T) {
	session := newAILayoutSession(t)
	session.input.SetText("Keep this draft")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	mainPages.SetRect(0, 0, 100, 30)
	draw := func() {
		screen.Clear()
		mainPages.Draw(screen)
	}
	draw()
	_, _, _, chatHeight := session.transcript.GetRect()
	session.toggleContext()
	for _, tab := range []string{"usage", "conversation"} {
		session.switchContextTab(tab)
		draw()
		text := simulationText(screen)
		if strings.Contains(text, "Keep this draft") || strings.Contains(text, " Message ") {
			t.Fatalf("composer is visible in %s tab", tab)
		}
		if slices.Contains(session.focusable, tview.Primitive(session.input)) {
			t.Fatal("hidden composer is still in focus traversal")
		}
		_, view := session.contextTabs.GetFrontPage()
		_, _, _, contextHeight := view.GetRect()
		if contextHeight <= chatHeight {
			t.Fatal("context did not reclaim composer space")
		}
		session.focus(session.input)
		if app.App.GetFocus() != view {
			t.Fatal("background completion can focus the hidden composer")
		}
	}
	session.hide()
	session.show()
	if app.App.GetFocus() != session.requestDetails {
		t.Fatal("reopening context focused the hidden composer")
	}
	session.toggleContext()
	draw()
	if app.App.GetFocus() != session.input || session.input.GetText() != "Keep this draft" || !strings.Contains(simulationText(screen), "Keep this draft") {
		t.Fatal("returning to chat did not restore composer and draft")
	}
}
