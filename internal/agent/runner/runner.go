//go:build disabled

// Package runner provides the CLI runner for the multi-agent incident response system.
// It wraps ADK's runner with Spectre-specific UI rendering and CLI interaction.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/moolen/spectre/internal/agent/audit"
	"github.com/moolen/spectre/internal/agent/commands"
	"github.com/moolen/spectre/internal/agent/incident"
	"github.com/moolen/spectre/internal/agent/model"
	"github.com/moolen/spectre/internal/agent/provider"
	"github.com/moolen/spectre/internal/agent/tools"
	"github.com/moolen/spectre/internal/agent/tui"
	"github.com/moolen/spectre/internal/mcp/client"
)

const (
	// AppName is the ADK application name for Spectre.
	AppName = "spectre"

	// DefaultUserID is used when no user ID is specified.
	DefaultUserID = "default"
)

// Config contains the runner configuration.
type Config struct {
	// SpectreAPIURL is the URL of the Spectre API server.
	SpectreAPIURL string

	// AnthropicAPIKey is the Anthropic API key.
	AnthropicAPIKey string

	// Model is the model name to use (e.g., "claude-sonnet-4-5-20250929").
	Model string

	// SessionID allows resuming a previous session (optional).
	SessionID string

	// AzureFoundryEndpoint is the Azure AI Foundry endpoint URL.
	// If set, Azure AI Foundry will be used instead of Anthropic.
	AzureFoundryEndpoint string

	// AzureFoundryAPIKey is the Azure AI Foundry API key.
	AzureFoundryAPIKey string

	// AuditLogPath is the path to write the audit log (JSONL format).
	// If empty, audit logging is disabled.
	AuditLogPath string

	// InitialPrompt is an optional prompt to send immediately when starting.
	// If set, this will be processed before entering interactive mode.
	InitialPrompt string

	// MockPort is the port for the mock LLM interactive mode server.
	// Only used when Model starts with "mock:interactive".
	MockPort int

	// MockTools enables mock tool responses when using mock LLM.
	// When true, tools return canned responses instead of calling the real Spectre API.
	MockTools bool
}

// Runner manages the multi-agent incident response system.
type Runner struct {
	config Config

	// ADK components
	adkRunner      *runner.Runner
	sessionService adksession.Service
	sessionID      string
	userID         string

	// Spectre components
	spectreClient *client.SpectreClient
	toolRegistry  *tools.Registry

	// Audit logging
	auditLogger *audit.Logger

	// LLM metrics tracking
	totalLLMRequests  int
	totalInputTokens  int
	totalOutputTokens int

	// TUI components
	tuiProgram           *tea.Program
	tuiPendingQuestion   *tools.PendingUserQuestion // Track pending question for TUI mode
	tuiPendingQuestionMu sync.Mutex                 // Protect pending question access

	// Mock LLM components
	mockInputServer *model.MockInputServer // Server for interactive mock mode
}

