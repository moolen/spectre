//go:build disabled

package commands

func init() {
	DefaultRegistry.Register(&QuitHandler{})
	DefaultRegistry.Register(&ExitHandler{})
}

// QuitHandler implements the /quit command.
type QuitHandler struct{}

func (h *QuitHandler) Entry() Entry {
	return Entry{
		Name:        "quit",
		Description: "Exit the agent",
		Usage:       "/quit",
	}
}

func (h *QuitHandler) Execute(ctx *Context, args []string) Result {
	if ctx.QuitFunc != nil {
		ctx.QuitFunc()
	}
	return Result{
		Success: true,
		Message: "Goodbye!",
		IsInfo:  true,
	}
}

// ExitHandler implements the /exit command (alias for quit).
type ExitHandler struct{}

func (h *ExitHandler) Entry() Entry {
	return Entry{
		Name:        "exit",
		Description: "Exit the agent",
		Usage:       "/exit",
	}
}

func (h *ExitHandler) Execute(ctx *Context, args []string) Result {
	if ctx.QuitFunc != nil {
		ctx.QuitFunc()
	}
	return Result{
		Success: true,
		Message: "Goodbye!",
		IsInfo:  true,
	}
}
