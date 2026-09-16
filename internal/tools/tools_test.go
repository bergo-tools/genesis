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

func TestUpdateStateTracksBatch(t *testing.T) {
	sess := &store.Session{State: map[string]any{}}
	var tracker map[string]any
	tc := &agent.TurnContext{Session: sess, Emit: func(e agent.Event) {
		if e.Tracker != nil {
			tracker = e.Tracker
		}
	}}
	tool := updateStateTool()
	_, err := tool.Handler(context.Background(), tc, json.RawMessage(
		`{"changes":[{"key":"inventory.gold","value":10},{"key":"flags.seen","op":"toggle"}],"values":{"cast.ilyra.trust":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(tracker) != 3 {
		t.Fatalf("expected 3 tracked keys, got %+v", tracker)
	}
	if v, _ := getPath(sess.State, "inventory.gold"); toFloatOrZero(v) != 10 {
		t.Fatalf("gold = %v", v)
	}
	if v, _ := getPath(sess.State, "flags.seen"); v != true {
		t.Fatalf("seen = %v", v)
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Kind != store.KindState {
		t.Fatalf("expected a tracker message, got %+v", sess.Messages)
	}
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(`{"changes":[]}`)); err == nil {
		t.Fatal("expected an error when nothing changes")
	}
}

func TestSceneToolUpdatesAndEmits(t *testing.T) {
	sess := &store.Session{}
	var got *store.Scene
	tc := &agent.TurnContext{Session: sess, Emit: func(e agent.Event) {
		if e.Type == agent.EventScene {
			got = e.Scene
		}
	}}
	if _, err := sceneTool().Handler(context.Background(), tc, json.RawMessage(`{"location":"灰烬堡","time":"黄昏"}`)); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Location != "灰烬堡" || got.Time != "黄昏" {
		t.Fatalf("bad scene event: %+v", got)
	}
	if sess.Scene.Location != "灰烬堡" {
		t.Fatalf("session scene not updated: %+v", sess.Scene)
	}
	if len(sess.Messages) != 1 || sess.Messages[0].Kind != store.KindScene {
		t.Fatalf("expected a scene message, got %+v", sess.Messages)
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

func toFloatOrZero(v any) float64 {
	f, _ := toFloat(v)
	return f
}
