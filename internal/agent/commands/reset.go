package commands

func init() {
	DefaultRegistry.Register(&ResetHandler{})
}

// ResetHandler implements the /reset command.
type ResetHandler struct{}

func (h *ResetHandler) Entry() Entry {
	return Entry{
		Name:        "reset",
		Description: "Clear session and start fresh",
		Usage:       "/reset",
	}
}

func (h *ResetHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement session reset
	return Result{
		Success: false,
		Message: "/reset - Not yet implemented",
		IsInfo:  true,
	}
}
