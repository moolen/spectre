package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const (
	// DefaultPageSize is the default number of resources per page
	DefaultPageSize = 100

	// MaxPageSize is the maximum allowed page size
	MaxPageSize = 500
)

// PaginationRequest contains pagination parameters for timeline queries
type PaginationRequest struct {
	// PageSize is the number of resources per page (default: 100, max: 500)
	PageSize int `json:"pageSize"`

	// Cursor is an opaque cursor string for fetching the next page (empty = first page)
	Cursor string `json:"cursor"`
}

// GetPageSize returns the page size, applying defaults and limits
func (p *PaginationRequest) GetPageSize() int {
	if p == nil || p.PageSize <= 0 {
		return DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		return MaxPageSize
	}
	return p.PageSize
}

// PaginationResponse contains pagination metadata in the response
type PaginationResponse struct {
	// NextCursor is the cursor for fetching the next page (empty = no more pages)
	NextCursor string `json:"nextCursor,omitempty"`

	// HasMore indicates if there are more pages available
	HasMore bool `json:"hasMore"`

	// PageSize is the actual page size used
	PageSize int `json:"pageSize"`
}

// ResourceCursor represents the decoded pagination cursor
// It encodes the last resource's sort key: (kind, namespace, name)
// This allows stable cursor-based pagination that survives data changes
type ResourceCursor struct {
	// Kind is the resource kind of the last resource on the previous page
	Kind string `json:"k"`

	// Namespace is the namespace of the last resource on the previous page
	Namespace string `json:"ns"`

	// Name is the name of the last resource on the previous page
	Name string `json:"n"`
}

// Encode returns a base64-encoded cursor string
func (c *ResourceCursor) Encode() string {
	if c == nil {
		return ""
	}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor parses a cursor string into a ResourceCursor
// Returns nil if the cursor is empty or invalid
func DecodeCursor(cursor string) (*ResourceCursor, error) {
	if cursor == "" {
		return nil, nil
	}

	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var rc ResourceCursor
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	return &rc, nil
}

// NewResourceCursor creates a new cursor from resource metadata
func NewResourceCursor(kind, namespace, name string) *ResourceCursor {
	return &ResourceCursor{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	}
}
