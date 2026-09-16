package server

import (
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// A re-roll must put the scene and world state back the way they were before
// the turn ran, otherwise the scene bar drifts out of sync with the messages.
func TestTruncateFromUserRestoresSnapshot(t *testing.T) {
	snap := &store.TurnSnapshot{
		Scene: store.Scene{Location: "Harbour", Time: "dusk"},
		State: map[string]any{"gold": 3},
	}
	sess := &store.Session{
		Messages: []*store.Message{
			{ID: "u1", Role: "user", Kind: store.KindUser, Text: "a", Snapshot: snap},
			{ID: "a1", Role: "assistant", Kind: store.KindSpeech, Text: "A"},
			{ID: "u2", Role: "user", Kind: store.KindUser, Text: "b"},
		},
		History: []llm.Message{
			{Role: llm.RoleUser, Content: "a", Ref: "u1"},
			{Role: llm.RoleUser, Content: "b", Ref: "u2"},
		},
		// The scene drifted after the turn ran.
		Scene: store.Scene{Location: "Palace", Time: "midnight"},
		State: map[string]any{"gold": 99},
	}
	if !truncateFromUser(sess, "u1") {
		t.Fatal("truncateFromUser should succeed")
	}
	if sess.Scene.Location != "Harbour" || sess.Scene.Time != "dusk" {
		t.Fatalf("scene not restored: %+v", sess.Scene)
	}
	if gold, ok := sess.State["gold"].(float64); !ok || gold != 3 {
		t.Fatalf("state not restored: %+v", sess.State)
	}
}

// turnBackup is what makes a failed re-roll recoverable.
func TestTurnBackupRestore(t *testing.T) {
	sess := &store.Session{
		Messages:       []*store.Message{{ID: "u1", Text: "one"}},
		History:        []llm.Message{{Role: llm.RoleUser, Content: "one"}},
		PendingChoices: []store.Choice{{Text: "x"}},
		PendingPrompt:  "p",
		Scene:          store.Scene{Location: "Harbour"},
		State:          map[string]any{"gold": 3},
	}
	backup := snapshotTurn(sess)
	// Simulate the destructive part of a re-roll.
	sess.Messages = append(sess.Messages, &store.Message{ID: "n1", Text: "new"})
	sess.History = append(sess.History, llm.Message{Role: llm.RoleAssistant, Content: "new"})
	sess.PendingChoices = nil
	sess.PendingPrompt = ""
	sess.Scene = store.Scene{Location: "Palace"}
	sess.State["gold"] = 99
	backup.restore(sess)
	if len(sess.Messages) != 1 || sess.Messages[0].ID != "u1" {
		t.Fatalf("messages not restored: %+v", sess.Messages)
	}
	if len(sess.History) != 1 || sess.History[0].Content != "one" {
		t.Fatalf("history not restored: %+v", sess.History)
	}
	if len(sess.PendingChoices) != 1 || sess.PendingPrompt != "p" {
		t.Fatalf("choices not restored: %+v %q", sess.PendingChoices, sess.PendingPrompt)
	}
	if sess.Scene.Location != "Harbour" {
		t.Fatalf("scene not restored: %+v", sess.Scene)
	}
	if gold, ok := sess.State["gold"].(float64); !ok || gold != 3 {
		t.Fatalf("state not restored: %+v", sess.State)
	}
}

// Editing keeps the OOC directive and works for sessions recorded before the
// transcript carried a Ref.
func TestSetUserMessageTextKeepsOocAndLegacySessions(t *testing.T) {
	sess := &store.Session{
		Messages: []*store.Message{{ID: "u1", Kind: store.KindUser, Text: "old", OOC: "be brief"}},
		History: []llm.Message{
			{Role: llm.RoleUser, Content: "[system] reminder"},
			{Role: llm.RoleUser, Content: "old\n\n[OOC] be brief"},
		},
	}
	setUserMessageText(sess, "u1", "new")
	want := "new\n\n[OOC] be brief"
	if got := sess.History[1].Content; got != want {
		t.Fatalf("legacy transcript = %q, want %q", got, want)
	}
	if sess.History[0].Content != "[system] reminder" {
		t.Fatal("the agent reminder must not be rewritten")
	}
}
