package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/moolen/spectre/internal/agent/commands"
)

const (
	iconSuccess = "✓"
	iconError   = "✗"
)

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID         string // Unique ID for this tool call (for matching start/complete)
	Name       string
	Status     Status
	Duration   time.Duration
	Summary    string
	StartTime  time.Time
	SpinnerKey string // Unique key for this tool's spinner
}

// AgentMessage represents a single message from an agent.
type AgentMessage struct {
	Content   string
	Timestamp time.Time
}

// AgentBlock represents an agent's activity block.
type AgentBlock struct {
	Name           string
	Status         Status
	Messages       []AgentMessage // All messages from this agent
	ToolCalls      []ToolCall
	StartTime      time.Time
	EndTime        time.Time
	ContentSpinKey string // Unique key for content spinner
}

// UserMessage represents a message submitted by the user.
type UserMessage struct {
	Content   string
	Timestamp time.Time
}

// InputHandler is called when the user submits input.
type InputHandler func(input string)

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	// Dimensions
	width  int
	height int

	// Agent blocks (current session)
	agentBlocks []AgentBlock
	activeAgent string

	// User messages (current session)
	userMessages []UserMessage

	// History of all previous sessions' output (for scrolling)
	history *strings.Builder

	// Context usage
	contextUsed int
	contextMax  int

	// UI Components
	textArea   textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model         // Legacy spinner for fallback
	spinnerMgr *SpinnerManager       // Manager for random animated spinners
	mdRenderer *glamour.TermRenderer // Markdown renderer

	// Event channel from runner
	eventCh <-chan interface{}

	// Input handler callback
	onInput InputHandler

	// State
	ready      bool
	quitting   bool
	inputMode  bool
	processing bool // True when agent is processing

	// User question state
	pendingQuestion  *UserQuestionMsg  // Non-nil when waiting for user to answer a question
	questionSelector *QuestionSelector // Selector UI for answering questions

	// Session info
	sessionID string
	apiURL    string
	modelName string

	// Error state
	lastError error

	// Command dropdown
	cmdDropdown *CommandDropdown
	cmdRegistry *commands.Registry
}

// NewModel creates a new TUI model.
func NewModel(eventCh <-chan interface{}, sessionID, apiURL, modelName string) Model {
	// Text area for multiline input
	ta := textarea.New()
	ta.Placeholder = "Describe an incident to investigate..."
	ta.Focus()
	ta.CharLimit = 4000
	ta.SetWidth(80)
	ta.SetHeight(2)   // Minimum 2 lines
	ta.MaxHeight = 10 // Maximum 10 lines before scrolling within textarea
	ta.ShowLineNumbers = false
	// Use SetPromptFunc to show prompt only on first line
	ta.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return "> "
		}
		return "  " // Same width as "> " for alignment
	})
	ta.FocusedStyle.Prompt = inputPromptStyle
	ta.BlurredStyle.Prompt = inputPromptStyle
	// Allow shift+enter for actual newlines (enter submits)
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")

	// Spinner for tools (legacy fallback)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = toolRunningStyle

	// Spinner manager for random animated spinners
	spinMgr := NewSpinnerManager()

	// Viewport for scrolling with mouse support
	vp := viewport.New(80, 20)
	vp.SetContent("")
	vp.MouseWheelEnabled = true

	// Create markdown renderer with dark style
	mdRenderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(76),
	)

	// Initialize command dropdown using the default registry
	cmdDropdown := NewCommandDropdown(commands.DefaultRegistry)

	// Initialize question selector
	questionSelector := NewQuestionSelector()

	return Model{
		textArea:         ta,
		viewport:         vp,
		spinner:          s,
		spinnerMgr:       spinMgr,
		mdRenderer:       mdRenderer,
		eventCh:          eventCh,
		sessionID:        sessionID,
		apiURL:           apiURL,
		modelName:        modelName,
		inputMode:        true,
		contextMax:       200000, // Default Claude context window
		history:          &strings.Builder{},
		cmdRegistry:      commands.DefaultRegistry,
		cmdDropdown:      cmdDropdown,
		questionSelector: questionSelector,
	}
}

// SetInputHandler sets the callback for handling user input.
func (m *Model) SetInputHandler(handler InputHandler) {
	m.onInput = handler
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	// Request window size immediately to avoid delay
	return tea.WindowSize()
}

// waitForEvent returns a command that waits for an event from the channel.
func (m *Model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		if m.eventCh == nil {
			return nil
		}
		event, ok := <-m.eventCh
		if !ok {
			return CompletedMsg{}
		}
		return waitForEventMsg{event: event}
	}
}

