package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

type completionFunc func(context.Context, string) (string, error)

func (complete completionFunc) Complete(ctx context.Context, prompt string) (string, error) {
	return complete(ctx, prompt)
}

func TestDisabledAIHasNoProviderSideEffects(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	for _, provider := range []string{"opencode-go", "chatgpt", "unknown"} {
		client, err := New(models.AIConfig{Provider: provider})
		if client != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("disabled %s: client=%v err=%v", provider, client, err)
		}
	}
}

func TestGenerateIncludesGroundedContextAndExplicitSQLOnly(t *testing.T) {
	request := Request{Dialect: "postgres", Database: "shop", Prompt: "Revenue by customer", Tables: []Table{{Name: "public.orders", Columns: []Column{{Name: "customer_id", Type: "bigint", Nullable: "NO", Comment: "Customer identifier"}}, PrimaryKeys: []string{"id"}, Relationships: []Relationship{{Constraint: "orders_customer", FromTable: "public.orders", FromColumn: "customer_id", ToTable: "public.customers", ToColumn: "id"}}}}}
	for _, selected := range []string{"", "SELECT 'sensitive-literal'"} {
		request.SelectedSQL = selected
		client := completionFunc(func(_ context.Context, prompt string) (string, error) {
			for _, expected := range []string{"postgres", "shop", "public.orders", "customer_id", "bigint", "Customer identifier", "orders_customer", "public.customers", "Do not invent", "composite keys", "untrusted data"} {
				if !strings.Contains(prompt, expected) {
					t.Errorf("missing context %q", expected)
				}
			}
			if strings.Contains(prompt, "sensitive-literal") != (selected != "") {
				t.Fatal("selected SQL privacy boundary violated")
			}
			return `{"sql":"SELECT customer_id FROM public.orders;","clarification":"","missing":"","warning":""}`, nil
		})
		result, err := Generate(context.Background(), client, request)
		if err != nil || result.SQL == "" {
			t.Fatalf("generate: %+v %v", result, err)
		}
	}
}

func TestGenerateBoundsAndClarification(t *testing.T) {
	calls := 0
	client := completionFunc(func(context.Context, string) (string, error) {
		calls++
		return `{"sql":"","clarification":"What defines active?","missing":"","warning":""}`, nil
	})
	if _, err := Generate(context.Background(), client, Request{Prompt: strings.Repeat("x", MaxContextBytes)}); err == nil || calls != 0 {
		t.Fatal("oversize context must fail before sending")
	}
	if _, err := Generate(context.Background(), client, Request{}); err == nil || calls != 0 {
		t.Fatal("empty requests must fail before sending")
	}
	first, err := Generate(context.Background(), client, Request{Prompt: "Active users"})
	if err != nil || first.Clarification == "" {
		t.Fatal("first clarification should be allowed")
	}
	if _, err := Generate(context.Background(), client, Request{Prompt: "Active users", Clarification: first.Clarification, Answer: "Recently active"}); err == nil {
		t.Fatal("second clarification should stop")
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
