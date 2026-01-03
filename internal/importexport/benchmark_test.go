package importexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

// BenchmarkParseJSONEvents benchmarks JSON parsing performance
func BenchmarkParseJSONEvents(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("events_%d", size), func(b *testing.B) {
			// Create test data
			events := make([]models.Event, size)
			for i := 0; i < size; i++ {
				events[i] = models.Event{
					ID:        fmt.Sprintf("event-%d", i),
					Timestamp: 1234567890000000000 + int64(i),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      fmt.Sprintf("deployment-%d", i),
						UID:       fmt.Sprintf("uid-%d", i),
					},
				}
			}

			req := BatchEventImportRequest{Events: events}
			jsonData, err := json.Marshal(req)
			if err != nil {
				b.Fatalf("Failed to marshal test data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(string(jsonData))
				_, err := Import(FromReader(reader))
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkImportFromFile benchmarks file import performance
func BenchmarkImportFromFile(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("events_%d", size), func(b *testing.B) {
			// Create temporary directory
			tmpDir := b.TempDir()
			testFile := filepath.Join(tmpDir, "test.json")

			// Create test data
			events := make([]models.Event, size)
			for i := 0; i < size; i++ {
				events[i] = models.Event{
					ID:        fmt.Sprintf("event-%d", i),
					Timestamp: 1234567890000000000 + int64(i),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      fmt.Sprintf("deployment-%d", i),
						UID:       fmt.Sprintf("uid-%d", i),
					},
				}
			}

			req := BatchEventImportRequest{Events: events}
			jsonData, err := json.MarshalIndent(req, "", "  ")
			if err != nil {
				b.Fatalf("Failed to marshal test data: %v", err)
			}

			if err := os.WriteFile(testFile, jsonData, 0644); err != nil {
				b.Fatalf("Failed to write test file: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := Import(FromFile(testFile))
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkImportFromDirectory benchmarks directory import performance
func BenchmarkImportFromDirectory(b *testing.B) {
	fileCounts := []int{5, 10, 20}
	eventsPerFile := 100

	for _, fileCount := range fileCounts {
		b.Run(fmt.Sprintf("files_%d", fileCount), func(b *testing.B) {
			// Create temporary directory
			tmpDir := b.TempDir()

			// Create test files
			for i := 0; i < fileCount; i++ {
				testFile := filepath.Join(tmpDir, fmt.Sprintf("events_%d.json", i))

				events := make([]models.Event, eventsPerFile)
				for j := 0; j < eventsPerFile; j++ {
					events[j] = models.Event{
						ID:        fmt.Sprintf("event-%d-%d", i, j),
						Timestamp: 1234567890000000000 + int64(i*eventsPerFile+j),
						Type:      models.EventTypeCreate,
						Resource: models.ResourceMetadata{
							Group:     "apps",
							Version:   "v1",
							Kind:      "Deployment",
							Namespace: "default",
							Name:      fmt.Sprintf("deployment-%d-%d", i, j),
							UID:       fmt.Sprintf("uid-%d-%d", i, j),
						},
					}
				}

				req := BatchEventImportRequest{Events: events}
				jsonData, err := json.MarshalIndent(req, "", "  ")
				if err != nil {
					b.Fatalf("Failed to marshal test data: %v", err)
				}

				if err := os.WriteFile(testFile, jsonData, 0644); err != nil {
					b.Fatalf("Failed to write test file: %v", err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := Import(FromDirectory(tmpDir))
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkEnrichment benchmarks enrichment performance
func BenchmarkEnrichment(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("events_%d", size), func(b *testing.B) {
			// Create test events with Kubernetes Event resources
			events := make([]models.Event, size)
			for i := 0; i < size; i++ {
				events[i] = models.Event{
					ID:        fmt.Sprintf("event-%d", i),
					Timestamp: 1234567890000000000 + int64(i),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: "default",
						Name:      fmt.Sprintf("event-%d", i),
						UID:       fmt.Sprintf("event-uid-%d", i),
					},
					Data: json.RawMessage(fmt.Sprintf(`{"involvedObject": {"uid": "pod-uid-%d"}}`, i)),
				}
			}

			req := BatchEventImportRequest{Events: events}
			jsonData, err := json.Marshal(req)
			if err != nil {
				b.Fatalf("Failed to marshal test data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(string(jsonData))
				_, err := Import(FromReader(reader))
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkValidation benchmarks validation performance
func BenchmarkValidation(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("events_%d", size), func(b *testing.B) {
			// Create test events
			events := make([]models.Event, size)
			for i := 0; i < size; i++ {
				events[i] = models.Event{
					ID:        fmt.Sprintf("event-%d", i),
					Timestamp: 1234567890000000000 + int64(i),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      fmt.Sprintf("deployment-%d", i),
						UID:       fmt.Sprintf("uid-%d", i),
					},
				}
			}

			req := BatchEventImportRequest{Events: events}
			jsonData, err := json.Marshal(req)
			if err != nil {
				b.Fatalf("Failed to marshal test data: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(string(jsonData))
				_, err := Import(FromReader(reader))
				if err != nil {
					b.Fatalf("Import failed: %v", err)
				}
			}
		})
	}
}
