package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
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
		ReasoningEffort   *string  `json:"reasoningEffort"`
		ChoicesEnabled    *bool    `json:"choicesEnabled"`
		SpeechModel       *string  `json:"speechModel"`
		SpeechVoice       *string  `json:"speechVoice"`
		SpeechFormat      *string  `json:"speechFormat"`
		SpeechSpeed       *float64 `json:"speechSpeed"`
		AutoSpeak         *bool    `json:"autoSpeak"`
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
		if body.ReasoningEffort != nil {
			c.ReasoningEffort = strings.TrimSpace(*body.ReasoningEffort)
		}
		if body.ChoicesEnabled != nil {
			c.ChoicesEnabled = *body.ChoicesEnabled
		}
		if body.SpeechModel != nil {
			c.SpeechModel = strings.TrimSpace(*body.SpeechModel)
		}
		if body.SpeechVoice != nil {
			c.SpeechVoice = *body.SpeechVoice
		}
		if body.SpeechFormat != nil && strings.TrimSpace(*body.SpeechFormat) != "" {
			c.SpeechFormat = strings.TrimSpace(*body.SpeechFormat)
		}
		if body.SpeechSpeed != nil && *body.SpeechSpeed > 0 {
			c.SpeechSpeed = *body.SpeechSpeed
		}
		if body.AutoSpeak != nil {
			c.AutoSpeak = *body.AutoSpeak
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Public())
}

