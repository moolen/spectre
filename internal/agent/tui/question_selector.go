package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

// QuestionSelectorOption represents a selectable option.
type QuestionSelectorOption struct {
	Label string
	Value string
}

// QuestionSelector is a component for answering agent questions with
// predefined options (Yes/No) and a free-form input field.
type QuestionSelector struct {
	// Question details
	question       string
	summary        string
	defaultConfirm bool

	// Options
	options       []QuestionSelectorOption
	selectedIndex int

	// Free-form input
	textInput    textarea.Model
	inputFocused bool // true when free-form input is focused

	// Dimensions
	width int
}

// NewQuestionSelector creates a new question selector.
func NewQuestionSelector() *QuestionSelector {
	// Create textarea for free-form input
	ta := textarea.New()
	ta.Placeholder = "Type a custom response..."
	ta.CharLimit = 1000
	ta.SetWidth(60)
	ta.SetHeight(2)
	ta.MaxHeight = 4
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return "> "
		}
		return "  "
	})
	ta.FocusedStyle.Prompt = inputPromptStyle
	ta.BlurredStyle.Prompt = inputPromptStyle.Foreground(colorMuted)
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")

	return &QuestionSelector{
		options: []QuestionSelectorOption{
			{Label: "Yes", Value: "yes"},
			{Label: "No", Value: "no"},
		},
		selectedIndex: 0,
		textInput:     ta,
		inputFocused:  false,
	}
}

// SetQuestion configures the selector with a question.
func (q *QuestionSelector) SetQuestion(question, summary string, defaultConfirm bool) {
	q.question = question
	q.summary = summary
	q.defaultConfirm = defaultConfirm

	// Set default selection based on defaultConfirm
	if defaultConfirm {
		q.selectedIndex = 0 // "Yes" is default
	} else {
		q.selectedIndex = 1 // "No" is default
	}

	// Clear any previous input
	q.textInput.Reset()
	q.inputFocused = false
}

// SetWidth sets the width of the selector.
func (q *QuestionSelector) SetWidth(width int) {
	q.width = width
	q.textInput.SetWidth(width - 8)
}

// MoveUp moves selection up.
func (q *QuestionSelector) MoveUp() {
	if q.inputFocused {
		// Moving up from input focuses the last option
		q.inputFocused = false
		q.textInput.Blur()
		q.selectedIndex = len(q.options) - 1
	} else if q.selectedIndex > 0 {
		q.selectedIndex--
	}
}

// MoveDown moves selection down.
func (q *QuestionSelector) MoveDown() {
	if !q.inputFocused {
		if q.selectedIndex < len(q.options)-1 {
			q.selectedIndex++
		} else {
			// Moving down from last option focuses the input
			q.inputFocused = true
			q.textInput.Focus()
		}
	}
}

// FocusInput focuses the free-form input field.
func (q *QuestionSelector) FocusInput() {
	q.inputFocused = true
	q.textInput.Focus()
}

// IsInputFocused returns true if the free-form input is focused.
func (q *QuestionSelector) IsInputFocused() bool {
	return q.inputFocused
}

// GetSelectedValue returns the selected value.
// If input is focused and has content, returns the input text.
// Otherwise returns the selected option value.
func (q *QuestionSelector) GetSelectedValue() string {
	if q.inputFocused {
		value := strings.TrimSpace(q.textInput.Value())
		if value != "" {
			return value
		}
	}
	if q.selectedIndex >= 0 && q.selectedIndex < len(q.options) {
		return q.options[q.selectedIndex].Value
	}
	return ""
}

// UpdateTextInput updates the textarea with a message.
func (q *QuestionSelector) UpdateTextInput(msg interface{}) {
	q.textInput, _ = q.textInput.Update(msg)
}

// View renders the question selector.
func (q *QuestionSelector) View() string {
	var b strings.Builder

	// Render options
	for i, opt := range q.options {
		var prefix string
		var style lipgloss.Style

		if !q.inputFocused && i == q.selectedIndex {
			prefix = questionSelectorCursorStyle.Render("▸ ")
			style = questionOptionSelectedStyle
		} else {
			prefix = "  "
			style = questionOptionStyle
		}

		b.WriteString(prefix)
		b.WriteString(style.Render(opt.Label))
		b.WriteString("\n")
	}

	// Separator
	b.WriteString("\n")

	// Free-form input label
	var inputLabel string
	if q.inputFocused {
		inputLabel = questionInputLabelSelectedStyle.Render("▸ Or type a response:")
	} else {
		inputLabel = questionInputLabelStyle.Render("  Or type a response:")
	}
	b.WriteString(inputLabel)
	b.WriteString("\n")

	// Input field with indentation
	b.WriteString("  ")
	b.WriteString(q.textInput.View())

	return questionSelectorBoxStyle.Width(q.width - 4).Render(b.String())
}

// Question selector styles
var (
	questionSelectorBoxStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(colorPrimary).
					Padding(1, 2)

	questionSelectorCursorStyle = lipgloss.NewStyle().
					Foreground(colorPrimary).
					Bold(true)

	questionOptionStyle = lipgloss.NewStyle().
				Foreground(colorText)

	questionOptionSelectedStyle = lipgloss.NewStyle().
					Foreground(colorPrimary).
					Bold(true)

	questionInputLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	questionInputLabelSelectedStyle = lipgloss.NewStyle().
					Foreground(colorPrimary).
					Bold(true)
)