// findOrCreateAgentBlock finds an existing agent block or creates a new one.
func (m *Model) findOrCreateAgentBlock(agentName string) int {
	for i, block := range m.agentBlocks {
		if block.Name == agentName {
			return i
		}
	}
	// Create new block with unique spinner key for content
	contentSpinKey := fmt.Sprintf("content-%s-%d", agentName, time.Now().UnixNano())
	m.agentBlocks = append(m.agentBlocks, AgentBlock{
		Name:           agentName,
		Status:         StatusActive,
		StartTime:      time.Now(),
		ContentSpinKey: contentSpinKey,
	})
	return len(m.agentBlocks) - 1
}

// addToolCall adds a tool call to an agent block.
func (m *Model) addToolCall(agentName, toolID, toolName string) {
	idx := m.findOrCreateAgentBlock(agentName)
	// Generate unique spinner key for this tool
	spinnerKey := fmt.Sprintf("tool-%s-%s-%d", agentName, toolID, time.Now().UnixNano())
	m.agentBlocks[idx].ToolCalls = append(m.agentBlocks[idx].ToolCalls, ToolCall{
		ID:         toolID,
		Name:       toolName,
		Status:     StatusActive,
		StartTime:  time.Now(),
		SpinnerKey: spinnerKey,
	})
}

// updateToolCall updates a tool call status by matching on tool ID.
func (m *Model) updateToolCall(agentName, toolID string, success bool, duration time.Duration, summary string) {
	idx := m.findOrCreateAgentBlock(agentName)
	for i := range m.agentBlocks[idx].ToolCalls {
		if m.agentBlocks[idx].ToolCalls[i].ID != toolID {
			continue
		}
		if success {
			m.agentBlocks[idx].ToolCalls[i].Status = StatusCompleted
		} else {
			m.agentBlocks[idx].ToolCalls[i].Status = StatusError
		}
		m.agentBlocks[idx].ToolCalls[i].Duration = duration
		m.agentBlocks[idx].ToolCalls[i].Summary = summary
		// Remove spinner for completed tool
		m.spinnerMgr.Remove(m.agentBlocks[idx].ToolCalls[i].SpinnerKey)
		break
	}
}

// updateAgentContent adds a new message to an agent block.
func (m *Model) updateAgentContent(agentName, content string) {
	idx := m.findOrCreateAgentBlock(agentName)
	// Append new message instead of replacing
	m.agentBlocks[idx].Messages = append(m.agentBlocks[idx].Messages, AgentMessage{
		Content:   content,
		Timestamp: time.Now(),
	})
}

// completeAgent marks an agent as completed.
func (m *Model) completeAgent(agentName string) {
	for i := range m.agentBlocks {
		if m.agentBlocks[i].Name == agentName {
			m.agentBlocks[i].Status = StatusCompleted
			m.agentBlocks[i].EndTime = time.Now()
			// Remove content spinner for completed agent
			m.spinnerMgr.Remove(m.agentBlocks[i].ContentSpinKey)
			break
		}
	}
}

// addUserMessage adds a user message to the current session.
func (m *Model) addUserMessage(content string) {
	m.userMessages = append(m.userMessages, UserMessage{
		Content:   content,
		Timestamp: time.Now(),
	})
}

// saveToHistory saves the current agent blocks to history and clears them.
func (m *Model) saveToHistory() {
	if len(m.agentBlocks) == 0 && len(m.userMessages) == 0 {
		return
	}

	// Add a separator if there's existing history
	if m.history.Len() > 0 {
		m.history.WriteString("\n")
		m.history.WriteString(strings.Repeat("═", 80))
		m.history.WriteString("\n\n")
	}

	// Render user messages first, then agent blocks
	for _, msg := range m.userMessages {
		m.history.WriteString("You: ")
		m.history.WriteString(msg.Content)
		m.history.WriteString("\n\n")
	}

	// Render current blocks to history
	for _, block := range m.agentBlocks {
		m.history.WriteString("[")
		m.history.WriteString(formatAgentName(block.Name))
		m.history.WriteString("]")
		m.history.WriteString("\n")

		for _, tc := range block.ToolCalls {
			icon := iconSuccess
			if tc.Status == StatusError {
				icon = iconError
			}
			m.history.WriteString("  ")
			m.history.WriteString(icon)
			m.history.WriteString(" ")
			m.history.WriteString(tc.Name)
			m.history.WriteString(" (")
			m.history.WriteString(tc.Duration.String())
			m.history.WriteString(")")
			if tc.Summary != "" {
				m.history.WriteString(" — ")
				m.history.WriteString(tc.Summary)
			}
			m.history.WriteString("\n")
		}

		// Render all messages
		if len(block.Messages) > 0 {
			m.history.WriteString("\n")
			for _, msg := range block.Messages {
				m.history.WriteString(m.renderMarkdown(msg.Content))
				m.history.WriteString("\n")
			}
		}
		m.history.WriteString("\n")
	}
}