// New creates a new multi-agent Runner.
func New(cfg Config) (*Runner, error) {
	r := &Runner{
		config:         cfg,
		userID:         DefaultUserID,
		sessionService: adksession.InMemoryService(),
	}

	// Initialize Spectre client
	r.spectreClient = client.NewSpectreClient(cfg.SpectreAPIURL)

	// Create session ID first (needed for default audit log path)
	var sessionID string
	if cfg.SessionID != "" {
		sessionID = cfg.SessionID
	} else {
		sessionID = uuid.NewString()
	}

	// Set default audit log path if not specified
	auditLogPath := cfg.AuditLogPath
	if auditLogPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			sessionsDir := filepath.Join(home, ".spectre", "sessions")
			if err := os.MkdirAll(sessionsDir, 0750); err == nil {
				auditLogPath = filepath.Join(sessionsDir, sessionID+".audit.log")
			}
		}
	}

	// Create structured logger for tool registry
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create LLM adapter - auto-detect provider based on configuration
	var llm adkmodel.LLM
	var err error

	if strings.HasPrefix(cfg.Model, "mock") {
		// Use mock LLM for testing
		llm, err = r.createMockLLM(cfg.Model, cfg.MockPort)
		if err != nil {
			return nil, fmt.Errorf("failed to create mock LLM: %w", err)
		}

		// Use mock tool registry for mock mode (returns canned responses)
		if cfg.MockTools {
			r.toolRegistry = tools.NewMockRegistry()
		} else {
			// Even in mock mode, can use real tools if explicitly disabled
			r.toolRegistry = tools.NewRegistry(tools.Dependencies{
				SpectreClient: r.spectreClient,
				Logger:        logger,
			})
		}
	} else {
		// Initialize real tool registry
		r.toolRegistry = tools.NewRegistry(tools.Dependencies{
			SpectreClient: r.spectreClient,
			Logger:        logger,
		})

		if cfg.AzureFoundryEndpoint != "" {
			// Use Azure AI Foundry provider
			azureCfg := provider.AzureFoundryConfig{
				Endpoint: cfg.AzureFoundryEndpoint,
				APIKey:   cfg.AzureFoundryAPIKey,
				Model:    cfg.Model,
			}
			llm, err = model.NewAzureFoundryLLM(azureCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create Azure Foundry LLM: %w", err)
			}
		} else {
			// Use Anthropic provider
			providerCfg := &provider.Config{
				Model: cfg.Model,
			}
			llm, err = model.NewAnthropicLLMWithKey(cfg.AnthropicAPIKey, providerCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create Anthropic LLM: %w", err)
			}
		}
	}

	// Create the incident response agent (single agent approach)
	incidentAgent, err := incident.New(llm, r.toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create incident agent: %w", err)
	}

	// Create ADK runner
	r.adkRunner, err = runner.New(runner.Config{
		AppName:        AppName,
		Agent:          incidentAgent,
		SessionService: r.sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ADK runner: %w", err)
	}

	// Set session ID
	r.sessionID = sessionID

	// Initialize audit logger with default or configured path
	if auditLogPath != "" {
		auditLogger, err := audit.NewLogger(auditLogPath, r.sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit logger: %w", err)
		}
		r.auditLogger = auditLogger
	}

	return r, nil
}

