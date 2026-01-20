package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// Connect establishes connection to FalkorDB
func (c *falkorClient) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to FalkorDB at %s:%d (graph: %s)", c.config.Host, c.config.Port, c.config.GraphName)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	// Create connection options with timeouts
	// Note: falkordb.ConnectionOption is an alias for redis.Options
	connOpts := &falkordb.ConnectionOption{
		Addr:         addr,
		Password:     c.config.Password,
		DialTimeout:  c.config.DialTimeout,
		ReadTimeout:  c.config.ReadTimeout,
		WriteTimeout: c.config.WriteTimeout,
		PoolSize:     c.config.PoolSize,
		MaxRetries:   c.config.MaxRetries,
	}

	// Create FalkorDB client
	db, err := falkordb.FalkorDBNew(connOpts)
	if err != nil {
		return fmt.Errorf("failed to create FalkorDB client: %w", err)
	}
	c.db = db

	// Select graph
	c.graph = db.SelectGraph(c.config.GraphName)

	c.logger.Info("Successfully connected to FalkorDB")
	return nil
}

// Close closes the connection
func (c *falkorClient) Close() error {
	c.logger.Info("Closing FalkorDB connection")
	if c.db != nil && c.db.Conn != nil {
		return c.db.Conn.Close()
	}
	return nil
}

// Ping checks if the connection is alive
func (c *falkorClient) Ping(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("client not connected")
	}
	// FalkorDB client doesn't have a direct Ping method, but we can execute a simple query
	_, err := c.graph.Query("RETURN 1", nil, nil)
	return err
}

// ExecuteQuery executes a Cypher query and returns results
func (c *falkorClient) ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error) {
	if c.graph == nil {
		return nil, fmt.Errorf("client not connected")
	}

	// Set query options with timeout if specified
	var options *falkordb.QueryOptions
	if query.Timeout > 0 {
		options = falkordb.NewQueryOptions().SetTimeout(query.Timeout)
	}

	// Execute query using FalkorDB client
	// The FalkorDB client handles parameter substitution internally
	startTime := time.Now()
	result, err := c.graph.Query(query.Query, query.Parameters, options)
	executionTime := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	// Convert FalkorDB QueryResult to our QueryResult format
	queryResult := convertFalkorDBResult(result)
	queryResult.Stats.ExecutionTime = executionTime

	return queryResult, nil
}

// convertFalkorDBResult converts a FalkorDB QueryResult to our QueryResult format
func convertFalkorDBResult(result *falkordb.QueryResult) *QueryResult {
	qr := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
		Stats:   QueryStats{},
	}

	// Extract rows - FalkorDB client handles all the parsing for us
	// Column names are extracted from the first record
	firstRow := true
	for result.Next() {
		record := result.Record()

		// Get column names from the first record
		if firstRow {
			qr.Columns = record.Keys()
			firstRow = false
		}

		// Get values for this row
		qr.Rows = append(qr.Rows, record.Values())
	}

	// Extract statistics from FalkorDB result
	// The FalkorDB client exposes statistics through the result object
	qr.Stats.NodesCreated = result.NodesCreated()
	qr.Stats.NodesDeleted = result.NodesDeleted()
	qr.Stats.RelationshipsCreated = result.RelationshipsCreated()
	qr.Stats.RelationshipsDeleted = result.RelationshipsDeleted()
	qr.Stats.PropertiesSet = result.PropertiesSet()
	qr.Stats.LabelsAdded = result.LabelsAdded()

	return qr
}

// CreateNode creates a node in the graph
func (c *falkorClient) CreateNode(ctx context.Context, nodeType NodeType, properties interface{}) error {
	// Convert properties to JSON for storage
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("failed to marshal node properties: %w", err)
	}

	var propsMap map[string]interface{}
	if err := json.Unmarshal(propsJSON, &propsMap); err != nil {
		return fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	// Build Cypher CREATE statement
	propsStr := buildPropertiesString(propsMap)
	cypherQuery := fmt.Sprintf("CREATE (n:%s %s)", nodeType, propsStr)

	query := GraphQuery{
		Query:      cypherQuery,
		Parameters: nil,
	}

	_, err = c.ExecuteQuery(ctx, query)
	return err
}

