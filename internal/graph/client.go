package graph

import (
	"context"
	"time"

	"github.com/FalkorDB/falkordb-go/v2"
	"github.com/moolen/spectre/internal/logging"
)

// Client provides an interface for interacting with FalkorDB
type Client interface {
	// Connect establishes connection to FalkorDB
	Connect(ctx context.Context) error

	// Close closes the connection
	Close() error

	// Ping checks if the connection is alive
	Ping(ctx context.Context) error

	// ExecuteQuery executes a Cypher query and returns results
	ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error)

	// CreateNode creates a node in the graph
	CreateNode(ctx context.Context, nodeType NodeType, properties interface{}) error

	// CreateEdge creates an edge between two nodes
	CreateEdge(ctx context.Context, edgeType EdgeType, fromUID, toUID string, properties interface{}) error

	// GetNode retrieves a node by UID
	GetNode(ctx context.Context, nodeType NodeType, uid string) (*Node, error)

	// DeleteNodesByTimestamp deletes nodes older than the given timestamp
	DeleteNodesByTimestamp(ctx context.Context, nodeType NodeType, timestampField string, cutoffNs int64) (int, error)

	// GetGraphStats retrieves overall graph statistics
	GetGraphStats(ctx context.Context) (*GraphStats, error)

	// InitializeSchema creates indexes and constraints
	InitializeSchema(ctx context.Context) error

	// DeleteGraph completely removes the graph (for testing purposes)
	DeleteGraph(ctx context.Context) error

	// CreateGraph creates a new named graph database
	CreateGraph(ctx context.Context, graphName string) error

	// DeleteGraphByName deletes a specific named graph database
	DeleteGraphByName(ctx context.Context, graphName string) error

	// GraphExists checks if a named graph exists
	GraphExists(ctx context.Context, graphName string) (bool, error)
}

// ClientConfig holds configuration for the FalkorDB client
type ClientConfig struct {
	Host         string        // FalkorDB host
	Port         int           // FalkorDB port
	Password     string        // optional password
	GraphName    string        // name of the graph database
	MaxRetries   int           // max connection retries
	DialTimeout  time.Duration // connection timeout
	ReadTimeout  time.Duration // read timeout
	WriteTimeout time.Duration // write timeout
	PoolSize     int           // connection pool size

	// Query cache settings
	QueryCacheEnabled  bool          // Enable query caching (default: false)
	QueryCacheMemoryMB int64         // Max cache memory in MB (default: 64)
	QueryCacheTTL      time.Duration // Cache TTL (default: 2 minutes)
}

// DefaultClientConfig returns default configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		GraphName:    "spectre",
		MaxRetries:   3,
		DialTimeout:  30 * time.Second,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		PoolSize:     10,

		// Query cache defaults
		QueryCacheEnabled:  false,
		QueryCacheMemoryMB: 64,
		QueryCacheTTL:      2 * time.Minute,
	}
}

// falkorClient implements the Client interface using FalkorDB Go client
type falkorClient struct {
	config ClientConfig
	logger *logging.Logger
	db     *falkordb.FalkorDB
	graph  *falkordb.Graph
}

// NewClient creates a new FalkorDB client, optionally with query caching
func NewClient(config ClientConfig) Client {
	client := &falkorClient{
		config: config,
		logger: logging.GetLogger("graph.client"),
	}

	// Wrap with caching if enabled
	if config.QueryCacheEnabled {
		cacheConfig := QueryCacheConfig{
			MaxMemoryMB: config.QueryCacheMemoryMB,
			TTL:         config.QueryCacheTTL,
			Enabled:     true,
		}

		cachedClient, err := NewCachedClient(client, cacheConfig, logging.GetLogger("graph.cache"))
		if err != nil {
			// Log error but continue without caching
			client.logger.Warn("Failed to create query cache, continuing without caching: %v", err)
			return client
		}
		return cachedClient
	}

	return client
}
