package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AzureFoundryProvider implements Provider using Azure AI Foundry with Anthropic models.
// Azure AI Foundry uses the same authentication as the standard Anthropic API:
// - Uses "x-api-key" header for authentication
// - Base URL format: https://{resource}.services.ai.azure.com/anthropic/
type AzureFoundryProvider struct {
	client   *http.Client
	config   AzureFoundryConfig
	endpoint string
}

// AzureFoundryConfig contains configuration for Azure AI Foundry.
type AzureFoundryConfig struct {
	// Endpoint is the Azure AI Foundry endpoint URL
	// Format: https://{resource}.services.ai.azure.com
	Endpoint string

	// APIKey is the Azure AI Foundry API key
	APIKey string

	// Model is the model identifier (e.g., "claude-3-5-sonnet")
	Model string

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative)
	Temperature float64

	// Timeout for HTTP requests (default: 120s)
	Timeout time.Duration
}

// DefaultAzureFoundryConfig returns sensible defaults for Azure AI Foundry.
func DefaultAzureFoundryConfig() AzureFoundryConfig {
	return AzureFoundryConfig{
		Model:       "claude-sonnet-4-5-20250929",
		MaxTokens:   4096,
		Temperature: 0.0,
		Timeout:     120 * time.Second,
	}
}

// NewAzureFoundryProvider creates a new Azure AI Foundry provider.
func NewAzureFoundryProvider(cfg AzureFoundryConfig) (*AzureFoundryProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("Azure AI Foundry endpoint is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Azure AI Foundry API key is required")
	}

	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = DefaultAzureFoundryConfig().Model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultAzureFoundryConfig().MaxTokens
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultAzureFoundryConfig().Timeout
	}

	// Normalize endpoint - ensure it ends with /anthropic
	endpoint := strings.TrimSuffix(cfg.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/anthropic") {
		endpoint += "/anthropic"
	}

	return &AzureFoundryProvider{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		config:   cfg,
		endpoint: endpoint,
	}, nil
}

// Chat implements Provider.Chat for Azure AI Foundry.
func (p *AzureFoundryProvider) Chat(ctx context.Context, systemPrompt string, messages []Message, tools []ToolDefinition) (*Response, error) {
	// Build the request body
	reqBody := p.buildRequest(systemPrompt, messages, tools)

	// Serialize to JSON
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.endpoint + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - Azure AI Foundry uses standard Anthropic "x-api-key" header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, p.parseErrorResponse(resp.StatusCode, body)
	}

	// Parse response
	return p.parseResponse(body)
}

// Name implements Provider.Name.
func (p *AzureFoundryProvider) Name() string {
	return "azure-foundry"
}

// Model implements Provider.Model.
func (p *AzureFoundryProvider) Model() string {
	return p.config.Model
}

// Request types for Azure AI Foundry (compatible with Anthropic API)

type azureRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	Messages    []azureMessage   `json:"messages"`
	System      []azureTextBlock `json:"system,omitempty"`
	Tools       []azureTool      `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
}

type azureMessage struct {
	Role    string             `json:"role"`
	Content []azureContentPart `json:"content"`
}

type azureContentPart struct {
	Type string `json:"type"`

	// For text blocks
	Text string `json:"text,omitempty"`

	// For tool_use blocks
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result blocks
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type azureTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type azureTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema azureInputSchema `json:"input_schema"`
}

type azureInputSchema struct {
	Type       string      `json:"type"`
	Properties interface{} `json:"properties,omitempty"`
	Required   []string    `json:"required,omitempty"`
}

// Response types

type azureResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []azureResponseBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   string               `json:"stop_reason"`
	StopSequence *string              `json:"stop_sequence"`
	Usage        azureUsage           `json:"usage"`
}

type azureResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type azureUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type azureErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// buildRequest creates the Azure AI Foundry request body.
func (p *AzureFoundryProvider) buildRequest(systemPrompt string, messages []Message, tools []ToolDefinition) azureRequest {
	req := azureRequest{
		Model:     p.config.Model,
		MaxTokens: p.config.MaxTokens,
	}

	// Add temperature if non-zero
	if p.config.Temperature > 0 {
		req.Temperature = p.config.Temperature
	}

	// Add system prompt
	if systemPrompt != "" {
		req.System = []azureTextBlock{
			{Type: "text", Text: systemPrompt},
		}
	}

	// Convert messages
	for _, msg := range messages {
		azureMsg := p.convertMessage(msg)
		req.Messages = append(req.Messages, azureMsg)
	}

	// Convert tools
	for _, tool := range tools {
		azureTool := p.convertTool(tool)
		req.Tools = append(req.Tools, azureTool)
	}

	return req
}

// convertMessage converts our Message to Azure format.
func (p *AzureFoundryProvider) convertMessage(msg Message) azureMessage {
	azureMsg := azureMessage{
		Role: string(msg.Role),
	}

	// Handle tool results (can have multiple for parallel tool calls)
	for _, toolResult := range msg.ToolResult {
		azureMsg.Content = append(azureMsg.Content, azureContentPart{
			Type:      "tool_result",
			ToolUseID: toolResult.ToolUseID,
			Content:   toolResult.Content,
			IsError:   toolResult.IsError,
		})
	}

	// Handle text content (only if no tool results)
	if msg.Content != "" && len(msg.ToolResult) == 0 {
		azureMsg.Content = append(azureMsg.Content, azureContentPart{
			Type: "text",
			Text: msg.Content,
		})
	}

	// Handle tool use (for assistant messages in history)
	for _, toolUse := range msg.ToolUse {
		azureMsg.Content = append(azureMsg.Content, azureContentPart{
			Type:  "tool_use",
			ID:    toolUse.ID,
			Name:  toolUse.Name,
			Input: toolUse.Input,
		})
	}

	return azureMsg
}

// convertTool converts our ToolDefinition to Azure format.
func (p *AzureFoundryProvider) convertTool(tool ToolDefinition) azureTool {
	properties := tool.InputSchema["properties"]
	required, _ := tool.InputSchema["required"].([]string)

	return azureTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: azureInputSchema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}
}

// parseResponse parses the Azure AI Foundry response.
func (p *AzureFoundryProvider) parseResponse(body []byte) (*Response, error) {
	var azureResp azureResponse
	if err := json.Unmarshal(body, &azureResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	response := &Response{
		Usage: Usage{
			InputTokens:  azureResp.Usage.InputTokens,
			OutputTokens: azureResp.Usage.OutputTokens,
		},
	}

	// Extract content and tool calls
	var textParts []string
	for _, block := range azureResp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			response.ToolCalls = append(response.ToolCalls, ToolUseBlock{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	response.Content = strings.Join(textParts, "")

	// Convert stop reason
	switch azureResp.StopReason {
	case "end_turn":
		response.StopReason = StopReasonEndTurn
	case "tool_use":
		response.StopReason = StopReasonToolUse
	case "max_tokens":
		response.StopReason = StopReasonMaxTokens
	default:
		response.StopReason = StopReasonEndTurn
	}

	return response, nil
}

// parseErrorResponse parses an error response from Azure AI Foundry.
func (p *AzureFoundryProvider) parseErrorResponse(statusCode int, body []byte) error {
	var errResp azureErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("Azure AI Foundry API error (status %d): %s", statusCode, string(body))
	}

	return fmt.Errorf("Azure AI Foundry API error (status %d, type: %s): %s",
		statusCode, errResp.Error.Type, errResp.Error.Message)
}
