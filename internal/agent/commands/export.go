//go:build disabled

package commands

import "fmt"

func init() {
	DefaultRegistry.Register(&ExportHandler{})
}

// ExportHandler implements the /export command.
type ExportHandler struct{}

func (h *ExportHandler) Entry() Entry {
	return Entry{
		Name:        "export",
		Description: "Export session to markdown",
		Usage:       "/export [file]",
	}
}

func (h *ExportHandler) Execute(ctx *Context, args []string) Result {
	filename := "session"
	if len(args) > 0 {
		filename = args[0]
	}

	// TODO: Implement export
	return Result{
		Success: false,
		Message: fmt.Sprintf("/export - Not yet implemented (would export to %s)", filename),
		IsInfo:  true,
	}
}
