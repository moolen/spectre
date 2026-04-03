package integrationadmin

import "github.com/moolen/spectre/internal/integration"

type Instance struct {
	Name       string                  `json:"name"`
	Type       string                  `json:"type"`
	Enabled    bool                    `json:"enabled"`
	Config     map[string]interface{}  `json:"config"`
	Health     string                  `json:"health"`
	DateAdded  string                  `json:"dateAdded"`
	SyncStatus *integration.SyncStatus `json:"syncStatus,omitempty"`
}

type TestConnectionRequest struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

type TestConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type SignalValidationResponse struct {
	Message string `json:"message"`
}
