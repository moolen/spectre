//go:build disabled

package commands

func init() {
	DefaultRegistry.Register(&SessionsHandler{})
}

// SessionsHandler implements the /sessions command.
type SessionsHandler struct{}

func (h *SessionsHandler) Entry() Entry {
	return Entry{
		Name:        "sessions",
		Description: "Browse and switch sessions",
		Usage:       "/sessions",
	}
}

func (h *SessionsHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement session browsing
	return Result{
		Success: false,
		Message: "/sessions - Not yet implemented (would browse previous sessions)",
		IsInfo:  true,
	}
}
