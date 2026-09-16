package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func TestMessageToolDefaultsAndKinds(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{{ID: "c1", Name: "Ilyra"}}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := messageTool()

	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"kind":"narration","text":"Rain."}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"text":"Hello."}`)); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(sess.Messages))
	}
	if sess.Messages[0].Speaker != "Narrator" || sess.Messages[0].Kind != store.KindNarration {
		t.Fatalf("bad narration: %+v", sess.Messages[0])
	}
	if sess.Messages[1].Speaker != "Ilyra" || sess.Messages[1].Kind != store.KindSpeech {
		t.Fatalf("bad speech: %+v", sess.Messages[1])
	}
	if !tc.PlayerFacing {
		t.Fatal("PlayerFacing should be set by the message tool")
	}
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"text":"  "}`)); err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestThinkIsPrivate(t *testing.T) {
	sess := &store.Session{}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	if _, err := thinkTool().Handler(context.Background(), tc, json.RawMessage(`{"text":"hm"}`)); err != nil {
		t.Fatal(err)
	}
	if tc.PlayerFacing {
		t.Fatal("think must not count as player facing")
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Kind != store.KindThought {
		t.Fatalf("bad thought: %+v", sess.Messages)
	}
}

func TestChoicesTool(t *testing.T) {
	tool := choicesTool()
	if !tool.Terminal {
		t.Fatal("choices must be terminal")
	}
	sess := &store.Session{}
	var got []store.Choice
	tc := &agent.TurnContext{Session: sess, Emit: func(e agent.Event) {
		if e.Type == agent.EventChoices {
			got = e.Choices
		}
	}}
	_, err := tool.Handler(context.Background(), tc, json.RawMessage(
		`{"narration":"The door creaks.","choices":[{"text":"Go in"},{"text":"Wait"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 choices, got %d", len(got))
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Kind != store.KindNarration {
		t.Fatalf("bad narration message: %+v", sess.Messages)
	}
	if !tc.ChoicesOffered {
		t.Fatal("ChoicesOffered should be set")
	}
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"choices":[]}`)); err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestUpdateStateTool(t *testing.T) {
	sess := &store.Session{State: map[string]any{}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := updateStateTool()
	call := func(args string) {
		t.Helper()
		if _, err := tool.Handler(context.Background(), tc, json.RawMessage(args)); err != nil {
			t.Fatalf("handler error for %s: %v", args, err)
		}
	}
	call(`{"path":"inventory.gold","op":"set","value":10}`)
	call(`{"path":"inventory.gold","op":"add","value":5}`)
	call(`{"path":"flags.seen","op":"toggle"}`)

	if v, _ := getPath(sess.State, "inventory.gold"); toFloatOrZero(v) != 15 {
		t.Fatalf("gold = %v, want 15", v)
	}
	if v, _ := getPath(sess.State, "flags.seen"); v != true {
		t.Fatalf("seen = %v, want true", v)
	}
}

func toFloatOrZero(v any) float64 {
	f, _ := toFloat(v)
	return f
}
