package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func sendMessageTool() *agent.Tool {
	return &agent.Tool{
		Name:     "send_message",
		Category: "narration",
		Description: "Emit one message the player will read. Set type to speech for quoted dialogue, " +
			"action for a character's physical deed, narration for world description, thought for a " +
			"character's private thought, or ooc for an out-of-character note. Call it multiple times " +
			"to produce multiple beats in one turn.",
		Parameters: object(map[string]any{
			"type":    enumProp("The kind of message.", "speech", "action", "narration", "thought", "ooc"),
			"speaker": stringProp("Character name delivering the message, or Narrator for world description. Defaults to the lead character."),
			"text":    stringProp("The message content. Do not wrap the whole line in quotes."),
			"mood":    stringProp("Optional short mood label for the speaker, e.g. wary, delighted."),
		}, "type", "text"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Type    string `json:"type"`
				Speaker string `json:"speaker"`
				Text    string `json:"text"`
				Mood    string `json:"mood"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			text := strings.TrimSpace(a.Text)
			if text == "" {
				return nil, errors.New("text is required")
			}
			kind := normalizeKind(a.Type)
			speaker := strings.TrimSpace(a.Speaker)
			if speaker == "" {
				if kind == store.KindNarration {
					speaker = "Narrator"
				} else {
					speaker = tc.PrimaryName()
				}
			}
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    kind,
				Speaker: speaker,
				Text:    text,
				Mood:    strings.TrimSpace(a.Mood),
			})
			return map[string]any{"ok": true}, nil
		},
	}
}

func thinkTool() *agent.Tool {
	return &agent.Tool{
		Name:        "think",
		Category:    "meta",
		Description: "Record a private planning note for yourself. This is shown to the player in a collapsed agent panel and is never spoken in character. Use it to plan the next beat.",
		Parameters:  object(map[string]any{"text": stringProp("Your private reasoning.")}, "text"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Text string `json:"text"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if tc.Emit != nil {
				tc.Emit(agent.Event{Type: agent.EventReasoning, Step: tc.Step, Text: strings.TrimSpace(a.Text)})
			}
			return map[string]any{"ok": true}, nil
		},
	}
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "speech", "say", "dialogue":
		return store.KindSpeech
	case "action", "act", "do":
		return store.KindAction
	case "narration", "narrate", "scene":
		return store.KindNarration
	case "thought", "think", "inner":
		return store.KindThought
	case "ooc", "meta":
		return store.KindOOC
	default:
		return store.KindSpeech
	}
}
