package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestDisabledAIHasNoProviderSideEffects(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	for _, provider := range []string{"opencode-go", "chatgpt", "unknown"} {
		client, err := New(models.AIConfig{Provider: provider})
		if client != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("disabled %s: client=%v err=%v", provider, client, err)
		}
	}
}

func TestDiscoveryIncludesGroundedContextAndExplicitSQLOnly(t *testing.T) {
	request := Request{Dialect: "postgres", Database: "shop", Prompt: "Revenue by customer", Tables: []Table{{Name: "public.orders", Columns: []Column{{Name: "customer_id", Type: "bigint", Nullable: "NO", Comment: "Customer identifier"}}, PrimaryKeys: []string{"id"}, Relationships: []Relationship{{Constraint: "orders_customer", FromTable: "public.orders", FromColumn: "customer_id", ToTable: "public.customers", ToColumn: "id"}}}}}
	for _, selected := range []string{"", "SELECT 'sensitive-literal'"} {
		request.SelectedSQL = selected
		client := stepFunc(func(_ context.Context, instructions string, messages []Message, _ []Tool) (Turn, error) {
			prompt := instructions + messages[0].Text
			for _, expected := range []string{"postgres", "shop", "public.orders", "customer_id", "bigint", "Customer identifier", "orders_customer", "public.customers", "Never invent", "composite foreign keys", "untrusted data"} {
				if !strings.Contains(prompt, expected) {
					t.Errorf("missing context %q", expected)
				}
			}
			if strings.Contains(prompt, "sensitive-literal") != (selected != "") {
				t.Fatal("selected SQL privacy boundary violated")
			}
			return Turn{Text: `{"sql":"SELECT customer_id FROM public.orders;","clarification":"","missing":"","warning":""}`}, nil
		})
		result, err := DiscoverSQL(context.Background(), client, request, nil)
		if err != nil || result.SQL == "" {
			t.Fatalf("generate: %+v %v", result, err)
		}
	}
}

func TestParseResultRejectsAmbiguousAndOversizedOutput(t *testing.T) {
	for _, output := range []string{"", "SELECT * FROM users", `{"sql":"SELECT 1","clarification":"Which database?"}`, `{"warning":"done"}`, strings.Repeat("x", MaxResponseBytes+1)} {
		if _, err := parseResult(output); err == nil {
			t.Fatalf("accepted invalid output: %.80s", output)
		}
	}
	result, err := parseResult("```json\n{\"sql\":\" SELECT 1; \",\"clarification\":\"\",\"missing\":\"\",\"warning\":\"\"}\n```")
	if err != nil || result.SQL != "SELECT 1;" {
		t.Fatalf("fenced JSON: %+v %v", result, err)
	}
}

func TestDiscoveryValidatesPromptBeforeCallingProvider(t *testing.T) {
	calls := 0
	client := stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) {
		calls++
		return Turn{Text: generatedResult}, nil
	})
	if _, err := DiscoverSQL(context.Background(), client, Request{}, nil); err == nil || calls != 0 {
		t.Fatal("empty request reached provider")
	}
	if _, err := DiscoverSQL(context.Background(), client, Request{Prompt: strings.Repeat("x", 120000)}, nil); err != nil || calls != 1 {
		t.Fatal("large context must reach provider")
	}
}