// Run starts the interactive agent loop with the TUI.
func (r *Runner) Run(ctx context.Context) error {
	// Check Spectre API connectivity
	if err := r.spectreClient.Ping(); err != nil {
		// We'll show this in the TUI later
		_ = err
	}

	// Create session
	_, err := r.sessionService.Create(ctx, &adksession.CreateRequest{
		AppName:   AppName,
		UserID:    r.userID,
		SessionID: r.sessionID,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Log session start to audit log
	if r.auditLogger != nil {
		_ = r.auditLogger.LogSessionStart(r.config.Model, r.config.SpectreAPIURL)
	}

	// Create event channel for TUI updates
	eventCh := make(chan interface{}, 100)

	// Create TUI model
	tuiModel := tui.NewModel(eventCh, r.sessionID, r.config.SpectreAPIURL, r.config.Model)

	// Create TUI program with a custom model that wraps the input handling
	wrappedModel := &tuiModelWrapper{
		Model:         &tuiModel,
		runner:        r,
		eventCh:       eventCh,
		ctx:           ctx,
		initialPrompt: r.config.InitialPrompt,
	}

	// Create TUI program
	r.tuiProgram = tea.NewProgram(
		wrappedModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support for scrolling
		tea.WithContext(ctx),
	)

	// Run the TUI program
	_, err = r.tuiProgram.Run()

	// Log session end and close audit logger
	if r.auditLogger != nil {
		_ = r.auditLogger.LogSessionMetrics(r.totalLLMRequests, r.totalInputTokens, r.totalOutputTokens)
		_ = r.auditLogger.LogSessionEnd()
		_ = r.auditLogger.Close()
	}

	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	close(eventCh)
	return nil
}

// tuiModelWrapper wraps the TUI model to intercept input submissions.
type tuiModelWrapper struct {
	*tui.Model
	runner        *Runner
	eventCh       chan interface{}
	ctx           context.Context
	initialPrompt string
}

// Update intercepts InputSubmittedMsg to trigger agent processing.
func (w *tuiModelWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for input submission
	if inputMsg, ok := msg.(tui.InputSubmittedMsg); ok {
		// Check if this is a slash command
		cmd := commands.ParseCommand(inputMsg.Input)
		if cmd != nil {
			// Execute command and send result
			go func() {
				ctx := &commands.Context{
					SessionID:         w.runner.sessionID,
					TotalLLMRequests:  w.runner.totalLLMRequests,
					TotalInputTokens:  w.runner.totalInputTokens,
					TotalOutputTokens: w.runner.totalOutputTokens,
					QuitFunc: func() {
						if w.runner.tuiProgram != nil {
							w.runner.tuiProgram.Quit()
						}
					},
				}
				result := commands.DefaultRegistry.Execute(ctx, cmd)
				w.eventCh <- tui.CommandExecutedMsg{
					Success: result.Success,
					Message: result.Message,
					IsInfo:  result.IsInfo,
				}
			}()
			// Don't process as a message to the LLM
		} else {
			// Not a command, process as normal message
			// Process the input in a goroutine
			go func() {
				// Check if this is a response to a pending question
				message := inputMsg.Input

				w.runner.tuiPendingQuestionMu.Lock()
				pendingQuestion := w.runner.tuiPendingQuestion
				if pendingQuestion != nil {
					// Parse the user response and build contextual message
					parsedResponse := tools.ParseUserResponse(inputMsg.Input, pendingQuestion.DefaultConfirm)

					if parsedResponse.Confirmed {
						message = fmt.Sprintf("User confirmed the incident summary. Please continue routing to root_cause_agent to proceed with the investigation. The user's confirmation response: %q", inputMsg.Input)
					} else if parsedResponse.HasClarification {
						message = fmt.Sprintf("User provided clarification instead of confirming. Their response: %q. Please process this clarification and re-confirm with the user if needed.", inputMsg.Input)
					} else {
						message = fmt.Sprintf("User rejected the summary with response: %q. Please ask what needs to be corrected.", inputMsg.Input)
					}

					// Clear the pending question
					w.runner.tuiPendingQuestion = nil
				}
				w.runner.tuiPendingQuestionMu.Unlock()

				if err := w.runner.processMessageWithTUI(w.ctx, message, w.eventCh); err != nil {
					w.eventCh <- tui.ErrorMsg{Error: err}
				}
			}()
		}
		// Continue with the normal update
	}

	// Delegate to the wrapped model
	newModel, cmd := w.Model.Update(msg)
	if m, ok := newModel.(*tui.Model); ok {
		w.Model = m
	}
	return w, cmd
}

// View delegates to the wrapped model.
func (w *tuiModelWrapper) View() string {
	return w.Model.View()
}

// Init delegates to the wrapped model and handles initial prompt.
func (w *tuiModelWrapper) Init() tea.Cmd {
	return w.Model.Init()
	// Temporarily disabled initial prompt handling for debugging
	// cmds := []tea.Cmd{w.Model.Init()}
	// if w.initialPrompt != "" && !w.promptSent {
	// 	w.promptSent = true
	// 	cmds = append(cmds, func() tea.Msg {
	// 		return tui.InitialPromptMsg{Prompt: w.initialPrompt}
	// 	})
	// }
	// return tea.Batch(cmds...)
}

// processMessageWithTUI processes a message and sends events to the TUI.
func (r *Runner) processMessageWithTUI(ctx context.Context, message string, eventCh chan<- interface{}) error {
	// Log user message to audit log
	if r.auditLogger != nil {
		_ = r.auditLogger.LogUserMessage(message)
	}

	// Create user content
	userContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: message},
		},
	}

	// Run the agent
	runConfig := agent.RunConfig{
		StreamingMode: agent.StreamingModeNone,
	}

	var currentAgent string
	var lastTextResponse string
	toolStartTimes := make(map[string]time.Time)                   // Key is tool call ID (or name if no ID)
	askUserQuestionArgs := make(map[string]map[string]interface{}) // Store ask_user_question args by tool key
	completedSent := false
	pipelineStart := time.Now()
	totalTokensUsed := 0
	var pendingQuestion *tools.PendingUserQuestion // Track if a user question is pending

	// Get model context window size (default to Claude's 200k)
	contextMax := 200000
	if r.config.Model == "claude-sonnet-4-5-20250929" || r.config.Model == "claude-3-5-sonnet-20241022" {
		contextMax = 200000
	} else if r.config.Model == "claude-3-opus-20240229" {
		contextMax = 200000
	} else if r.config.Model == "claude-3-haiku-20240307" {
		contextMax = 200000
	}

	for event, err := range r.adkRunner.Run(ctx, r.userID, r.sessionID, userContent, runConfig) {
		if err != nil {
			if r.auditLogger != nil {
				_ = r.auditLogger.LogError(currentAgent, err)
			}
			eventCh <- tui.ErrorMsg{Error: err}
			return fmt.Errorf("agent error: %w", err)
		}

		if event == nil {
			continue
		}

		// Update context usage from event metadata
		if event.UsageMetadata != nil {
			// Use prompt token count as the "context used" since it represents
			// how much of the context window is being used for input
			if event.UsageMetadata.PromptTokenCount > 0 {
				totalTokensUsed = int(event.UsageMetadata.PromptTokenCount)
				eventCh <- tui.ContextUpdateMsg{
					Used: totalTokensUsed,
					Max:  contextMax,
				}

				// Track LLM metrics
				inputTokens := int(event.UsageMetadata.PromptTokenCount)
				outputTokens := int(event.UsageMetadata.CandidatesTokenCount)

				r.totalLLMRequests++
				r.totalInputTokens += inputTokens
				r.totalOutputTokens += outputTokens

				// Determine provider
				provider := "anthropic"
				if r.config.AzureFoundryEndpoint != "" {
					provider = "azure_foundry"
				}

				// Determine stop reason based on event content
				stopReason := "end_turn"
				if event.Content != nil {
					for _, part := range event.Content.Parts {
						if part.FunctionCall != nil {
							stopReason = "tool_use"
							break
						}
					}
				}

				// Log LLM request to audit log
				if r.auditLogger != nil {
					_ = r.auditLogger.LogLLMRequest(provider, r.config.Model, inputTokens, outputTokens, stopReason)
				}
			}
		}

		// Check for agent change (from event.Author)
		if event.Author != "" && event.Author != currentAgent {
			currentAgent = event.Author
			eventCh <- tui.AgentActivatedMsg{Name: currentAgent}

			// Log agent activation to audit log
			if r.auditLogger != nil {
				_ = r.auditLogger.LogAgentActivated(currentAgent)
			}
		}

		// Check for function calls (tool use)
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil {
					toolName := part.FunctionCall.Name
					// Use ID if available, otherwise fall back to name
					toolKey := part.FunctionCall.ID
					if toolKey == "" {
						toolKey = toolName
					}
					toolStartTimes[toolKey] = time.Now()

					// Store args for ask_user_question so we can extract them when response arrives
					if toolName == "ask_user_question" {
						askUserQuestionArgs[toolKey] = part.FunctionCall.Args
					}

					eventCh <- tui.ToolStartedMsg{
						Agent:    currentAgent,
						ToolID:   toolKey,
						ToolName: toolName,
					}

					// Log tool start to audit log
					if r.auditLogger != nil {
						_ = r.auditLogger.LogToolStart(currentAgent, toolName, part.FunctionCall.Args)
					}
				}
				if part.FunctionResponse != nil {
					toolName := part.FunctionResponse.Name
					// Use ID if available, otherwise fall back to name
					toolKey := part.FunctionResponse.ID
					if toolKey == "" {
						toolKey = toolName
					}

					// Calculate duration
					var duration time.Duration
					if startTime, ok := toolStartTimes[toolKey]; ok {
						duration = time.Since(startTime)
						delete(toolStartTimes, toolKey) // Clean up
					}

					// Check if tool succeeded (simple heuristic)
					success := true
					summary := ""
					if errMsg, exists := part.FunctionResponse.Response["error"]; exists && errMsg != nil {
						success = false
						summary = fmt.Sprintf("%v", errMsg)
					}

					// Check if this is ask_user_question with pending status
					if toolName == "ask_user_question" {
						if status, ok := part.FunctionResponse.Response["status"].(string); ok && status == "pending" {
							// Extract the question from the stored FunctionCall args
							if args, ok := askUserQuestionArgs[toolKey]; ok {
								question := ""
								summary := ""
								defaultConfirm := false

								if q, ok := args["question"].(string); ok {
									question = q
								}
								if s, ok := args["summary"].(string); ok {
									summary = s
								}
								if dc, ok := args["default_confirm"].(bool); ok {
									defaultConfirm = dc
								}

								if question != "" {
									pendingQuestion = &tools.PendingUserQuestion{
										Question:       question,
										Summary:        summary,
										DefaultConfirm: defaultConfirm,
										AgentName:      currentAgent,
									}
								}

								// Clean up stored args
								delete(askUserQuestionArgs, toolKey)
							}

							if r.auditLogger != nil {
								_ = r.auditLogger.LogEventReceived("tui-ask-user-pending", currentAgent, map[string]interface{}{
									"tool_name":        toolName,
									"status":           status,
									"pending_question": pendingQuestion != nil,
								})
							}
						}
					}

					eventCh <- tui.ToolCompletedMsg{
						Agent:    currentAgent,
						ToolID:   toolKey,
						ToolName: toolName,
						Success:  success,
						Duration: duration,
						Summary:  summary,
					}

					// Log tool completion to audit log
					if r.auditLogger != nil {
						_ = r.auditLogger.LogToolComplete(currentAgent, toolName, success, duration, part.FunctionResponse.Response)
					}
				}
			}
		}

		// Check for text response
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" && !part.Thought {
					lastTextResponse = part.Text
					eventCh <- tui.AgentTextMsg{
						Agent:   currentAgent,
						Content: part.Text,
						IsFinal: false,
					}

					// Log agent text to audit log (non-final)
					if r.auditLogger != nil {
						_ = r.auditLogger.LogAgentText(currentAgent, part.Text, false)
					}
				}
			}
		}

		// Check for pending user question in state delta
		if event.Actions.StateDelta != nil {
			// Log state delta for debugging
			if r.auditLogger != nil {
				keys := make([]string, 0, len(event.Actions.StateDelta))
				for key := range event.Actions.StateDelta {
					keys = append(keys, key)
				}
				_ = r.auditLogger.LogEventReceived("tui-state-delta", currentAgent, map[string]interface{}{
					"keys":                 keys,
					"escalate":             event.Actions.Escalate,
					"skip_summarization":   event.Actions.SkipSummarization,
					"has_pending_question": event.Actions.StateDelta[incident.StateKeyPendingUserQuestion] != nil,
				})
			}

			if questionJSON, ok := event.Actions.StateDelta[incident.StateKeyPendingUserQuestion]; ok {
				if jsonStr, ok := questionJSON.(string); ok {
					var q tools.PendingUserQuestion
					if err := json.Unmarshal([]byte(jsonStr), &q); err == nil {
						pendingQuestion = &q
					}
				}
			}
		}

		// Also check if escalate is set (even without state delta)
		if event.Actions.Escalate && r.auditLogger != nil {
			_ = r.auditLogger.LogEventReceived("tui-escalate", currentAgent, map[string]interface{}{
				"escalate":           true,
				"has_state_delta":    event.Actions.StateDelta != nil,
				"skip_summarization": event.Actions.SkipSummarization,
			})
		}

		// Check if this is a final response
		if event.IsFinalResponse() {
			// Send AgentCompletedMsg to mark the agent as done (content was already sent)
			if lastTextResponse != "" {
				eventCh <- tui.AgentTextMsg{
					Agent:   currentAgent,
					Content: "", // Don't resend content, just mark as final
					IsFinal: true,
				}

				// Log final agent text to audit log
				if r.auditLogger != nil {
					_ = r.auditLogger.LogAgentText(currentAgent, lastTextResponse, true)
				}
			}

			// Check if we have a pending user question - if so, don't send CompletedMsg yet
			if pendingQuestion != nil {
				// Store on runner for the TUI wrapper to access when user responds
				r.tuiPendingQuestionMu.Lock()
				r.tuiPendingQuestion = pendingQuestion
				r.tuiPendingQuestionMu.Unlock()

				// Send the question to the TUI
				eventCh <- tui.UserQuestionMsg{
					Question:       pendingQuestion.Question,
					Summary:        pendingQuestion.Summary,
					DefaultConfirm: pendingQuestion.DefaultConfirm,
					AgentName:      pendingQuestion.AgentName,
				}
				// Don't send CompletedMsg - wait for user response
				// Clear pendingQuestion so we don't process it again after the loop
				pendingQuestion = nil
				completedSent = true // Mark as "completed" to prevent duplicate handling
				continue
			}

			eventCh <- tui.CompletedMsg{}
			completedSent = true

			// Log pipeline completion to audit log
			if r.auditLogger != nil {
				_ = r.auditLogger.LogPipelineComplete(time.Since(pipelineStart))
			}
		}
	}

	// Ensure we always send a completed message when the loop finishes
	if !completedSent {
		eventCh <- tui.CompletedMsg{}

		// Log pipeline completion even if no final response was received
		if r.auditLogger != nil {
			_ = r.auditLogger.LogPipelineComplete(time.Since(pipelineStart))
		}
	}

	return nil
}

