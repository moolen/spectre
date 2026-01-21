//go:build disabled

package commands

func init() {
	DefaultRegistry.Register(&SummaryHandler{})
}

// SummaryHandler implements the /summary command.
type SummaryHandler struct{}

func (h *SummaryHandler) Entry() Entry {
	return Entry{
		Name:        "summary",
		Description: "Generate incident briefing",
		Usage:       "/summary",
	}
}

func (h *SummaryHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement summary display
	return Result{
		Success: false,
		Message: "/summary - Not yet implemented (would display incident briefing)",
		IsInfo:  true,
	}
}
