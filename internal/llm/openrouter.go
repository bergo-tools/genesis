package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
)

// OpenRouterClient adapts the official OpenRouter Go SDK to the llm.Client
// interface. The SDK speaks the OpenAI-compatible chat protocol, so a custom
// base URL also lets Genesis talk to any compatible endpoint.
type OpenRouterClient struct {
	sdk   *openrouter.OpenRouter
	model string
}

// NewOpenRouterClient builds a client. baseURL and model may be empty.
func NewOpenRouterClient(apiKey, baseURL, model string) *OpenRouterClient {
	opts := []openrouter.SDKOption{}
	if strings.TrimSpace(apiKey) != "" {
		opts = append(opts, openrouter.WithSecurity(apiKey))
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, openrouter.WithServerURL(strings.TrimRight(baseURL, "/")))
	}
	return &OpenRouterClient{sdk: openrouter.New(opts...), model: model}
}

// Complete performs one non-streaming, tool-capable chat completion.
func (c *OpenRouterClient) Complete(ctx context.Context, req Request) (*Response, error) {
	if c == nil || c.sdk == nil {
		return nil, errors.New("llm: openrouter client is not configured")
	}
	model := req.Model
	if strings.TrimSpace(model) == "" {
		model = c.model
	}
	sdkReq := components.ChatRequest{
		Messages: toSDKMessages(req.Messages),
	}
	if strings.TrimSpace(model) != "" {
		sdkReq.Model = openrouter.Pointer(model)
	}
	if len(req.Tools) > 0 {
		sdkReq.Tools = toSDKTools(req.Tools)
		sdkReq.ToolChoice = toSDKToolChoice(req.ToolChoice)
		sdkReq.ParallelToolCalls = optionalnullable.From(openrouter.Pointer(req.ParallelToolCalls))
	}
	if req.Temperature > 0 {
		sdkReq.Temperature = optionalnullable.From(openrouter.Pointer(req.Temperature))
	}
	if req.MaxTokens > 0 {
		sdkReq.MaxTokens = optionalnullable.From(openrouter.Pointer(int64(req.MaxTokens)))
	}
	if level := normalizeEffort(req.ReasoningEffort); level != "" {
		sdkReq.ReasoningEffort = optionalnullable.From(openrouter.Pointer(components.ChatRequestReasoningEffort(level)))
	}

	res, err := c.sdk.Chat.Send(ctx, sdkReq, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("llm: empty response from provider")
	}
	if res.ChatResult == nil {
		if res.EventStream != nil {
			_ = res.EventStream.Close()
		}
		return nil, errors.New("llm: provider returned a stream but streaming was not requested")
	}
	result := res.ChatResult
	if len(result.Choices) == 0 {
		return nil, errors.New("llm: provider returned no choices")
	}
	choice := result.Choices[0]
	out := &Response{
		Model:        result.Model,
		FinishReason: finishReason(choice.FinishReason),
	}
	if content, ok := choice.Message.Content.Get(); ok && content != nil {
		out.Content = assistantContentToText(*content)
	}
	if reasoning, ok := choice.Message.Reasoning.Get(); ok && reasoning != nil {
		out.Reasoning = *reasoning
	}
	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	if result.Usage != nil {
		out.Usage = &Usage{
			PromptTokens:     int(result.Usage.PromptTokens),
			CompletionTokens: int(result.Usage.CompletionTokens),
			TotalTokens:      int(result.Usage.TotalTokens),
		}
	}
	return out, nil
}

// normalizeEffort maps UI values onto OpenRouter reasoning effort values.
// An empty result means "do not send the parameter".
func normalizeEffort(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "default", "provider":
		return ""
	case "off", "none", "disabled", "false", "no":
		return "none"
	case "minimal", "low", "medium", "high", "max", "xhigh":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return ""
	}
}

func assistantContentToText(content components.ChatAssistantMessageContent) string {
	if content.Str != nil {
		return *content.Str
	}
	b, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &parts); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		if p.Text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(p.Text)
	}
	return sb.String()
}

