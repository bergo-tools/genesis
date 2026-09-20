package server

import (
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

func TestTurnContentMarksChoices(t *testing.T) {
	cases := []struct {
		text, ooc string
		choice    bool
		want      string
	}{
		{"Use magic", "", false, "Use magic"},
		{"Use magic", "", true, "Use magic\n\n[choice]"},
		{"", "", true, ""},
		{"look", "be brief", true, "look\n\n[OOC] be brief\n\n[choice]"},
		{"", "be brief", true, "[OOC] be brief\n\n[choice]"},
	}
	for _, c := range cases {
		if got := turnContent(c.text, c.ooc, c.choice); got != c.want {
			t.Fatalf("turnContent(%q, %q, %v) = %q, want %q", c.text, c.ooc, c.choice, got, c.want)
		}
	}
}

// Editing a choice keeps the marker, so a re-run still tells the model to
// expand the action instead of treating it as prose the player typed.
func TestSetUserMessageTextKeepsChoiceMark(t *testing.T) {
	sess := &store.Session{
		Messages: []*store.Message{{ID: "u1", Kind: store.KindUser, Text: "Use magic", Choice: true}},
		History:  []llm.Message{{Role: llm.RoleUser, Content: "Use magic\n\n[choice]", Ref: "u1"}},
	}
	setUserMessageText(sess, "u1", "Cast the ward instead")
	want := "Cast the ward instead\n\n[choice]"
	if got := sess.History[0].Content; got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}
	if !sess.Messages[0].Choice {
		t.Fatal("the display message must stay marked as a choice")
	}
}
