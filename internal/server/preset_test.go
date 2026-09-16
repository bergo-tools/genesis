package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/store"
)

func doJSON(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// Regression test: a preset is content only. Even when a stale client sends
// model and generation options, a session started from it must snapshot the
// global config and inherit just the story instructions.
func TestPresetDoesNotPinGeneration(t *testing.T) {
	srv, cfg := newTestServer(t)

	if rec := putConfig(t, srv, `{"model":"global/model","temperature":0.5,"maxTokens":777,"maxSteps":4,"reasoningEffort":"low","choicesEnabled":true,"disabledTools":["scene"]}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config = %d: %s", rec.Code, rec.Body.String())
	}
	want := cfg.Get()

	rec := doJSON(t, srv, http.MethodPost, "/api/stories", `{
      "title":"Pinned",
      "model":"preset/model",
      "settings":{"temperature":1.9,"maxTokens":64,"maxSteps":1,"reasoningEffort":"max","choicesEnabled":false,"disabledTools":["message","choices"],"systemPrompt":"be terse"}
    }`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/stories = %d: %s", rec.Code, rec.Body.String())
	}
	var story store.Story
	if err := json.Unmarshal(rec.Body.Bytes(), &story); err != nil {
		t.Fatal(err)
	}
	if story.Settings.SystemPrompt != "be terse" {
		t.Fatalf("story instructions not stored: %q", story.Settings.SystemPrompt)
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/sessions", `{"storyId":"`+story.ID+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/sessions = %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	sess, err := srv.store.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	if sess.Model != want.Model {
		t.Fatalf("session model = %q, want global %q", sess.Model, want.Model)
	}
	if sess.Settings.Temperature != want.Temperature || sess.Settings.MaxTokens != want.MaxTokens ||
		sess.Settings.MaxSteps != want.MaxSteps || sess.Settings.ReasoningEffort != want.ReasoningEffort ||
		sess.Settings.ChoicesEnabled != want.ChoicesEnabled {
		t.Fatalf("session generation = %#v, want global %#v", sess.Settings, want)
	}
	if len(sess.Settings.DisabledTools) != 1 || sess.Settings.DisabledTools[0] != "scene" {
		t.Fatalf("session disabledTools = %#v, want [scene]", sess.Settings.DisabledTools)
	}
	if sess.Settings.SystemPrompt != "be terse" {
		t.Fatalf("session story instructions = %q, want %q", sess.Settings.SystemPrompt, "be terse")
	}
}
