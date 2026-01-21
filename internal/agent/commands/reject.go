//go:build disabled

package commands

import (
	"fmt"
	"strconv"
)

func init() {
	DefaultRegistry.Register(&RejectHandler{})
}

// RejectHandler implements the /reject command.
type RejectHandler struct{}

func (h *RejectHandler) Entry() Entry {
	return Entry{
		Name:        "reject",
		Description: "Reject a hypothesis",
		Usage:       "/reject <num>",
	}
}

func (h *RejectHandler) Execute(ctx *Context, args []string) Result {
	if len(args) == 0 {
		return Result{
			Success: false,
			Message: "Usage: /reject <hypothesis-number>",
			IsInfo:  true,
		}
	}

	_, err := strconv.Atoi(args[0])
	if err != nil {
		return Result{
			Success: false,
			Message: fmt.Sprintf("Invalid hypothesis number: %s", args[0]),
			IsInfo:  true,
		}
	}

	// TODO: Implement reject hypothesis
	return Result{
		Success: false,
		Message: "/reject - Not yet implemented (would reject hypothesis)",
		IsInfo:  true,
	}
}
