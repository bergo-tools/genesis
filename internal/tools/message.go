package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func messageTool() *agent.Tool {
	return &agent.Tool{
		Name:     "message",
		Category: "narrative",
		Description: "Say something to the player as one of the story's characters. kind is speech " +
			"(quoted dialogue), action (a physical deed) or narration (world description). Call it " +
			"several times to build a scene beat by beat.",
		Parameters: object(map[string]any{
			"speaker": stringProp("Exact name of the character speaking or acting. Use Narrator for world description."),
			"text":    stringProp("The line or description. Do not wrap the whole line in quotes."),
			"kind":    enumProp("Message kind.", "speech", "action", "narration"),
			"mood":    stringProp("Optional short mood label for the speaker."),
		}, "text"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Speaker string `json:"speaker"`
				Text    string `json:"text"`
				Kind    string `json:"kind"`
				Mood    string `json:"mood"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			text := strings.TrimSpace(a.Text)
			if text == "" {
				return nil, errors.New("text is required")
			}
			kind := playerKind(a.Kind)
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    kind,
				Speaker: resolveSpeaker(tc, a.Speaker, kind),
				Text:    text,
				Mood:    strings.TrimSpace(a.Mood),
			})
			return map[string]any{"ok": true}, nil
		},
	}
}

// playerKind normalises a kind emitted by the message tool. It never returns
// "thought": inner monologue belongs to the think tool.
func playerKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "action", "act", "do":
		return store.KindAction
	case "narration", "narrate", "scene":
		return store.KindNarration
	default:
		return store.KindSpeech
	}
}

func resolveSpeaker(tc *agent.TurnContext, name, kind string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	if kind == store.KindNarration {
		return "Narrator"
	}
	return tc.PrimaryName()
}
