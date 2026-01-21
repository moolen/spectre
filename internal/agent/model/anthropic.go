//go:build disabled

// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/moolen/spectre/internal/agent/provider"
)

// AnthropicLLM implements the ADK model.LLM interface by wrapping
// the existing Spectre Anthropic provider.
type AnthropicLLM struct {
	provider *provider.AnthropicProvider
}

// NewAnthropicLLM creates a new AnthropicLLM adapter.
// If cfg is nil, default configuration is used.
func NewAnthropicLLM(cfg *provider.Config) (*AnthropicLLM, error) {
	c := provider.DefaultConfig()
	if cfg != nil {
		c = *cfg
	}

	p, err := provider.NewAnthropicProvider(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create anthropic provider: %w", err)
	}

	return &AnthropicLLM{provider: p}, nil
}

// NewAnthropicLLMWithKey creates a new AnthropicLLM adapter with an explicit API key.
func NewAnthropicLLMWithKey(apiKey string, cfg *provider.Config) (*AnthropicLLM, error) {
	c := provider.DefaultConfig()
	if cfg != nil {
		c = *cfg
	}

	p, err := provider.NewAnthropicProviderWithKey(apiKey, c)
	if err != nil {
		return nil, fmt.Errorf("failed to create anthropic provider: %w", err)
	}

	return &AnthropicLLM{provider: p}, nil
}

// NewAnthropicLLMFromProvider wraps an existing AnthropicProvider.
func NewAnthropicLLMFromProvider(p *provider.AnthropicProvider) *AnthropicLLM {
	return &AnthropicLLM{provider: p}
}

// Name returns the model identifier.
func (a *AnthropicLLM) Name() string {
	return a.provider.Model()
}

// GenerateContent implements model.LLM.GenerateContent.
// It converts ADK request format to our provider format, calls the provider,
// and converts the response back to ADK format.
func (a *AnthropicLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Convert request
		systemPrompt := extractSystemPrompt(req.Config)
		messages := convertContentsToMessages(req.Contents)
		tools := convertToolsFromADK(req.Config)

		// Call the underlying provider (non-streaming only for now)
		resp, err := a.provider.Chat(ctx, systemPrompt, messages, tools)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic chat failed: %w", err))
			return
		}

		// Convert response to ADK format
		llmResp := convertResponseToLLMResponse(resp)
		yield(llmResp, nil)
	}
}

// extractSystemPrompt extracts the system instruction from the config.
func extractSystemPrompt(cfg *genai.GenerateContentConfig) string {
	if cfg == nil || cfg.SystemInstruction == nil {
		return ""
	}

	var parts []string
	for _, part := range cfg.SystemInstruction.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "\n" + parts[i]
	}
	return result
}

// convertContentsToMessages converts genai.Content slice to provider.Message slice.
func convertContentsToMessages(contents []*genai.Content) []provider.Message {
	var messages []provider.Message

	for _, content := range contents {
		if content == nil {
			continue
		}

		msg := provider.Message{}

		// Map roles: "user" -> RoleUser, "model" -> RoleAssistant
		switch content.Role {
		case "user":
			msg.Role = provider.RoleUser
		case "model":
			msg.Role = provider.RoleAssistant
		default:
			msg.Role = provider.RoleUser
		}

		// Process parts
		for _, part := range content.Parts {
			if part == nil {
				continue
			}

			// Handle text content
			if part.Text != "" {
				if msg.Content != "" {
					msg.Content += "\n"
				}
				msg.Content += part.Text
			}

			// Handle function calls (model requesting tool use)
			if part.FunctionCall != nil {
				toolUse := provider.ToolUseBlock{
					ID:   part.FunctionCall.ID,
					Name: part.FunctionCall.Name,
				}
				// Convert Args map to json.RawMessage
				if part.FunctionCall.Args != nil {
					argsJSON, err := json.Marshal(part.FunctionCall.Args)
					if err == nil {
						toolUse.Input = argsJSON
					}
				}
				msg.ToolUse = append(msg.ToolUse, toolUse)
			}

			// Handle function responses (user providing tool results)
			if part.FunctionResponse != nil {
				// Function responses become tool results
				// Convert the response map to a string
				responseStr := ""
				if part.FunctionResponse.Response != nil {
					respJSON, err := json.Marshal(part.FunctionResponse.Response)
					if err == nil {
						responseStr = string(respJSON)
					}
				}
				msg.ToolResult = append(msg.ToolResult, provider.ToolResultBlock{
					ToolUseID: part.FunctionResponse.ID,
					Content:   responseStr,
					IsError:   false,
				})
			}
		}

		// Only add message if it has content, tool use, or tool result
		if msg.Content != "" || len(msg.ToolUse) > 0 || len(msg.ToolResult) > 0 {
			messages = append(messages, msg)
		}
	}

	return messages
}

