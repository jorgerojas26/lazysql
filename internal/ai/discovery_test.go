package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type stepFunc func(context.Context, string, []Message, []Tool) (Turn, error)

func (step stepFunc) Step(ctx context.Context, instructions string, messages []Message, tools []Tool) (Turn, error) {
	return step(ctx, instructions, messages, tools)
}

const generatedResult = `{"sql":"SELECT id FROM public.orders;","clarification":"","missing":"","warning":""}`

func discoveryCall(id, name, arguments string) ToolCall {
	return ToolCall{ID: id, Name: name, Arguments: json.RawMessage(arguments)}
}

func TestDiscoveryFollowsMetadataWithoutPreloadingSchema(t *testing.T) {
	round := 0
	client := stepFunc(func(_ context.Context, instructions string, messages []Message, tools []Tool) (Turn, error) {
		if len(tools) != 4 || !strings.Contains(instructions, "Never execute SQL") {
			t.Fatal("missing restricted tools or instructions")
		}
		round++
		switch round {
		case 1:
			if strings.Contains(messages[0].Text, "hidden_metadata") || !strings.Contains(messages[0].Text, "postgres") || !strings.Contains(messages[0].Text, "public") {
				t.Fatal("initial context is not lightweight")
			}
			return Turn{Calls: []ToolCall{discoveryCall("list", "list_tables", `{"search":"orders"}`)}}, nil
		case 2:
			if !strings.Contains(messages[len(messages)-1].Text, "public.orders") {
				t.Fatal("missing table listing")
			}
			return Turn{Calls: []ToolCall{discoveryCall("describe", "describe_tables", `{"names":["public.orders"]}`), discoveryCall("relations", "get_relationships", `{"names":["public.orders"]}`)}}, nil
		case 3:
			if !strings.Contains(messages[len(messages)-1].Text, "public.items") || !strings.Contains(messages[len(messages)-2].Text, "bigint") {
				t.Fatal("metadata or relationship tool results missing")
			}
			return Turn{Calls: []ToolCall{discoveryCall("related", "describe_tables", `{"names":["public.items"]}`)}}, nil
		default:
			return Turn{Text: generatedResult}, nil
		}
	})
	calls := atomic.Int32{}
	execute := func(_ context.Context, name string, args json.RawMessage) (any, error) {
		calls.Add(1)
		switch name {
		case "list_tables":
			return map[string]any{"tables": []string{"public.orders"}}, nil
		case "get_relationships":
			return []Relationship{{FromTable: "public.items", FromColumn: "order_id", ToTable: "public.orders", ToColumn: "id"}}, nil
		default:
			return []Table{{Name: "public.orders", Columns: []Column{{Name: "id", Type: "bigint"}}}}, nil
		}
	}
	result, err := DiscoverSQL(context.Background(), client, Request{Dialect: "postgres", Schemas: []string{"public"}, Prompt: "Order items", Tables: []Table{{Name: "hidden_metadata"}}}, execute)
	if err != nil || result.SQL == "" || round != 4 || calls.Load() != 4 {
		t.Fatalf("discovery = %+v, %v, %d rounds, %d calls", result, err, round, calls.Load())
	}
}

func TestDiscoveryRejectsForbiddenOrMalformedCallsBeforeExecution(t *testing.T) {
	for _, calls := range [][]ToolCall{
		{discoveryCall("1", "execute_sql", `{"sql":"DROP TABLE orders"}`)},
		{discoveryCall("1", "read_file", `{}`)},
		{discoveryCall("", "list_tables", `{}`)},
		{discoveryCall("1", "list_tables", `{}`), discoveryCall("1", "list_tables", `{}`)},
		{discoveryCall("1", "list_tables", `not json`)},
	} {
		client := stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) { return Turn{Calls: calls}, nil })
		_, err := DiscoverSQL(context.Background(), client, Request{Prompt: "Orders"}, func(context.Context, string, json.RawMessage) (any, error) {
			t.Fatal("forbidden call executed")
			return nil, nil
		})
		if err == nil {
			t.Fatal("invalid tool calls accepted")
		}
	}
}

func TestDiscoveryBoundsRoundsContextAndToolOutput(t *testing.T) {
	rounds := 0
	client := stepFunc(func(_ context.Context, _ string, messages []Message, _ []Tool) (Turn, error) {
		rounds++
		if rounds > 1 && !strings.Contains(messages[len(messages)-1].Text, "exceeds the limit") {
			t.Fatal("oversized metadata was not replaced with actionable error")
		}
		return Turn{Calls: []ToolCall{discoveryCall("list", "list_tables", `{}`)}}, nil
	})
	_, err := DiscoverSQL(context.Background(), client, Request{Prompt: "Orders"}, func(context.Context, string, json.RawMessage) (any, error) {
		return strings.Repeat("x", MaxToolBytes+1), nil
	})
	if err == nil || rounds != MaxDiscoveryRounds {
		t.Fatalf("unbounded rounds: %d %v", rounds, err)
	}
	client = stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) {
		t.Fatal("oversized input sent to provider")
		return Turn{}, nil
	})
	if _, err := DiscoverSQL(context.Background(), client, Request{Prompt: strings.Repeat("x", MaxContextBytes)}, nil); err == nil {
		t.Fatal("unbounded context accepted")
	}
}

