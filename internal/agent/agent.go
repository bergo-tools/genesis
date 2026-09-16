package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zp/genesis/internal/llm"
	"github.com/zp/genesis/internal/store"
)

const maxHistoryMessages = 80

// Config carries default generation settings, overridable per session.
type Config struct {
	Model             string
	Temperature       float64
	MaxTokens         int
	MaxSteps          int
	ToolChoice        string
	SystemPrompt      string
	ParallelToolCalls bool
}

// Agent runs the tool-calling loop that drives every roleplay turn.
type Agent struct {
	store    *store.Store
	registry *Registry
	client   func() (llm.Client, error)
	config   func() Config
}

// New wires an agent together. client and config are functions so that
// configuration changes (API key, model) take effect without a restart.
func New(st *store.Store, reg *Registry, client func() (llm.Client, error), cfg func() Config) *Agent {
	return &Agent{store: st, registry: reg, client: client, config: cfg}
}

// Registry exposes the tool registry.
func (a *Agent) Registry() *Registry { return a.registry }

// Continue runs the agent until a terminal tool fires or the step budget runs
// out. It mutates sess in place; the caller is responsible for persisting it.
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

		if len(resp.ToolCalls) == 0 {
			// The model broke protocol. Salvage any visible text so the user
			// still gets a reply, then stop.
			tc := &TurnContext{Session: sess, Emit: emit, Step: step}
			if txt := strings.TrimSpace(resp.Content); txt != "" {
				tc.Show(&store.Message{
					Role:    llm.RoleAssistant,
					Kind:    store.KindNarration,
					Speaker: narratorName(sess),
					Text:    txt,
				})
				sess.History = append(sess.History, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
			}
			emit(Event{Type: EventNotice, Step: step, Text: "The model returned text instead of a tool call. Try a stronger tool-calling model or set tool choice to required."})
			break
		}

		sess.History = append(sess.History, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			Reasoning: resp.Reasoning,
			ToolCalls: resp.ToolCalls,
		})

		tc := &TurnContext{Session: sess, Emit: emit, Step: step}
		terminal := false
		for _, call := range resp.ToolCalls {
			a.executeTool(ctx, tc, call)
			if tool, ok := a.registry.Get(call.Name); ok && tool.Terminal {
				terminal = true
			}
		}
		if terminal {
			break
		}
	}
	return nil
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

	toolMsg := llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: string(resultJSON)}
	if json.Valid(resultJSON) {
		tc.Session.History = append(tc.Session.History, toolMsg)
	}
}

func (a *Agent) buildMessages(sess *store.Session) []llm.Message {
	history := trimHistory(sess.History, maxHistoryMessages)
	msgs := make([]llm.Message, 0, len(history)+1)
	msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: a.SystemPrompt(sess)})
	msgs = append(msgs, history...)
	return msgs
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

// parseArgs unmarshals tool arguments, tolerating empty input.
func parseArgs(raw json.RawMessage, dst any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}
