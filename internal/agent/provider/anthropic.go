//go:build disabled

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements Provider using the Anthropic Claude API.
type AnthropicProvider struct {
	client anthropic.Client
	config Config
}

// NewAnthropicProvider creates a new Anthropic provider.
// The API key is read from the ANTHROPIC_API_KEY environment variable by default.
func NewAnthropicProvider(cfg Config) (*AnthropicProvider, error) {
	if cfg.Model == "" {
		cfg.Model = DefaultConfig().Model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultConfig().MaxTokens
	}

	client := anthropic.NewClient()

	return &AnthropicProvider{
		client: client,
		config: cfg,
	}, nil
}

// NewAnthropicProviderWithKey creates a new Anthropic provider with an explicit API key.
func NewAnthropicProviderWithKey(apiKey string, cfg Config) (*AnthropicProvider, error) {
	if cfg.Model == "" {
		cfg.Model = DefaultConfig().Model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultConfig().MaxTokens
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &AnthropicProvider{
		client: client,
		config: cfg,
	}, nil
}

// Chat implements Provider.Chat for Anthropic.
func (p *AnthropicProvider) Chat(ctx context.Context, systemPrompt string, messages []Message, tools []ToolDefinition) (*Response, error) {
	// Convert messages to Anthropic format
	anthropicMessages := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		anthropicMsg := p.convertMessage(msg)
		anthropicMessages = append(anthropicMessages, anthropicMsg)
	}

	// Build the request parameters
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.config.Model),
		MaxTokens: int64(p.config.MaxTokens),
		Messages:  anthropicMessages,
	}

	// Add system prompt if provided
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	// Add tools if provided
	if len(tools) > 0 {
		anthropicTools := make([]anthropic.ToolUnionParam, 0, len(tools))
		for _, tool := range tools {
			anthropicTool := p.convertToolDefinition(tool)
			anthropicTools = append(anthropicTools, anthropicTool)
		}
		params.Tools = anthropicTools
	}

	// Make the API call
	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API call failed: %w", err)
	}

	// Convert response
	return p.convertResponse(resp), nil
}

// Name implements Provider.Name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Model implements Provider.Model.
func (p *AnthropicProvider) Model() string {
	return p.config.Model
}

// convertMessage converts our Message to Anthropic's MessageParam.
func (p *AnthropicProvider) convertMessage(msg Message) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ToolResult)+1+len(msg.ToolUse))

	// Handle tool results (can have multiple for parallel tool calls)
	for _, toolResult := range msg.ToolResult {
		blocks = append(blocks, anthropic.NewToolResultBlock(
			toolResult.ToolUseID,
			toolResult.Content,
			toolResult.IsError,
		))
	}

	// Handle text content (only if no tool results)
	if msg.Content != "" && len(msg.ToolResult) == 0 {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}

	// Handle tool use (for assistant messages in history)
	for _, toolUse := range msg.ToolUse {
		blocks = append(blocks, anthropic.NewToolUseBlock(
			toolUse.ID,
			toolUse.Input,
			toolUse.Name,
		))
	}

	if msg.Role == RoleAssistant {
		return anthropic.NewAssistantMessage(blocks...)
	}
	return anthropic.NewUserMessage(blocks...)
}

// convertToolDefinition converts our ToolDefinition to Anthropic's ToolParam.
func (p *AnthropicProvider) convertToolDefinition(tool ToolDefinition) anthropic.ToolUnionParam {
	// Extract properties and required from input schema
	properties := tool.InputSchema["properties"]
	required, _ := tool.InputSchema["required"].([]string)

	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   required,
			},
		},
	}
}

// convertResponse converts Anthropic's Message to our Response.
func (p *AnthropicProvider) convertResponse(resp *anthropic.Message) *Response {
	response := &Response{
		Usage: Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
	}

	// Extract content and tool calls from content blocks
	var textParts []string
	for i := range resp.Content {
		block := &resp.Content[i]
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use": //nolint:goconst // block.Type is different type than StopReasonToolUse constant
			response.ToolCalls = append(response.ToolCalls, ToolUseBlock{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	response.Content = strings.Join(textParts, "")

	// Convert stop reason
	switch resp.StopReason {
	case anthropic.StopReasonEndTurn:
		response.StopReason = StopReasonEndTurn
	case anthropic.StopReasonToolUse:
		response.StopReason = StopReasonToolUse
	case anthropic.StopReasonMaxTokens:
		response.StopReason = StopReasonMaxTokens
	case anthropic.StopReasonStopSequence, anthropic.StopReasonPauseTurn, anthropic.StopReasonRefusal:
		response.StopReason = StopReasonEndTurn
	default:
		response.StopReason = StopReasonEndTurn
	}

	return response
}
