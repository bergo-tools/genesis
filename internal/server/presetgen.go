package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// presetGenSystem is the designer brief. The model answers with one
// draft_preset tool call, so nothing here asks for prose.
const presetGenSystem = `You design presets for Genesis, an agentic roleplay game master.
A preset is the reusable seed of a story: a title, a one-line pitch, an opening narration, a
suggested player character, standing story instructions, and a small cast.

- Write in the language the user wrote in.
- The cast is portrayed by the model, so each character needs a name and a concrete description:
  who they are, what they want, what they hide. No stat blocks, no sample dialogue.
- Three to five characters. Give them reasons to want different things from each other.
- The opening is two to four short paragraphs of second-person narration that drop the player
  into a live situation with an immediate question hanging in the air.
- The instructions set tone, themes and pacing for the game master. No mechanics.
- Never mention game systems, dice, or tools by name.`

type presetGenRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
}

// presetGenArgs is the argument shape of the draft_preset tool.
type presetGenArgs struct {
	Title              string `json:"title"`
	Genre              string `json:"genre"`
	Description        string `json:"description"`
	Opening            string `json:"opening"`
	PersonaDescription string `json:"personaDescription"`
	Instructions       string `json:"instructions"`
	Characters         []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"characters"`
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func presetGenTool() llm.ToolDef {
	character := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        str("The character's full name."),
			"description": str("Who they are, what they want, what they hide."),
		},
		"required":             []string{"name", "description"},
		"additionalProperties": false,
	}
	return llm.ToolDef{
		Name:        "draft_preset",
		Description: "Return the finished preset. Call this exactly once.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":              str("Short, evocative title."),
				"genre":              str("A few words, for example dark fantasy."),
				"description":        str("One or two sentences for the preset list."),
				"opening":            str("The opening narration, two to four short paragraphs."),
				"personaDescription": str("The suggested player character, one sentence."),
				"instructions":       str("Standing story instructions for the game master."),
				"characters": map[string]any{
					"type": "array", "description": "Three to five characters.", "items": character,
				},
			},
			"required": []string{
				"title", "genre", "description", "opening", "personaDescription", "instructions", "characters",
			},
			"additionalProperties": false,
		},
	}
}

// handleGenerateStory drafts a preset from a description. It never saves: the
// client gets the draft, shows it in the editor, and saves what the user keeps.
func (s *Server) handleGenerateStory(w http.ResponseWriter, r *http.Request) {
	var body presetGenRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	prompt := strings.TrimSpace(body.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, errors.New("describe the story you want first"))
		return
	}
	client, err := s.newClient()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	model := strings.TrimSpace(body.Model)
	if model == "" {
		model = strings.TrimSpace(s.cfg.Get().Model)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	resp, err := client.Complete(ctx, llm.Request{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: presetGenSystem},
			{Role: llm.RoleUser, Content: prompt},
		},
		Tools:       []llm.ToolDef{presetGenTool()},
		ToolChoice:  llm.ToolChoiceRequired,
		Temperature: 0.9,
		MaxTokens:   4000,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	call, ok := presetToolCall(resp)
	if !ok {
		writeError(w, http.StatusBadGateway, errors.New("the model did not return a preset; try again or describe it differently"))
		return
	}
	var args presetGenArgs
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		writeError(w, http.StatusBadGateway, errors.New("the model returned a malformed preset"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"story": draftStory(args), "usage": resp.Usage})
}

func presetToolCall(resp *llm.Response) (llm.ToolCall, bool) {
	if resp == nil {
		return llm.ToolCall{}, false
	}
	for _, c := range resp.ToolCalls {
		if c.Name == "draft_preset" {
			return c, true
		}
	}
	return llm.ToolCall{}, false
}

// draftStory turns the model's answer into an unsaved preset. The id stays
// empty so the client saves it as a new preset once the user is happy with it.
func draftStory(a presetGenArgs) *store.Story {
	st := &store.Story{
		Title:       strings.TrimSpace(a.Title),
		Genre:       strings.TrimSpace(a.Genre),
		Description: strings.TrimSpace(a.Description),
		Opening:     strings.TrimSpace(a.Opening),
		Persona:     store.Persona{Description: strings.TrimSpace(a.PersonaDescription)},
		State:       map[string]any{},
		Settings:    store.Settings{SystemPrompt: strings.TrimSpace(a.Instructions)},
	}
	if st.Title == "" {
		st.Title = "Untitled preset"
	}
	for _, c := range a.Characters {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		st.Characters = append(st.Characters, &store.Character{
			ID:          store.NewID(),
			Name:        name,
			Description: strings.TrimSpace(c.Description),
		})
	}
	return st
}
