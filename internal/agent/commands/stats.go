package commands

import (
	"fmt"
	"strings"
)

func init() {
	DefaultRegistry.Register(&StatsHandler{})
}

// StatsHandler implements the /stats command.
type StatsHandler struct{}

func (h *StatsHandler) Entry() Entry {
	return Entry{
		Name:        "stats",
		Description: "Show session statistics",
		Usage:       "/stats",
	}
}

func (h *StatsHandler) Execute(ctx *Context, args []string) Result {
	var msg strings.Builder
	msg.WriteString("Session Statistics:\n\n")
	msg.WriteString(fmt.Sprintf("  LLM Requests:    %d\n", ctx.TotalLLMRequests))
	msg.WriteString(fmt.Sprintf("  Input Tokens:    %d\n", ctx.TotalInputTokens))
	msg.WriteString(fmt.Sprintf("  Output Tokens:   %d\n", ctx.TotalOutputTokens))
	msg.WriteString(fmt.Sprintf("  Session ID:      %s\n", ctx.SessionID))

	return Result{
		Success: true,
		Message: msg.String(),
		IsInfo:  true,
	}
}
