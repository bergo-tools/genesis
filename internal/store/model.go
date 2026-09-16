package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/zp/genesis/internal/llm"
)

// Session is a single roleplay conversation plus its world state.
type Session struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	Model      string         `json:"model,omitempty"`
	Settings   Settings       `json:"settings"`
	Persona    Persona        `json:"persona"`
	Characters []*Character   `json:"characters"`
	Scene      Scene          `json:"scene"`
	Messages   []*Message     `json:"messages"`
	Memories   []*Memory      `json:"memories"`
	State      map[string]any `json:"state,omitempty"`

	// History is the model-facing transcript (the system prompt is rebuilt each
	// turn and therefore is not stored here).
	History []llm.Message `json:"history,omitempty"`
}

// Settings holds per-session generation overrides.
type Settings struct {
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"maxTokens"`
	MaxSteps     int     `json:"maxSteps"`
	ToolChoice   string  `json:"toolChoice"`
	SystemPrompt string  `json:"systemPrompt,omitempty"`
}

// Persona describes the player.
type Persona struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Character is an AI-controlled actor.
type Character struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Personality string         `json:"personality,omitempty"`
	Scenario    string         `json:"scenario,omitempty"`
	Appearance  string         `json:"appearance,omitempty"`
	Avatar      string         `json:"avatar,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	State       map[string]any `json:"state,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// Scene is the current location and atmosphere.
type Scene struct {
	Location   string `json:"location,omitempty"`
	Time       string `json:"time,omitempty"`
	Weather    string `json:"weather,omitempty"`
	Background string `json:"background,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// Message kinds rendered by the UI.
const (
	KindUser      = "user"
	KindSpeech    = "speech"
	KindAction    = "action"
	KindNarration = "narration"
	KindThought   = "thought"
	KindOOC       = "ooc"
	KindDice      = "dice"
	KindPrompt    = "prompt"
	KindSystem    = "system"
)

// Message is a single displayable event in the chat.
type Message struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Kind      string          `json:"kind"`
	Speaker   string          `json:"speaker,omitempty"`
	Text      string          `json:"text,omitempty"`
	Mood      string          `json:"mood,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	Meta      map[string]any  `json:"meta,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// Memory is a long-term fact the agent chose to remember.
type Memory struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Importance int       `json:"importance"`
	Tags       []string  `json:"tags,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Choice is a suggested player action.
type Choice struct {
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
}

// Public returns a copy of the session safe to send to the browser.
func (s *Session) Public() *Session {
	if s == nil {
		return nil
	}
	cp := *s
	cp.History = nil
	return &cp
}

// FindCharacter returns the first character matching id or name.
func (s *Session) FindCharacter(idOrName string) *Character {
	if s == nil {
		return nil
	}
	for _, c := range s.Characters {
		if c == nil {
			continue
		}
		if c.ID == idOrName || equalFold(c.Name, idOrName) {
			return c
		}
	}
	return nil
}

// PrimaryCharacter returns the first character, if any.
func (s *Session) PrimaryCharacter() *Character {
	if s == nil || len(s.Characters) == 0 {
		return nil
	}
	return s.Characters[0]
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// NewID returns a short random identifier.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(b[:])
}