// convertToolsFromADK converts ADK tool configuration to provider.ToolDefinition slice.
func convertToolsFromADK(cfg *genai.GenerateContentConfig) []provider.ToolDefinition {
	if cfg == nil || len(cfg.Tools) == 0 {
		return nil
	}

	var tools []provider.ToolDefinition

	for _, tool := range cfg.Tools {
		if tool == nil || len(tool.FunctionDeclarations) == 0 {
			continue
		}

		for _, fn := range tool.FunctionDeclarations {
			if fn == nil {
				continue
			}

			toolDef := provider.ToolDefinition{
				Name:        fn.Name,
				Description: fn.Description,
				InputSchema: convertSchemaToMap(fn.Parameters, fn.ParametersJsonSchema),
			}
			tools = append(tools, toolDef)
		}
	}

	return tools
}

// convertSchemaToMap converts a genai.Schema or raw JSON schema to a map.
func convertSchemaToMap(schema *genai.Schema, jsonSchema any) map[string]interface{} {
	// If a raw JSON schema is provided, use it directly
	if jsonSchema != nil {
		if m, ok := jsonSchema.(map[string]interface{}); ok {
			return m
		}
		// Try to convert via JSON marshaling
		data, err := json.Marshal(jsonSchema)
		if err == nil {
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil {
				return m
			}
		}
	}

	// Convert genai.Schema to map
	if schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	result := make(map[string]interface{})

	// Set type
	if schema.Type != "" {
		result["type"] = schemaTypeToString(schema.Type)
	} else {
		result["type"] = "object"
	}

	// Set description
	if schema.Description != "" {
		result["description"] = schema.Description
	}

	// Set properties (for object types)
	if len(schema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, propSchema := range schema.Properties {
			props[name] = convertSchemaToMap(propSchema, nil)
		}
		result["properties"] = props
	}

	// Set required fields
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Set items (for array types)
	if schema.Items != nil {
		result["items"] = convertSchemaToMap(schema.Items, nil)
	}

	// Set enum values
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	return result
}

// schemaTypeToString converts genai.Type to a JSON Schema type string.
func schemaTypeToString(t genai.Type) string {
	const typeObject = "object"

	switch t {
	case genai.TypeString:
		return "string"
	case genai.TypeNumber:
		return "number"
	case genai.TypeInteger:
		return "integer"
	case genai.TypeBoolean:
		return "boolean"
	case genai.TypeArray:
		return "array"
	case genai.TypeObject:
		return typeObject
	case genai.TypeUnspecified, genai.TypeNULL:
		return typeObject
	default:
		return typeObject
	}
}

// convertResponseToLLMResponse converts a provider.Response to model.LLMResponse.
func convertResponseToLLMResponse(resp *provider.Response) *model.LLMResponse {
	if resp == nil {
		return &model.LLMResponse{}
	}

	// Build content parts
	parts := make([]*genai.Part, 0, 1+len(resp.ToolCalls))

	// Add text content if present
	if resp.Content != "" {
		parts = append(parts, &genai.Part{
			Text: resp.Content,
		})
	}

	// Add function calls if present
	for _, toolCall := range resp.ToolCalls {
		// Convert json.RawMessage to map[string]any
		var args map[string]any
		if toolCall.Input != nil {
			_ = json.Unmarshal(toolCall.Input, &args)
		}

		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   toolCall.ID,
				Name: toolCall.Name,
				Args: args,
			},
		})
	}

	// Create the content
	content := &genai.Content{
		Parts: parts,
		Role:  "model",
	}

	// Map finish reason
	var finishReason genai.FinishReason
	switch resp.StopReason {
	case provider.StopReasonEndTurn:
		finishReason = genai.FinishReasonStop
	case provider.StopReasonToolUse:
		finishReason = genai.FinishReasonStop // ADK handles tool use differently
	case provider.StopReasonMaxTokens:
		finishReason = genai.FinishReasonMaxTokens
	case provider.StopReasonError:
		finishReason = genai.FinishReasonOther
	default:
		finishReason = genai.FinishReasonStop
	}

	return &model.LLMResponse{
		Content:      content,
		FinishReason: finishReason,
		TurnComplete: true,
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			// Token counts from API are int but proto uses int32. Values are always positive and typically < 100k.
			// #nosec G115 -- Token counts are bounded by API limits (max context ~200k tokens fits in int32)
			PromptTokenCount:     int32(resp.Usage.InputTokens),
			CandidatesTokenCount: int32(resp.Usage.OutputTokens), // #nosec G115 -- Safe conversion, bounded values
			TotalTokenCount:      int32(resp.Usage.InputTokens + resp.Usage.OutputTokens), // #nosec G115 -- Safe conversion, bounded values
		},
	}
}

// Ensure AnthropicLLM implements model.LLM at compile time.
var _ model.LLM = (*AnthropicLLM)(nil)
