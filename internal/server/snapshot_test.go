package server

import (
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// A re-roll must put the world state back the way it was before the turn ran,
// otherwise the world panel drifts out of sync with the messages.
func TestTruncateFromUserRestoresSnapshot(t *testing.T) {
	snap := &store.TurnSnapshot{State: map[string]any{"gold": 3}}
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
		// The world drifted after the turn ran.
		State: map[string]any{"gold": 99},
	}
	if !truncateFromUser(sess, "u1") {
		t.Fatal("truncateFromUser should succeed")
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
		State:          map[string]any{"gold": 3},
	}
	backup := snapshotTurn(sess)
	// Simulate the destructive part of a re-roll.
	sess.Messages = append(sess.Messages, &store.Message{ID: "n1", Text: "new"})
	sess.History = append(sess.History, llm.Message{Role: llm.RoleAssistant, Content: "new"})
	sess.PendingChoices = nil
	sess.State["gold"] = 99
	backup.restore(sess)
	if len(sess.Messages) != 1 || sess.Messages[0].ID != "u1" {
		t.Fatalf("messages not restored: %+v", sess.Messages)
	}
	if len(sess.History) != 1 || sess.History[0].Content != "one" {
		t.Fatalf("history not restored: %+v", sess.History)
	}
	if len(sess.PendingChoices) != 1 {
		t.Fatalf("choices not restored: %+v", sess.PendingChoices)
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
