package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

type fakeClient struct {
	responses []*llm.Response
	index     int
	requests  []llm.Request
}

func (f *fakeClient) Complete(_ context.Context, req llm.Request) (*llm.Response, error) {
	f.requests = append(f.requests, req)
	if f.index >= len(f.responses) {
		return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "auto", Name: "finish"}}}, nil
	}
	r := f.responses[f.index]
	f.index++
	return r, nil
}

func newTestAgent(t *testing.T, client llm.Client) *Agent {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Register(&Tool{
		Name: "say", Description: "say", Category: "test",
		Handler: func(_ context.Context, tc *TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(args, &a)
			tc.Show(&store.Message{Role: llm.RoleAssistant, Kind: store.KindSpeech, Speaker: "Tester", Text: a.Text})
			return map[string]any{"ok": true}, nil
		},
	})
	reg.Register(&Tool{
		Name: "lookup", Description: "lookup", Category: "test", Query: true,
		Handler: func(context.Context, *TurnContext, json.RawMessage) (any, error) {
			return map[string]any{"value": 42}, nil
		},
	})
	reg.Register(&Tool{
		Name: "finish", Description: "finish", Category: "flow", Terminal: true,
		Handler: func(context.Context, *TurnContext, json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	})
	return New(st, reg, func() (llm.Client, error) { return client, nil }, func() Config {
		return Config{ToolChoice: "auto", MaxSteps: 5, Temperature: 1, MaxTokens: 256, ParallelToolCalls: true}
	})
}

func TestContinueRunsToolsUntilTerminal(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{
			{ID: "1", Name: "lookup", Arguments: "{}"},
			{ID: "2", Name: "say", Arguments: `{"text":"hello"}`},
		}},
		{ToolCalls: []llm.ToolCall{{ID: "3", Name: "finish", Arguments: "{}"}}},
	}}
	a := newTestAgent(t, client)
	sess := &store.Session{ID: "abc", History: []llm.Message{{Role: llm.RoleUser, Content: "hi"}}}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}

	var events []Event
	if err := a.Continue(context.Background(), sess, func(e Event) { events = append(events, e) }); err != nil {
		t.Fatal(err)
	}
	if client.index != 2 {
		t.Fatalf("expected 2 completions, got %d", client.index)
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Text != "hello" {
		t.Fatalf("unexpected messages: %+v", sess.Messages)
	}
	if len(sess.History) != 6 {
		t.Fatalf("expected 6 history entries, got %d: %+v", len(sess.History), sess.History)
	}
	var sawToolEnd, sawMessage bool
	for _, e := range events {
		switch e.Type {
		case EventToolEnd:
			sawToolEnd = true
		case EventMessage:
			sawMessage = true
		}
	}
	if !sawToolEnd || !sawMessage {
		t.Fatalf("missing expected events: %+v", events)
	}
	// system + user + assistant(tool calls) + two tool results
	if len(client.requests[1].Messages) != 5 {
		t.Fatalf("second request should include the tool exchange, got %d messages", len(client.requests[1].Messages))
	}
}

func TestContinueFallsBackToPlainText(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{{Content: "plain narration"}}}
	a := newTestAgent(t, client)
	sess := &store.Session{ID: "abc", History: []llm.Message{{Role: llm.RoleUser, Content: "hi"}}}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	var notice bool
	if err := a.Continue(context.Background(), sess, func(e Event) {
		if e.Type == EventNotice {
			notice = true
		}
	}); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Kind != store.KindNarration {
		t.Fatalf("expected salvaged narration, got %+v", sess.Messages)
	}
	if !notice {
		t.Fatal("expected a protocol notice event")
	}
}

func TestContinueReportsUnknownTool(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "1", Name: "nope", Arguments: "{}"}}},
	}}
	a := newTestAgent(t, client)
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	var foundError bool
	for _, m := range sess.History {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "unknown tool") {
			foundError = true
		}
	}
	if !foundError {
		t.Fatalf("expected a tool error in history, got %+v", sess.History)
	}
}

func TestTrimHistoryKeepsToolPairs(t *testing.T) {
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "a"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "x"}}},
		{Role: llm.RoleTool, ToolCallID: "1"},
		{Role: llm.RoleUser, Content: "b"},
	}
	got := trimHistory(history, 2)
	if len(got) != 2 || got[0].Role != llm.RoleTool {
		// start advanced past the orphaned tool message
		if len(got) != 1 || got[0].Role != llm.RoleUser {
			t.Fatalf("unexpected trim result: %+v", got)
		}
	}
}

func TestRegistryDefsOrderAndMeta(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{Name: "b", Description: "second", Category: "x"})
	reg.Register(&Tool{Name: "a", Description: "first", Category: "y"})
	defs := reg.Defs()
	if len(defs) != 2 || defs[0].Name != "b" || defs[1].Name != "a" {
		t.Fatalf("unexpected defs order: %+v", defs)
	}
	meta := reg.Meta()
	if meta[0].Parameters == nil {
		t.Fatal("missing default parameters schema")
	}
}
