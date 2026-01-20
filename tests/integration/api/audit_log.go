package api

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

// ExtractTimestampAndResourceUIDFromFile extracts the timestamp and UID for a specific resource kind
func ExtractTimestampAndResourceUIDFromFile(jsonlPath, targetKind string) (int64, string, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	var lastTimestamp int64
	var resourceUID string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}

		// Extract UID if this is the target resource kind
		if event.Resource.Kind == targetKind {
			resourceUID = event.Resource.UID
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, "", err
	}

	return lastTimestamp, resourceUID, nil
}

// ExtractTimestampAndPodUIDFromFile extracts the timestamp from the last event and pod UID from the JSONL file
// The pod UID should be the pod that relates to the resource under test (prefers ReplicaSet/StatefulSet owned pods)
func ExtractTimestampAndPodUIDFromFile(jsonlPath string) (int64, string, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	var lastTimestamp int64
	var podUID string
	var failedPodUID string // Track pods with error states (preferred as symptoms)
	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines (default is 64KB)
	// Use 10MB buffer to handle very large resource definitions
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Track the last timestamp
		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}

		// Extract pod UID if this is a Pod resource
		// Prefer pods that are owned by ReplicaSet (which are managed by Deployment/HelmRelease)
		// or StatefulSet (for StatefulSet scenarios)
		if event.Resource.Kind == "Pod" {
			// Try to parse the data field to check for owner and status
			if len(event.Data) > 0 {
				var data map[string]interface{}
				if err := json.Unmarshal(event.Data, &data); err == nil {
					isOwnedByController := false
					if metadata, ok := data["metadata"].(map[string]interface{}); ok {
						if ownerRefs, ok := metadata["ownerReferences"].([]interface{}); ok {
							for _, ref := range ownerRefs {
								if refMap, ok := ref.(map[string]interface{}); ok {
									if kind, ok := refMap["kind"].(string); ok {
										// Prefer ReplicaSet-owned pods (for HelmRelease/Deployment scenarios)
										// or StatefulSet-owned pods (for StatefulSet scenarios)
										if kind == "ReplicaSet" || kind == "StatefulSet" {
											isOwnedByController = true
											podUID = event.Resource.UID
											break
										}
									}
								}
							}
						}
					}

					// Check if this pod has an error state (Evicted, Failed, CrashLoopBackOff, etc.)
					// These are more useful as symptoms for causal analysis
					if isOwnedByController {
						if status, ok := data["status"].(map[string]interface{}); ok {
							reason, _ := status["reason"].(string)
							phase, _ := status["phase"].(string)

							// Check for error states
							if reason == "Evicted" || phase == "Failed" {
								failedPodUID = event.Resource.UID
							}

							// Check container statuses for error states
							if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
								for _, cs := range containerStatuses {
									if csMap, ok := cs.(map[string]interface{}); ok {
										if state, ok := csMap["state"].(map[string]interface{}); ok {
											if waiting, ok := state["waiting"].(map[string]interface{}); ok {
												waitReason, _ := waiting["reason"].(string)
												if waitReason == "CrashLoopBackOff" || waitReason == "ImagePullBackOff" {
													failedPodUID = event.Resource.UID
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
			// If we haven't found a ReplicaSet/StatefulSet-owned pod yet, use any pod as fallback
			if podUID == "" {
				podUID = event.Resource.UID
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, "", err
	}

	// Prefer pods with error states as they are better symptoms for causal analysis
	if failedPodUID != "" {
		return lastTimestamp, failedPodUID, nil
	}

	return lastTimestamp, podUID, nil
}
