// Package server exposes the Genesis HTTP API and embeds the web client.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/auth"
	"github.com/zp/genesis/internal/config"
	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
	"github.com/zp/genesis/web"
)

// Server wires configuration, storage, the tool registry and the agent into an
// HTTP handler.
type Server struct {
	cfg      *config.Store
	store    *store.Store
	stories  *store.StoryStore
	registry *agent.Registry
	agent    *agent.Agent
	auth     *auth.Authenticator
	assets   http.Handler
}

// New constructs a Server.
func New(cfg *config.Store, st *store.Store, stories *store.StoryStore, reg *agent.Registry, authenticator *auth.Authenticator) *Server {
	if authenticator == nil {
		authenticator = auth.New("")
	}
	s := &Server{cfg: cfg, store: st, stories: stories, registry: reg, auth: authenticator}
	s.agent = agent.New(st, reg, s.newClient, s.agentConfig)
	sub, err := fs.Sub(web.Files, ".")
	if err != nil {
		panic("server: embedded web assets unavailable: " + err.Error())
	}
	s.assets = http.FileServer(http.FS(sub))
	return s
}

func (s *Server) newClient() (llm.Client, error) {
	c := s.cfg.Get()
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("no API key configured; add one in Settings")
	}
	return llm.NewOpenRouterClient(c.APIKey, c.BaseURL, c.Model), nil
}

func (s *Server) agentConfig() agent.Config {
	c := s.cfg.Get()
	return agent.Config{
		Model:             c.Model,
		Temperature:       c.Temperature,
		MaxTokens:         c.MaxTokens,
		MaxSteps:          c.MaxSteps,
		ToolChoice:        c.ToolChoice,
		SystemPrompt:      c.SystemPrompt,
		ParallelToolCalls: c.ParallelToolCalls,
		ReasoningEffort:   c.ReasoningEffort,
	}
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/tools", s.handleTools)
	mux.HandleFunc("GET /api/stories", s.handleListStories)
	mux.HandleFunc("POST /api/stories", s.handleCreateStory)
	mux.HandleFunc("GET /api/stories/{id}", s.handleGetStory)
	mux.HandleFunc("PATCH /api/stories/{id}", s.handlePatchStory)
	mux.HandleFunc("DELETE /api/stories/{id}", s.handleDeleteStory)
	mux.HandleFunc("POST /api/stories/{id}/assets", s.handleStoryAssetUpload)
	mux.HandleFunc("GET /api/stories/{id}/assets/{name}", s.handleStoryAssetGet)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.handlePatchSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /api/sessions/{id}/messages", s.handleMessage)
	mux.HandleFunc("POST /api/sessions/{id}/opening", s.handleOpening)
	mux.HandleFunc("POST /api/sessions/{id}/regenerate", s.handleRegenerate)
	mux.HandleFunc("POST /api/sessions/{id}/assets", s.handleAssetUpload)
	mux.HandleFunc("GET /api/sessions/{id}/assets/{name}", s.handleAssetGet)
	mux.HandleFunc("POST /api/sessions/{id}/speech", s.handleSpeech)
	mux.HandleFunc("GET /api/speech/models", s.handleSpeechModels)
	mux.Handle("GET /", s.assets)
	return s.withLogging(s.requireAuth(mux))
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

// streamTurn runs the agent and streams newline-delimited JSON events.
func (s *Server) streamTurn(w http.ResponseWriter, r *http.Request, sess *store.Session, userMsg *store.Message, title string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming not supported by this connection"))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/x-ndjson; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	var mu sync.Mutex
	emit := func(ev agent.Event) {
		mu.Lock()
		defer mu.Unlock()
		if err := enc.Encode(ev); err != nil {
			return
		}
		flusher.Flush()
	}

	if userMsg != nil {
		emit(agent.Event{Type: agent.EventUserMessage, Message: userMsg})
	}
	if title != "" {
		emit(agent.Event{Type: agent.EventTitle, Title: title})
	}

	runErr := s.agent.Continue(r.Context(), sess, emit)
	if runErr != nil && !errors.Is(runErr, r.Context().Err()) {
		emit(agent.Event{Type: agent.EventError, Error: runErr.Error()})
	}
	if err := s.store.Save(sess); err != nil {
		emit(agent.Event{Type: agent.EventError, Error: err.Error()})
	}
	emit(agent.Event{Type: agent.EventTurnEnd})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, status, map[string]any{"error": msg})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<20))
	return dec.Decode(dst)
}
