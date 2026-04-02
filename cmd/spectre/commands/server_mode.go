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
	Name          string
	Backend       string
	IngestionMode string
	Embedded      bool
	ImportOnly    bool
	AuditOnly     bool
	StartGraph    bool
	StartWatcher  bool
	StartMCP      bool
}

func resolveServerRuntimeMode(in serverModeInput) (serverRuntimeMode, error) {
	if in.Embedded {
		if !in.WatcherEnabled && in.ImportPath == "" {
			return serverRuntimeMode{}, fmt.Errorf("--embedded with watcher disabled requires --import-path")
		}

		ingestionMode := "live"
		importOnly := false
		startWatcher := true
		if !in.WatcherEnabled {
			ingestionMode = "import-only"
			importOnly = true
			startWatcher = false
		}

		return serverRuntimeMode{
			Name:          "embedded",
			Backend:       "embedded",
			IngestionMode: ingestionMode,
			Embedded:      true,
			ImportOnly:    importOnly,
			AuditOnly:     false,
			StartGraph:    false,
			StartWatcher:  startWatcher,
			StartMCP:      true,
		}, nil
	}

	auditOnly := !in.GraphEnabled && in.AuditLogPath != "" && in.WatcherEnabled
	if !in.GraphEnabled && !auditOnly {
		return serverRuntimeMode{}, fmt.Errorf("graph-enabled flag must be set to true, or use audit-only mode")
	}

	return serverRuntimeMode{
		Name:          "graph",
		Backend:       "falkor",
		IngestionMode: "live",
		Embedded:      false,
		ImportOnly:    false,
		AuditOnly:     auditOnly,
		StartGraph:    in.GraphEnabled,
		StartWatcher:  in.WatcherEnabled,
		StartMCP:      !auditOnly,
	}, nil
}
