package watcher

import (
	"encoding/json"
	"fmt"
)

// ManagedFieldsPruner removes managedFields from Kubernetes resource objects
type ManagedFieldsPruner struct{}

// NewManagedFieldsPruner creates a new pruner
func NewManagedFieldsPruner() *ManagedFieldsPruner {
	return &ManagedFieldsPruner{}
}

// Prune removes managedFields from the JSON data
func (p *ManagedFieldsPruner) Prune(data []byte) ([]byte, error) {
	// Parse the JSON
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Remove managedFields from metadata
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		delete(metadata, "managedFields")
	}

	// Remove other fields that can be large and are not needed
	// Keep only essential fields
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		// These fields are typically large and can be pruned
		largeFields := []string{
			"managedFields", // Field ownership tracking
		}

		for _, field := range largeFields {
			delete(metadata, field)
		}
	}

	// Marshal back to JSON
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return result, nil
}

// PruneWithFields removes specified fields in addition to managedFields
func (p *ManagedFieldsPruner) PruneWithFields(data []byte, fieldsToRemove []string) ([]byte, error) {
	// Parse the JSON
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Remove managedFields and other fields from metadata
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		// Always remove managedFields
		delete(metadata, "managedFields")

		// Remove additional fields
		for _, field := range fieldsToRemove {
			delete(metadata, field)
		}
	}

	// Marshal back to JSON
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return result, nil
}

// GetPrunedSize calculates the size reduction from pruning
func (p *ManagedFieldsPruner) GetPrunedSize(original []byte, pruned []byte) int64 {
	return int64(len(original) - len(pruned))
}
