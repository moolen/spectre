//go:build disabled

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// Update handles all incoming messages and updates the model accordingly.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Filter out OSC escape sequences (terminal color responses like ]11;rgb:...)
		// These are not actual keyboard input and should be ignored
		// OSC sequences can appear as: "11;rgb:...", "]11;...", or just "11;rgb:..."
		keyStr := msg.String()
		if strings.Contains(keyStr, "rgb:") ||
			strings.HasPrefix(keyStr, "11;") ||
			strings.HasPrefix(keyStr, "]11;") ||
			(keyStr != "" && keyStr[0] == ']' && strings.Contains(keyStr, ";")) {
			// Ignore OSC color response sequences
			return m, nil
		}
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		// Set ready immediately on first WindowSizeMsg to avoid delay
		m.ready = true

		m.width = msg.Width
		m.height = msg.Height
		m.textArea.SetWidth(msg.Width - 4)
		m.questionSelector.SetWidth(msg.Width)

		// Update markdown renderer word wrap width only if dimensions changed or not initialized
		// Avoid recreating renderer unnecessarily as it may trigger terminal queries
		if m.mdRenderer == nil || m.width != msg.Width {
			m.mdRenderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width-8),
			)
		}

		// Calculate viewport height:
		// Total height - header(1) - separator(1) - separator(1) - input(2-10 lines) - help(1) - margins(2)
		// Use minimum input height of 2 for calculation
		inputHeight := 2
		viewportHeight := msg.Height - 7 - inputHeight
		if viewportHeight < 3 {
			viewportHeight = 3
		}
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = viewportHeight

		m.updateViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Tick all custom spinners
		m.spinnerMgr.TickAll()
		// Re-render viewport to update spinner animation
		if m.processing {
			m.updateViewport()
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case waitForEventMsg:
		return m.handleWaitForEventMsg(msg)

	case AgentActivatedMsg:
		return m.handleAgentActivated(msg)

	case AgentTextMsg:
		return m.handleAgentText(msg)

	case ToolStartedMsg:
		return m.handleToolStarted(msg)

	case ToolCompletedMsg:
		return m.handleToolCompleted(msg)

	case ContextUpdateMsg:
		m.contextUsed = msg.Used
		m.contextMax = msg.Max
		return m, nil

	case ErrorMsg:
		m.lastError = msg.Error
		return m, nil

	case CompletedMsg:
		// All events processed
		m.inputMode = true
		m.processing = false
		m.updateViewport()
		return m, nil

	case UserQuestionMsg:
		return m.handleUserQuestion(msg)

	case InitialPromptMsg:
		return m.handleInitialPrompt(msg)

	case CommandExecutedMsg:
		return m.handleCommandExecuted(msg)
	}

	// Update text area
	if m.inputMode {
		var cmd tea.Cmd
		m.textArea, cmd = m.textArea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKeyMsg handles keyboard input.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle Ctrl+C immediately
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle Esc - close dropdown first, then quit
	if msg.String() == "esc" {
		if m.cmdDropdown.IsVisible() {
			m.cmdDropdown.Hide()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	}

	// Handle question selector input when a question is pending
	if m.pendingQuestion != nil && m.inputMode {
		return m.handleQuestionSelectorInput(msg)
	}

	// Handle dropdown-specific keys when visible
	if m.cmdDropdown.IsVisible() {
		const (
			keyDown  = "down"
			keyEnter = "enter"
		)

		switch msg.String() {
		case "up":
			m.cmdDropdown.MoveUp()
			return m, nil
		case keyDown:
			m.cmdDropdown.MoveDown()
			return m, nil
		case keyEnter:
			// Select command and insert into textarea
			if cmd := m.cmdDropdown.SelectedCommand(); cmd != nil {
				m.textArea.SetValue("/" + cmd.Name + " ")
				m.textArea.CursorEnd()
			}
			m.cmdDropdown.Hide()
			return m, nil
		case "tab":
			// Tab also completes
			if cmd := m.cmdDropdown.SelectedCommand(); cmd != nil {
				m.textArea.SetValue("/" + cmd.Name + " ")
				m.textArea.CursorEnd()
			}
			m.cmdDropdown.Hide()
			return m, nil
		}
	}

	const keyEnter = "enter"

	switch msg.String() {
	case keyEnter:
		if m.inputMode {
			value := m.textArea.Value()

			// Check if the line ends with a backslash (line continuation)
			if strings.HasSuffix(value, "\\") {
				// Remove the backslash and insert a newline instead
				m.textArea.SetValue(strings.TrimSuffix(value, "\\") + "\n")
				// Move cursor to end
				m.textArea.CursorEnd()
				return m, nil
			}

			// Submit if there's content
			if value != "" {
				// Trim the input but preserve internal newlines
				input := strings.TrimSpace(value)
				m.textArea.Reset()
				m.inputMode = false

				// Check if this is a response to a pending question
				if m.pendingQuestion != nil {
					// This is a response to a user question, not a new message
					m.pendingQuestion = nil
					m.textArea.Placeholder = "Describe an incident to investigate..."
					// Don't reset pipeline, just continue processing
					m.processing = true
					m.updateViewport()

					// Return input submitted message AND resume event listening AND start spinner
					return m, tea.Batch(
						func() tea.Msg {
							return InputSubmittedMsg{Input: input}
						},
						m.waitForEvent(),
						m.spinner.Tick,
					)
				} else {
					// This is a new user message - add it to the viewport
					m.addUserMessage(input)
					m.resetPipeline()
				}

				m.updateViewport()

				// Return input submitted message AND start listening for events AND start spinner
				return m, tea.Batch(
					func() tea.Msg {
						return InputSubmittedMsg{Input: input}
					},
					m.waitForEvent(),
					m.spinner.Tick,
				)
			}
		}

	case "pgup":
		// Always allow page up/down for scrolling, even in input mode
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case "pgdown":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case "ctrl+up":
		// Scroll up with ctrl+up even in input mode
		m.viewport.LineUp(3)
		return m, nil

	case "ctrl+down":
		// Scroll down with ctrl+down even in input mode
		m.viewport.LineDown(3)
		return m, nil

	case "up", "k":
		// Scroll up in viewport when not in input mode
		if !m.inputMode {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case "down", "j":
		// Scroll down in viewport when not in input mode
		if !m.inputMode {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	// Pass through to text area
	if m.inputMode {
		var cmd tea.Cmd
		m.textArea, cmd = m.textArea.Update(msg)

		// Update dropdown state based on input
		m.updateDropdownState()

		return m, cmd
	}

	return m, nil
}

// handleQuestionSelectorInput handles keyboard input for the question selector.
func (m *Model) handleQuestionSelectorInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const (
		keyDown  = "down"
		keyEnter = "enter"
	)

	switch msg.String() {
	case "up":
		m.questionSelector.MoveUp()
		return m, nil

	case keyDown:
		m.questionSelector.MoveDown()
		return m, nil

	case "tab":
		// Tab toggles between options and input
		if m.questionSelector.IsInputFocused() {
			m.questionSelector.inputFocused = false
			m.questionSelector.textInput.Blur()
		} else {
			m.questionSelector.FocusInput()
		}
		return m, nil

	case keyEnter:
		// If in free-form input with content, check for line continuation
		if m.questionSelector.IsInputFocused() {
			value := m.questionSelector.textInput.Value()
			if strings.HasSuffix(value, "\\") {
				// Line continuation
				m.questionSelector.textInput.SetValue(strings.TrimSuffix(value, "\\") + "\n")
				return m, nil
			}
		}

		// Submit the selected value
		input := m.questionSelector.GetSelectedValue()
		if input != "" {
			m.inputMode = false
			m.pendingQuestion = nil
			m.processing = true
			m.updateViewport()

			return m, tea.Batch(
				func() tea.Msg {
					return InputSubmittedMsg{Input: input}
				},
				m.waitForEvent(),
			)
		}
		return m, nil

	case "pgup":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case "pgdown":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case "ctrl+up":
		m.viewport.LineUp(3)
		return m, nil

	case "ctrl+down":
		m.viewport.LineDown(3)
		return m, nil
	}

	// If input is focused, pass keystrokes to textarea
	if m.questionSelector.IsInputFocused() {
		m.questionSelector.UpdateTextInput(msg)
	}

	return m, nil
}

// updateDropdownState manages dropdown visibility based on current input.
func (m *Model) updateDropdownState() {
	value := m.textArea.Value()

	// Check if input starts with "/" and has no space yet
	if strings.HasPrefix(value, "/") {
		query := strings.TrimPrefix(value, "/")
		// Don't show dropdown if there's a space (command already complete)
		if !strings.Contains(query, " ") {
			if !m.cmdDropdown.IsVisible() {
				m.cmdDropdown.Show()
			}
			m.cmdDropdown.SetQuery(query)
		} else {
			m.cmdDropdown.Hide()
		}
	} else {
		m.cmdDropdown.Hide()
	}
}

// handleWaitForEventMsg handles events received from the event channel.
func (m *Model) handleWaitForEventMsg(msg waitForEventMsg) (*Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Process the wrapped event
	switch event := msg.event.(type) {
	case AgentActivatedMsg:
		m, _ = m.handleAgentActivated(event)
	case AgentTextMsg:
		m, _ = m.handleAgentText(event)
	case ToolStartedMsg:
		m, _ = m.handleToolStarted(event)
	case ToolCompletedMsg:
		m, _ = m.handleToolCompleted(event)
	case ContextUpdateMsg:
		m.contextUsed = event.Used
		m.contextMax = event.Max
	case ErrorMsg:
		m.lastError = event.Error
		m.updateViewport()
	case UserQuestionMsg:
		// Handle user question - don't wait for more events until user responds
		m, _ = m.handleUserQuestion(event)
		return m, nil
	case CompletedMsg:
		m.inputMode = true
		m.processing = false
		m.updateViewport()
		// Don't wait for more events - we're done
		return m, nil
	}

	// Continue waiting for more events
	cmds = append(cmds, m.waitForEvent())

	return m, tea.Batch(cmds...)
}

// handleAgentActivated handles when a new agent becomes active.
func (m *Model) handleAgentActivated(msg AgentActivatedMsg) (*Model, tea.Cmd) {
	// Complete previous agent if any
	if m.activeAgent != "" && m.activeAgent != msg.Name {
		m.completeAgent(m.activeAgent)
	}

	m.activeAgent = msg.Name
	m.findOrCreateAgentBlock(msg.Name)
	m.updateViewport()

	return m, m.spinner.Tick
}

// handleAgentText handles text output from an agent.
//nolint:unparam // Matches Bubble Tea interface pattern
func (m *Model) handleAgentText(msg AgentTextMsg) (*Model, tea.Cmd) {
	// Only add content if it's not empty (final messages may have empty content)
	if msg.Content != "" {
		m.updateAgentContent(msg.Agent, msg.Content)
	}

	if msg.IsFinal {
		m.completeAgent(msg.Agent)
	}

	m.updateViewport()
	return m, nil
}

// handleToolStarted handles when a tool call begins.
func (m *Model) handleToolStarted(msg ToolStartedMsg) (*Model, tea.Cmd) {
	m.addToolCall(msg.Agent, msg.ToolID, msg.ToolName)
	m.updateViewport()
	return m, m.spinner.Tick
}

// handleToolCompleted handles when a tool call completes.
//
//nolint:unparam // Matches Bubble Tea interface pattern
func (m *Model) handleToolCompleted(msg ToolCompletedMsg) (*Model, tea.Cmd) {
	m.updateToolCall(msg.Agent, msg.ToolID, msg.Success, msg.Duration, msg.Summary)
	m.updateViewport()
	return m, nil
}

// handleUserQuestion handles when an agent asks a question via ask_user_question tool.
//
//nolint:unparam // Matches Bubble Tea interface pattern
func (m *Model) handleUserQuestion(msg UserQuestionMsg) (*Model, tea.Cmd) {
	// Store the pending question
	m.pendingQuestion = &msg

	// Add the question to the viewport content
	m.addQuestionToContent(msg)

	// Configure the question selector
	m.questionSelector.SetQuestion(msg.Question, msg.Summary, msg.DefaultConfirm)
	m.questionSelector.SetWidth(m.width)

	// Enable input mode so user can respond
	m.inputMode = true
	m.processing = false

	m.updateViewport()
	return m, nil
}

// addQuestionToContent adds the user question to the viewport.
func (m *Model) addQuestionToContent(msg UserQuestionMsg) {
	// Create a question block in the agent blocks
	agentName := msg.AgentName
	if agentName == "" {
		agentName = "system"
	}

	// Build the question content
	var content strings.Builder
	if msg.Summary != "" {
		content.WriteString(msg.Summary)
		content.WriteString("\n\n")
	}
	content.WriteString("Question: ")
	content.WriteString(msg.Question)
	if msg.DefaultConfirm {
		content.WriteString(" [Y/n]")
	} else {
		content.WriteString(" [y/N]")
	}

	// Update the agent's content with the question
	idx := m.findOrCreateAgentBlock(agentName)
	m.agentBlocks[idx].Messages = []AgentMessage{{
		Content:   content.String(),
		Timestamp: time.Now(),
	}}
}

// handleInitialPrompt handles the initial prompt when TUI starts with a pre-set message.
func (m *Model) handleInitialPrompt(msg InitialPromptMsg) (*Model, tea.Cmd) {
	// Add the initial prompt as a user message so it's visible in the content view
	m.addUserMessage(msg.Prompt)

	// Reset state for processing
	m.inputMode = false
	m.processing = true

	m.updateViewport()

	// Return InputSubmittedMsg to trigger processing AND start listening for events AND start spinner
	return m, tea.Batch(
		func() tea.Msg {
			return InputSubmittedMsg{Input: msg.Prompt}
		},
		m.waitForEvent(),
		m.spinner.Tick,
	)
}

// handleCommandExecuted handles the result of a command execution.
//
//nolint:unparam // Matches Bubble Tea interface pattern
func (m *Model) handleCommandExecuted(msg CommandExecutedMsg) (*Model, tea.Cmd) {
	// If it's an info-only message (like /help or /stats), just display it
	if msg.IsInfo {
		// Create a pseudo-agent block for the command result
		idx := m.findOrCreateAgentBlock("system")
		m.agentBlocks[idx].Messages = append(m.agentBlocks[idx].Messages, AgentMessage{
			Content:   msg.Message,
			Timestamp: time.Now(),
		})
		m.agentBlocks[idx].Status = StatusCompleted
	} else if !msg.Success {
		// Error message
		m.lastError = fmt.Errorf("%s", msg.Message)
	}

	// Enable input mode for next command
	m.inputMode = true

	m.updateViewport()

	return m, nil
}
