package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func TestParseNotation(t *testing.T) {
	cases := []struct {
		in           string
		count, sides int
		modifier     int
		expectError  bool
	}{
		{"2d6+3", 2, 6, 3, false},
		{"1d20", 1, 20, 0, false},
		{"d8-2", 1, 8, -2, false},
		{"4d10", 4, 10, 0, false},
		{"", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
		{"0d6", 0, 0, 0, true},
		{"2d1", 0, 0, 0, true},
	}
	for _, c := range cases {
		count, sides, mod, err := parseNotation(c.in)
		if c.expectError {
			if err == nil {
				t.Fatalf("expected error for %q", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", c.in, err)
		}
		if count != c.count || sides != c.sides || mod != c.modifier {
			t.Fatalf("%q => %d,%d,%d want %d,%d,%d", c.in, count, sides, mod, c.count, c.sides, c.modifier)
		}
	}
}

func TestUpdateStateToolMutatesSession(t *testing.T) {
	sess := &store.Session{State: map[string]any{}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := updateStateTool()

	call := func(args string) {
		t.Helper()
		if _, err := tool.Handler(context.Background(), tc, json.RawMessage(args)); err != nil {
			t.Fatalf("handler error: %v", err)
		}
	}
	call(`{"path":"inventory.gold","op":"set","value":10}`)
	call(`{"path":"inventory.gold","op":"add","value":5}`)
	call(`{"path":"flags.visited","op":"toggle"}`)

	if got, _ := getPath(sess.State, "inventory.gold"); toFloatOrZero(got) != 15 {
		t.Fatalf("gold = %v, want 15", got)
	}
	if got, _ := getPath(sess.State, "flags.visited"); got != true {
		t.Fatalf("visited = %v, want true", got)
	}
}

func TestSendMessageToolEmits(t *testing.T) {
	sess := &store.Session{}
	var shown []*store.Message
	tc := &agent.TurnContext{Session: sess, Emit: func(e agent.Event) {
		if e.Type == agent.EventMessage {
			shown = append(shown, e.Message)
		}
	}}
	tool := sendMessageTool()
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"type":"narration","text":"The tide rises."}`)); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || shown[0].Kind != store.KindNarration || shown[0].Speaker != "Narrator" {
		t.Fatalf("unexpected message: %+v", shown)
	}
}

func toFloatOrZero(v any) float64 {
	f, _ := toFloat(v)
	return f
}
