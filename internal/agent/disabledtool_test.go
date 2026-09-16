package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// A tool switched off for a story must not run even if the model asks for it.
func TestDisabledToolCallIsRefused(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "1", Name: "lookup", Arguments: "{}"}}},
		{ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish", Arguments: "{}"}}},
	}}
	a := newTestAgentCfg(t, client, Config{ToolChoice: "auto", MaxSteps: 3, DisabledTools: []string{"lookup"}})
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	var refused bool
	for _, m := range sess.History {
		if m.Role != llm.RoleTool {
			continue
		}
		if strings.Contains(m.Content, "42") {
			t.Fatalf("disabled tool still executed: %q", m.Content)
		}
		if strings.Contains(m.Content, "not available") {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("expected a refusal in the transcript: %+v", sess.History)
	}
}

// A disabled choices tool must not be able to end the turn or persist branches.
func TestDisabledChoicesCannotEndTurn(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "1", Name: "choices", Arguments: "{}"}}},
		{ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish", Arguments: "{}"}}},
	}}
	a := newTestAgentCfg(t, client, Config{ToolChoice: "auto", MaxSteps: 3, DisabledTools: []string{"choices"}})
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if client.index != 2 {
		t.Fatalf("disabled choices ended the turn after %d completions, want 2", client.index)
	}
	if len(sess.PendingChoices) != 0 {
		t.Fatalf("disabled choices persisted branches: %+v", sess.PendingChoices)
	}
}
