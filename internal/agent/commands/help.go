package commands

import (
	"fmt"
	"strings"
)

func init() {
	DefaultRegistry.Register(&HelpHandler{})
}

// HelpHandler implements the /help command.
type HelpHandler struct{}

func (h *HelpHandler) Entry() Entry {
	return Entry{
		Name:        "help",
		Description: "Show help message",
		Usage:       "/help",
	}
}

func (h *HelpHandler) Execute(ctx *Context, args []string) Result {
	entries := DefaultRegistry.AllEntries()

	var msg strings.Builder
	msg.WriteString("Available Commands:\n\n")

	for _, e := range entries {
		msg.WriteString(fmt.Sprintf("  %-20s %s\n", e.Usage, e.Description))
	}

	return Result{
		Success: true,
		Message: msg.String(),
		IsInfo:  true,
	}
}
