//go:build disabled

package commands

func init() {
	DefaultRegistry.Register(&CompactHandler{})
}

// CompactHandler implements the /compact command.
type CompactHandler struct{}

func (h *CompactHandler) Entry() Entry {
	return Entry{
		Name:        "compact",
		Description: "Summarize conversation",
		Usage:       "/compact [prompt]",
	}
}

func (h *CompactHandler) Execute(ctx *Context, args []string) Result {
	// TODO: Implement compaction
	return Result{
		Success: false,
		Message: "/compact - Not yet implemented (would summarize conversation to free up context)",
		IsInfo:  true,
	}
}
