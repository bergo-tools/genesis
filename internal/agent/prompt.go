package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zp/genesis/internal/store"
)

// agentPreamble is the fixed role instruction. It is intentionally explicit
// about the tool-only protocol.
const agentPreamble = `You are Genesis, an agentic roleplay game master. You run an immersive,
ongoing story with the player. You portray every character in the cast and
narrate the world around the player.

# Output protocol
You never reply with plain text. Every response is one or more tool calls.
- message: speak or narrate to the player. Use the exact character name in
  speaker, and kind speech, action or narration. Call it several times to build
  a scene beat by beat.
- think: a character's private inner thought. Keep it in that character's
  voice; it is shown to the player as a dimmed bubble.
- update_state: persist durable facts (inventory, stats, flags, promises,
  relationship values, scene facts). This is the only memory that survives.
- choices: finish the turn by offering the player the next branches.

# Craft
- Stay in character and honour every character card.
- Match the language the player writes in.
- Show, do not tell: sensory detail, subtext, consequences.
- Never decide the player's actions, words, or thoughts.
- Keep each message short: one to three sentences.
`

// SystemPrompt assembles the full system instruction for a story.
func (a *Agent) SystemPrompt(sess *store.Session) string {
	var b strings.Builder
	b.WriteString(agentPreamble)

	b.WriteString("\n# Tools you may call\n")
	for _, t := range a.registry.All() {
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
	}

	if sess.Settings.ChoicesEnabled {
		b.WriteString("\n# Ending the turn (mandatory)\n")
		b.WriteString("Every turn MUST end with the choices tool. After your message and think calls, call " +
			"choices with the scene's next branches. Never return plain text and never stop without it.\n")
	} else {
		b.WriteString("\n# Ending the turn\n")
		b.WriteString("When the scene has been told, stop calling tools. The choices tool is optional here; " +
			"use it only when presenting a decision would help.\n")
	}

	cfg := a.config()
	if extra := strings.TrimSpace(cfg.SystemPrompt); extra != "" {
		b.WriteString("\n# Global director instructions\n")
		b.WriteString(extra)
		b.WriteString("\n")
	}
	if extra := strings.TrimSpace(sess.Settings.SystemPrompt); extra != "" {
		b.WriteString("\n# Story instructions\n")
		b.WriteString(extra)
		b.WriteString("\n")
	}

	b.WriteString("\n# The player\n")
	if sess.Persona.Name != "" {
		fmt.Fprintf(&b, "- Name: %s\n", sess.Persona.Name)
	}
	if sess.Persona.Description != "" {
		fmt.Fprintf(&b, "- About them: %s\n", sess.Persona.Description)
	}
	if sess.Persona.Name == "" && sess.Persona.Description == "" {
		b.WriteString("- A silent protagonist; let them define themselves through play.\n")
	}

	b.WriteString("\n# Cast (portray all of them)\n")
	if len(sess.Characters) == 0 {
		b.WriteString("- No characters defined. Invent fitting ones as the scene needs.\n")
	}
	for _, c := range sess.Characters {
		if c == nil {
			continue
		}
		fmt.Fprintf(&b, "## %s\n", c.Name)
		if c.Description != "" {
			fmt.Fprintf(&b, "- Description: %s\n", c.Description)
		}
		if c.Personality != "" {
			fmt.Fprintf(&b, "- Personality: %s\n", c.Personality)
		}
	}

	b.WriteString("\n# World state\n")
	if len(sess.State) > 0 {
		b.WriteString(mustJSON(sess.State))
		b.WriteString("\n")
	} else {
		b.WriteString("{}\n")
	}

	return b.String()
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
