package mcp

import "github.com/moolen/spectre/internal/mcp/client"

// Re-export types and client
type SpectreClient = client.SpectreClient
type TimelineResponse = client.TimelineResponse
type TimelineResource = client.TimelineResource
type StatusSegment = client.StatusSegment
type K8sEvent = client.K8sEvent
type MetadataResponse = client.MetadataResponse
type TimeRange = client.TimeRange

// NewSpectreClient creates a new Spectre API client
func NewSpectreClient(baseURL string) *SpectreClient {
	return client.NewSpectreClient(baseURL)
}
