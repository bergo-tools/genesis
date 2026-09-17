package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func choicesTool() *agent.Tool {
	return &agent.Tool{
		Name:     "choices",
		Category: "flow",
		Terminal: true,
		Description: "Finish the turn by offering the player the next branches of the story. Optionally " +
			"include narration to set the scene, then two to four concrete choices. This is what hands " +
			"control back to the player, who can pick one or type something of their own.",
		Parameters: object(map[string]any{
			"narration": stringProp("Optional scene description or question shown to the player before the choices."),
			"choices": arrayProp("Two to four next steps for the player. Keep each one a short label.", object(map[string]any{
				"text": stringProp("Short imperative label, a handful of words."),
			}, "text")),
			"last_call": lastCallProp(),
		}, "choices"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Narration string         `json:"narration"`
				Choices   []store.Choice `json:"choices"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			cleaned := a.Choices[:0]
			for _, c := range a.Choices {
				if strings.TrimSpace(c.Text) == "" {
					continue
				}
				c.Text = strings.TrimSpace(c.Text)
				cleaned = append(cleaned, c)
			}
			if len(cleaned) == 0 {
				return nil, errors.New("at least one choice is required")
			}
			if len(cleaned) > 6 {
				cleaned = cleaned[:6]
			}
			narration := strings.TrimSpace(a.Narration)
			if narration != "" {
				tc.Show(&store.Message{
					Role:    "assistant",
					Kind:    store.KindNarration,
					Speaker: "Narrator",
					Text:    narration,
				})
			}
			tc.ChoicesOffered = true
			tc.Session.PendingChoices = cleaned
			tc.Session.PendingPrompt = narration
			if tc.Emit != nil {
				tc.Emit(agent.Event{
					Type:    agent.EventChoices,
					Step:    tc.Step,
					Prompt:  narration,
					Choices: cleaned,
				})
			}
			return map[string]any{"ok": true, "count": len(cleaned)}, nil
		},
	}
}
