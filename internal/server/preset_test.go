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

// Regression test: model choice and generation options are global. A preset
// may not pin them and neither may a session, even when a stale client sends
// them. The only setting a story or session owns is its instructions.
func TestGenerationOptionsAreGlobal(t *testing.T) {
	srv, cfg := newTestServer(t)

	if rec := putConfig(t, srv, `{"model":"global/model","temperature":0.5,"maxTokens":777,"maxSteps":4,"reasoningEffort":"low","choicesEnabled":true,"disabledTools":["narrator"]}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config = %d: %s", rec.Code, rec.Body.String())
	}

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
	if strings.Contains(rec.Body.String(), `"model"`) {
		t.Fatalf("preset persisted a model: %s", rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/sessions", "{\"storyId\":\""+story.ID+"\"}")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/sessions = %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	if strings.Contains(rec.Body.String(), `"model"`) {
		t.Fatalf("session persisted a model: %s", rec.Body.String())
	}

	// A stale client trying to pin generation options must not get them in.
	rec = doJSON(t, srv, http.MethodPatch, "/api/sessions/"+id, `{"model":"hack/model","settings":{"temperature":1.7,"maxTokens":9,"systemPrompt":"new"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /api/sessions = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"model"`) {
		t.Fatalf("patch persisted a model: %s", rec.Body.String())
	}

	sess, err := srv.store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Settings.SystemPrompt != "new" {
		t.Fatalf("story instructions = %q, want %q", sess.Settings.SystemPrompt, "new")
	}
	if got := cfg.Get().Model; got != "global/model" {
		t.Fatalf("config model = %q, want global/model", got)
	}
	if got := cfg.Get().MaxTokens; got != 777 {
		t.Fatalf("config maxTokens = %d, want 777", got)
	}
}
