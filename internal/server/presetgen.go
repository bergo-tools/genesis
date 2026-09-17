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
// draft_preset call, so the brief explains the tool and shows an example
// rather than dictating a formula.
const presetGenSystem = `You design presets for Genesis, an agentic roleplay game master.

A preset is the seed of a story: a title and a pitch, an opening, a suggested player character,
some standing guidance for the game master, and a small cast that the model will portray.

Answer with one draft_preset call and fill in every field. The characters are played by the model,
so a name plus a concrete description is enough: who they are, what they want, what they hide.

For "a library the sea is slowly taking", that could come out as:

  title: The Drowned Library
  genre: melancholy fantasy
  description: An archivist keeps the last library the sea has not swallowed.
  opening: Salt water reached the third gallery today; she works with her boots in it.
  personaDescription: An apprentice sent to catalogue whatever is left.
  instructions: Quiet and damp. Let every loss be permanent.
  characters: Ilyra, the archivist, who knows exactly what is already gone; Brother Od, who
    claims he can hear the shelves breathe.

Write in the language the user wrote in.`

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
			"name":        str("The character's name."),
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
				"title":              str("A short name for the preset."),
				"genre":              str("A few words, for example dark fantasy."),
				"description":        str("One or two sentences for the preset list."),
				"opening":            str("The narration shown when a session starts."),
				"personaDescription": str("The suggested player character."),
				"instructions":       str("Standing guidance for the game master."),
				"characters": map[string]any{
					"type": "array", "description": "The cast the game master portrays.", "items": character,
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
