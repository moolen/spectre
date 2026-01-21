//go:build disabled

package commands

import (
	"fmt"
	"strconv"
)

func init() {
	DefaultRegistry.Register(&PinHandler{})
}

// PinHandler implements the /pin command.
type PinHandler struct{}

func (h *PinHandler) Entry() Entry {
	return Entry{
		Name:        "pin",
		Description: "Confirm hypothesis as root cause",
		Usage:       "/pin <num>",
	}
}

func (h *PinHandler) Execute(ctx *Context, args []string) Result {
	if len(args) == 0 {
		return Result{
			Success: false,
			Message: "Usage: /pin <hypothesis-number>",
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

	// TODO: Implement pin hypothesis
	return Result{
		Success: false,
		Message: "/pin - Not yet implemented (would confirm hypothesis as root cause)",
		IsInfo:  true,
	}
}
