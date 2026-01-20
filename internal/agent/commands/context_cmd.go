package commands

func init() {
	DefaultRegistry.Register(&ContextHandler{})
}

// ContextHandler implements the /context command.
type ContextHandler struct{}

func (h *ContextHandler) Entry() Entry {
	return Entry{
		Name:        "context",
		Description: "Show analysis context",
		Usage:       "/context",
	}
}

func (h *ContextHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement context display
	return Result{
		Success: false,
		Message: "/context - Not yet implemented (would display analysis context)",
		IsInfo:  true,
	}
}