// SessionID returns the current session ID.
func (r *Runner) SessionID() string {
	return r.sessionID
}

// ProcessMessageForTUI is a public method to process a message and send events to a channel.
// This is used by the TUI to trigger agent runs.
func (r *Runner) ProcessMessageForTUI(ctx context.Context, message string, eventCh chan<- interface{}) error {
	return r.processMessageWithTUI(ctx, message, eventCh)
}

// createMockLLM creates a mock LLM based on the model specification.
// Model spec format: "mock", "mock:scenario-name", "mock:interactive", or "mock:/path/to/scenario.yaml"
func (r *Runner) createMockLLM(modelSpec string, mockPort int) (adkmodel.LLM, error) {
	// Parse the model spec
	parts := strings.SplitN(modelSpec, ":", 2)

	if len(parts) == 1 {
		// Just "mock" - use default scenario
		return model.NewMockLLMFromName("ask_user")
	}

	scenario := parts[1]

	// Handle interactive mode
	if scenario == "interactive" {
		mockLLM, err := model.NewMockLLMInteractive(mockPort)
		if err != nil {
			return nil, err
		}
		r.mockInputServer = mockLLM.InputServer()

		// Start the input server
		go func() {
			if err := r.mockInputServer.Start(context.Background()); err != nil {
				// Log error but don't fail - the agent can still run
				fmt.Fprintf(os.Stderr, "Warning: mock input server failed to start: %v\n", err)
			}
		}()

		fmt.Fprintf(os.Stderr, "Mock LLM interactive mode: send input to port %d\n", r.mockInputServer.Port())
		fmt.Fprintf(os.Stderr, "Use: spectre mock --port %d --text \"your response\"\n", r.mockInputServer.Port())

		return mockLLM, nil
	}

	// Check if it's a file path
	if strings.HasSuffix(scenario, ".yaml") || strings.HasSuffix(scenario, ".yml") || strings.Contains(scenario, "/") {
		return model.NewMockLLM(scenario)
	}

	// Otherwise, treat as a scenario name to load from ~/.spectre/scenarios/
	return model.NewMockLLMFromName(scenario)
}

// MockInputServerPort returns the port of the mock input server (for interactive mode).
// Returns 0 if not in interactive mock mode.
func (r *Runner) MockInputServerPort() int {
	if r.mockInputServer != nil {
		return r.mockInputServer.Port()
	}
	return 0
}
