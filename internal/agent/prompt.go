package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zp/genesis/internal/store"
)

const agentPreamble = `You are Genesis, an agentic roleplay engine and game master. You run an
immersive, ongoing story. You portray every non-player character and narrate
the world around the player.

# Non-negotiable protocol
1. You never reply with plain text. Every response is one or more tool calls.
2. Use send_message for everything the player should read: speech, action,
   narration, inner thoughts, or out-of-character notes. Choose the type field
   accordingly. Call it several times to produce several beats.
3. Use the world tools (set_scene, update_character, update_state, remember)
   whenever the fiction changes. Keep the world state accurate and current.
4. Use the information tools (roll, recall, get_state) before committing to an
   uncertain outcome. Never invent a dice result or a memory.
5. End every turn with await_player when the player must act, or end_turn when
   the scene simply continues. Anything after these is never seen.
6. Never narrate or decide the player's actions, words, or thoughts.

# Style
- Stay in character at all times and honour every character card.
- Match the language the player uses. If the player writes Chinese, write in
  Chinese; otherwise use the language of the scene.
- Keep each send_message focused: one to three sentences, under ~120 words.
- Show, do not tell. Use concrete sensory detail and subtext.
- Advance the scene every turn: add a beat, a complication, or a real choice.
`

// SystemPrompt assembles the full system instruction for a session.
func (a *Agent) SystemPrompt(sess *store.Session) string {
	var b strings.Builder
	b.WriteString(agentPreamble)

	b.WriteString("\n# Tools you may call\n")
	for _, t := range a.registry.All() {
		fmt.Fprintf(&b, "- %s (category: %s): %s\n", t.Name, t.Category, t.Description)
	}

	cfg := a.config()
	if extra := strings.TrimSpace(cfg.SystemPrompt); extra != "" {
		b.WriteString("\n# Global director instructions\n")
		b.WriteString(extra)
		b.WriteString("\n")
	}
	if extra := strings.TrimSpace(sess.Settings.SystemPrompt); extra != "" {
		b.WriteString("\n# Session instructions\n")
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

	b.WriteString("\n# Characters you portray\n")
	if len(sess.Characters) == 0 {
		b.WriteString("- None defined yet. Invent fitting characters as the scene needs.\n")
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
		if c.Appearance != "" {
			fmt.Fprintf(&b, "- Appearance: %s\n", c.Appearance)
		}
		if c.Scenario != "" {
			fmt.Fprintf(&b, "- Scenario: %s\n", c.Scenario)
		}
		if len(c.Tags) > 0 {
			fmt.Fprintf(&b, "- Tags: %s\n", strings.Join(c.Tags, ", "))
		}
		if len(c.State) > 0 {
			fmt.Fprintf(&b, "- Current state: %s\n", mustJSON(c.State))
		}
	}

	b.WriteString("\n# Scene\n")
	writeScene(&b, sess.Scene)

	b.WriteString("\n# World state\n")
	if len(sess.State) > 0 {
		b.WriteString(mustJSON(sess.State))
		b.WriteString("\n")
	} else {
		b.WriteString("{}\n")
	}

	if mem := topMemories(sess.Memories, 16); len(mem) > 0 {
		b.WriteString("\n# Long-term memory\n")
		for _, m := range mem {
			fmt.Fprintf(&b, "- (importance %d) %s\n", m.Importance, m.Content)
		}
	}

	return b.String()
}

func writeScene(b *strings.Builder, s store.Scene) {
	wrote := false
	for _, f := range []struct {
		label string
		value string
	}{
		{"Location", s.Location},
		{"Time", s.Time},
		{"Weather", s.Weather},
		{"Background", s.Background},
		{"Notes", s.Notes},
	} {
		if strings.TrimSpace(f.value) == "" {
			continue
		}
		fmt.Fprintf(b, "- %s: %s\n", f.label, f.value)
		wrote = true
	}
	if !wrote {
		b.WriteString("- Not yet established.\n")
	}
}

func topMemories(mems []*store.Memory, limit int) []*store.Memory {
	out := make([]*store.Memory, 0, len(mems))
	for _, m := range mems {
		if m != nil && strings.TrimSpace(m.Content) != "" {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Importance != out[j].Importance {
			return out[i].Importance > out[j].Importance
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
