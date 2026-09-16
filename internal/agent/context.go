package agent

import (
	"encoding/json"
	"time"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

// Event types streamed to the browser as newline-delimited JSON.
const (
	EventStatus      = "status"
	EventMessage     = "message"
	EventState       = "state"
	EventChoices     = "choices"
	EventToolStart   = "tool_start"
	EventToolEnd     = "tool_end"
	EventReasoning   = "reasoning"
	EventUsage       = "usage"
	EventTurnEnd     = "turn_end"
	EventError       = "error"
	EventNotice      = "notice"
	EventTitle       = "title"
	EventUserMessage = "user_message"
)

// ToolEvent describes a single tool invocation for the activity feed.
type ToolEvent struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Event is one streamed update from the agent.
type Event struct {
	Type    string         `json:"type"`
	Step    int            `json:"step,omitempty"`
	Status  string         `json:"status,omitempty"`
	Text    string         `json:"text,omitempty"`
	Title   string         `json:"title,omitempty"`
	Message *store.Message `json:"message,omitempty"`
	State   map[string]any `json:"state,omitempty"`
	Choices []store.Choice `json:"choices,omitempty"`
	Prompt  string         `json:"prompt,omitempty"`
	Tool    *ToolEvent     `json:"tool,omitempty"`
	Usage   *llm.Usage     `json:"usage,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// TurnContext is handed to every tool handler for the duration of a turn.
type TurnContext struct {
	Session *store.Session
	Emit    func(Event)
	Step    int

	// PlayerFacing is set when a tool shows the player something other than a
	// private thought, i.e. speech, action, narration or a choice prompt.
	PlayerFacing bool
	// ChoicesOffered is set when the choices tool runs.
	ChoicesOffered bool
}

// AddMessage appends a display message to the story, assigning id/time.
func (tc *TurnContext) AddMessage(m *store.Message) {
	if tc == nil || tc.Session == nil || m == nil {
		return
	}
	if m.ID == "" {
		m.ID = store.NewID()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	tc.Session.Messages = append(tc.Session.Messages, m)
}

// Show appends a display message and streams it to the browser.
func (tc *TurnContext) Show(m *store.Message) {
	if tc == nil {
		return
	}
	tc.AddMessage(m)
	if tc.Emit != nil {
		tc.Emit(Event{Type: EventMessage, Step: tc.Step, Message: m})
	}
	if m != nil && m.Role == "assistant" && m.Kind != store.KindThought && m.Kind != store.KindState {
		tc.PlayerFacing = true
	}
}

// PrimaryName returns the lead character's name or a sensible fallback.
func (tc *TurnContext) PrimaryName() string {
	return narratorName(tc.Session)
}

func narratorName(sess *store.Session) string {
	if sess != nil {
		if c := sess.PrimaryCharacter(); c != nil && c.Name != "" {
			return c.Name
		}
	}
	return "Narrator"
}
