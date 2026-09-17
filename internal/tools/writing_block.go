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
		"text": stringProp("The line itself."),
	}, "type", "text")
	return &agent.Tool{
		Name:     "writing_block",
		Category: "narrative",
		Description: "One beat of the scene, from one character. blocks is an ordered list: text blocks " +
			"are the prose the player reads (dialogue, an action, a description), thought blocks are " +
			"that character's private inner voice, rendered dimmed. Interleave them to follow the " +
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
			prose := proseOf(blocks)
			kind := store.KindThought
			if prose != "" {
				kind = store.KindSpeech
			}
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    kind,
				Speaker: resolveSpeaker(tc, a.Speaker),
				Blocks:  blocks,
				Text:    prose,
			})
			return map[string]any{"ok": true, "blocks": len(blocks)}, nil
		},
	}
}

// cleanBlocks trims the list, drops empty lines and normalises the type.
func cleanBlocks(in []store.Block) []store.Block {
	out := make([]store.Block, 0, len(in))
	for _, b := range in {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		kind := store.BlockText
		if strings.EqualFold(strings.TrimSpace(b.Type), store.BlockThought) {
			kind = store.BlockThought
		}
		out = append(out, store.Block{Type: kind, Text: text})
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

func resolveSpeaker(tc *agent.TurnContext, name string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return tc.PrimaryName()
}
