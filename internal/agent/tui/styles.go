package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	colorPrimary   = lipgloss.Color("#00D4FF") // Cyan
	colorSuccess   = lipgloss.Color("#10B981") // Green
	colorWarning   = lipgloss.Color("#F59E0B") // Yellow/Orange
	colorError     = lipgloss.Color("#EF4444") // Red
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorText      = lipgloss.Color("#E5E7EB") // Light gray
	colorDim       = lipgloss.Color("#4B5563") // Darker gray
)

// Header styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	contextBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	contextBarFilledStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	contextBarWarningStyle = lipgloss.NewStyle().
				Foreground(colorWarning)

	contextBarDangerStyle = lipgloss.NewStyle().
				Foreground(colorError)
)


// Input styles
var (
	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)
)

// User message styles
var (
	userMessageStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E3A5F")). // Dark blue background
				Foreground(colorText).
				Padding(0, 1).
				MarginBottom(1)

	userMessageLabelStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)
)

// Separator style
var (
	separatorStyle = lipgloss.NewStyle().
		Foreground(colorDim)
)

// Help bar style
var (
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorPrimary)
)

// Command dropdown styles
var (
	dropdownStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	dropdownItemStyle = lipgloss.NewStyle().
				Foreground(colorText).
				PaddingLeft(1)

	dropdownSelectedStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Background(lipgloss.Color("#1E3A5F")).
				Bold(true).
				PaddingLeft(1)

	dropdownCmdStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	dropdownDescStyle = lipgloss.NewStyle().
				Foreground(colorMuted)
)

// Tool spinner style
var (
	toolRunningStyle = lipgloss.NewStyle().
				Foreground(colorPrimary)
)
