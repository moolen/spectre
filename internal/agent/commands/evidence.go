//go:build disabled

package commands

func init() {
	DefaultRegistry.Register(&EvidenceHandler{})
}

// EvidenceHandler implements the /evidence command.
type EvidenceHandler struct{}

func (h *EvidenceHandler) Entry() Entry {
	return Entry{
		Name:        "evidence",
		Description: "Show collected evidence",
		Usage:       "/evidence",
	}
}

func (h *EvidenceHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement evidence display
	return Result{
		Success: false,
		Message: "/evidence - Not yet implemented (would display collected evidence)",
		IsInfo:  true,
	}
}
