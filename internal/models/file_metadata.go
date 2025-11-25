package models

import "time"

// FileMetadata stores information about an entire hourly file for quick access
type FileMetadata struct {
	// CreatedAt is the Unix timestamp when the file was created
	CreatedAt int64 `json:"createdAt"`

	// FinalizedAt is the Unix timestamp when the hour completed
	FinalizedAt int64 `json:"finalizedAt"`

	// TotalEvents is the total number of events in the file
	TotalEvents int64 `json:"totalEvents"`

	// TotalUncompressedBytes is the sum of all uncompressed event sizes
	TotalUncompressedBytes int64 `json:"totalUncompressedBytes"`

	// TotalCompressedBytes is the sum of all compressed event sizes
	TotalCompressedBytes int64 `json:"totalCompressedBytes"`

	// CompressionRatio is the ratio of compressed to uncompressed (0.0 to 1.0)
	CompressionRatio float32 `json:"compressionRatio"`

	// ResourceTypes is a set of unique resource kinds in the file
	ResourceTypes map[string]bool `json:"resourceTypes"`

	// Namespaces is a set of unique namespaces in the file
	Namespaces map[string]bool `json:"namespaces"`
}

// Validate checks that the file metadata is well-formed
func (f *FileMetadata) Validate() error {
	// Check timestamps
	if f.CreatedAt < 0 || f.FinalizedAt < 0 {
		return NewValidationError("timestamps must be non-negative")
	}

	if f.FinalizedAt < f.CreatedAt {
		return NewValidationError("finalizedAt cannot be before createdAt")
	}

	// Check event count
	if f.TotalEvents < 0 {
		return NewValidationError("totalEvents must be non-negative")
	}

	// Check byte sizes
	if f.TotalUncompressedBytes < 0 || f.TotalCompressedBytes < 0 {
		return NewValidationError("byte counts must be non-negative")
	}

	// Check compression ratio
	if f.CompressionRatio < 0.0 || f.CompressionRatio > 1.0 {
		return NewValidationError("compressionRatio must be between 0.0 and 1.0")
	}

	// Maps can be empty (file can be empty)

	return nil
}

// GetCreatedTime returns the created timestamp as time.Time
func (f *FileMetadata) GetCreatedTime() time.Time {
	return time.Unix(f.CreatedAt, 0)
}

// GetFinalizedTime returns the finalized timestamp as time.Time
func (f *FileMetadata) GetFinalizedTime() time.Time {
	return time.Unix(f.FinalizedAt, 0)
}

// GetDuration returns the duration from creation to finalization
func (f *FileMetadata) GetDuration() time.Duration {
	return time.Duration((f.FinalizedAt - f.CreatedAt)) * time.Second
}

// GetAverageEventSize returns the average uncompressed event size in bytes
func (f *FileMetadata) GetAverageEventSize() float64 {
	if f.TotalEvents == 0 {
		return 0.0
	}
	return float64(f.TotalUncompressedBytes) / float64(f.TotalEvents)
}

// GetCompressionSavings returns the total bytes saved by compression
func (f *FileMetadata) GetCompressionSavings() int64 {
	return f.TotalUncompressedBytes - f.TotalCompressedBytes
}

// IsEmpty checks if the file contains no events
func (f *FileMetadata) IsEmpty() bool {
	return f.TotalEvents == 0
}

// ContainsResource checks if the file contains events of a given resource type
func (f *FileMetadata) ContainsResource(kind string) bool {
	return f.ResourceTypes[kind]
}

// ContainsNamespace checks if the file contains events from a given namespace
func (f *FileMetadata) ContainsNamespace(namespace string) bool {
	return f.Namespaces[namespace]
}

// GetResourceTypes returns a list of all resource kinds in the file
func (f *FileMetadata) GetResourceTypes() []string {
	kinds := make([]string, 0, len(f.ResourceTypes))
	for k := range f.ResourceTypes {
		kinds = append(kinds, k)
	}
	return kinds
}

// GetNamespaces returns a list of all namespaces in the file
func (f *FileMetadata) GetNamespaces() []string {
	namespaces := make([]string, 0, len(f.Namespaces))
	for ns := range f.Namespaces {
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

// IsValid checks if the file metadata is valid
func (f *FileMetadata) IsValid() bool {
	return f.Validate() == nil
}
