package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/llm"
)

func TestDraftStoryFromToolArgs(t *testing.T) {
	var a presetGenArgs
	raw := `{
      "title":"The Drowned Library",
      "genre":"melancholy fantasy",
      "description":"An archivist keeps a library the sea is slowly taking.",
      "opening":"Salt water reaches the third gallery now.",
      "personaDescription":"An apprentice sent to catalogue what is left.",
      "instructions":"Quiet, damp, elegiac. Let loss accumulate.",
      "characters":[
        {"name":"Ilyra","description":"The archivist. Knows what is already lost."},
        {"name":"  ","description":"dropped"},
        {"name":"Brother Od","description":"Claims to hear the shelves breathe."}
      ]
    }`
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	st := draftStory(a)
	if st.Title != "The Drowned Library" || st.Genre != "melancholy fantasy" {
		t.Fatalf("title/genre not mapped: %+v", st)
	}
	if st.Persona.Description != "An apprentice sent to catalogue what is left." {
		t.Fatalf("persona not mapped: %+v", st.Persona)
	}
	if st.Settings.SystemPrompt != "Quiet, damp, elegiac. Let loss accumulate." {
		t.Fatalf("instructions not mapped: %q", st.Settings.SystemPrompt)
	}
	if st.ID != "" {
		t.Fatal("a draft must stay unsaved, so it carries no id")
	}
	if len(st.Characters) != 2 {
		t.Fatalf("blank characters must be dropped, got %+v", st.Characters)
	}
	if st.Characters[0].Name != "Ilyra" || st.Characters[1].Name != "Brother Od" {
		t.Fatalf("unexpected cast: %+v", st.Characters)
	}
	if st.Characters[0].ID == "" {
		t.Fatal("characters need an id so the editor can key them")
	}
}

func TestDraftStoryFallsBackToATitle(t *testing.T) {
	st := draftStory(presetGenArgs{})
	if st.Title != "Untitled preset" {
		t.Fatalf("title = %q", st.Title)
	}
}

func TestPresetToolCall(t *testing.T) {
	resp := &llm.Response{ToolCalls: []llm.ToolCall{
		{Name: "something_else"},
		{Name: "draft_preset", Arguments: "{}"},
	}}
	if _, ok := presetToolCall(resp); !ok {
		t.Fatal("draft_preset call not found")
	}
	if _, ok := presetToolCall(&llm.Response{}); ok {
		t.Fatal("an empty response must not yield a call")
	}
}

// Regression test: the endpoint validates before it reaches the provider, so a
// missing description or a missing API key fails fast with a useful message.
func TestGenerateStoryValidation(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doJSON(t, srv, http.MethodPost, "/api/stories/generate", `{"prompt":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank prompt = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "describe") {
		t.Fatalf("unhelpful error: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/stories/generate", `{"prompt":"a drowned library"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing key = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "API key") {
		t.Fatalf("expected the API key error, got: %s", rec.Body.String())
	}
}
