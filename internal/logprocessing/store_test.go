package logprocessing

import (
	"strings"
	"testing"
)

func TestNewTemplateStore(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	if store == nil {
		t.Fatal("NewTemplateStore returned nil")
	}

	if store.namespaces == nil {
		t.Error("namespaces map not initialized")
	}

	if store.config.SimTh != config.SimTh {
		t.Errorf("config not stored correctly: got %v, want %v", store.config.SimTh, config.SimTh)
	}
}

func TestProcessBasicLog(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process a simple log
	templateID, err := store.Process("default", "connected to 10.0.0.1")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if templateID == "" {
		t.Error("Process returned empty template ID")
	}

	// Retrieve template
	template, err := store.GetTemplate("default", templateID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	if template.ID != templateID {
		t.Errorf("template ID mismatch: got %s, want %s", template.ID, templateID)
	}

	if template.Namespace != "default" {
		t.Errorf("template namespace mismatch: got %s, want default", template.Namespace)
	}

	// Pattern should contain <IP> due to masking
	if !strings.Contains(template.Pattern, "<IP>") {
		t.Errorf("template pattern should contain <IP>, got: %s", template.Pattern)
	}

	if template.Count != 1 {
		t.Errorf("template count should be 1, got: %d", template.Count)
	}
}

func TestProcessSameTemplateTwice(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process two logs that should map to same template (different IPs)
	id1, err := store.Process("default", "connected to 10.0.0.1")
	if err != nil {
		t.Fatalf("Process first log failed: %v", err)
	}

	id2, err := store.Process("default", "connected to 10.0.0.2")
	if err != nil {
		t.Fatalf("Process second log failed: %v", err)
	}

	// Both should map to same template due to IP masking
	if id1 != id2 {
		t.Errorf("expected same template ID for both logs, got %s and %s", id1, id2)
	}

	// Retrieve template and verify count
	template, err := store.GetTemplate("default", id1)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	if template.Count != 2 {
		t.Errorf("template count should be 2, got: %d", template.Count)
	}

	// Verify pattern is masked correctly
	// After PreProcess (lowercase) and masking, <*> from Drain becomes <IP> or <NUM>
	if !strings.Contains(template.Pattern, "connected") {
		t.Errorf("pattern should contain 'connected', got %q", template.Pattern)
	}
	if !strings.Contains(template.Pattern, "<") {
		t.Errorf("pattern should contain masked variables, got %q", template.Pattern)
	}
}

func TestProcessMultipleNamespaces(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process same log in two different namespaces
	id1, err := store.Process("ns1", "server started on port 8080")
	if err != nil {
		t.Fatalf("Process ns1 failed: %v", err)
	}

	id2, err := store.Process("ns2", "server started on port 8080")
	if err != nil {
		t.Fatalf("Process ns2 failed: %v", err)
	}

	// IDs should be different (different namespaces)
	if id1 == id2 {
		t.Error("expected different template IDs for different namespaces")
	}

	// Both templates should exist
	t1, err := store.GetTemplate("ns1", id1)
	if err != nil {
		t.Fatalf("GetTemplate ns1 failed: %v", err)
	}

	t2, err := store.GetTemplate("ns2", id2)
	if err != nil {
		t.Fatalf("GetTemplate ns2 failed: %v", err)
	}

	if t1.Namespace != "ns1" {
		t.Errorf("ns1 template has wrong namespace: %s", t1.Namespace)
	}

	if t2.Namespace != "ns2" {
		t.Errorf("ns2 template has wrong namespace: %s", t2.Namespace)
	}
}

func TestListTemplates(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process several logs
	logs := []string{
		"connected to 10.0.0.1",
		"connected to 10.0.0.2",
		"disconnected from 192.168.1.1",
		"error: connection timeout",
	}

	for _, log := range logs {
		_, err := store.Process("default", log)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
	}

	// List templates
	templates, err := store.ListTemplates("default")
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(templates) == 0 {
		t.Fatal("ListTemplates returned empty list")
	}

	// First template should have highest count (sorted by count descending)
	// "connected to" pattern appears twice
	if templates[0].Count < templates[len(templates)-1].Count {
		t.Error("templates not sorted by count descending")
	}
}

func TestGetTemplate_NamespaceNotFound(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	_, err := store.GetTemplate("nonexistent", "some-id")
	if err != ErrNamespaceNotFound {
		t.Errorf("expected ErrNamespaceNotFound, got: %v", err)
	}
}

func TestGetTemplate_TemplateNotFound(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Create namespace by processing a log
	store.Process("default", "test log")

	// Try to get non-existent template
	_, err := store.GetTemplate("default", "nonexistent-id")
	if err != ErrTemplateNotFound {
		t.Errorf("expected ErrTemplateNotFound, got: %v", err)
	}
}

func TestListTemplates_NamespaceNotFound(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	_, err := store.ListTemplates("nonexistent")
	if err != ErrNamespaceNotFound {
		t.Errorf("expected ErrNamespaceNotFound, got: %v", err)
	}
}

func TestGetNamespaces(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Initially empty
	namespaces := store.GetNamespaces()
	if len(namespaces) != 0 {
		t.Errorf("expected empty namespaces, got: %v", namespaces)
	}

	// Add some namespaces
	store.Process("ns1", "log message 1")
	store.Process("ns2", "log message 2")
	store.Process("ns3", "log message 3")

	namespaces = store.GetNamespaces()
	if len(namespaces) != 3 {
		t.Errorf("expected 3 namespaces, got: %d", len(namespaces))
	}

	// Verify all namespaces present (order doesn't matter)
	found := make(map[string]bool)
	for _, ns := range namespaces {
		found[ns] = true
	}

	for _, expected := range []string{"ns1", "ns2", "ns3"} {
		if !found[expected] {
			t.Errorf("namespace %s not found in result", expected)
		}
	}
}

func TestProcessWithJSONLog(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process JSON log with message field
	jsonLog := `{"level":"info","message":"connected to 10.0.0.1","timestamp":"2024-01-01T00:00:00Z"}`

	id1, err := store.Process("default", jsonLog)
	if err != nil {
		t.Fatalf("Process JSON log failed: %v", err)
	}

	// Process plain text version - should map to same template
	id2, err := store.Process("default", "connected to 10.0.0.2")
	if err != nil {
		t.Fatalf("Process plain log failed: %v", err)
	}

	// Should be same template (message field extracted, IPs masked)
	if id1 != id2 {
		t.Errorf("JSON and plain logs should map to same template, got %s and %s", id1, id2)
	}

	template, _ := store.GetTemplate("default", id1)
	if template.Count != 2 {
		t.Errorf("expected count 2, got: %d", template.Count)
	}
}

func TestProcessConcurrent(t *testing.T) {
	config := DefaultDrainConfig()
	store := NewTemplateStore(config)

	// Process logs concurrently to test thread safety
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			for j := 0; j < 100; j++ {
				store.Process("default", "log message from goroutine")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have exactly one template with count=1000
	templates, err := store.ListTemplates("default")
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(templates) != 1 {
		t.Errorf("expected 1 template, got: %d", len(templates))
	}

	if templates[0].Count != 1000 {
		t.Errorf("expected count 1000, got: %d", templates[0].Count)
	}
}
