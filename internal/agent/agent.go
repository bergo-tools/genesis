package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

const (
	maxHistoryMessages = 80
	maxReminders       = 2
)

const (
	choicesReminder = "[system] You ended the turn without calling the choices tool. Choices are " +
		"enabled, so you MUST finish with the choices tool before stopping. Call it now with the " +
		"player's next two to four options."
	messageReminder = "[system] You have not shown the player anything yet. Use the message tool to " +
		"speak or narrate, then finish the turn with the choices tool."
)

// Config carries default generation settings, overridable per story.
type Config struct {
	Model             string
	Temperature       float64
	MaxTokens         int
	MaxSteps          int
	ToolChoice        string
	SystemPrompt      string
	ParallelToolCalls bool
	ReasoningEffort   string
}

// Agent runs the tool-calling loop that drives every roleplay turn.
type Agent struct {
	store    *store.Store
	registry *Registry
	client   func() (llm.Client, error)
	config   func() Config
}

// New wires an agent together. client and config are functions so that
// configuration changes take effect without a restart.
func New(st *store.Store, reg *Registry, client func() (llm.Client, error), cfg func() Config) *Agent {
	return &Agent{store: st, registry: reg, client: client, config: cfg}
}

// Registry exposes the tool registry.
func (a *Agent) Registry() *Registry { return a.registry }

// Continue runs the agent until the choices tool fires, the model stops, or
// the step budget is exhausted. It mutates sess in place; the caller persists.
func (a *Agent) Continue(ctx context.Context, sess *store.Session, emit func(Event)) error {
	if sess == nil {
		return errors.New("agent: nil session")
	}
	if emit == nil {
		emit = func(Event) {}
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	cfg := a.config()

	steps := sess.Settings.MaxSteps
	if steps <= 0 {
		steps = cfg.MaxSteps
	}
	if steps <= 0 {
		steps = 6
	}
	toolChoice := llm.ToolChoice(strings.TrimSpace(sess.Settings.ToolChoice))
	if toolChoice == "" {
		toolChoice = llm.ToolChoice(strings.TrimSpace(cfg.ToolChoice))
	}
	if toolChoice == "" {
		toolChoice = llm.ToolChoiceAuto
	}
	temp := sess.Settings.Temperature
	if temp == 0 {
		temp = cfg.Temperature
	}
	maxTokens := sess.Settings.MaxTokens
	if maxTokens <= 0 {
		maxTokens = cfg.MaxTokens
	}
	reasoning := strings.TrimSpace(sess.Settings.ReasoningEffort)
	if reasoning == "" {
		reasoning = cfg.ReasoningEffort
	}
	choicesEnabled := sess.Settings.ChoicesEnabled

	turnPlayerFacing := false
	turnChoices := false
	reminders := 0

	for step := 1; step <= steps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		emit(Event{Type: EventStatus, Status: "thinking", Step: step})

		resp, err := client.Complete(ctx, llm.Request{
			Model:             sess.Model,
			Messages:          a.buildMessages(sess),
			Tools:             a.registry.Defs(),
			ToolChoice:        toolChoice,
			Temperature:       temp,
			MaxTokens:         maxTokens,
			ParallelToolCalls: cfg.ParallelToolCalls,
			ReasoningEffort:   reasoning,
		})
		if err != nil {
			return err
		}
		if resp == nil {
			return errors.New("agent: empty completion")
		}
		if resp.Usage != nil {
			emit(Event{Type: EventUsage, Step: step, Usage: resp.Usage})
		}
		if strings.TrimSpace(resp.Reasoning) != "" {
			emit(Event{Type: EventReasoning, Step: step, Text: resp.Reasoning})
		}

		tc := &TurnContext{Session: sess, Emit: emit, Step: step}

		if len(resp.ToolCalls) == 0 {
			if txt := strings.TrimSpace(resp.Content); txt != "" {
				tc.Show(&store.Message{
					Role:    llm.RoleAssistant,
					Kind:    store.KindNarration,
					Speaker: narratorName(sess),
					Text:    txt,
				})
				sess.History = append(sess.History, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
			}
			turnPlayerFacing = turnPlayerFacing || tc.PlayerFacing

			if choicesEnabled && !turnChoices {
				if reminders < maxReminders {
					reminders++
					appendReminder(sess, choicesReminder)
					continue
				}
				emit(Event{Type: EventNotice, Step: step, Text: "The model stopped before offering choices; ending the turn."})
				break
			}
			if !turnPlayerFacing {
				if reminders < maxReminders {
					reminders++
					appendReminder(sess, messageReminder)
					continue
				}
			}
			break
		}

		sess.History = append(sess.History, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			Reasoning: resp.Reasoning,
			ToolCalls: resp.ToolCalls,
		})

		terminal := false
		for _, call := range resp.ToolCalls {
			if call.Name == "choices" {
				tc.ChoicesOffered = true
			}
			a.executeTool(ctx, tc, call)
			if tool, ok := a.registry.Get(call.Name); ok && tool.Terminal {
				terminal = true
			}
		}
		turnPlayerFacing = turnPlayerFacing || tc.PlayerFacing
		turnChoices = turnChoices || tc.ChoicesOffered

		if terminal || turnChoices {
			break
		}
		if choicesEnabled {
			if reminders < maxReminders {
				reminders++
				appendReminder(sess, choicesReminder)
				continue
			}
			emit(Event{Type: EventNotice, Step: step, Text: "The model did not call choices; ending the turn."})
			break
		}
	}
	return nil
}