// chatModelInfo is a text model offered for roleplay, with a hint about
// whether it supports tool calling (which Genesis relies on entirely).
type chatModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context int    `json:"context"`
	Tools   bool   `json:"tools"`
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	if strings.TrimSpace(c.APIKey) == "" {
		writeError(w, http.StatusBadRequest, errors.New("add an API key before listing models"))
		return
	}
	models, err := fetchChatModels(r.Context(), c.BaseURL, c.APIKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// fetchChatModels lists text-capable chat models and marks which ones support
// the "tools" parameter. Tool-capable models sort first.
func fetchChatModels(ctx context.Context, baseURL, apiKey string) ([]chatModelInfo, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = config.Default().BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models?output_modalities=text", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data []struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			ContextLength       int      `json:"context_length"`
			SupportedParameters []string `json:"supported_parameters"`
			Architecture        struct {
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]chatModelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		if mods := m.Architecture.OutputModalities; len(mods) > 0 && !hasString(mods, "text") {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		out = append(out, chatModelInfo{
			ID:      m.ID,
			Name:    name,
			Context: m.ContextLength,
			Tools:   hasString(m.SupportedParameters, "tools"),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tools != out[j].Tools {
			return out[i].Tools
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func hasString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func (s *Server) handleTools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"tools": s.registry.Meta()})
}

// settingsInput distinguishes "absent" from a zero value.
type settingsInput struct {
	Temperature     *float64 `json:"temperature"`
	MaxTokens       *int     `json:"maxTokens"`
	MaxSteps        *int     `json:"maxSteps"`
	ToolChoice      *string  `json:"toolChoice"`
	SystemPrompt    *string  `json:"systemPrompt"`
	ReasoningEffort *string  `json:"reasoningEffort"`
	ChoicesEnabled  *bool    `json:"choicesEnabled"`
}

func applySettings(dst *store.Settings, in *settingsInput) {
	if dst == nil || in == nil {
		return
	}
	if in.Temperature != nil && *in.Temperature > 0 {
		dst.Temperature = *in.Temperature
	}
	if in.MaxTokens != nil && *in.MaxTokens > 0 {
		dst.MaxTokens = *in.MaxTokens
	}
	if in.MaxSteps != nil && *in.MaxSteps > 0 {
		dst.MaxSteps = *in.MaxSteps
	}
	if in.ToolChoice != nil && strings.TrimSpace(*in.ToolChoice) != "" {
		dst.ToolChoice = strings.TrimSpace(*in.ToolChoice)
	}
	if in.SystemPrompt != nil {
		dst.SystemPrompt = *in.SystemPrompt
	}
	if in.ReasoningEffort != nil {
		dst.ReasoningEffort = strings.TrimSpace(*in.ReasoningEffort)
	}
	if in.ChoicesEnabled != nil {
		dst.ChoicesEnabled = *in.ChoicesEnabled
	}
}

type characterInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Personality string `json:"personality"`
	Avatar      string `json:"avatar"`
	Voice       string `json:"voice"`
}

func toCharacter(in characterInput) *store.Character {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = store.NewID()
	}
	return &store.Character{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Personality: strings.TrimSpace(in.Personality),
		Avatar:      strings.TrimSpace(in.Avatar),
		Voice:       strings.TrimSpace(in.Voice),
		CreatedAt:   time.Now().UTC(),
	}
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
			"avatar":       sess.Avatar,
			"model":        sess.Model,
			"characters":   sess.CharacterNames(),
			"messageCount": len(sess.Messages),
			"createdAt":    sess.CreatedAt,
			"updatedAt":    sess.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title      string           `json:"title"`
		Avatar     string           `json:"avatar"`
		Model      string           `json:"model"`
		Persona    *store.Persona   `json:"persona"`
		Characters []characterInput `json:"characters"`
		Greeting   string           `json:"greeting"`
		Settings   *settingsInput   `json:"settings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg := s.cfg.Get()
	sess := &store.Session{
		Title:  strings.TrimSpace(body.Title),
		Avatar: strings.TrimSpace(body.Avatar),
		Model:  strings.TrimSpace(body.Model),
		Settings: store.Settings{
			Temperature:     cfg.Temperature,
			MaxTokens:       cfg.MaxTokens,
			MaxSteps:        cfg.MaxSteps,
			ToolChoice:      cfg.ToolChoice,
			ReasoningEffort: cfg.ReasoningEffort,
			ChoicesEnabled:  cfg.ChoicesEnabled,
		},
		State: map[string]any{},
	}
	if body.Persona != nil {
		sess.Persona = *body.Persona
	}
	for _, in := range body.Characters {
		if c := toCharacter(in); c != nil {
			sess.Characters = append(sess.Characters, c)
		}
	}
	applySettings(&sess.Settings, body.Settings)

	if g := strings.TrimSpace(body.Greeting); g != "" {
		speaker := "Narrator"
		if len(sess.Characters) > 0 {
			speaker = sess.Characters[0].Name
		}
		sess.Messages = append(sess.Messages, &store.Message{
			ID: store.NewID(), Role: "assistant", Kind: store.KindSpeech,
			Speaker: speaker, Text: g, CreatedAt: time.Now().UTC(),
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
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, sess.Public())
}

func (s *Server) handlePatchSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lock := s.store.TurnLock(id)
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(id)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	var body struct {
		Title      *string          `json:"title"`
		Avatar     *string          `json:"avatar"`
		Model      *string          `json:"model"`
		Persona    *store.Persona   `json:"persona"`
		Characters []characterInput `json:"characters"`
		State      map[string]any   `json:"state"`
		Settings   *settingsInput   `json:"settings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Title != nil {
		sess.Title = strings.TrimSpace(*body.Title)
	}
	if body.Avatar != nil {
		sess.Avatar = strings.TrimSpace(*body.Avatar)
	}
	if body.Model != nil {
		sess.Model = strings.TrimSpace(*body.Model)
	}
	if body.Persona != nil {
		sess.Persona = *body.Persona
	}
	if body.State != nil {
		sess.State = body.State
	}
	if body.Characters != nil {
		chars := make([]*store.Character, 0, len(body.Characters))
		for _, in := range body.Characters {
			if c := toCharacter(in); c != nil {
				chars = append(chars, c)
			}
		}
		sess.Characters = chars
	}
	applySettings(&sess.Settings, body.Settings)
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sess.Public())
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Delete(r.PathValue("id")); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text   string   `json:"text"`
		Images []string `json:"images"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	text := strings.TrimSpace(body.Text)
	images := cleanAssetNames(body.Images)
	if text == "" && len(images) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("text or images are required"))
		return
	}

	lock := s.store.TurnLock(id)
	lock.Lock()
	defer lock.Unlock()

	sess, err := s.store.Get(id)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}

	user := &store.Message{
		ID: store.NewID(), Role: "user", Kind: store.KindUser,
		Speaker: sess.Persona.Name, Text: text, Images: images, CreatedAt: time.Now().UTC(),
	}
	sess.Messages = append(sess.Messages, user)
	sess.History = append(sess.History, llm.Message{Role: llm.RoleUser, Content: text, Images: assetImages(images)})

	title := ""
	if strings.TrimSpace(sess.Title) == "" {
		sess.Title = deriveTitle(firstNonEmpty(text, "A picture"))
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
		writeError(w, statusFor(err), err)
		return
	}
	sess.History = append(sess.History, llm.Message{
		Role: llm.RoleUser,
		Content: "[system] Begin the story now. Establish the scene with message, give the cast " +
			"something to react to, then finish with the choices tool.",
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
		writeError(w, statusFor(err), err)
		return
	}
	truncateLastTurn(sess)
	if err := s.store.Save(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.streamTurn(w, r, sess, nil, "")
}

func (s *Server) handleAssetUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.Get(id); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxAssetBytes+(1<<20))
	if err := r.ParseMultipartForm(store.MaxAssetBytes + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("expected a multipart image upload"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("missing form field \"file\""))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	name, err := s.store.SaveAsset(id, file, ext)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"name": name,
		"url":  "/api/sessions/" + id + "/assets/" + name,
	})
}

func (s *Server) handleAssetGet(w http.ResponseWriter, r *http.Request) {
	s.serveAsset(w, r.PathValue("id"), r.PathValue("name"))
}

func mimeForAsset(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".opus", ".ogg":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".pcm":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func cleanAssetNames(in []string) []string {
	out := make([]string, 0, len(in))
	for _, name := range in {
		name = strings.TrimSpace(name)
		if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
			continue
		}
		out = append(out, name)
	}
	return out
}

func assetImages(names []string) []llm.Image {
	if len(names) == 0 {
		return nil
	}
	out := make([]llm.Image, 0, len(names))
	for _, n := range names {
		out = append(out, llm.Image{Name: n})
	}
	return out
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func statusFor(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
