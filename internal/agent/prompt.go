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
- Use only the tools listed below; never invent one.
- Express narration, dialogue, inner thought and state changes through tools.
- Persist durable facts: inventory, stats, flags, promises, relationships.
- Never invent a tool result.

# Craft
- Stay in character and honour every character card.
- Match the language the player writes in.
- Show, do not tell: sensory detail, subtext, consequences.
- Never decide the player's actions, words, or thoughts.
- Keep each message short: one to three sentences.
- A user message prefixed with [OOC] is an out-of-character instruction to you, not something a
  character said. Follow it, then continue the story in character.
- Put a character's private thought in the message's thought field, not a separate call.
- Give each character one message call per turn. Only call message again as the same character
  after another character or the scene has reacted to what they did.
- Use scene whenever the story moves somewhere new.
`

// SystemPrompt assembles the full system instruction for a story.
func (a *Agent) SystemPrompt(sess *store.Session) string {
	var b strings.Builder
	b.WriteString(agentPreamble)

	active := a.activeTools()

	b.WriteString("\n# Tools you may call\n")
	for _, t := range active {
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
	}
	b.WriteString("- Every tool also accepts last_call. Set last_call=true on the call you believe is your " +
		"final one for this turn.\n")

	if hasTool(active, "choices") {
		b.WriteString("\n# Ending the turn (mandatory)\n")
		b.WriteString("Every turn MUST end with the choices tool. After your message and think calls, call " +
			"choices with the scene's next branches. last_call does not end the turn while choices is " +
			"enabled; never return plain text and never stop without calling choices.\n")
	} else {
		b.WriteString("\n# Ending the turn\n")
		b.WriteString("The choices tool is not available. When the scene is told, end the turn by setting " +
			"last_call=true on your final tool call.\n")
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
