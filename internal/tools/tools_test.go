package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func TestWritingBlockKeepsOrderAndThoughts(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{{ID: "c1", Name: "Ilyra"}}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := writingBlockTool()

	raw := `{"speaker":"Ilyra","blocks":[
	  {"type":"text","text":"她向前一步。"},
	  {"type":"thought","text":"他太年轻了。"},
	  {"type":"text","text":"站住。"}
	]}`
	if _, err := tool.Handler(context.Background(), tc, json.RawMessage(raw)); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(sess.Messages))
	}
	m := sess.Messages[0]
	if len(m.Blocks) != 3 {
		t.Fatalf("want 3 blocks, got %+v", m.Blocks)
	}
	if m.Blocks[0].Type != store.BlockText || m.Blocks[0].Text != "她向前一步。" {
		t.Fatalf("block 0 = %+v", m.Blocks[0])
	}
	if m.Blocks[1].Type != store.BlockThought || m.Blocks[1].Text != "他太年轻了。" {
		t.Fatalf("block 1 = %+v", m.Blocks[1])
	}
	if m.Blocks[2].Type != store.BlockText || m.Blocks[2].Text != "站住。" {
		t.Fatalf("block 2 = %+v", m.Blocks[2])
	}
	if !strings.Contains(m.Text, "她向前一步。") || !strings.Contains(m.Text, "站住。") {
		t.Fatalf("joined prose = %q", m.Text)
	}
	if strings.Contains(m.Text, "他太年轻了。") {
		t.Fatalf("a thought must not leak into the joined prose: %q", m.Text)
	}
	if m.Thought != "" {
		t.Fatal("thoughts belong in blocks, not the legacy field")
	}
	if !tc.PlayerFacing {
		t.Fatal("a beat with prose should count as player facing")
	}

	// A thoughts-only beat is allowed but is not the player-facing beat.
	tc2 := &agent.TurnContext{Session: &store.Session{}, Emit: func(agent.Event) {}}
	if _, err := tool.Handler(context.Background(), tc2, json.RawMessage(`{"speaker":"Ilyra","blocks":[{"type":"thought","text":"嗯。"}]}`)); err != nil {
		t.Fatal(err)
	}
	if tc2.PlayerFacing {
		t.Fatal("thoughts alone must not count as player facing")
	}
	if _, err := tool.Handler(context.Background(), tc2, json.RawMessage(`{"speaker":"Ilyra","blocks":[]}`)); err == nil {
		t.Fatal("expected an error when there are no blocks")
	}
	if _, err := tool.Handler(context.Background(), tc2, json.RawMessage(`{"speaker":"Ilyra","blocks":[{"type":"text","text":"   "}]}`)); err == nil {
		t.Fatal("expected an error when every block is blank")
	}
}

// A beat with no speaker must fail loudly: quietly falling back to the first
// character is how one character's lines get attributed to another.
func TestWritingBlockRequiresSpeaker(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{{ID: "c1", Name: "Ilyra"}}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	if _, err := writingBlockTool().Handler(context.Background(), tc,
		json.RawMessage(`{"blocks":[{"type":"text","text":"站住。"}]}`)); err == nil {
		t.Fatal("expected an error when the speaker is missing")
	}
	if len(sess.Messages) != 0 {
		t.Fatalf("no message may be attributed without a speaker: %+v", sess.Messages)
	}
}

// Models drop titles. "奥尔多" must still land on "奥尔多修士" so the avatar and
// grouping stay right, while an unknown name is left alone and stays visible.
func TestWritingBlockMatchesCastName(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{
		{ID: "c1", Name: "瑟蕾丝·维恩"},
		{ID: "c2", Name: "奥尔多修士"},
	}}
	tc := &agent.TurnContext{Session: sess, Emit: func(agent.Event) {}}
	tool := writingBlockTool()
	call := func(speaker string) string {
		t.Helper()
		arg, err := json.Marshal(map[string]any{
			"speaker": speaker,
			"blocks":  []map[string]string{{"type": "text", "text": "…"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tool.Handler(context.Background(), tc, arg); err != nil {
			t.Fatal(err)
		}
		return sess.Messages[len(sess.Messages)-1].Speaker
	}
	if got := call("奥尔多"); got != "奥尔多修士" {
		t.Fatalf("partial name = %q, want the cast spelling", got)
	}
	if got := call("奥尔"); got != "奥尔多修士" {
		t.Fatalf("shorter prefix = %q, want the cast spelling", got)
	}
	if got := call("某位路人"); got != "某位路人" {
		t.Fatalf("unknown name = %q, want it left as written", got)
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
