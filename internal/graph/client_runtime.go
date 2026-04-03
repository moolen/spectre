package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/FalkorDB/falkordb-go/v2"
)

// Connect establishes connection to FalkorDB.
func (c *falkorClient) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to FalkorDB at %s:%d (graph: %s)", c.config.Host, c.config.Port, c.config.GraphName)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	// Note: falkordb.ConnectionOption is an alias for redis.Options.
	connOpts := &falkordb.ConnectionOption{
		Addr:         addr,
		Password:     c.config.Password,
		DialTimeout:  c.config.DialTimeout,
		ReadTimeout:  c.config.ReadTimeout,
		WriteTimeout: c.config.WriteTimeout,
		PoolSize:     c.config.PoolSize,
		MaxRetries:   c.config.MaxRetries,
	}

	db, err := falkordb.FalkorDBNew(connOpts)
	if err != nil {
		return fmt.Errorf("failed to create FalkorDB client: %w", err)
	}

	c.db = db
	c.graph = db.SelectGraph(c.config.GraphName)

	c.logger.Info("Successfully connected to FalkorDB")
	return nil
}

// Close closes the connection.
func (c *falkorClient) Close() error {
	c.logger.Info("Closing FalkorDB connection")
	if c.db != nil && c.db.Conn != nil {
		return c.db.Conn.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (c *falkorClient) Ping(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("client not connected")
	}

	_, err := c.graph.Query("RETURN 1", nil, nil)
	return err
}

// ExecuteQuery executes a Cypher query and returns results.
func (c *falkorClient) ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error) {
	if c.graph == nil {
		return nil, fmt.Errorf("client not connected")
	}

	var options *falkordb.QueryOptions
	if query.Timeout > 0 {
		options = falkordb.NewQueryOptions().SetTimeout(query.Timeout)
	}

	startTime := time.Now()
	result, err := c.graph.Query(query.Query, query.Parameters, options)
	executionTime := time.Since(startTime)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	queryResult := convertFalkorDBResult(result)
	queryResult.Stats.ExecutionTime = executionTime

	return queryResult, nil
}

func convertFalkorDBResult(result *falkordb.QueryResult) *QueryResult {
	qr := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
		Stats:   QueryStats{},
	}

	firstRow := true
	for result.Next() {
		record := result.Record()
		if firstRow {
			qr.Columns = record.Keys()
			firstRow = false
		}
		qr.Rows = append(qr.Rows, record.Values())
	}

	qr.Stats.NodesCreated = result.NodesCreated()
	qr.Stats.NodesDeleted = result.NodesDeleted()
	qr.Stats.RelationshipsCreated = result.RelationshipsCreated()
	qr.Stats.RelationshipsDeleted = result.RelationshipsDeleted()
	qr.Stats.PropertiesSet = result.PropertiesSet()
	qr.Stats.LabelsAdded = result.LabelsAdded()

	return qr
}
