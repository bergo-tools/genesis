package server

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

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
