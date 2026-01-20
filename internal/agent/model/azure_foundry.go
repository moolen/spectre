// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/model"

	"github.com/moolen/spectre/internal/agent/provider"
)

// AzureFoundryLLM implements the ADK model.LLM interface by wrapping
// the existing Spectre Azure AI Foundry provider.
type AzureFoundryLLM struct {
	provider *provider.AzureFoundryProvider
}

// NewAzureFoundryLLM creates a new AzureFoundryLLM adapter.
// If cfg is nil, default configuration is used with the provided endpoint and key.
func NewAzureFoundryLLM(cfg provider.AzureFoundryConfig) (*AzureFoundryLLM, error) {
	p, err := provider.NewAzureFoundryProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure foundry provider: %w", err)
	}

	return &AzureFoundryLLM{provider: p}, nil
}

// NewAzureFoundryLLMFromProvider wraps an existing AzureFoundryProvider.
func NewAzureFoundryLLMFromProvider(p *provider.AzureFoundryProvider) *AzureFoundryLLM {
	return &AzureFoundryLLM{provider: p}
}

// Name returns the model identifier.
func (a *AzureFoundryLLM) Name() string {
	return a.provider.Model()
}

// GenerateContent implements model.LLM.GenerateContent.
// It converts ADK request format to our provider format, calls the provider,
// and converts the response back to ADK format.
func (a *AzureFoundryLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Convert request using shared conversion functions
		systemPrompt := extractSystemPrompt(req.Config)
		messages := convertContentsToMessages(req.Contents)
		tools := convertToolsFromADK(req.Config)

		// Call the underlying provider (non-streaming only for now)
		resp, err := a.provider.Chat(ctx, systemPrompt, messages, tools)
		if err != nil {
			yield(nil, fmt.Errorf("azure foundry chat failed: %w", err))
			return
		}

		// Convert response to ADK format using shared conversion function
		llmResp := convertResponseToLLMResponse(resp)
		yield(llmResp, nil)
	}
}

// Ensure AzureFoundryLLM implements model.LLM at compile time.
var _ model.LLM = (*AzureFoundryLLM)(nil)