func finishReason(v *components.ChatFinishReasonEnum) string {
	if v == nil {
		return ""
	}
	return string(*v)
}

func toSDKMessages(msgs []Message) []components.ChatMessages {
	out := make([]components.ChatMessages, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			out = append(out, components.CreateChatMessagesSystem(components.ChatSystemMessage{
				Content: components.CreateChatSystemMessageContentStr(m.Content),
				Role:    components.ChatSystemMessageRoleSystem,
			}))
		case RoleUser:
			out = append(out, toSDKUserMessage(m))
		case RoleAssistant:
			am := components.ChatAssistantMessage{}
			if m.Content != "" {
				content := components.CreateChatAssistantMessageContentStr(m.Content)
				am.Content = optionalnullable.From(&content)
			}
			for _, tc := range m.ToolCalls {
				args := tc.Arguments
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				am.ToolCalls = append(am.ToolCalls, components.ChatToolCall{
					ID:   tc.ID,
					Type: components.ChatToolCallTypeFunction,
					Function: components.ChatToolCallFunction{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
			out = append(out, components.CreateChatMessagesAssistant(am))
		case RoleTool:
			out = append(out, components.CreateChatMessagesTool(components.ChatToolMessage{
				Content:    components.CreateChatToolMessageContentStr(m.Content),
				Role:       components.ChatToolMessageRoleTool,
				ToolCallID: m.ToolCallID,
			}))
		}
	}
	return out
}

func toSDKUserMessage(m Message) components.ChatMessages {
	if len(m.Images) == 0 {
		return components.CreateChatMessagesUser(components.ChatUserMessage{
			Content: components.CreateChatUserMessageContentStr(m.Content),
			Role:    components.ChatUserMessageRoleUser,
		})
	}
	items := make([]components.ChatContentItems, 0, len(m.Images)+1)
	if strings.TrimSpace(m.Content) != "" {
		items = append(items, components.CreateChatContentItemsText(components.ChatContentText{
			Text: m.Content,
			Type: components.ChatContentTextTypeText,
		}))
	}
	for _, img := range m.Images {
		if strings.TrimSpace(img.DataURL) == "" {
			continue
		}
		items = append(items, components.CreateChatContentItemsImageURL(components.ChatContentImage{
			ImageURL: components.ChatContentImageImageURL{URL: img.DataURL},
			Type:     components.ChatContentImageType("image_url"),
		}))
	}
	if len(items) == 0 {
		return components.CreateChatMessagesUser(components.ChatUserMessage{
			Content: components.CreateChatUserMessageContentStr(m.Content),
			Role:    components.ChatUserMessageRoleUser,
		})
	}
	return components.CreateChatMessagesUser(components.ChatUserMessage{
		Content: components.CreateChatUserMessageContentArrayOfChatContentItems(items),
		Role:    components.ChatUserMessageRoleUser,
	})
}

func toSDKTools(defs []ToolDef) []components.ChatFunctionTool {
	out := make([]components.ChatFunctionTool, 0, len(defs))
	for _, d := range defs {
		fn := components.ChatFunctionToolFunctionFunction{
			Name:       d.Name,
			Parameters: d.Parameters,
		}
		if d.Description != "" {
			desc := d.Description
			fn.Description = &desc
		}
		out = append(out, components.CreateChatFunctionToolChatFunctionToolFunction(components.ChatFunctionToolFunction{
			Function: fn,
			Type:     components.ChatFunctionToolTypeFunction,
		}))
	}
	return out
}

func toSDKToolChoice(choice ToolChoice) *components.ChatToolChoice {
	switch choice {
	case ToolChoiceRequired:
		c := components.CreateChatToolChoiceChatToolChoiceRequired(components.ChatToolChoiceRequiredRequired)
		return &c
	case ToolChoiceNone:
		c := components.CreateChatToolChoiceChatToolChoiceNone(components.ChatToolChoiceNoneNone)
		return &c
	default:
		c := components.CreateChatToolChoiceChatToolChoiceAuto(components.ChatToolChoiceAutoAuto)
		return &c
	}
}
