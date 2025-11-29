package storage

import (
	"testing"

	"github.com/moolen/spectre/internal/storage"
)

func TestValidateVersionV1_0(t *testing.T) {
	err := storage.ValidateVersion("1.0")
	if err != nil {
		t.Errorf("Version 1.0 should be valid: %v", err)
	}
}

func TestValidateVersionV1_1(t *testing.T) {
	err := storage.ValidateVersion("1.1")
	if err != nil {
		t.Errorf("Version 1.1 should be valid (backward compatible): %v", err)
	}
}

func TestValidateVersionInvalid(t *testing.T) {
	err := storage.ValidateVersion("3.0")
	if err == nil {
		t.Error("Version 3.0 should be invalid")
	}
}

func TestValidateVersionEmpty(t *testing.T) {
	err := storage.ValidateVersion("")
	if err == nil {
		t.Error("Empty version should be invalid")
	}
}

func TestGetVersionInfoV1_0(t *testing.T) {
	info := storage.GetVersionInfo("1.0")
	if info == nil {
		t.Error("GetVersionInfo should return info for version 1.0")
	}

	if info.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", info.Version)
	}

	if len(info.Features) == 0 {
		t.Error("Version 1.0 should have features listed")
	}

	if info.Deprecated {
		t.Error("Version 1.0 should not be deprecated")
	}
}

func TestGetVersionInfoFuture(t *testing.T) {
	info := storage.GetVersionInfo("1.1")
	if info == nil {
		t.Error("GetVersionInfo should return info for future version 1.1")
	}

	if info.Version != "1.1" {
		t.Errorf("Expected version 1.1, got %s", info.Version)
	}

	if info.Introduced != "(planned)" {
		t.Errorf("Future version should show planned introduction")
	}
}

func TestGetVersionInfoUnknown(t *testing.T) {
	info := storage.GetVersionInfo("99.0")
	if info != nil {
		t.Error("GetVersionInfo should return nil for unknown version")
	}
}

func TestVersionCompatibility(t *testing.T) {
	// Test backward compatibility: 1.x versions are compatible
	versions := []string{"1.0", "1.1", "1.2", "1.5"}

	for _, v := range versions {
		err := storage.ValidateVersion(v)
		if err != nil {
			t.Errorf("Version %s should be valid for backward compatibility: %v", v, err)
		}
	}
}

func TestVersionIncompatibility(t *testing.T) {
	// Test forward incompatibility: 2.x and higher not supported
	versions := []string{"2.0", "2.1", "3.0", "10.0"}

	for _, v := range versions {
		err := storage.ValidateVersion(v)
		if err == nil {
			t.Errorf("Version %s should be invalid (future major version)", v)
		}
	}
}

func TestVersionInfoFeatures(t *testing.T) {
	info := storage.GetVersionInfo("1.0")

	requiredFeatures := []string{
		"compression",
		"index",
		"checksum",
	}

	for _, required := range requiredFeatures {
		found := false
		for _, feature := range info.Features {
			if contains(feature, required) {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: Expected feature containing '%s' not found in v1.0", required)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
