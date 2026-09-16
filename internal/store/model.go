package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/zp/genesis/internal/llm"
)

// Session is one conversation: the cast and defaults snapshotted from a
// story preset, plus this playthrough's transcript and world state.
type Session struct {
	ID string `json:"id"`
	// StoryID is the preset this conversation was started from.
	StoryID    string         `json:"storyId,omitempty"`
	StoryTitle string         `json:"storyTitle,omitempty"`
	Title      string         `json:"title"`
	Avatar     string         `json:"avatar,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	Model      string         `json:"model,omitempty"`
	Settings   Settings       `json:"settings"`
	Persona    Persona        `json:"persona"`
	Characters []*Character   `json:"characters"`
	Messages   []*Message     `json:"messages"`
	State      map[string]any `json:"state,omitempty"`
	Scene      Scene          `json:"scene,omitempty"`
	// PendingChoices holds the branches offered by the last choices call so
	// they survive a page reload. They are cleared when the player replies.
	PendingChoices []Choice `json:"pendingChoices,omitempty"`
	PendingPrompt  string   `json:"pendingPrompt,omitempty"`

	// History is the model-facing transcript. The system prompt is rebuilt on
	// every turn and is therefore not stored here. Image entries keep only the
	// asset name; bytes are resolved when a request is built.
	History []llm.Message `json:"history,omitempty"`
}

// Story is a reusable preset: a cast, an opening, and default settings.
// Sessions are started from a story and snapshot its cast and defaults.
type Story struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Avatar      string         `json:"avatar,omitempty"`
	Description string         `json:"description,omitempty"`
	Genre       string         `json:"genre,omitempty"`
	Model       string         `json:"model,omitempty"`
	Opening     string         `json:"opening,omitempty"`
	Persona     Persona        `json:"persona,omitempty"`
	Characters  []*Character   `json:"characters"`
	Settings    Settings       `json:"settings"`
	State       map[string]any `json:"state,omitempty"`
	Scene       Scene          `json:"scene,omitempty"`
	Builtin     bool           `json:"builtin,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// Settings holds generation options, shared by stories and sessions.
type Settings struct {
	Temperature     float64 `json:"temperature"`
	MaxTokens       int     `json:"maxTokens"`
	MaxSteps        int     `json:"maxSteps"`
	ToolChoice      string  `json:"toolChoice"`
	SystemPrompt    string  `json:"systemPrompt,omitempty"`
	ReasoningEffort string  `json:"reasoningEffort,omitempty"`
	ChoicesEnabled  bool    `json:"choicesEnabled"`
	// DisabledTools lists tool names switched off for this story. An empty
	// list means every registered tool is available.
	DisabledTools []string `json:"disabledTools,omitempty"`
}

// Persona describes the player.
type Persona struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Character is one AI-controlled actor in the story.
type Character struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Personality string    `json:"personality,omitempty"`
	Avatar      string    `json:"avatar,omitempty"`
	Voice       string    `json:"voice,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Scene describes where the story currently is.
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
	KindPrompt    = "prompt"
	KindState     = "state"
	KindScene     = "scene"
	KindSystem    = "system"
)

// Message is one displayable event in the chat.
type Message struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Kind      string          `json:"kind"`
	Speaker   string          `json:"speaker,omitempty"`
	Text      string          `json:"text,omitempty"`
	Thought   string          `json:"thought,omitempty"`
	Mood      string          `json:"mood,omitempty"`
	Images    []string        `json:"images,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// Choice is one branch offered to the player.
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

// CharacterNames returns the cast names in order.
func (s *Session) CharacterNames() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Characters))
	for _, c := range s.Characters {
		if c != nil && c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names
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
