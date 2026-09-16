package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

func TestLastCallEndsTurnWithoutChoices(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "1", Name: "say", Arguments: `{"text":"bye","last_call":true}`}}},
		{ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish", Arguments: "{}"}}},
	}}
	a := newTestAgent(t, client)
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if client.index != 1 {
		t.Fatalf("last_call should end the turn after one step, got %d completions", client.index)
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Text != "bye" {
		t.Fatalf("expected the emitted message, got %+v", sess.Messages)
	}
}

func TestLastCallIgnoredWhileChoicesEnabled(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "1", Name: "say", Arguments: `{"text":"hi","last_call":true}`}}},
		{ToolCalls: []llm.ToolCall{{ID: "2", Name: "choices", Arguments: "{}"}}},
	}}
	a := newTestAgentCfg(t, client, Config{ChoicesEnabled: true, MaxSteps: 5, Temperature: 1, MaxTokens: 256, ToolChoice: "auto"})
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if client.index != 2 {
		t.Fatalf("last_call must not end the turn while choices is enabled, got %d completions", client.index)
	}
}

func TestActiveToolsRespectsDisabled(t *testing.T) {
	client := &fakeClient{}
	a := newTestAgentCfg(t, client, Config{
		ToolChoice: "auto", MaxSteps: 1, DisabledTools: []string{"lookup"},
	})
	sess := &store.Session{ID: "abc"}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	for _, tool := range a.activeTools() {
		if tool.Name == "lookup" || tool.Name == "choices" {
			t.Fatalf("tool %q should not be active", tool.Name)
		}
	}
	prompt := a.SystemPrompt(sess)
	if strings.Contains(prompt, "- lookup:") {
		t.Fatal("disabled tool leaked into the system prompt")
	}
	if strings.Contains(prompt, "- choices:") {
		t.Fatal("choices leaked into the system prompt while disabled")
	}

	// The request must not advertise the disabled tool either.
	if err := a.Continue(context.Background(), sess, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) == 0 {
		t.Fatal("expected a completion request")
	}
	for _, def := range client.requests[0].Tools {
		if def.Name == "lookup" {
			t.Fatal("disabled tool was sent to the model")
		}
	}
}
