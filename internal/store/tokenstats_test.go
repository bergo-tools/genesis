package store

import (
	"testing"

	"github.com/zp/genesis/internal/llm"
)

func TestSessionAddUsageAccumulates(t *testing.T) {
	var s Session
	s.AddUsage(llm.Usage{PromptTokens: 100, CompletionTokens: 20, CachedTokens: 40, Cost: 0.001})
	s.AddUsage(llm.Usage{PromptTokens: 150, CompletionTokens: 30, CachedTokens: 90, Cost: 0.002})
	got := s.Tokens
	if got.LastPromptTokens != 150 || got.LastCompletionTokens != 30 || got.LastCachedTokens != 90 {
		t.Fatalf("last stats wrong: %+v", got)
	}
	if got.TotalPromptTokens != 250 || got.TotalCompletionTokens != 50 || got.TotalCachedTokens != 130 {
		t.Fatalf("totals wrong: %+v", got)
	}
	if got.Requests != 2 {
		t.Fatalf("requests = %d, want 2", got.Requests)
	}
	if got.TotalCost < 0.0029 || got.TotalCost > 0.0031 {
		t.Fatalf("cost = %v, want ~0.003", got.TotalCost)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}
}