// resetPipeline resets the state for a new investigation.
func (m *Model) resetPipeline() {
	// Save current output to history first
	m.saveToHistory()

	m.agentBlocks = nil
	m.userMessages = nil
	m.activeAgent = ""
	m.lastError = nil
	m.processing = true
	// Clear all spinners for fresh start
	m.spinnerMgr.Clear()
}

// updateViewport updates the viewport content with history and current blocks.
func (m *Model) updateViewport() {
	var content strings.Builder

	// Add history
	if m.history.Len() > 0 {
		content.WriteString(m.history.String())
	}

	// Add current user messages
	for _, msg := range m.userMessages {
		content.WriteString(m.renderUserMessagePlain(msg))
	}

	// Add current agent blocks
	for _, block := range m.agentBlocks {
		content.WriteString(m.renderAgentBlockPlain(block))
	}

	m.viewport.SetContent(content.String())
	// Scroll to bottom when new content is added
	m.viewport.GotoBottom()
}

// renderUserMessagePlain renders a user message for the viewport.
func (m *Model) renderUserMessagePlain(msg UserMessage) string {
	var b strings.Builder

	// Label
	b.WriteString(userMessageLabelStyle.Render("You: "))

	// Content with background - wrap long lines
	maxWidth := m.width - 10
	if maxWidth < 40 {
		maxWidth = 40
	}
	if maxWidth > 100 {
		maxWidth = 100
	}

	lines := wrapText(msg.Content, maxWidth)
	content := strings.Join(lines, "\n     ") // Indent continuation lines
	b.WriteString(userMessageStyle.Render(content))
	b.WriteString("\n\n")

	return b.String()
}

// wrapText wraps text to fit within maxWidth characters.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= maxWidth {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)

	return lines
}

// renderAgentBlockPlain renders an agent block as plain text for the viewport.
func (m *Model) renderAgentBlockPlain(block AgentBlock) string {
	var b strings.Builder

	statusIcon := "●"
	if block.Status == StatusCompleted {
		statusIcon = "✓"
	} else if block.Status == StatusError {
		statusIcon = "✗"
	}

	b.WriteString(statusIcon)
	b.WriteString(" [")
	b.WriteString(formatAgentName(block.Name))
	b.WriteString("]")
	b.WriteString("\n")

	// Render tool calls first (they come before the final text response)
	for _, tc := range block.ToolCalls {
		var icon string
		if tc.Status == StatusCompleted {
			icon = "✓"
		} else if tc.Status == StatusError {
			icon = iconError
		} else {
			// Use unique spinner for each tool
			icon = m.spinnerMgr.Get(tc.SpinnerKey).View()
		}
		b.WriteString("  ")
		b.WriteString(icon)
		b.WriteString(" ")
		b.WriteString(tc.Name)
		if tc.Status != StatusActive {
			b.WriteString(" (")
			b.WriteString(tc.Duration.String())
			b.WriteString(")")
		}
		if tc.Summary != "" {
			b.WriteString(" — ")
			b.WriteString(tc.Summary)
		}
		b.WriteString("\n")
	}

	// Render all messages (agent's text responses) after tool calls
	if len(block.Messages) > 0 {
		// Show loading indicator on the last message if agent is still active
		for i, msg := range block.Messages {
			isLastMessage := i == len(block.Messages)-1
			// Render markdown content
			renderedContent := m.renderMarkdown(msg.Content)
			if isLastMessage && block.Status == StatusActive {
				// Put spinner inline with the content (trim leading newlines from markdown)
				b.WriteString("  ")
				b.WriteString(m.spinnerMgr.Get(block.ContentSpinKey).View())
				b.WriteString(" ")
				b.WriteString(strings.TrimLeft(renderedContent, "\n"))
			} else {
				b.WriteString(renderedContent)
			}
			if !isLastMessage {
				b.WriteString("\n")
			}
		}
	} else if block.Status == StatusActive && len(block.ToolCalls) == 0 {
		// Show loading indicator when agent is active but no content yet
		b.WriteString("  ")
		b.WriteString(m.spinnerMgr.Get(block.ContentSpinKey).View())
		b.WriteString(" Thinking...\n")
	}

	b.WriteString("\n")

	return b.String()
}

// renderMarkdown renders markdown content with styling.
func (m *Model) renderMarkdown(content string) string {
	if m.mdRenderer == nil {
		return content
	}

	rendered, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing whitespace but preserve structure
	return strings.TrimRight(rendered, "\n") + "\n"
}

// HandleInput is called by the runner to submit input to the TUI
func (m *Model) HandleInput(input string) {
	if m.onInput != nil {
		m.onInput(input)
	}
}
