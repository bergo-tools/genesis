package agent

import (
	"context"
	"testing"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// The usage event must carry the session's running token stats so the UI can
// show context occupancy without another round trip.
func TestUsageEventCarriesSessionStats(t *testing.T) {
	client := &fakeClient{responses: []*llm.Response{
		{Usage: &llm.Usage{PromptTokens: 100, CompletionTokens: 20, CachedTokens: 40}},
		{ToolCalls: []llm.ToolCall{{ID: "2", Name: "finish", Arguments: "{}"}}},
	}}
	a := newTestAgent(t, client)
	sess := &store.Session{ID: "abc", Settings: store.Settings{ChoicesEnabled: false, MaxSteps: 3}}
	if err := a.store.Create(sess); err != nil {
		t.Fatal(err)
	}
	var got *store.TokenStats
	if err := a.Continue(context.Background(), sess, func(e Event) {
		if e.Type == EventUsage {
			got = e.Tokens
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("usage event did not carry token stats")
	}
	if got.TotalPromptTokens != 100 || got.LastCachedTokens != 40 || got.Requests != 1 {
		t.Fatalf("unexpected stats: %+v", got)
	}
	if sess.Tokens.TotalPromptTokens != 100 {
		t.Fatalf("session stats not updated: %+v", sess.Tokens)
	}
}
