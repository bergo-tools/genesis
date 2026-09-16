package agent

import (
	"strings"
	"testing"

	"github.com/zp/genesis/internal/llm"
)

func TestTrimHistoryHonoursTokenBudget(t *testing.T) {
	huge := strings.Repeat("word ", 4000) // roughly 5000 estimated tokens
	history := []llm.Message{
		{Role: llm.RoleUser, Content: huge},
		{Role: llm.RoleAssistant, Content: "recent"},
	}
	got := trimHistory(history, 0, 100)
	if len(got) != 1 || got[0].Content != "recent" {
		t.Fatalf("token budget did not trim the old message: %+v", got)
	}
	// The newest message is always kept, however large it is.
	got = trimHistory([]llm.Message{{Role: llm.RoleAssistant, Content: huge}}, 0, 1)
	if len(got) != 1 {
		t.Fatalf("the newest message must never be dropped: %+v", got)
	}
}
