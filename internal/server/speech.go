package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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

const maxSpeechInputRunes = 4000

func (s *Server) speechClient() (*llm.OpenRouterClient, error) {
	c := s.cfg.Get()
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("no API key configured; add one in Settings")
	}
	return llm.NewOpenRouterClient(c.APIKey, c.BaseURL, ""), nil
}

// handleSpeech synthesizes one text block. Results are content-addressed and
// cached on disk, so replaying the same block is free.
func (s *Server) handleSpeech(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		MessageID string `json:"messageId"`
		Text      string `json:"text"`
		Speaker   string `json:"speaker"`
		Voice     string `json:"voice"`
		Force     bool   `json:"force"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sess, err := s.store.Get(id)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}

	text := strings.TrimSpace(body.Text)
	speaker := strings.TrimSpace(body.Speaker)
	if text == "" && strings.TrimSpace(body.MessageID) != "" {
		if m := findMessage(sess, body.MessageID); m != nil {
			text = strings.TrimSpace(m.Text)
			if speaker == "" {
				speaker = m.Speaker
			}
		}
	}
	if text == "" {
		writeError(w, http.StatusBadRequest, errors.New("text or a valid messageId is required"))
		return
	}
	runes := []rune(text)
	if len(runes) > maxSpeechInputRunes {
		text = string(runes[:maxSpeechInputRunes])
	}

	cfg := s.cfg.Get()
	model := strings.TrimSpace(cfg.SpeechModel)
	if model == "" {
		writeError(w, http.StatusBadRequest, errors.New("no speech model configured; set one in Settings"))
		return
	}
	voice := resolveVoice(sess, speaker, body.Voice, cfg.SpeechVoice)
	name := speechAssetName(model, voice, cfg.SpeechFormat, text) + llm.SpeechExtension(cfg.SpeechFormat)

	if !body.Force && s.store.AssetExists(id, name) {
		s.serveAsset(w, id, name)
		return
	}

	client, err := s.speechClient()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := client.Speech(r.Context(), llm.SpeechRequest{
		Model:  model,
		Voice:  voice,
		Input:  text,
		Format: cfg.SpeechFormat,
		Speed:  cfg.SpeechSpeed,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if err := s.store.WriteAsset(id, name, res.Audio); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.serveAsset(w, id, name)
}

func (s *Server) serveAsset(w http.ResponseWriter, id, name string) {
	f, err := s.store.OpenAsset(id, name)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", mimeForAsset(name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func speechAssetName(model, voice, format, text string) string {
	sum := sha1.Sum([]byte(model + "|" + voice + "|" + format + "|" + text))
	return "speech-" + hex.EncodeToString(sum[:])[:20]
}

func resolveVoice(sess *store.Session, speaker, explicit, fallback string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if sess != nil && strings.TrimSpace(speaker) != "" {
		if c := sess.FindCharacter(speaker); c != nil && strings.TrimSpace(c.Voice) != "" {
			return strings.TrimSpace(c.Voice)
		}
	}
	return strings.TrimSpace(fallback)
}

func findMessage(sess *store.Session, id string) *store.Message {
	if sess == nil {
		return nil
	}
	for _, m := range sess.Messages {
		if m != nil && m.ID == id {
			return m
		}
	}
	return nil
}

// speechModelInfo is a TTS model together with the voices that model accepts.
// Each provider names its voices differently, so the catalog is per model.
type speechModelInfo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Voices []string `json:"voices"`
}

func (s *Server) handleSpeechModels(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	if strings.TrimSpace(c.APIKey) == "" {
		writeError(w, http.StatusBadRequest, errors.New("add an API key before listing speech models"))
		return
	}
	models, err := fetchSpeechModels(r.Context(), c.BaseURL, c.APIKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// fetchSpeechModels lists TTS models and their supported voices. OpenRouter
// exposes the catalog on the model object as "supported_voices"; filtering by
// output_modalities=speech keeps the response small.
func fetchSpeechModels(ctx context.Context, baseURL, apiKey string) ([]speechModelInfo, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = config.Default().BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models?output_modalities=speech", nil)
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
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data []struct {
			ID     string   `json:"id"`
			Name   string   `json:"name"`
			Voices []string `json:"supported_voices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]speechModelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		out = append(out, speechModelInfo{ID: m.ID, Name: name, Voices: m.Voices})
	}
	return out, nil
}
