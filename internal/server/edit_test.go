package server

import (
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

func TestTruncateFromUser(t *testing.T) {
	sess := &store.Session{
		Messages: []*store.Message{
			{ID: "u1", Role: "user", Kind: store.KindUser, Text: "a"},
			{ID: "a1", Role: "assistant", Kind: store.KindSpeech, Text: "A"},
			{ID: "u2", Role: "user", Kind: store.KindUser, Text: "b"},
			{ID: "a2", Role: "assistant", Kind: store.KindSpeech, Text: "B"},
		},
		History: []llm.Message{
			{Role: llm.RoleUser, Content: "a", Ref: "u1"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "t1", Name: "message"}}},
			{Role: llm.RoleTool, ToolCallID: "t1", Content: "{}"},
			{Role: llm.RoleUser, Content: "b", Ref: "u2"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "t2", Name: "choices"}}},
			{Role: llm.RoleTool, ToolCallID: "t2", Content: "{}"},
		},
		PendingChoices: []store.Choice{{Text: "x"}},
		PendingPrompt:  "p",
	}
	if !truncateFromUser(sess, "u1") {
		t.Fatal("expected truncateFromUser to report success")
	}
	if len(sess.Messages) != 1 || sess.Messages[0].ID != "u1" {
		t.Fatalf("messages not truncated: %+v", sess.Messages)
	}
	if len(sess.History) != 1 || sess.History[0].Ref != "u1" {
		t.Fatalf("history not truncated: %+v", sess.History)
	}
	if len(sess.PendingChoices) != 0 || sess.PendingPrompt != "" {
		t.Fatal("pending choices should be cleared")
	}
	if lastUserMessageID(sess) != "u1" {
		t.Fatalf("lastUserMessageID = %q", lastUserMessageID(sess))
	}
	if truncateFromUser(sess, "nope") {
		t.Fatal("unknown message should fail")
	}
}

func TestSetUserMessageText(t *testing.T) {
	sess := &store.Session{
		Messages: []*store.Message{{ID: "u1", Kind: store.KindUser, Text: "old"}},
		History:  []llm.Message{{Role: llm.RoleUser, Content: "old", Ref: "u1"}},
	}
	setUserMessageText(sess, "u1", "new")
	if sess.Messages[0].Text != "new" {
		t.Fatalf("display text not updated: %q", sess.Messages[0].Text)
	}
	if sess.History[0].Content != "new" {
		t.Fatalf("transcript not updated: %q", sess.History[0].Content)
	}
}
