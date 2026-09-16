// Package llm defines a small provider-neutral chat model used by the agent
// loop. It mirrors only the subset of the OpenAI-style chat API that Genesis
// needs so tool-calling and multimodal logic stay SDK-independent.
package llm

import "context"

// Roles understood by the agent loop.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Image is an attached picture. Name is persisted with the story; DataURL is
// resolved on demand and never written to disk.
type Image struct {
	Name    string `json:"name,omitempty"`
	DataURL string `json:"dataUrl,omitempty"`
}

// ToolCall is a single request from the model to invoke a tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message is one entry in the model-facing transcript.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Images     []Image    `json:"images,omitempty"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string     `json:"toolCallId,omitempty"`
	Name       string     `json:"name,omitempty"`
	Reasoning  string     `json:"reasoning,omitempty"`
}

// ToolDef describes a callable tool using JSON Schema.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolChoice controls how the model may use tools.
type ToolChoice string

const (
	ToolChoiceAuto     ToolChoice = "auto"
	ToolChoiceRequired ToolChoice = "required"
	ToolChoiceNone     ToolChoice = "none"
)

// Request is a single completion request.
type Request struct {
	Model             string
	Messages          []Message
	Tools             []ToolDef
	ToolChoice        ToolChoice
	Temperature       float64
	MaxTokens         int
	ParallelToolCalls bool
	// ReasoningEffort is one of "", "none"/"off", "minimal", "low", "medium",
	// "high", "max". Empty leaves the provider default untouched.
	ReasoningEffort string
}

// Usage reports token accounting.
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// Response is a single completion result.
type Response struct {
	Content      string
	Reasoning    string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        *Usage
	Model        string
}

// Client is implemented by any backend able to perform tool-calling chat
// completions.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
