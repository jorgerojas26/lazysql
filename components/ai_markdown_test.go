package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func markdownPlain(source string, width int) string {
	return tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetText(chatMarkdown(source, width)).GetText(true)
}

func TestAIMessageMarkdownFormatsStructuredExplanation(t *testing.T) {
	source := "## Registered users\n\n- **App accounts:** `core_app.User`\n- **Profiles:** `core_app.UserProfile`\n\n## Breathing exercises\n\n1. **Catalog:** `tila.Breathe`\n2. **Sessions:** `core_app.BreatheHistory`\n\n**In short:** definitions live in tila; usage in core_app."
	plain := markdownPlain(source, 80)
	for _, expected := range []string{"Registered users\n\n", "• App accounts: core_app.User", "• Profiles: core_app.UserProfile", "Breathing exercises\n\n", "1. Catalog: tila.Breathe", "2. Sessions: core_app.BreatheHistory", "In short: definitions live in tila; usage in core_app."} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("formatted explanation omitted %q:\n%s", expected, plain)
		}
	}
	for _, delimiter := range []string{"##", "**", "`"} {
		if strings.Contains(plain, delimiter) {
			t.Fatalf("Markdown delimiter %q is still visible", delimiter)
		}
	}
	view := tview.NewTextView().SetDynamicColors(true).SetText(chatMarkdown(source, 80))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 25)
	view.SetRect(0, 0, 80, 25)
	view.Draw(screen)
	for _, test := range []struct {
		text  string
		bold  bool
		color tcell.Color
	}{{"Registered users", true, app.Styles.SecondaryTextColor}, {"App accounts:", true, app.Styles.PrimaryTextColor}, {"core_app.User", false, app.Styles.SecondaryTextColor}} {
		x, y, found := findAIChatText(screen, test.text)
		if !found {
			t.Fatalf("styled text %q not visible", test.text)
		}
		_, _, style, _ := screen.GetContent(x, y)
		foreground, _, attributes := style.Decompose()
		if foreground != test.color || (attributes&tcell.AttrBold != 0) != test.bold {
			t.Fatalf("incorrect style for %q: %v", test.text, style)
		}
	}
}

func TestAIMarkdownKeepsIdentifiersAndUnsupportedFormattingLiteral(t *testing.T) {
	for _, test := range []struct{ source, expected string }{
		{"core_app.UserProfile.user_id → core_app.User.id", "core_app.UserProfile.user_id → core_app.User.id"},
		{"`a*b_c` and ``a`b``", "a*b_c and ``a`b``"},
		{"**bold with `inline_code`**", "bold with inline_code"},
		{"***bold italic*** and _italic_", "***bold italic*** and _italic_"},
		{"\\*literal\\* and \\`backticks\\`", "*literal* and `backticks`"},
		{"SELECT * FROM users WHERE count > 2 * 3;", "SELECT * FROM users WHERE count > 2 * 3;"},
		{"**unfinished bold", "**unfinished bold"},
		{"`unfinished code", "`unfinished code"},
	} {
		if got := markdownPlain(test.source, 100); got != test.expected {
			t.Fatalf("%q rendered as %q, want %q", test.source, got, test.expected)
		}
	}
}

func TestAIMarkdownEscapesUIRegionsAndStripsTerminalControls(t *testing.T) {
	source := "## [red]\n\n- **[\"sql-0\"]** `[-]`\n\n```text\n[red] [\"sql-1\"]\n```\n\x1b[31mterminal\x07"
	markup := chatMarkdown(source, 80)
	if strings.ContainsAny(markup, "\x1b\x07") {
		t.Fatal("raw terminal controls reached formatted output")
	}
	plain := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetText(markup).GetText(true)
	for _, literal := range []string{"[red]", `["sql-0"]`, "[-]", `["sql-1"]`} {
		if !strings.Contains(plain, literal) {
			t.Fatalf("model UI markup was interpreted rather than escaped: %q", plain)
		}
	}
	if strings.Contains(markup, `["sql-0"]`) || strings.Contains(markup, `["sql-1"]`) {
		t.Fatal("model created clickable query regions")
	}
}