// CreateEdge creates an edge between two nodes
func (c *falkorClient) CreateEdge(ctx context.Context, edgeType EdgeType, fromUID, toUID string, properties interface{}) error {
	// Convert properties to JSON
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("failed to marshal edge properties: %w", err)
	}

	var propsMap map[string]interface{}
	if err := json.Unmarshal(propsJSON, &propsMap); err != nil {
		return fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	// Build Cypher MATCH + CREATE statement
	propsStr := buildPropertiesString(propsMap)
	cypherQuery := fmt.Sprintf(
		"MATCH (a {uid: '%s'}), (b {uid: '%s'}) CREATE (a)-[r:%s %s]->(b)",
		escapeCypherString(fromUID),
		escapeCypherString(toUID),
		edgeType,
		propsStr,
	)

	query := GraphQuery{
		Query:      cypherQuery,
		Parameters: nil,
	}

	_, err = c.ExecuteQuery(ctx, query)
	return err
}

// GetNode retrieves a node by UID
func (c *falkorClient) GetNode(ctx context.Context, nodeType NodeType, uid string) (*Node, error) {
	cypherQuery := fmt.Sprintf(
		"MATCH (n:%s {uid: '%s'}) RETURN n",
		nodeType,
		escapeCypherString(uid),
	)

	query := GraphQuery{
		Query:      cypherQuery,
		Parameters: nil,
	}

	result, err := c.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("node not found: %s/%s", nodeType, uid)
	}

	// Parse the node from the first row
	if len(result.Rows[0]) == 0 {
		return nil, fmt.Errorf("empty result row")
	}

	// Extract node properties
	nodeProps, err := ParseNodeFromResult(result.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse node: %w", err)
	}

	// Marshal properties to JSON
	propsJSON, err := json.Marshal(nodeProps)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal properties: %w", err)
	}

	return &Node{
		Type:       nodeType,
		Properties: json.RawMessage(propsJSON),
	}, nil
}

// DeleteNodesByTimestamp deletes nodes older than the given timestamp
func (c *falkorClient) DeleteNodesByTimestamp(ctx context.Context, nodeType NodeType, timestampField string, cutoffNs int64) (int, error) {
	cypherQuery := fmt.Sprintf(
		"MATCH (n:%s) WHERE n.%s < %d DETACH DELETE n",
		nodeType,
		timestampField,
		cutoffNs,
	)

	query := GraphQuery{
		Query:      cypherQuery,
		Parameters: nil,
	}

	result, err := c.ExecuteQuery(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.Stats.NodesDeleted, nil
}

// GetGraphStats retrieves overall graph statistics
func (c *falkorClient) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	// Query node counts by type
	nodeCountQuery := `
		MATCH (n)
		RETURN labels(n)[0] as type, count(n) as count
	`

	nodeResult, err := c.ExecuteQuery(ctx, GraphQuery{Query: nodeCountQuery})
	if err != nil {
		return nil, fmt.Errorf("failed to query node counts: %w", err)
	}

	// Query edge counts by type
	edgeCountQuery := `
		MATCH ()-[r]->()
		RETURN type(r) as type, count(r) as count
	`

	edgeResult, err := c.ExecuteQuery(ctx, GraphQuery{Query: edgeCountQuery})
	if err != nil {
		return nil, fmt.Errorf("failed to query edge counts: %w", err)
	}

	// Query timestamp range for ChangeEvent nodes
	timestampQuery := `
		MATCH (e:ChangeEvent)
		RETURN min(e.timestamp) as oldest, max(e.timestamp) as newest
	`

	timestampResult, err := c.ExecuteQuery(ctx, GraphQuery{Query: timestampQuery})
	if err != nil {
		return nil, fmt.Errorf("failed to query timestamps: %w", err)
	}

	// Parse results into GraphStats
	stats := &GraphStats{
		NodesByType: make(map[NodeType]int),
		EdgesByType: make(map[EdgeType]int),
	}

	// Parse node counts
	// Expected format: [["ResourceIdentity", 50], ["ChangeEvent", 100], ...]
	for _, row := range nodeResult.Rows {
		if len(row) >= 2 {
			if nodeType, ok := row[0].(string); ok {
				switch count := row[1].(type) {
				case int64:
					stats.NodesByType[NodeType(nodeType)] = int(count)
					stats.NodeCount += int(count)
				case float64:
					stats.NodesByType[NodeType(nodeType)] = int(count)
					stats.NodeCount += int(count)
				}
			}
		}
	}

	// Parse edge counts
	// Expected format: [["OWNS", 30], ["CHANGED", 100], ...]
	for _, row := range edgeResult.Rows {
		if len(row) >= 2 {
			if edgeType, ok := row[0].(string); ok {
				switch count := row[1].(type) {
				case int64:
					stats.EdgesByType[EdgeType(edgeType)] = int(count)
					stats.EdgeCount += int(count)
				case float64:
					stats.EdgesByType[EdgeType(edgeType)] = int(count)
					stats.EdgeCount += int(count)
				}
			}
		}
	}

	// Parse timestamps
	// Expected format: [[oldest_timestamp, newest_timestamp]]
	if len(timestampResult.Rows) > 0 && len(timestampResult.Rows[0]) >= 2 {
		switch oldest := timestampResult.Rows[0][0].(type) {
		case int64:
			stats.OldestTimestamp = oldest
		case float64:
			stats.OldestTimestamp = int64(oldest)
		}

		switch newest := timestampResult.Rows[0][1].(type) {
		case int64:
			stats.NewestTimestamp = newest
		case float64:
			stats.NewestTimestamp = int64(newest)
		}
	}

	c.logger.Debug("Graph stats: %d nodes, %d edges (oldest: %d, newest: %d)",
		stats.NodeCount, stats.EdgeCount, stats.OldestTimestamp, stats.NewestTimestamp)

	return stats, nil
}