func appendReminder(sess *store.Session, text string) {
	if sess == nil {
		return
	}
	sess.History = append(sess.History, llm.Message{Role: llm.RoleUser, Content: text})
}

func (a *Agent) executeTool(ctx context.Context, tc *TurnContext, call llm.ToolCall) {
	raw := json.RawMessage(strings.TrimSpace(call.Arguments))
	if len(raw) == 0 || !json.Valid(raw) {
		raw = json.RawMessage("{}")
	}
	ev := &ToolEvent{ID: call.ID, Name: call.Name, Args: raw}
	if tc.Emit != nil {
		tc.Emit(Event{Type: EventToolStart, Step: tc.Step, Tool: ev})
	}

	tool, ok := a.registry.Get(call.Name)
	var (
		result any
		err    error
	)
	if !ok {
		err = fmt.Errorf("unknown tool %q", call.Name)
	} else {
		result, err = tool.Handler(ctx, tc, raw)
	}

	var resultJSON json.RawMessage
	if err != nil {
		resultJSON, _ = json.Marshal(map[string]any{"error": err.Error()})
		ev.Error = err.Error()
	} else {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			resultJSON, _ = json.Marshal(map[string]any{"error": err.Error()})
			ev.Error = err.Error()
		}
	}
	ev.Result = resultJSON
	if tc.Emit != nil {
		tc.Emit(Event{Type: EventToolEnd, Step: tc.Step, Tool: ev})
	}
	if json.Valid(resultJSON) {
		tc.Session.History = append(tc.Session.History, llm.Message{
			Role:       llm.RoleTool,
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    string(resultJSON),
		})
	}
}

func (a *Agent) buildMessages(sess *store.Session) []llm.Message {
	history := trimHistory(sess.History, maxHistoryMessages)
	msgs := make([]llm.Message, 0, len(history)+1)
	msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: a.SystemPrompt(sess)})
	for _, m := range history {
		if m.Role == llm.RoleUser && len(m.Images) > 0 {
			cp := m
			cp.Images = a.resolveImages(sess.ID, m.Images)
			msgs = append(msgs, cp)
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs
}

// resolveImages turns stored asset names into data URLs for the provider.
func (a *Agent) resolveImages(storyID string, images []llm.Image) []llm.Image {
	if a.store == nil {
		return nil
	}
	out := make([]llm.Image, 0, len(images))
	for _, img := range images {
		if strings.TrimSpace(img.DataURL) != "" {
			out = append(out, img)
			continue
		}
		name := strings.TrimSpace(img.Name)
		if name == "" {
			continue
		}
		path, err := a.store.AssetPath(storyID, name)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, llm.Image{
			Name:    name,
			DataURL: "data:" + mimeForExt(filepath.Ext(name)) + ";base64," + base64.StdEncoding.EncodeToString(data),
		})
	}
	return out
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// trimHistory keeps the newest messages without orphaning tool results that
// must immediately follow their assistant tool-call message.
func trimHistory(h []llm.Message, max int) []llm.Message {
	if max <= 0 || len(h) <= max {
		return h
	}
	start := len(h) - max
	for start < len(h) && h[start].Role == llm.RoleTool {
		start++
	}
	return h[start:]
}
