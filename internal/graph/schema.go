package graph

import "context"

// Schema provides utilities for graph schema management.
type Schema struct {
	client Client
}

// NewSchema creates a new Schema manager.
func NewSchema(client Client) *Schema {
	return &Schema{client: client}
}

// Initialize sets up the graph schema with indexes and constraints.
func (s *Schema) Initialize(ctx context.Context) error {
	return s.client.InitializeSchema(ctx)
}
