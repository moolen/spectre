//go:build disabled

package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// App manages the TUI application lifecycle.
type App struct {
	program *tea.Program
	model   *Model
	eventCh chan interface{}

	// Callback for when user submits input
	onInput func(string) error
}

// Config contains configuration for the TUI app.
type Config struct {
	SessionID string
	APIURL    string
	ModelName string

	// OnInput is called when the user submits input.
	// The TUI will send events through the event channel.
	OnInput func(input string) error
}

// NewApp creates a new TUI application.
func NewApp(cfg Config) *App {
	eventCh := make(chan interface{}, 100)
	model := NewModel(eventCh, cfg.SessionID, cfg.APIURL, cfg.ModelName)

	app := &App{
		model:   &model,
		eventCh: eventCh,
		onInput: cfg.OnInput,
	}

	return app
}

// Run starts the TUI application.
func (a *App) Run(ctx context.Context) error {
	// Create program with alt screen for full TUI experience
	a.program = tea.NewProgram(
		a.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support for scrolling
		tea.WithContext(ctx),
	)

	// Handle input submissions in a goroutine
	go a.handleInputLoop(ctx)

	// Run the program
	finalModel, err := a.program.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if user quit
	if m, ok := finalModel.(*Model); ok && m.quitting {
		return nil
	}

	return nil
}

// handleInputLoop handles input submissions from the TUI.
func (a *App) handleInputLoop(ctx context.Context) {
	// We need to wait for InputSubmittedMsg and call onInput
	// This is handled via the tea.Program's messages
}

// SendEvent sends an event to the TUI for display.
func (a *App) SendEvent(event interface{}) {
	select {
	case a.eventCh <- event:
	default:
		// Channel full, drop event to prevent blocking
	}
}

// Send sends a message directly to the tea.Program.
func (a *App) Send(msg tea.Msg) {
	if a.program != nil {
		a.program.Send(msg)
	}
}

// Close closes the event channel.
func (a *App) Close() {
	close(a.eventCh)
}

// EventChannel returns the event channel for sending events.
func (a *App) EventChannel() chan<- interface{} {
	return a.eventCh
}

// IsTerminal returns true if stdout is a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// RunSimple creates and runs the TUI in a simple blocking mode.
// This is useful for testing or simple integrations.
func RunSimple(ctx context.Context, cfg Config) error {
	app := NewApp(cfg)
	return app.Run(ctx)
}
