package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func offerChoicesTool() *agent.Tool {
	return &agent.Tool{
		Name:     "offer_choices",
		Category: "flow",
		Description: "Offer the player a small set of concrete suggested actions. Use this to make the " +
			"next decision legible. This does not end the turn by itself.",
		Parameters: object(map[string]any{
			"prompt": stringProp("Optional question or prompt shown above the choices."),
			"choices": arrayProp("Two to four suggested actions.", object(map[string]any{
				"text":        stringProp("Short imperative label."),
				"description": stringProp("Optional one-line detail."),
			}, "text")),
		}, "choices"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Prompt  string         `json:"prompt"`
				Choices []store.Choice `json:"choices"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if len(a.Choices) > 6 {
				a.Choices = a.Choices[:6]
			}
			if strings.TrimSpace(a.Prompt) != "" {
				tc.Show(&store.Message{Role: "assistant", Kind: store.KindPrompt, Speaker: tc.PrimaryName(), Text: strings.TrimSpace(a.Prompt)})
			}
			if tc.Emit != nil {
				tc.Emit(agent.Event{Type: agent.EventChoices, Step: tc.Step, Prompt: a.Prompt, Choices: a.Choices})
			}
			return map[string]any{"ok": true}, nil
		},
	}
}

func awaitPlayerTool() *agent.Tool {
	return &agent.Tool{
		Name:     "await_player",
		Category: "flow",
		Terminal: true,
		Description: "End your turn and hand control back to the player. Call this once the scene is " +
			"ready for the player to act.",
		Parameters: object(map[string]any{
			"prompt": stringProp("Optional in-character question or nudge addressed to the player."),
		}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Prompt string `json:"prompt"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Prompt) != "" {
				tc.Show(&store.Message{Role: "assistant", Kind: store.KindPrompt, Speaker: tc.PrimaryName(), Text: strings.TrimSpace(a.Prompt)})
			}
			return map[string]any{"ok": true, "awaiting": "player"}, nil
		},
	}
}

func endTurnTool() *agent.Tool {
	return &agent.Tool{
		Name:        "end_turn",
		Category:    "flow",
		Terminal:    true,
		Description: "End your turn when the scene continues on its own and the player does not need to respond.",
		Parameters:  object(map[string]any{"note": stringProp("Optional private note about what happens next.")}),
		Handler: func(_ context.Context, _ *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Note string `json:"note"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
	}
}
