package graph

import (
	"context"
	"strings"

	"github.com/moolen/spectre/internal/logging"
)

// CachedClient wraps a Client with query caching for read operations
type CachedClient struct {
	underlying Client
	cache      *QueryCache
	logger     *logging.Logger
}

// NewCachedClient creates a new cached client wrapper
func NewCachedClient(client Client, config QueryCacheConfig, logger *logging.Logger) (*CachedClient, error) {
	cache, err := NewQueryCache(config, logger)
	if err != nil {
		return nil, err
	}

	return &CachedClient{
		underlying: client,
		cache:      cache,
		logger:     logger,
	}, nil
}

// Connect establishes connection to FalkorDB (delegates to underlying client)
func (c *CachedClient) Connect(ctx context.Context) error {
	return c.underlying.Connect(ctx)
}

// Close closes the connection (delegates to underlying client)
func (c *CachedClient) Close() error {
	return c.underlying.Close()
}

// Ping checks if the connection is alive (delegates to underlying client)
func (c *CachedClient) Ping(ctx context.Context) error {
	return c.underlying.Ping(ctx)
}

// ExecuteQuery executes a Cypher query with caching for read queries
func (c *CachedClient) ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error) {
	// Skip caching for write queries
	if isWriteQuery(query.Query) {
		return c.underlying.ExecuteQuery(ctx, query)
	}

	key := MakeQueryKey(query)

	// Check cache
	if result, ok := c.cache.Get(key); ok {
		return result, nil
	}

	// Execute query
	result, err := c.underlying.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.Put(key, result)

	return result, nil
}

// CreateNode creates a node in the graph (delegates to underlying client)
func (c *CachedClient) CreateNode(ctx context.Context, nodeType NodeType, properties interface{}) error {
	return c.underlying.CreateNode(ctx, nodeType, properties)
}

// CreateEdge creates an edge between two nodes (delegates to underlying client)
func (c *CachedClient) CreateEdge(ctx context.Context, edgeType EdgeType, fromUID, toUID string, properties interface{}) error {
	return c.underlying.CreateEdge(ctx, edgeType, fromUID, toUID, properties)
}

// GetNode retrieves a node by UID (with caching)
func (c *CachedClient) GetNode(ctx context.Context, nodeType NodeType, uid string) (*Node, error) {
	// GetNode is a read operation, but it's already cached at query level
	// Just delegate to underlying client
	return c.underlying.GetNode(ctx, nodeType, uid)
}

// DeleteNodesByTimestamp deletes nodes older than the given timestamp (delegates to underlying client)
func (c *CachedClient) DeleteNodesByTimestamp(ctx context.Context, nodeType NodeType, timestampField string, cutoffNs int64) (int, error) {
	return c.underlying.DeleteNodesByTimestamp(ctx, nodeType, timestampField, cutoffNs)
}

// GetGraphStats retrieves overall graph statistics (delegates to underlying client)
func (c *CachedClient) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	return c.underlying.GetGraphStats(ctx)
}

// InitializeSchema creates indexes and constraints (delegates to underlying client)
func (c *CachedClient) InitializeSchema(ctx context.Context) error {
	return c.underlying.InitializeSchema(ctx)
}

// CacheStats returns cache statistics
func (c *CachedClient) CacheStats() QueryCacheStats {
	return c.cache.Stats()
}

// ClearCache clears the query cache
func (c *CachedClient) ClearCache() {
	c.cache.Clear()
}

// isWriteQuery checks if a query is a write operation that should bypass the cache
func isWriteQuery(query string) bool {
	// Normalize query for checking
	upper := strings.ToUpper(strings.TrimSpace(query))

	// Write operation keywords that should bypass cache
	writeKeywords := []string{
		"CREATE",
		"MERGE",
		"DELETE",
		"DETACH DELETE",
		"SET",
		"REMOVE",
	}

	for _, keyword := range writeKeywords {
		if strings.Contains(upper, keyword) {
			return true
		}
	}

	return false
}
