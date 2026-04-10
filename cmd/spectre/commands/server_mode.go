package commands

type runtimeName string
type storageBackend string
type ingestionMode string

const (
	runtimeNameEmbedded runtimeName = "embedded"

	backendEmbedded storageBackend = "embedded"

	ingestionModeLive       ingestionMode = "live"
	ingestionModeImportOnly ingestionMode = "import-only"
)

type serverModeInput struct {
	WatcherEnabled bool
	ImportPath     string
}

type serverRuntimeMode struct {
	Name          string
	Backend       string
	IngestionMode string
	ImportOnly    bool
	StartWatcher  bool
	StartMCP      bool
}

func resolveServerRuntimeMode(in serverModeInput) (serverRuntimeMode, error) {
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
		ImportOnly:    importOnly,
		StartWatcher:  startWatcher,
		StartMCP:      true,
	}, nil
}