// InitializeSchema creates indexes and constraints
func (c *falkorClient) InitializeSchema(ctx context.Context) error {
	c.logger.Info("Initializing graph schema for graph: %s", c.config.GraphName)

	// FalkorDB indexes are created using:
	// GRAPH.QUERY graphName "CREATE INDEX ON :Label(property)"
	//
	// Key indexes needed:
	// 1. ResourceIdentity.uid (primary lookup)
	// 2. ChangeEvent.timestamp (time-range queries)
	// 3. ChangeEvent.id (idempotency)
	// 4. K8sEvent.timestamp
	// 5. Composite indexes for namespace graph queries

	indexes := []string{
		// Primary indexes
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.uid)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.kind)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.namespace)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.deleted)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.firstSeen)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.id)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.timestamp)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.status)",
		"CREATE INDEX FOR (n:K8sEvent) ON (n.timestamp)",
	}

	for _, indexQuery := range indexes {
		_, err := c.ExecuteQuery(ctx, GraphQuery{Query: indexQuery})
		if err != nil {
			// FalkorDB may return error if index already exists, log but continue
			c.logger.Warn("Failed to create index (may already exist): %v", err)
		}
	}

	c.logger.Info("Schema initialization complete")
	return nil
}

// DeleteGraph completely removes the graph (for testing purposes)
func (c *falkorClient) DeleteGraph(ctx context.Context) error {
	if c.graph == nil {
		return fmt.Errorf("client not connected")
	}

	// Use GRAPH.DELETE command to completely remove the graph
	err := c.graph.Delete()
	if err != nil {
		// Ignore "empty key" error which means graph doesn't exist yet
		if strings.Contains(err.Error(), "empty key") {
			c.logger.Debug("Graph '%s' does not exist, nothing to delete", c.config.GraphName)
		} else {
			return fmt.Errorf("failed to delete graph: %w", err)
		}
	} else {
		c.logger.Info("Graph '%s' deleted", c.config.GraphName)
	}

	// Re-select the graph (it will be recreated on next operation)
	c.graph = c.db.SelectGraph(c.config.GraphName)

	return nil
}

// Helper functions

// buildPropertiesString converts a map to Cypher property syntax
// Example: {name: "foo", age: 30} -> {name: 'foo', age: 30}
func buildPropertiesString(props map[string]interface{}) string {
	if len(props) == 0 {
		return ""
	}

	parts := make([]string, 0, len(props))
	for key, value := range props {
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = fmt.Sprintf("'%s'", escapeCypherString(v))
		case bool:
			valueStr = fmt.Sprintf("%t", v)
		case int, int64, float64:
			valueStr = fmt.Sprintf("%v", v)
		case []string:
			// Array of strings
			escaped := make([]string, len(v))
			for i, s := range v {
				escaped[i] = fmt.Sprintf("'%s'", escapeCypherString(s))
			}
			valueStr = fmt.Sprintf("[%s]", strings.Join(escaped, ", "))
		default:
			// For complex types, marshal to JSON string
			jsonBytes, _ := json.Marshal(v)
			valueStr = fmt.Sprintf("'%s'", escapeCypherString(string(jsonBytes)))
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, valueStr))
	}

	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

