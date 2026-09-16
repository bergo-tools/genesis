package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/auth"
	"github.com/zp/genesis/internal/config"
	"github.com/zp/genesis/internal/store"
)

func newTestServer(t *testing.T) (*Server, *config.Store) {
	return newTestServerWithAuth(t, "")
}

// newTestServerWithAuth builds a server whose gate is enabled by password (an
// empty password leaves the app open).
func newTestServerWithAuth(t *testing.T, password string) (*Server, *config.Store) {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	stories, err := store.NewStoryStore(filepath.Join(t.TempDir(), "stories"))
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, st, stories, agent.NewRegistry(), auth.New(password)), cfg
}

func putConfig(t *testing.T, srv *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// Regression test: the speech fields must round-trip through PUT /api/config,
// otherwise the UI can never change the TTS model.
func TestPutConfigPersistsSpeechFields(t *testing.T) {
	srv, cfg := newTestServer(t)
	rec := putConfig(t, srv, `{"speechModel":"openai/gpt-4o-mini-tts","speechVoice":"shimmer","speechFormat":"wav","speechSpeed":1.25,"autoSpeak":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config = %d: %s", rec.Code, rec.Body.String())
	}
	got := cfg.Get()
	if got.SpeechModel != "openai/gpt-4o-mini-tts" {
		t.Fatalf("speechModel not persisted: %q", got.SpeechModel)
	}
	if got.SpeechVoice != "shimmer" {
		t.Fatalf("speechVoice not persisted: %q", got.SpeechVoice)
	}
	if got.SpeechFormat != "wav" {
		t.Fatalf("speechFormat not persisted: %q", got.SpeechFormat)
	}
	if got.SpeechSpeed != 1.25 {
		t.Fatalf("speechSpeed not persisted: %v", got.SpeechSpeed)
	}
	if !got.AutoSpeak {
		t.Fatal("autoSpeak not persisted")
	}

	// An explicit empty voice must be allowed so it can be cleared.
	if rec := putConfig(t, srv, `{"speechVoice":""}`); rec.Code != http.StatusOK {
		t.Fatalf("clearing voice failed: %d", rec.Code)
	}
	if got := cfg.Get().SpeechVoice; got != "" {
		t.Fatalf("speechVoice should be cleared, got %q", got)
	}
}

// Regression test: the Settings panel's tool switches are saved via
// PUT /api/config, so disabledTools must be parsed there.
func TestPutConfigPersistsDisabledTools(t *testing.T) {
	srv, cfg := newTestServer(t)
	if rec := putConfig(t, srv, `{"disabledTools":["scene","scene"," "]}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config = %d: %s", rec.Code, rec.Body.String())
	}
	if got := cfg.Get().DisabledTools; len(got) != 1 || got[0] != "scene" {
		t.Fatalf("disabledTools not persisted/cleaned: %#v", got)
	}
	// An explicitly empty list must clear them.
	if rec := putConfig(t, srv, `{"disabledTools":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("clearing tools failed: %d", rec.Code)
	}
	if got := cfg.Get().DisabledTools; len(got) != 0 {
		t.Fatalf("disabledTools should be cleared, got %#v", got)
	}
}
