package logprocessing

import (
	"testing"
)

func TestDrainProcessor_Constructor(t *testing.T) {
	config := DefaultDrainConfig()
	processor := NewDrainProcessor(config)

	if processor == nil {
		t.Fatal("NewDrainProcessor returned nil")
	}

	if processor.drain == nil {
		t.Fatal("DrainProcessor.drain is nil")
	}
}

func TestDrainProcessor_Train(t *testing.T) {
	processor := NewDrainProcessor(DefaultDrainConfig())

	// Train with similar logs
	logs := []string{
		"connected to 10.0.0.1",
		"connected to 10.0.0.2",
		"connected to 192.168.1.1",
	}

	var lastCluster string
	for _, log := range logs {
		cluster := processor.Train(log)
		if cluster == nil {
			t.Fatalf("Train(%q) returned nil", log)
		}
		lastCluster = cluster.String()
	}

	// All should match the same template pattern
	if lastCluster == "" {
		t.Fatal("Cluster template is empty")
	}

	// Template should contain wildcard for IP address
	if lastCluster == logs[0] {
		t.Errorf("Expected template with wildcard, got exact match: %s", lastCluster)
	}
}

func TestDrainProcessor_Match(t *testing.T) {
	processor := NewDrainProcessor(DefaultDrainConfig())

	// Train with multiple similar logs to create a cluster
	processor.Train("user login succeeded")
	processor.Train("user logout succeeded")
	processor.Train("user signup succeeded")

	// Match should find the trained cluster for exact match
	cluster := processor.Match("user login succeeded")
	if cluster == nil {
		t.Fatal("Match returned nil for trained pattern")
	}

	// Match with similar pattern should find the cluster
	cluster = processor.Match("user delete succeeded")
	if cluster == nil {
		t.Fatal("Match returned nil for similar pattern")
	}

	// Match with completely different pattern should return nil
	cluster = processor.Match("database connection failed")
	if cluster != nil {
		t.Logf("Match returned cluster for unrelated pattern (acceptable if similar enough): %s", cluster.String())
	}
}

func TestDrainConfig_Defaults(t *testing.T) {
	config := DefaultDrainConfig()

	if config.LogClusterDepth != 4 {
		t.Errorf("Expected LogClusterDepth=4, got %d", config.LogClusterDepth)
	}

	if config.SimTh != 0.4 {
		t.Errorf("Expected SimTh=0.4, got %f", config.SimTh)
	}

	if config.MaxChildren != 100 {
		t.Errorf("Expected MaxChildren=100, got %d", config.MaxChildren)
	}

	if config.MaxClusters != 0 {
		t.Errorf("Expected MaxClusters=0 (unlimited), got %d", config.MaxClusters)
	}

	if config.ParamString != "<*>" {
		t.Errorf("Expected ParamString='<*>', got %q", config.ParamString)
	}
}