func TestAIMarkdownListWrapsWithHangingIndent(t *testing.T) {
	for _, width := range []int{18, 32, 60} {
		source := "- **Profile details:** `core_app.UserProfile` linked through user_id to the registered account."
		plain := markdownPlain(source, width)
		lines := strings.Split(plain, "\n")
		if !strings.HasPrefix(lines[0], "• ") || len(lines) < 2 {
			t.Fatal("list was not formatted and wrapped")
		}
		for _, line := range lines {
			if tview.TaggedStringWidth(tview.Escape(line)) > width {
				t.Fatalf("list exceeds width %d: %q", width, line)
			}
		}
		for _, line := range lines[1:] {
			if !strings.HasPrefix(line, "  ") {
				t.Fatalf("wrapped list lost indentation: %q", line)
			}
		}
	}
}

func TestAIMarkdownFencesAndStreamingPrefixesAreSafe(t *testing.T) {
	source := "## Users\n\n- **Accounts:** `core_app.User`\n\n```sql\nSELECT id FROM users;\n```\n\n```text\n[red] literal\n```"
	for length := range len(source) + 1 {
		markup := chatMarkdown(source[:length], 60)
		view := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetText(markup)
		if strings.ContainsAny(view.GetText(true), "\x1b\x07") {
			t.Fatal("streaming prefix emitted terminal controls")
		}
	}
	plain := markdownPlain(source, 60)
	for _, expected := range []string{"```sql", "SELECT id FROM users;", "[red] literal"} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("fenced output omitted %q: %s", expected, plain)
		}
	}
}

func TestAIMarkdownDoesNotChangeRawHistoryOrUserMessages(t *testing.T) {
	session := newAILayoutSession(t)
	source := "## Heading\n\n- **Label:** `core_app.User`"
	session.appendMessage(ai.ChatMessage{Role: "user", Text: source})
	session.appendMessage(ai.ChatMessage{Role: "assistant", Text: source, SQL: "SELECT 1;"})
	request := session.makeRequest("Next question")
	if session.history[0].Text != source || session.history[1].Text != source || request.History[1].Text != source {
		t.Fatal("display formatting modified model context/history")
	}
	user := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetText(session.messageMarkup(session.history[0], 0, 80)).GetText(true)
	if !strings.Contains(user, source) {
		t.Fatal("user message was interpreted as Markdown")
	}
	assistant := session.messageMarkup(session.history[1], 1, 80)
	if !strings.Contains(assistant, `["sql-1"]`) || strings.Contains(tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetText(assistant).GetText(true), "## Heading") {
		t.Fatalf("formatted assistant response lost query regions or was not formatted: %q", assistant)
	}
	cached := session.cachedHistory
	session.streaming = ai.Result{Message: "## Live reply\n\n- **Details:** `users.id`"}
	session.renderChat()
	if session.cachedHistory != cached || session.history[1].Text != source {
		t.Fatal("stream formatting invalidated completed history")
	}
	if !strings.Contains(session.transcript.GetText(true), "• Details: users.id") {
		t.Fatal("streaming explanation is not Markdown formatted")
	}
}

func BenchmarkAIChatMarkdown(b *testing.B) {
	source := strings.Repeat("## Registered users\n\n- **App accounts:** `core_app.User`\n- **Profiles:** `core_app.UserProfile`\n\n", 20)
	b.ReportAllocs()
	for range b.N {
		chatMarkdown(source, 80)
	}
}

func TestAIMarkdownStylesDoNotLeakIntoFollowingLines(t *testing.T) {
	view := tview.NewTextView().SetDynamicColors(true).SetText(chatMarkdown("## A heading\n\n- **Label:** `users.id`\nNormal text", 50))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(50, 10)
	view.SetRect(0, 0, 50, 10)
	view.Draw(screen)
	x, y, found := findAIChatText(screen, "Normal text")
	if !found {
		t.Fatal("normal text not visible")
	}
	_, _, style, _ := screen.GetContent(x, y)
	color, _, attributes := style.Decompose()
	if color != app.Styles.PrimaryTextColor || attributes&tcell.AttrBold != 0 {
		t.Fatal("Markdown style leaked into plain text")
	}
}
