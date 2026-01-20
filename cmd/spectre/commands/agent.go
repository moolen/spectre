package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/moolen/spectre/internal/agent/runner"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start the interactive AI agent for incident response",
	Long: `Start an interactive AI-powered incident response agent that helps
investigate Kubernetes cluster issues using natural language.

The agent connects to a running Spectre server and uses Claude to analyze
cluster state, resource relationships, and causal chains.

The agent uses a full terminal UI (TUI) that shows:
- Pipeline progress (intake -> gathering -> hypothesis -> review)
- Which agent is currently active
- Tool calls with timing information
- Context window usage

Examples:
  # Start agent
  spectre agent

  # Connect to a specific Spectre server
  spectre agent --spectre-url http://localhost:8080

  # Use a specific model
  spectre agent --model claude-sonnet-4-5-20250929

  # Use Azure AI Foundry instead of Anthropic
  spectre agent --azure-foundry-endpoint https://your-resource.services.ai.azure.com --azure-foundry-key your-api-key
`,
	RunE: runAgent,
}

var (
	agentSpectreURL           string
	agentAnthropicKey         string
	agentModel                string
	agentAzureFoundryEndpoint string
	agentAzureFoundryKey      string
	agentAuditLog             string
	agentPrompt               string
	agentMockPort             int
	agentMockTools            bool
)

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().StringVar(&agentSpectreURL, "spectre-url", "http://localhost:8080",
		"Spectre API server URL")
	agentCmd.Flags().StringVar(&agentAnthropicKey, "anthropic-key", "",
		"Anthropic API key (defaults to ANTHROPIC_API_KEY env var)")
	agentCmd.Flags().StringVar(&agentModel, "model", "claude-sonnet-4-5-20250929",
		"Claude model to use")

	// Azure AI Foundry flags
	agentCmd.Flags().StringVar(&agentAzureFoundryEndpoint, "azure-foundry-endpoint", "",
		"Azure AI Foundry endpoint URL")
	agentCmd.Flags().StringVar(&agentAzureFoundryKey, "azure-foundry-key", "",
		"Azure AI Foundry API key")

	// Audit logging flag
	agentCmd.Flags().StringVar(&agentAuditLog, "audit-log", "",
		"Path to write agent audit log (JSONL format). If empty, audit logging is disabled.")

	// Initial prompt flag
	agentCmd.Flags().StringVar(&agentPrompt, "prompt", "",
		"Initial prompt to send to the agent (useful for scripting)")

	// Mock LLM flags
	agentCmd.Flags().IntVar(&agentMockPort, "mock-port", 0,
		"Port for mock LLM interactive mode server (0 = random port)")
	agentCmd.Flags().BoolVar(&agentMockTools, "mock-tools", false,
		"Use mock tool responses (canned data instead of real Spectre API)")
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Initialize logging
	if err := setupLog(logLevelFlags); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Get API key
	apiKey := agentAnthropicKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// Handle Azure AI Foundry environment variables
	azureEndpoint := agentAzureFoundryEndpoint
	if azureEndpoint == "" {
		if resource := os.Getenv("ANTHROPIC_FOUNDRY_RESOURCE"); resource != "" {
			azureEndpoint = "https://" + resource + ".services.ai.azure.com"
		}
	}
	azureKey := agentAzureFoundryKey
	if azureKey == "" {
		azureKey = os.Getenv("ANTHROPIC_FOUNDRY_API_KEY")
	}

	// Check for API key - either Anthropic or Azure AI Foundry (skip for mock models)
	isMockModel := strings.HasPrefix(agentModel, "mock")
	if !isMockModel {
		if azureEndpoint != "" {
			if azureKey == "" {
				return fmt.Errorf("Azure AI Foundry API key required. Set ANTHROPIC_FOUNDRY_API_KEY environment variable or use --azure-foundry-key flag")
			}
		} else {
			if apiKey == "" {
				return fmt.Errorf("Anthropic API key required. Set ANTHROPIC_API_KEY environment variable or use --anthropic-key flag")
			}
		}
	}

	cfg := runner.Config{
		SpectreAPIURL:        agentSpectreURL,
		AnthropicAPIKey:      apiKey,
		Model:                agentModel,
		AzureFoundryEndpoint: azureEndpoint,
		AzureFoundryAPIKey:   azureKey,
		AuditLogPath:         agentAuditLog,
		InitialPrompt:        agentPrompt,
		MockPort:             agentMockPort,
		MockTools:            agentMockTools || isMockModel, // Default to mock tools when using mock model
	}

	r, err := runner.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create multi-agent runner: %w", err)
	}

	return r.Run(ctx)
}
