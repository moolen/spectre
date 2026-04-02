package commands

import "fmt"

type runtimeName string
type storageBackend string
type ingestionMode string

const (
	runtimeNameEmbedded runtimeName = "embedded"
	runtimeNameGraph    runtimeName = "graph"

	backendEmbedded storageBackend = "embedded"
	backendFalkor   storageBackend = "falkor"

	ingestionModeLive       ingestionMode = "live"
	ingestionModeImportOnly ingestionMode = "import-only"
)

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
		ingestionMode := ingestionModeLive
		importOnly := false
		startWatcher := true
		if !in.WatcherEnabled {
			ingestionMode = ingestionModeImportOnly
			importOnly = true
			startWatcher = false
		}

		return serverRuntimeMode{
			Name:          string(runtimeNameEmbedded),
			Backend:       string(backendEmbedded),
			IngestionMode: string(ingestionMode),
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
		Name:    string(runtimeNameGraph),
		Backend: string(backendFalkor),
		// Non-embedded mode currently reports the primary ingestion family, not whether watcher is active.
		IngestionMode: string(ingestionModeLive),
		Embedded:      false,
		ImportOnly:    false,
		AuditOnly:     auditOnly,
		StartGraph:    in.GraphEnabled,
		StartWatcher:  in.WatcherEnabled,
		StartMCP:      !auditOnly,
	}, nil
}
