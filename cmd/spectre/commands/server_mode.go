package commands

import "fmt"

type serverModeInput struct {
	Embedded       bool
	GraphEnabled   bool
	WatcherEnabled bool
	ImportPath     string
	AuditLogPath   string
}

type serverRuntimeMode struct {
	Name         string
	Embedded     bool
	AuditOnly    bool
	StartGraph   bool
	StartWatcher bool
	StartMCP     bool
}

func resolveServerRuntimeMode(in serverModeInput) (serverRuntimeMode, error) {
	if in.Embedded && in.ImportPath == "" {
		return serverRuntimeMode{}, fmt.Errorf("--embedded requires --import-path")
	}

	if in.Embedded {
		return serverRuntimeMode{
			Name:         "embedded",
			Embedded:     true,
			AuditOnly:    false,
			StartGraph:   false,
			StartWatcher: false,
			StartMCP:     false,
		}, nil
	}

	auditOnly := !in.GraphEnabled && in.AuditLogPath != "" && in.WatcherEnabled
	if !in.GraphEnabled && !auditOnly {
		return serverRuntimeMode{}, fmt.Errorf("graph-enabled flag must be set to true, or use audit-only mode")
	}

	return serverRuntimeMode{
		Name:         "graph",
		Embedded:     false,
		AuditOnly:    auditOnly,
		StartGraph:   in.GraphEnabled,
		StartWatcher: in.WatcherEnabled,
		StartMCP:     !auditOnly,
	}, nil
}
