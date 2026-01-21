//go:build disabled

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI.
func (m *Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing...\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Separator
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")

	// Scrollable content area (viewport)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Error message if any
	if m.lastError != nil {
		b.WriteString(m.renderError())
		b.WriteString("\n")
	}

	// Separator before input
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")

	// Command dropdown (above input when visible)
	if m.cmdDropdown.IsVisible() {
		m.cmdDropdown.SetWidth(m.width)
		b.WriteString(m.cmdDropdown.View())
		b.WriteString("\n")
	}

	// Input
	b.WriteString(m.renderInput())

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderHeader renders the title and context usage bar.
func (m *Model) renderHeader() string {
	// Title
	title := titleStyle.Render("SPECTRE")

	// Context bar
	contextBar := m.renderContextBar()

	// Session info
	sessionInfo := lipgloss.NewStyle().
		Foreground(colorMuted).
		Render(fmt.Sprintf("Session: %s", truncateString(m.sessionID, 8)))

	// Calculate spacing
	titleWidth := lipgloss.Width(title)
	barWidth := lipgloss.Width(contextBar)
	sessionWidth := lipgloss.Width(sessionInfo)
	spacing := m.width - titleWidth - barWidth - sessionWidth - 4

	if spacing < 0 {
		spacing = 1
	}

	return fmt.Sprintf("%s%s%s%s%s",
		title,
		strings.Repeat(" ", spacing/2),
		sessionInfo,
		strings.Repeat(" ", spacing-spacing/2),
		contextBar,
	)
}

// renderContextBar renders the context usage progress bar.
func (m *Model) renderContextBar() string {
	if m.contextMax == 0 {
		return ""
	}

	percentage := float64(m.contextUsed) / float64(m.contextMax) * 100
	barWidth := 12
	filledWidth := int(float64(barWidth) * percentage / 100)

	if filledWidth > barWidth {
		filledWidth = barWidth
	}

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", barWidth-filledWidth)

	// Color based on usage
	var barStyle lipgloss.Style
	switch {
	case percentage >= 90:
		barStyle = contextBarDangerStyle
	case percentage >= 70:
		barStyle = contextBarWarningStyle
	default:
		barStyle = contextBarFilledStyle
	}

	return fmt.Sprintf("[%s%s] %.0f%% ctx",
		barStyle.Render(filled),
		contextBarStyle.Render(empty),
		percentage,
	)
}
// renderSeparator renders a horizontal separator line.
func (m *Model) renderSeparator() string {
	return separatorStyle.Render(strings.Repeat("─", m.width-2))
}

// renderInput renders the input area.
func (m *Model) renderInput() string {
	if m.inputMode {
		// Show question selector when there's a pending question
		if m.pendingQuestion != nil {
			return m.questionSelector.View()
		}
		return m.textArea.View()
	}
	return lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true).
		Render("Processing... (press Ctrl+C to cancel)")
}

// renderError renders an error message.
func (m *Model) renderError() string {
	return lipgloss.NewStyle().
		Foreground(colorError).
		Bold(true).
		Render(fmt.Sprintf("Error: %v", m.lastError))
}

// renderHelp renders the help bar at the bottom.
func (m *Model) renderHelp() string {
	var keys []struct {
		key  string
		desc string
	}

	// Show different help keys when question selector is active
	if m.pendingQuestion != nil {
		keys = []struct {
			key  string
			desc string
		}{
			{"up/down", "select"},
			{"enter", "confirm"},
			{"pgup/pgdn", "scroll"},
			{"ctrl+c", "quit"},
		}
	} else {
		keys = []struct {
			key  string
			desc string
		}{
			{"enter", "submit"},
			{"shift+enter", "newline"},
			{"pgup/pgdn", "scroll"},
			{"ctrl+c", "quit"},
		}
	}

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		part := fmt.Sprintf("%s %s",
			helpKeyStyle.Render(k.key),
			k.desc,
		)
		parts = append(parts, part)
	}

	return helpStyle.Render(strings.Join(parts, " • "))
}

// Helper functions

// formatAgentName converts agent names to display format.
// e.g., "incident_intake_agent" -> "incident_intake"
func formatAgentName(name string) string {
	// Remove "_agent" suffix if present
	name = strings.TrimSuffix(name, "_agent")
	return name
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
