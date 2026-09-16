package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zp/genesis/internal/config"
	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Public())
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey            *string  `json:"apiKey"`
		BaseURL           *string  `json:"baseUrl"`
		Model             *string  `json:"model"`
		Temperature       *float64 `json:"temperature"`
		MaxTokens         *int     `json:"maxTokens"`
		MaxSteps          *int     `json:"maxSteps"`
		ToolChoice        *string  `json:"toolChoice"`
		SystemPrompt      *string  `json:"systemPrompt"`
		ParallelToolCalls *bool    `json:"parallelToolCalls"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	err := s.cfg.Update(func(c *config.Config) {
		if body.APIKey != nil {
			key := strings.TrimSpace(*body.APIKey)
			if key == "__clear__" {
				c.APIKey = ""
			} else if key != "" {
				c.APIKey = key
			}
		}
		if body.BaseURL != nil && strings.TrimSpace(*body.BaseURL) != "" {
			c.BaseURL = strings.TrimSpace(*body.BaseURL)
		}
		if body.Model != nil && strings.TrimSpace(*body.Model) != "" {
			c.Model = strings.TrimSpace(*body.Model)
		}
		if body.Temperature != nil {
			c.Temperature = *body.Temperature
		}
		if body.MaxTokens != nil && *body.MaxTokens > 0 {
			c.MaxTokens = *body.MaxTokens
		}
		if body.MaxSteps != nil && *body.MaxSteps > 0 {
			c.MaxSteps = *body.MaxSteps
		}
		if body.ToolChoice != nil && strings.TrimSpace(*body.ToolChoice) != "" {
			c.ToolChoice = strings.TrimSpace(*body.ToolChoice)
		}
		if body.SystemPrompt != nil {
			c.SystemPrompt = *body.SystemPrompt
		}
		if body.ParallelToolCalls != nil {
			c.ParallelToolCalls = *body.ParallelToolCalls
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Public())
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	if strings.TrimSpace(c.APIKey) == "" {
		writeError(w, http.StatusBadRequest, errors.New("add an API key before listing models"))
		return
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = config.Default().BaseURL
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/models", nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		writeError(w, resp.StatusCode, fmt.Errorf("provider: %s", strings.TrimSpace(string(body))))
		return
	}
	var parsed struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	models := make([]map[string]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID == "" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, map[string]string{"id": m.ID, "name": name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) handleTools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"tools": s.registry.Meta()})
}

func (s *Server) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	sessions, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, map[string]any{
			"id":           sess.ID,
			"title":        sessionTitle(sess),
			"model":        sess.Model,
			"character":    firstCharacterName(sess),
			"messageCount": len(sess.Messages),
			"createdAt":    sess.CreatedAt,
			"updatedAt":    sess.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title     string         `json:"title"`
		Model     string         `json:"model"`
		Persona   *store.Persona `json:"persona"`
		Character struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Personality string   `json:"personality"`
			Scenario    string   `json:"scenario"`
			Appearance  string   `json:"appearance"`
			Avatar      string   `json:"avatar"`
			Greeting    string   `json:"greeting"`
			Tags        []string `json:"tags"`
		} `json:"character"`
		Settings *store.Settings `json:"settings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg := s.cfg.Get()
	sess := &store.Session{
		Title: strings.TrimSpace(body.Title),
		Model: strings.TrimSpace(body.Model),
		Settings: store.Settings{
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
			MaxSteps:    cfg.MaxSteps,
			ToolChoice:  cfg.ToolChoice,
		},
		State: map[string]any{},
	}
	if body.Persona != nil {
		sess.Persona = *body.Persona
	}
	if body.Settings != nil {
		mergeSettings(&sess.Settings, body.Settings)
	}
	if name := strings.TrimSpace(body.Character.Name); name != "" || strings.TrimSpace(body.Character.Description) != "" {
		if name == "" {
			name = "Character"
		}
		c := &store.Character{
			ID:          store.NewID(),
			Name:        name,
			Description: strings.TrimSpace(body.Character.Description),
			Personality: strings.TrimSpace(body.Character.Personality),
			Scenario:    strings.TrimSpace(body.Character.Scenario),
			Appearance:  strings.TrimSpace(body.Character.Appearance),
			Avatar:      strings.TrimSpace(body.Character.Avatar),
			Tags:        body.Character.Tags,
			State:       map[string]any{},
			CreatedAt:   time.Now().UTC(),
		}
		sess.Characters = append(sess.Characters, c)
		g := strings.TrimSpace(body.Character.Greeting)
		if g == "" {
			g = defaultGreeting(c)
		}
		sess.Messages = append(sess.Messages, &store.Message{
			ID: store.NewID(), Role: "assistant", Kind: store.KindSpeech, Speaker: c.Name, Text: g, CreatedAt: time.Now().UTC(),
		})
		sess.History = append(sess.History, llm.Message{Role: llm.RoleAssistant, Content: g})
	}
	if err := s.store.Create(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess.Public())
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.store.Get(r.PathValue("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, sess.Public())
}

func (s *Server) handlePatchSession(w http.ResponseWriter, r *http.Request) {
	lock := s.store.TurnLock(r.PathValue("id"))
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(r.PathValue("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	var body struct {
		Title      *string            `json:"title"`
		Model      *string            `json:"model"`
		Persona    *store.Persona     `json:"persona"`
		Scene      *store.Scene       `json:"scene"`
		Settings   *store.Settings    `json:"settings"`
		State      map[string]any     `json:"state"`
		Characters []*store.Character `json:"characters"`
		Memories   []*store.Memory    `json:"memories"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Title != nil {
		sess.Title = strings.TrimSpace(*body.Title)
	}
	if body.Model != nil {
		sess.Model = strings.TrimSpace(*body.Model)
	}
	if body.Persona != nil {
		sess.Persona = *body.Persona
	}
	if body.Scene != nil {
		sess.Scene = *body.Scene
	}
	if body.Settings != nil {
		mergeSettings(&sess.Settings, body.Settings)
	}
	if body.State != nil {
		sess.State = body.State
	}
	if body.Characters != nil {
		sess.Characters = body.Characters
	}
	if body.Memories != nil {
		sess.Memories = body.Memories
	}
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sess.Public())
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Delete(r.PathValue("id")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, errors.New("text is required"))
		return
	}

	lock := s.store.TurnLock(id)
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}

	user := &store.Message{
		ID: store.NewID(), Role: "user", Kind: store.KindUser,
		Speaker: sess.Persona.Name, Text: text, CreatedAt: time.Now().UTC(),
	}
	sess.Messages = append(sess.Messages, user)
	sess.History = append(sess.History, llm.Message{Role: llm.RoleUser, Content: text})

	title := ""
	if strings.TrimSpace(sess.Title) == "" {
		sess.Title = deriveTitle(text)
		title = sess.Title
	}
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.streamTurn(w, r, sess, user, title)
}

func (s *Server) handleOpening(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lock := s.store.TurnLock(id)
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	sess.History = append(sess.History, llm.Message{
		Role:    llm.RoleUser,
		Content: "Begin the story now. Establish the scene with set_scene, introduce the characters with send_message, then stop and let the player act.",
	})
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.streamTurn(w, r, sess, nil, "")
}

func (s *Server) handleRegenerate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lock := s.store.TurnLock(id)
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	truncateLastTurn(sess)
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.streamTurn(w, r, sess, nil, "")
}

func truncateLastTurn(sess *store.Session) {
	lastUser := -1
	for i := len(sess.History) - 1; i >= 0; i-- {
		if sess.History[i].Role == llm.RoleUser {
			lastUser = i
			break
		}
	}
	if lastUser >= 0 {
		sess.History = sess.History[:lastUser+1]
	} else {
		sess.History = nil
	}
	lastMsgUser := -1
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Kind == store.KindUser {
			lastMsgUser = i
			break
		}
	}
	if lastMsgUser >= 0 {
		sess.Messages = sess.Messages[:lastMsgUser+1]
	} else {
		sess.Messages = nil
	}
}

func mergeSettings(dst *store.Settings, src *store.Settings) {
	if src.Temperature > 0 {
		dst.Temperature = src.Temperature
	}
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.MaxSteps > 0 {
		dst.MaxSteps = src.MaxSteps
	}
	if strings.TrimSpace(src.ToolChoice) != "" {
		dst.ToolChoice = strings.TrimSpace(src.ToolChoice)
	}
	if strings.TrimSpace(src.SystemPrompt) != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
}

func deriveTitle(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	runes := []rune(text)
	if len(runes) > 48 {
		return strings.TrimSpace(string(runes[:48])) + "…"
	}
	if text == "" {
		return "New story"
	}
	return text
}

func sessionTitle(sess *store.Session) string {
	if sess == nil {
		return "Untitled"
	}
	if strings.TrimSpace(sess.Title) != "" {
		return sess.Title
	}
	return "Untitled"
}

func firstCharacterName(sess *store.Session) string {
	if c := sess.PrimaryCharacter(); c != nil {
		return c.Name
	}
	return ""
}

func defaultGreeting(c *store.Character) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.Scenario) != "" {
		return c.Scenario
	}
	return fmt.Sprintf("*%s comes into view.*", c.Name)
}
