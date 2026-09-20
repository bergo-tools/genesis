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
		Description: "Finish the turn by offering the player the next branches of the story: two to " +
			"four concrete choices. This is what hands control back to the player, who can pick one " +
			"or type something of their own.",
		Parameters: object(map[string]any{
			"choices": arrayProp("Two to four next steps for the player. Keep each one a short label.", object(map[string]any{
				"text": stringProp("Short imperative label, a handful of words."),
			}, "text")),
			"last_call": lastCallProp(),
		}, "choices"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Choices []store.Choice `json:"choices"`
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
			tc.ChoicesOffered = true
			tc.Session.PendingChoices = cleaned
			if tc.Emit != nil {
				tc.Emit(agent.Event{
					Type:    agent.EventChoices,
					Step:    tc.Step,
					Choices: cleaned,
				})
			}
			return map[string]any{"ok": true, "count": len(cleaned)}, nil
		},
	}
}
