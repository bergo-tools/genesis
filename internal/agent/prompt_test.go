package agent

import (
	"strings"
	"testing"

	"github.com/zp/genesis/internal/store"
)

// The writing brief is what keeps the prose vivid instead of clipped. If a
// future edit drops it the output goes flat again, so guard the lines that do
// the work.
func TestSystemPromptKeepsTheWritingBrief(t *testing.T) {
	a := newTestAgent(t, &fakeClient{})
	prompt := a.SystemPrompt(&store.Session{})
	for _, want := range []string{
		"Write like a novelist",
		"read like a page",
		"Be concrete",
		"Vary the rhythm",
		"subtext",
		"End on motion",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the writing brief lost %q", want)
		}
	}
	// There is no tool that records world state, so asking for it only makes
	// the model invent one.
	if strings.Contains(prompt, "Persist durable facts") {
		t.Fatal("the brief must not ask for state the tools cannot record")
	}
}
