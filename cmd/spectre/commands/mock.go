//go:build disabled

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/agent/model"
	"github.com/spf13/cobra"
)

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "Send input to a mock LLM agent running in interactive mode",
	Long: `Send text or tool calls to a mock LLM agent running in interactive mode.

This command connects to a mock LLM server started with 'spectre agent --model mock:interactive'
and injects responses that the mock LLM will return to the agent.

Examples:
  # Send a text response
  spectre mock --port 9999 --text "I'll investigate the failing pods now"

  # Send a tool call (JSON format)
  spectre mock --port 9999 --tool list_pods --args '{"namespace": "default"}'

  # Send both text and a tool call
  spectre mock --port 9999 --text "Let me check the pods" --tool list_pods --args '{"namespace": "default"}'
`,
	RunE: runMock,
}

var (
	mockPort     int
	mockText     string
	mockTool     string
	mockToolArgs string
)

func init() {
	rootCmd.AddCommand(mockCmd)

	mockCmd.Flags().IntVar(&mockPort, "port", 0,
		"Port of the mock LLM interactive mode server (required)")
	mockCmd.Flags().StringVar(&mockText, "text", "",
		"Text response to send to the mock LLM")
	mockCmd.Flags().StringVar(&mockTool, "tool", "",
		"Tool name to call (used with --args)")
	mockCmd.Flags().StringVar(&mockToolArgs, "args", "{}",
		"Tool arguments as JSON (used with --tool)")

	_ = mockCmd.MarkFlagRequired("port")
}

func runMock(cmd *cobra.Command, args []string) error {
	// Validate input
	if mockText == "" && mockTool == "" {
		return fmt.Errorf("either --text or --tool must be specified")
	}

	// Build the input
	input := &model.InteractiveInput{}

	if mockText != "" {
		input.Text = mockText
	}

	if mockTool != "" {
		// Parse tool arguments
		var toolArgs map[string]interface{}
		if err := json.Unmarshal([]byte(mockToolArgs), &toolArgs); err != nil {
			return fmt.Errorf("invalid JSON in --args: %w", err)
		}

		input.ToolCalls = []model.MockToolCall{
			{
				Name: mockTool,
				Args: toolArgs,
			},
		}
	}

	// Create client and send
	client := model.NewMockInputClientWithPort(mockPort)
	resp, err := client.Send(input)
	if err != nil {
		return fmt.Errorf("failed to send to mock server: %w", err)
	}

	// Print response
	if resp.IsOK() {
		fmt.Printf("OK: %s\n", resp.Message)
	} else {
		return fmt.Errorf("server error: %s", resp.Error)
	}

	return nil
}