func TestDiscoveryRunsIndependentToolsConcurrentlyAndCancels(t *testing.T) {
	started := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) {
		return Turn{Calls: []ToolCall{discoveryCall("a", "list_tables", `{}`), discoveryCall("b", "get_relationships", `{"names":["orders"]}`)}}, nil
	})
	done := make(chan error, 1)
	go func() {
		_, err := DiscoverSQL(ctx, client, Request{Prompt: "Orders"}, func(ctx context.Context, _ string, _ json.RawMessage) (any, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		})
		done <- err
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("independent calls were serialized")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("discovery did not cancel")
	}
}

func TestDiscoveryPreservesClarificationRulesAndProviderFailures(t *testing.T) {
	client := stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) {
		return Turn{Text: `{"sql":"","clarification":"What counts as active?","missing":"","warning":""}`}, nil
	})
	request := Request{Prompt: "Active users"}
	if result, err := DiscoverSQL(context.Background(), client, request, nil); err != nil || result.Clarification == "" {
		t.Fatalf("clarification: %+v %v", result, err)
	}
	request.Answer = "Last month"
	if result, err := DiscoverSQL(context.Background(), client, request, nil); err != nil || result.Clarification == "" {
		t.Fatal("chat clarification was not preserved")
	}
	failure := errors.New("provider unavailable")
	client = stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) { return Turn{}, failure })
	if _, err := DiscoverSQL(context.Background(), client, request, nil); !errors.Is(err, failure) {
		t.Fatal("provider failure lost")
	}
}

func TestDiscoveryRechecksPrematureMissingSchemaOnce(t *testing.T) {
	calls := 0
	client := stepFunc(func(_ context.Context, _ string, messages []Message, _ []Tool) (Turn, error) {
		calls++
		if calls == 1 {
			return Turn{Text: `{"sql":"","clarification":"","missing":"No Users table in acl","warning":""}`}, nil
		}
		if !strings.Contains(messages[len(messages)-1].Text, "other accessible databases") {
			t.Fatal("missing schema did not trigger wider discovery")
		}
		return Turn{Text: generatedResult}, nil
	})
	result, err := DiscoverSQL(context.Background(), client, Request{Prompt: "Last registered users", Database: "acl"}, nil)
	if err != nil || result.SQL == "" || calls != 2 {
		t.Fatalf("premature missing retry: %+v %v (%d calls)", result, err, calls)
	}
	calls = 0
	client = stepFunc(func(context.Context, string, []Message, []Tool) (Turn, error) {
		calls++
		return Turn{Text: `{"sql":"","clarification":"","missing":"No accessible user metadata","warning":""}`}, nil
	})
	result, err = DiscoverSQL(context.Background(), client, Request{Prompt: "Users"}, nil)
	if err != nil || result.Missing == "" || calls != 2 {
		t.Fatal("genuinely missing metadata was not bounded")
	}
}

func TestDiscoveryCarriesConversationForRefinementAndExplanation(t *testing.T) {
	history := []ChatMessage{{Role: "user", Text: "Last ten users"}, {Role: "assistant", Database: "app", SQL: "SELECT id FROM Users ORDER BY created_at DESC LIMIT 10;"}}
	client := stepFunc(func(_ context.Context, instructions string, messages []Message, _ []Tool) (Turn, error) {
		var request Request
		if err := json.Unmarshal([]byte(messages[0].Text), &request); err != nil {
			t.Fatal(err)
		}
		if request.Prompt != "Explain the ordering" || len(request.History) != 2 || request.History[1].SQL != history[1].SQL || !strings.Contains(instructions, "most recent generated SQL") {
			t.Fatal("follow-up lost conversation context")
		}
		return Turn{Text: `{"sql":"","clarification":"","missing":"","warning":"","message":"Newest registrations come first because created_at is sorted descending."}`}, nil
	})
	result, err := DiscoverSQL(context.Background(), client, Request{Prompt: "Explain the ordering", History: history}, nil)
	if err != nil || result.Message == "" || result.SQL != "" {
		t.Fatalf("conversational answer: %+v %v", result, err)
	}
}

func TestDiscoveryTrimsOldHistoryWithoutLosingLatestQuery(t *testing.T) {
	history := []ChatMessage{}
	for i := 0; i < 12; i++ {
		history = append(history, ChatMessage{Role: "user", Text: strings.Repeat("old conversation ", 1000)}, ChatMessage{Role: "assistant", SQL: "SELECT 1;"})
	}
	history = append(history, ChatMessage{Role: "user", Text: "Current request"}, ChatMessage{Role: "assistant", SQL: "SELECT email FROM Users;"})
	client := stepFunc(func(_ context.Context, _ string, messages []Message, _ []Tool) (Turn, error) {
		var request Request
		if err := json.Unmarshal([]byte(messages[0].Text), &request); err != nil {
			t.Fatal(err)
		}
		if !request.HistoryTruncated || len(request.History) >= len(history) || request.History[len(request.History)-1].SQL != "SELECT email FROM Users;" {
			t.Fatal("context pruning lost the latest query")
		}
		return Turn{Text: generatedResult}, nil
	})
	if _, err := DiscoverSQL(context.Background(), client, Request{Prompt: "Add names", History: history}, nil); err != nil {
		t.Fatal(err)
	}
	if len(history) != 26 || history[0].Text == "" {
		t.Fatal("caller history was mutated")
	}
}
