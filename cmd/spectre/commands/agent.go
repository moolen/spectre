package commands

import (
	"fmt"

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
	// Agent command is temporarily disabled - HTTP client was removed in Phase 7
	// TODO: Refactor agent to use integrated server's gRPC/Connect API instead of HTTP REST
	return fmt.Errorf("agent command is temporarily disabled (HTTP client removed in Phase 7). Use MCP tools via integrated server on port 8080")
}
