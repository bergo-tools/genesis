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
		Description: "One beat of the scene, from one character. text is what the player reads: speech, " +
			"an action, or narration. thought is that character's private inner voice and is rendered " +
			"in the same block, dimmed. Call it several times to build a scene.",
		Parameters: object(map[string]any{
			"speaker":   stringProp("Exact character name. Use Narrator for world description."),
			"text":      stringProp("What the player reads. May be empty when you only have a thought."),
			"thought":   stringProp("Optional private inner thought, shown dimmed under the text."),
			"kind":      enumProp("Message kind.", "speech", "action", "narration"),
			"mood":      stringProp("Optional short mood label for the speaker."),
			"last_call": lastCallProp(),
		}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Speaker string `json:"speaker"`
				Text    string `json:"text"`
				Thought string `json:"thought"`
				Kind    string `json:"kind"`
				Mood    string `json:"mood"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			text := strings.TrimSpace(a.Text)
			thought := strings.TrimSpace(a.Thought)
			if text == "" && thought == "" {
				return nil, errors.New("text or thought is required")
			}
			kind := playerKind(a.Kind)
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    kind,
				Speaker: resolveSpeaker(tc, a.Speaker, kind),
				Text:    text,
				Thought: thought,
				Mood:    strings.TrimSpace(a.Mood),
			})
			return map[string]any{"ok": true}, nil
		},
	}
}

// playerKind normalises a kind emitted by the message tool.
func playerKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "action", "act", "do":
		return store.KindAction
	case "narration", "narrate", "scene":
		return store.KindNarration
	case "thought", "think", "inner":
		return store.KindThought
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
