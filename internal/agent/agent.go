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
	// maxHistoryTokens is an approximate budget for the transcript sent to the
	// provider. Latin text is ~4 chars/token and CJK ~1 token/char, so the
	// estimate below errs on the safe side; long stories are trimmed from the
	// front instead of growing until the provider rejects the request.
	maxHistoryTokens = 24000
	maxReminders     = 2
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
	// ChoicesEnabled and DisabledTools are global too: a session cannot switch
	// a tool on or off for itself.
	ChoicesEnabled bool
	DisabledTools  []string
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

	// Generation options are global: every session follows the current
	// config, so editing Settings changes the next turn everywhere.
	steps := cfg.MaxSteps
	if steps <= 0 {
		steps = 6
	}
	toolChoice := llm.ToolChoice(strings.TrimSpace(cfg.ToolChoice))
	if toolChoice == "" {
		toolChoice = llm.ToolChoiceAuto
	}
	temp := cfg.Temperature
	maxTokens := cfg.MaxTokens
	reasoning := strings.TrimSpace(cfg.ReasoningEffort)
	active := a.activeTools()
	choicesRequired := hasTool(active, "choices")
	allowed := make(map[string]bool, len(active))
	for _, t := range active {
		allowed[t.Name] = true
	}

	turnPlayerFacing := false
	turnChoices := false
	reminders := 0

	for step := 1; step <= steps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		emit(Event{Type: EventStatus, Status: "thinking", Step: step})

		resp, err := client.Complete(ctx, llm.Request{
			Model:             cfg.Model,
			Messages:          a.buildMessages(sess),
			Tools:             toolDefs(active),
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
			sess.AddUsage(*resp.Usage)
			stats := sess.Tokens
			emit(Event{Type: EventUsage, Step: step, Usage: resp.Usage, Tokens: &stats})
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

			if choicesRequired && !turnChoices {
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
		lastCall := false
		for _, call := range resp.ToolCalls {
			// Only an allowed tool may steer the turn: a disabled tool must
			// not offer choices, force last_call or terminate the turn.
			if allowed[call.Name] {
				if call.Name == "choices" {
					tc.ChoicesOffered = true
				}
				if wantsLastCall(call.Arguments) {
					lastCall = true
				}
			}
			a.executeTool(ctx, tc, call, allowed)
			if allowed[call.Name] {
				if tool, ok := a.registry.Get(call.Name); ok && tool.Terminal {
					terminal = true
				}
			}
		}
		turnPlayerFacing = turnPlayerFacing || tc.PlayerFacing
		turnChoices = turnChoices || tc.ChoicesOffered

		if terminal || turnChoices {
			break
		}
		if choicesRequired {
			if reminders < maxReminders {
				reminders++
				appendReminder(sess, choicesReminder)
				continue
			}
			emit(Event{Type: EventNotice, Step: step, Text: "The model did not call choices; ending the turn."})
			break
		}
		if lastCall {
			if turnPlayerFacing {
				break
			}
			if reminders < maxReminders {
				reminders++
				appendReminder(sess, messageReminder)
				continue
			}
			break
		}
	}
	return nil
}

// activeTools returns the tools exposed to the model, honouring the global
// disabled list and the choices toggle.
func (a *Agent) activeTools() []*Tool {
	cfg := a.config()
	disabled := map[string]bool{}
	for _, name := range cfg.DisabledTools {
		disabled[strings.TrimSpace(name)] = true
	}
	choicesOn := cfg.ChoicesEnabled && !disabled["choices"]
	out := make([]*Tool, 0, len(a.registry.All()))
	for _, t := range a.registry.All() {
		if disabled[t.Name] {
			continue
		}
		if t.Name == "choices" && !choicesOn {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toolDefs(tools []*Tool) []llm.ToolDef {
	out := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		out = append(out, llm.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	return out
}

func hasTool(tools []*Tool, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

// wantsLastCall reports whether a tool call carries last_call=true.
func wantsLastCall(args string) bool {
	args = strings.TrimSpace(args)
	if args == "" || !json.Valid([]byte(args)) {
		return false
	}
	var probe struct {
		LastCall bool `json:"last_call"`
	}
	if err := json.Unmarshal([]byte(args), &probe); err != nil {
		return false
	}
	return probe.LastCall
}

func appendReminder(sess *store.Session, text string) {
	if sess == nil {
		return
	}
	sess.History = append(sess.History, llm.Message{Role: llm.RoleUser, Content: text})
}

func (a *Agent) executeTool(ctx context.Context, tc *TurnContext, call llm.ToolCall, allowed map[string]bool) {
	raw := json.RawMessage(strings.TrimSpace(call.Arguments))
	if len(raw) == 0 || !json.Valid(raw) {
		raw = json.RawMessage("{}")
	}
	ev := &ToolEvent{ID: call.ID, Name: call.Name, Args: raw}
	if tc.Emit != nil {
		tc.Emit(Event{Type: EventToolStart, Step: tc.Step, Tool: ev})
	}

	tool, known := a.registry.Get(call.Name)
	var (
		result any
		err    error
	)
	switch {
	case !known:
		err = fmt.Errorf("unknown tool %q", call.Name)
	case !allowed[call.Name]:
		// A tool can be switched off for this story; refuse even if the model
		// still asks for it, so per-story toggles really are authoritative.
		err = fmt.Errorf("tool %q is not available in this story", call.Name)
	default:
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
	history := trimHistory(sess.History, maxHistoryMessages, maxHistoryTokens)
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
// must immediately follow their assistant tool-call message. It applies both a
// message cap and an approximate token budget so a long story cannot grow past
// the provider's context window.
func trimHistory(h []llm.Message, maxMessages, maxTokens int) []llm.Message {
	if len(h) == 0 {
		return h
	}
	start := 0
	if maxMessages > 0 && len(h) > maxMessages {
		start = len(h) - maxMessages
	}
	if maxTokens > 0 {
		used := 0
		boundary := start
		for i := len(h) - 1; i >= start; i-- {
			t := estimateTokens(h[i])
			// Always keep at least the newest message, even if it alone is big.
			if used+t > maxTokens && i < len(h)-1 {
				break
			}
			used += t
			boundary = i
		}
		start = boundary
	}
	for start < len(h) && h[start].Role == llm.RoleTool {
		start++
	}
	return h[start:]
}

// estimateTokens approximates the provider token count for one message.
func estimateTokens(m llm.Message) int {
	n := 4 + estimateTextTokens(m.Content) + estimateTextTokens(m.Reasoning)
	for _, tc := range m.ToolCalls {
		n += estimateTextTokens(tc.Name) + estimateTextTokens(tc.Arguments)
	}
	return n
}

// estimateTextTokens counts non-ASCII runes whole and Latin text at roughly
// four characters per token.
func estimateTextTokens(s string) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			other++
		}
	}
	return ascii/4 + other
}
