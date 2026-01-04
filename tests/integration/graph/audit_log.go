package graph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/moolen/spectre/internal/models"
)

// LoadAuditLog reads events from a JSONL audit log file
func LoadAuditLog(filePath string) ([]models.Event, error) {
	return LoadAuditLogFiltered(filePath, nil)
}

// LoadAuditLogFiltered loads events with optional filtering
func LoadAuditLogFiltered(filePath string, filter func(models.Event) bool) ([]models.Event, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	var events []models.Event
	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines (default is 64KB)
	// Use 10MB buffer to handle very large resource definitions
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			// Log warning but continue - skip malformed lines
			fmt.Printf("Warning: Skipping malformed JSON on line %d: %v\n", lineNum, err)
			continue
		}

		// Validate event
		if err := event.Validate(); err != nil {
			fmt.Printf("Warning: Skipping invalid event on line %d: %v\n", lineNum, err)
			continue
		}

		// Apply filter if provided
		if filter != nil && !filter(event) {
			continue
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading audit log file: %w", err)
	}

	return events, nil
}

// LoadAuditLogByResource filters events by resource kind
func LoadAuditLogByResource(filePath string, kind string) ([]models.Event, error) {
	return LoadAuditLogFiltered(filePath, func(e models.Event) bool {
		return e.Resource.Kind == kind
	})
}

// LoadAuditLogByNamespace filters events by namespace
func LoadAuditLogByNamespace(filePath string, namespace string) ([]models.Event, error) {
	return LoadAuditLogFiltered(filePath, func(e models.Event) bool {
		return e.Resource.Namespace == namespace
	})
}
