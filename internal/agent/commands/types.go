// Package commands provides slash command handling for the agent TUI.
package commands

// Command represents a parsed slash command.
type Command struct {
	Name string
	Args []string
}

// Result contains the result of command execution.
type Result struct {
	Success bool
	Message string
	IsInfo  bool // true for info messages (help, summary, etc)
}

// Entry describes a command for the dropdown and help display.
type Entry struct {
	Name        string // e.g., "help" (without the leading slash)
	Description string // e.g., "Show this help message"
	Usage       string // e.g., "/help" or "/pin <num>"
}

// Context provides handlers access to runner state.
type Context struct {
	SessionID         string
	TotalLLMRequests  int
	TotalInputTokens  int
	TotalOutputTokens int
	QuitFunc          func() // Signal app to quit
}

// Handler is the interface that command handlers must implement.
type Handler interface {
	// Entry returns the command metadata for dropdown/help display.
	Entry() Entry

	// Execute runs the command with the given context and arguments.
	Execute(ctx *Context, args []string) Result
}
