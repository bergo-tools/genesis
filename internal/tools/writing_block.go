package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

// writingBlockTool writes one character's beat as an ordered list of prose
// and thought blocks, so a private thought can sit between two lines of
// dialogue the way it does in a novel.
func writingBlockTool() *agent.Tool {
	block := object(map[string]any{
		"type": enumProp("text is prose the player reads; thought is the character's private inner voice.", "text", "thought"),
		"kind": enumProp("Text blocks only: speech (default), action or narration.", "speech", "action", "narration"),
		"text": stringProp("The line itself."),
	}, "type", "text")
	return &agent.Tool{
		Name:     "writing_block",
		Category: "narrative",
		Description: "One beat of the scene, from one character. blocks is an ordered list: text blocks " +
			"are what the player reads (speech, an action or narration), thought blocks are that " +
			"character's private inner voice, rendered dimmed. Interleave them to follow the " +
			"character's mind from line to line. Call it once per character per turn.",
		Parameters: object(map[string]any{
			"speaker":   stringProp("Exact character name from the cast."),
			"blocks":    arrayProp("This character's beat, in order.", block),
			"last_call": lastCallProp(),
		}, "speaker", "blocks"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Speaker string        `json:"speaker"`
				Blocks  []store.Block `json:"blocks"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			blocks := cleanBlocks(a.Blocks)
			if len(blocks) == 0 {
				return nil, errors.New("blocks is required")
			}
			kind := messageKind(blocks)
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    kind,
				Speaker: resolveSpeaker(tc, a.Speaker, kind),
				Blocks:  blocks,
				Text:    proseOf(blocks),
			})
			return map[string]any{"ok": true, "blocks": len(blocks)}, nil
		},
	}
}

// cleanBlocks trims the list, drops empty lines and normalises the type/kind.
func cleanBlocks(in []store.Block) []store.Block {
	out := make([]store.Block, 0, len(in))
	for _, b := range in {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(b.Type), store.BlockThought) {
			out = append(out, store.Block{Type: store.BlockThought, Text: text})
			continue
		}
		out = append(out, store.Block{Type: store.BlockText, Kind: textKind(b.Kind), Text: text})
	}
	return out
}

// proseOf joins the text blocks. Text is what TTS reads, what a title is drawn
// from, and what older clients know how to render.
func proseOf(blocks []store.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == store.BlockText {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// messageKind is the message-level kind: the first prose block's kind, or
// thought when the beat is thoughts only.
func messageKind(blocks []store.Block) string {
	for _, b := range blocks {
		if b.Type == store.BlockText {
			return b.Kind
		}
	}
	return store.KindThought
}

func textKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "action", "act", "do":
		return store.KindAction
	case "narration", "narrate":
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
