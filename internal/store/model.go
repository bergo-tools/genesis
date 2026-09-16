package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/zp/genesis/internal/llm"
)

// Session is one conversation: the cast and story instructions snapshotted
// from a preset, plus this playthrough's transcript and world state. Model and
// generation options are not stored here — they are read from the global
// config on every turn, so a Settings change applies to running stories too.
type Session struct {
	ID string `json:"id"`
	// StoryID is the preset this conversation was started from.
	StoryID    string         `json:"storyId,omitempty"`
	StoryTitle string         `json:"storyTitle,omitempty"`
	Title      string         `json:"title"`
	Avatar     string         `json:"avatar,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	Settings   Settings       `json:"settings"`
	Persona    Persona        `json:"persona"`
	Characters []*Character   `json:"characters"`
	Messages   []*Message     `json:"messages"`
	State      map[string]any `json:"state,omitempty"`
	Scene      Scene          `json:"scene,omitempty"`
	// Tokens records how much of the model's context window this session uses.
	Tokens TokenStats `json:"tokens,omitempty"`
	// PendingChoices holds the branches offered by the last choices call so
	// they survive a page reload. They are cleared when the player replies.
	PendingChoices []Choice `json:"pendingChoices,omitempty"`
	PendingPrompt  string   `json:"pendingPrompt,omitempty"`

	// History is the model-facing transcript. The system prompt is rebuilt on
	// every turn and is therefore not stored here. Image entries keep only the
	// asset name; bytes are resolved when a request is built.
	History []llm.Message `json:"history,omitempty"`
}

// Story is a reusable preset: a cast, an opening, and its story instructions.
// Neither a preset nor a session carries generation options — model choice,
// temperature, token budgets and tool switches are global (internal/config).
type Story struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Avatar      string         `json:"avatar,omitempty"`
	Description string         `json:"description,omitempty"`
	Genre       string         `json:"genre,omitempty"`
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

// Settings is the content-level configuration shared by presets and sessions.
// The only option it holds is the story instructions. The JSON shape stays
// "settings"/"systemPrompt", so records written when sessions still pinned
// model and generation options load unchanged; those extra keys are ignored.
type Settings struct {
	SystemPrompt string `json:"systemPrompt,omitempty"`
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
	KindScene     = "scene"
	KindSystem    = "system"
)

// Message is one displayable event in the chat.
type Message struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Kind    string `json:"kind"`
	Speaker string `json:"speaker,omitempty"`
	Text    string `json:"text,omitempty"`
	Thought string `json:"thought,omitempty"`
	// OOC is an out-of-character instruction the player sent with this
	// message. It is shown separately and passed to the model as [OOC] text.
	OOC       string    `json:"ooc,omitempty"`
	Mood      string    `json:"mood,omitempty"`
	Images    []string  `json:"images,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// Snapshot is the world as it was before this user turn ran. A re-roll or
	// edit restores it so the scene cannot drift out of sync with the messages.
	Snapshot *TurnSnapshot `json:"snapshot,omitempty"`
}

// TurnSnapshot captures the mutable world state a turn may change.
type TurnSnapshot struct {
	Scene Scene          `json:"scene,omitempty"`
	State map[string]any `json:"state,omitempty"`
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

// TokenStats records the token accounting of one session. Last* describes the
// most recent completion (the current context size); Total* accumulates over
// the whole session.
type TokenStats struct {
	LastPromptTokens      int       `json:"lastPromptTokens,omitempty"`
	LastCompletionTokens  int       `json:"lastCompletionTokens,omitempty"`
	LastCachedTokens      int       `json:"lastCachedTokens,omitempty"`
	TotalPromptTokens     int       `json:"totalPromptTokens,omitempty"`
	TotalCompletionTokens int       `json:"totalCompletionTokens,omitempty"`
	TotalCachedTokens     int       `json:"totalCachedTokens,omitempty"`
	TotalCost             float64   `json:"totalCost,omitempty"`
	Requests              int       `json:"requests,omitempty"`
	UpdatedAt             time.Time `json:"updatedAt,omitempty"`
}

// AddUsage folds one completion's accounting into the session.
func (s *Session) AddUsage(u llm.Usage) {
	if s == nil {
		return
	}
	s.Tokens.LastPromptTokens = u.PromptTokens
	s.Tokens.LastCompletionTokens = u.CompletionTokens
	s.Tokens.LastCachedTokens = u.CachedTokens
	s.Tokens.TotalPromptTokens += u.PromptTokens
	s.Tokens.TotalCompletionTokens += u.CompletionTokens
	s.Tokens.TotalCachedTokens += u.CachedTokens
	s.Tokens.TotalCost += u.Cost
	s.Tokens.Requests++
	s.Tokens.UpdatedAt = time.Now().UTC()
}

// NewID returns a short random identifier.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(b[:])
}
