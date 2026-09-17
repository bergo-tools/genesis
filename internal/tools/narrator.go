package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

// narratorTool is the omniscient voice: it paints the world rather than putting
// words in a character's mouth. It replaced the structured scene tool,
// which pinned the story to a small set of fields and read as a form to fill in.
func narratorTool() *agent.Tool {
	return &agent.Tool{
		Name:     "narrator",
		Category: "narrative",
		Description: "The narrator's voice. Describe the world and move the plot: what a place " +
			"looks, sounds and smells like, light and weather, time passing, what happens next, " +
			"and consequences the player cannot see yet. Use it to open a scene before the cast " +
			"acts, to spell out what the player's last choice actually did, and to bridge between " +
			"their beats. Never put dialogue here.",
		Parameters: object(map[string]any{
			"text":      stringProp("One short paragraph of scene or plot description."),
			"last_call": lastCallProp(),
		}, "text"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Text string `json:"text"`
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
				Kind:    store.KindNarration,
				Speaker: "Narrator",
				Text:    text,
			})
			return map[string]any{"ok": true}, nil
		},
	}
}
