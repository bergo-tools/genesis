package llm

import (
	"testing"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
)

func TestUsageFromSDKReadsCacheAndCost(t *testing.T) {
	cached := int64(40)
	reasoning := int64(7)
	cost := 0.0012
	u := &components.ChatUsage{
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
		PromptTokensDetails: optionalnullable.From(&components.ChatUsagePromptTokensDetails{
			CachedTokens: &cached,
		}),
		CompletionTokensDetails: optionalnullable.From(&components.ChatUsageCompletionTokensDetails{
			ReasoningTokens: optionalnullable.From(&reasoning),
		}),
		Cost: optionalnullable.From(&cost),
	}
	got := usageFromSDK(u)
	if got == nil {
		t.Fatal("usageFromSDK returned nil")
	}
	if got.PromptTokens != 100 || got.CompletionTokens != 20 || got.TotalTokens != 120 {
		t.Fatalf("base counts wrong: %+v", got)
	}
	if got.CachedTokens != 40 {
		t.Fatalf("cached tokens = %d, want 40", got.CachedTokens)
	}
	if got.ReasoningTokens != 7 {
		t.Fatalf("reasoning tokens = %d, want 7", got.ReasoningTokens)
	}
	if got.Cost != cost {
		t.Fatalf("cost = %v, want %v", got.Cost, cost)
	}
	if usageFromSDK(nil) != nil {
		t.Fatal("nil input should produce nil usage")
	}
}
