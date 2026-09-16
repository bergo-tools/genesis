package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func thinkTool() *agent.Tool {
	return &agent.Tool{
		Name:     "think",
		Category: "narrative",
		Description: "Record a character's private inner thought. It is shown to the player as a dimmed " +
			"thought bubble, so keep it in that character's voice: subtext, doubt, want, or a plan. " +
			"Use it before deciding how the character acts.",
		Parameters: object(map[string]any{
			"speaker":   stringProp("Name of the character whose head we are in. Defaults to the lead character."),
			"text":      stringProp("The inner thought."),
			"last_call": lastCallProp(),
		}, "text"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Speaker string `json:"speaker"`
				Text    string `json:"text"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			text := strings.TrimSpace(a.Text)
			if text == "" {
				return nil, errors.New("text is required")
			}
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    store.KindThought,
				Speaker: resolveSpeaker(tc, a.Speaker, store.KindThought),
				Text:    text,
			})
			return map[string]any{"ok": true}, nil
		},
	}
}