// escapeCypherString escapes single quotes in Cypher strings
func escapeCypherString(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

// replaceCypherParameters replaces $param placeholders with actual values
// It sorts parameters by length (longest first) to avoid partial replacements
// where one parameter name is a prefix of another (e.g., $deleted and $deletedAt)
func replaceCypherParameters(query string, params map[string]interface{}) string {
	result := query

	// Sort parameter keys by length (longest first) to avoid prefix collision
	// This ensures $deletedAt is processed before $deleted
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}

	// Sort by length descending, then alphabetically for stability
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) || (len(keys[j]) == len(keys[i]) && keys[j] < keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, key := range keys {
		value := params[key]
		placeholder := "$" + key
		var replacement string

		switch v := value.(type) {
		case string:
			replacement = fmt.Sprintf("'%s'", escapeCypherString(v))
		case bool:
			replacement = fmt.Sprintf("%t", v)
		case int:
			replacement = fmt.Sprintf("%d", v)
		case int64:
			replacement = fmt.Sprintf("%d", v)
		case float64:
			replacement = fmt.Sprintf("%f", v)
		case []string:
			escaped := make([]string, len(v))
			for i, s := range v {
				escaped[i] = fmt.Sprintf("'%s'", escapeCypherString(s))
			}
			replacement = fmt.Sprintf("[%s]", strings.Join(escaped, ", "))
		default:
			// For complex types, marshal to JSON string
			jsonBytes, _ := json.Marshal(v)
			replacement = fmt.Sprintf("'%s'", escapeCypherString(string(jsonBytes)))
		}

		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result
}

// parseGraphQueryResult parses the result from GRAPH.QUERY command
// FalkorDB returns results in a specific format:
// [
//
//	[header1, header2, ...],           // Column names
//	[row1_col1, row1_col2, ...],      // Data rows
//	[row2_col1, row2_col2, ...],
//	[statistics]                       // Query statistics
//
// ]
func parseGraphQueryResult(result interface{}) (*QueryResult, error) {
	// FalkorDB returns an array
	resultArray, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	queryResult := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
		Stats:   QueryStats{},
	}

	if len(resultArray) == 0 {
		return queryResult, nil
	}

	// First element is column headers (if query returns data)
	// Last element is always statistics
	// Everything in between is data rows

	// Parse columns (first element, if exists and is not stats)
	if len(resultArray) > 0 {
		if columns, ok := resultArray[0].([]interface{}); ok {
			queryResult.Columns = make([]string, len(columns))
			for i, col := range columns {
				if colStr, ok := col.(string); ok {
					queryResult.Columns[i] = colStr
				}
			}
		}
	}

	// Parse rows (all elements except first and last)
	// When resultArray has exactly 2 elements, it's just [columns, stats] with no data rows
	// When resultArray has 3+ elements, it's [columns, row1, ..., rowN, stats]
	// Note: FalkorDB sometimes returns [columns, [], stats] for empty results
	if len(resultArray) > 2 {
		for i := 1; i < len(resultArray)-1; i++ {
			if row, ok := resultArray[i].([]interface{}); ok {
				// Skip empty rows
				if len(row) > 0 {
					queryResult.Rows = append(queryResult.Rows, row)
				}
			}
		}
	}

	// Parse statistics (last element)
	if len(resultArray) > 0 {
		if statsArray, ok := resultArray[len(resultArray)-1].([]interface{}); ok {
			queryResult.Stats = parseQueryStats(statsArray)
		}
	}

	return queryResult, nil
}

// parseQueryStats extracts statistics from the stats array
func parseQueryStats(statsArray []interface{}) QueryStats {
	stats := QueryStats{}

	// FalkorDB returns stats as an array of strings like:
	// ["Labels added: 1", "Nodes created: 1", "Properties set: 5", "Query internal execution time: 0.234 milliseconds"]
	for _, stat := range statsArray {
		if statStr, ok := stat.(string); ok {
			// Parse different stat types
			if strings.Contains(statStr, "Nodes created:") {
				_, _ = fmt.Sscanf(statStr, "Nodes created: %d", &stats.NodesCreated)
			} else if strings.Contains(statStr, "Nodes deleted:") {
				_, _ = fmt.Sscanf(statStr, "Nodes deleted: %d", &stats.NodesDeleted)
			} else if strings.Contains(statStr, "Relationships created:") {
				_, _ = fmt.Sscanf(statStr, "Relationships created: %d", &stats.RelationshipsCreated)
			} else if strings.Contains(statStr, "Relationships deleted:") {
				_, _ = fmt.Sscanf(statStr, "Relationships deleted: %d", &stats.RelationshipsDeleted)
			} else if strings.Contains(statStr, "Properties set:") {
				_, _ = fmt.Sscanf(statStr, "Properties set: %d", &stats.PropertiesSet)
			} else if strings.Contains(statStr, "Labels added:") {
				_, _ = fmt.Sscanf(statStr, "Labels added: %d", &stats.LabelsAdded)
			}
		}
	}

	return stats
}
