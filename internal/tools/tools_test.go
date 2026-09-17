package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func TestMessageMergesThought(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{{ID: "c1", Name: "Ilyra"}}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := messageTool()

	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"speaker":"Ilyra","text":"站住。","thought":"他太年轻了。"}`)); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(sess.Messages))
	}
	m := sess.Messages[0]
	if m.Text != "站住。" || m.Thought != "他太年轻了。" || m.Kind != store.KindSpeech {
		t.Fatalf("bad message: %+v", m)
	}
	if !tc.PlayerFacing {
		t.Fatal("a message with text should count as player facing")
	}

	// A thought-only message is allowed but is not the player-facing beat.
	tc2 := &agent.TurnContext{Session: &store.Session{}, Emit: func(agent.Event) {}}
	if _, err := tool.Handler(context.Background(), tc2, json.RawMessage(`{"thought":"嗯。"}`)); err != nil {
		t.Fatal(err)
	}
	if tc2.PlayerFacing {
		t.Fatal("a thought-only message must not count as player facing")
	}
	if _, err := tool.Handler(context.Background(), tc2, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected an error when both text and thought are empty")
	}
}

func TestNarratorToolShowsNarration(t *testing.T) {
	sess := &store.Session{}
	var events []agent.Event
	tc := &agent.TurnContext{Session: sess, Emit: func(e agent.Event) { events = append(events, e) }}
	if _, err := narratorTool().Handler(context.Background(), tc, json.RawMessage(`{"text":"灰烬落在你肩上。"}`)); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(sess.Messages))
	}
	m := sess.Messages[0]
	if m.Kind != store.KindNarration || m.Speaker != "Narrator" || m.Text != "灰烬落在你肩上。" {
		t.Fatalf("bad narration: %+v", m)
	}
	if !tc.PlayerFacing {
		t.Fatal("narration should count as player facing")
	}
	if len(events) != 1 || events[0].Type != agent.EventMessage {
		t.Fatalf("expected one message event, got %+v", events)
	}
	if _, err := narratorTool().Handler(context.Background(), tc, json.RawMessage(`{"text":"   "}`)); err == nil {
		t.Fatal("expected an error for empty text")
	}
}

func TestChoicesPersistOnSession(t *testing.T) {
	if !choicesTool().Terminal {
		t.Fatal("choices must be terminal")
	}
	sess := &store.Session{}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	if _, err := choicesTool().Handler(context.Background(), tc, json.RawMessage(
		`{"narration":"门开了。","choices":[{"text":"进去"},{"text":"等"}]}`)); err != nil {
		t.Fatal(err)
	}
	if len(sess.PendingChoices) != 2 || sess.PendingPrompt != "门开了。" {
		t.Fatalf("choices not persisted on the session: %+v", sess.PendingChoices)
	}
}
