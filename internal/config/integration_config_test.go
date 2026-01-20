package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIntegrationsFileValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with single instance",
			yaml: `
schema_version: v1
instances:
  - name: victorialogs-prod
    type: victorialogs
    enabled: true
    config:
      url: "http://victorialogs:9428"
`,
			wantErr: false,
		},
		{
			name: "valid config with multiple instances",
			yaml: `
schema_version: v1
instances:
  - name: victorialogs-prod
    type: victorialogs
    enabled: true
    config:
      url: "http://victorialogs:9428"
  - name: victorialogs-staging
    type: victorialogs
    enabled: false
    config:
      url: "http://victorialogs-staging:9428"
`,
			wantErr: false,
		},
		{
			name: "invalid schema version",
			yaml: `
schema_version: v2
instances:
  - name: test
    type: victorialogs
    enabled: true
    config:
      url: "http://test:9428"
`,
			wantErr: true,
			errMsg:  "unsupported schema_version",
		},
		{
			name: "missing instance name",
			yaml: `
schema_version: v1
instances:
  - type: victorialogs
    enabled: true
    config:
      url: "http://test:9428"
`,
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing instance type",
			yaml: `
schema_version: v1
instances:
  - name: test
    enabled: true
    config:
      url: "http://test:9428"
`,
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "duplicate instance names",
			yaml: `
schema_version: v1
instances:
  - name: victorialogs-prod
    type: victorialogs
    enabled: true
    config:
      url: "http://victorialogs-1:9428"
  - name: victorialogs-prod
    type: victorialogs
    enabled: true
    config:
      url: "http://victorialogs-2:9428"
`,
			wantErr: true,
			errMsg:  "duplicate instance name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config IntegrationsFile
			err := yaml.Unmarshal([]byte(tt.yaml), &config)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			err = config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" {
					// Check if error message contains expected substring
					errStr := err.Error()
					if len(errStr) < len(tt.errMsg) || errStr[:len(tt.errMsg)] != tt.errMsg[:len(tt.errMsg)] {
						// Simple substring check
						found := false
						for i := 0; i <= len(errStr)-len(tt.errMsg); i++ {
							if errStr[i:i+len(tt.errMsg)] == tt.errMsg {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("Expected error containing %q, got %q", tt.errMsg, errStr)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}
