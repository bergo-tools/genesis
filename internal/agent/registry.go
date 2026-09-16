package agent

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/zp/genesis/internal/llm"
)

// Handler executes one tool call. It may mutate the session through tc and must
// return a JSON-serialisable value that is fed back to the model.
type Handler func(ctx context.Context, tc *TurnContext, args json.RawMessage) (any, error)

// Tool describes a single callable capability.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Category    string
	Handler     Handler
	// Terminal tools end the agent turn once every call in the step has run.
	Terminal bool
	// Query marks read-only information tools.
	Query bool
}

// ToolMeta is the browser-facing description of a tool.
type ToolMeta struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Parameters  map[string]any `json:"parameters"`
	Terminal    bool           `json:"terminal"`
	Query       bool           `json:"query"`
}

// Registry is an ordered collection of tools. Adding a capability is a single
// Register call, which is what makes new tools cheap to introduce.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
	order []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register adds or replaces a tool.
func (r *Registry) Register(t *Tool) {
	if t == nil || t.Name == "" {
		panic("agent: tool requires a name")
	}
	if t.Parameters == nil {
		t.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name]; !exists {
		r.order = append(r.order, t.Name)
	}
	r.tools[t.Name] = t
}

// Get looks a tool up by name.
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// All returns tools in registration order.
func (r *Registry) All() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Tool, 0, len(r.order))
	for _, name := range r.order {
		if t, ok := r.tools[name]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Names returns tool names in registration order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Defs converts the registry into model-facing tool definitions.
func (r *Registry) Defs() []llm.ToolDef {
	all := r.All()
	out := make([]llm.ToolDef, 0, len(all))
	for _, t := range all {
		out = append(out, llm.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	return out
}

// Meta returns browser-facing metadata for every tool.
func (r *Registry) Meta() []ToolMeta {
	all := r.All()
	out := make([]ToolMeta, 0, len(all))
	for _, t := range all {
		out = append(out, ToolMeta{
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
			Parameters:  t.Parameters,
			Terminal:    t.Terminal,
			Query:       t.Query,
		})
	}
	return out
}
