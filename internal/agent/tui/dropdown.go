//go:build disabled

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/moolen/spectre/internal/agent/commands"
)

const (
	maxDropdownItems = 8
)

// CommandDropdown manages the command dropdown state.
type CommandDropdown struct {
	visible       bool
	selectedIndex int
	query         string
	filtered      []commands.Entry
	registry      *commands.Registry
	width         int
}

// NewCommandDropdown creates a new dropdown.
func NewCommandDropdown(registry *commands.Registry) *CommandDropdown {
	return &CommandDropdown{
		registry: registry,
		filtered: registry.AllEntries(),
		width:    60,
	}
}

// Show makes the dropdown visible and resets selection.
func (d *CommandDropdown) Show() {
	d.visible = true
	d.selectedIndex = 0
}

// Hide hides the dropdown.
func (d *CommandDropdown) Hide() {
	d.visible = false
	d.query = ""
	d.selectedIndex = 0
	d.filtered = d.registry.AllEntries()
}

// IsVisible returns whether the dropdown is currently shown.
func (d *CommandDropdown) IsVisible() bool {
	return d.visible
}

// SetQuery updates the filter query and refreshes the filtered list.
func (d *CommandDropdown) SetQuery(query string) {
	d.query = query
	d.filtered = d.registry.FuzzyMatch(query)
	// Reset selection if it's out of bounds
	if d.selectedIndex >= len(d.filtered) {
		d.selectedIndex = 0
	}
}

// MoveUp moves selection up (wraps around).
func (d *CommandDropdown) MoveUp() {
	if len(d.filtered) == 0 {
		return
	}
	d.selectedIndex--
	if d.selectedIndex < 0 {
		d.selectedIndex = len(d.filtered) - 1
		// Cap at max visible items
		if d.selectedIndex >= maxDropdownItems {
			d.selectedIndex = maxDropdownItems - 1
		}
	}
}

// MoveDown moves selection down (wraps around).
func (d *CommandDropdown) MoveDown() {
	if len(d.filtered) == 0 {
		return
	}
	d.selectedIndex++
	maxIndex := len(d.filtered) - 1
	if maxIndex >= maxDropdownItems {
		maxIndex = maxDropdownItems - 1
	}
	if d.selectedIndex > maxIndex {
		d.selectedIndex = 0
	}
}

// SelectedCommand returns the currently selected command.
func (d *CommandDropdown) SelectedCommand() *commands.Entry {
	if len(d.filtered) == 0 || d.selectedIndex >= len(d.filtered) {
		return nil
	}
	return &d.filtered[d.selectedIndex]
}

// SetWidth sets the rendering width.
func (d *CommandDropdown) SetWidth(width int) {
	d.width = width
}

// View renders the dropdown using lipgloss.
func (d *CommandDropdown) View() string {
	if !d.visible || len(d.filtered) == 0 {
		return ""
	}

	var lines []string

	for i, cmd := range d.filtered {
		if i >= maxDropdownItems {
			break
		}

		// Format: /command  Description
		cmdText := dropdownCmdStyle.Render("/" + cmd.Name)
		descText := dropdownDescStyle.Render(cmd.Description)

		// Calculate spacing for alignment
		cmdWidth := lipgloss.Width(cmdText)
		padding := 16 - cmdWidth
		if padding < 1 {
			padding = 1
		}

		line := cmdText + strings.Repeat(" ", padding) + descText

		if i == d.selectedIndex {
			lines = append(lines, dropdownSelectedStyle.Width(d.width-6).Render(line))
		} else {
			lines = append(lines, dropdownItemStyle.Width(d.width-6).Render(line))
		}
	}

	// Show count if more items exist
	if len(d.filtered) > maxDropdownItems {
		remaining := len(d.filtered) - maxDropdownItems
		lines = append(lines, dropdownDescStyle.Render(
			fmt.Sprintf("  ... and %d more", remaining),
		))
	}

	content := strings.Join(lines, "\n")
	return dropdownStyle.Width(d.width - 4).Render(content)
}
