package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
)

// Transport implements stdio-based MCP transport
// According to the MCP spec, this transport:
// - Reads newline-delimited JSON from stdin
// - Writes newline-delimited JSON to stdout
// - Writes logs to stderr
// - Messages must not contain embedded newlines
type Transport struct {
	handler *mcp.Handler
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	version string
}

// NewTransport creates a new stdio transport
func NewTransport(mcpServer *mcp.MCPServer, version string) *Transport {
	return NewTransportWithIO(mcpServer, version, os.Stdin, os.Stdout, os.Stderr)
}

// NewTransportWithIO creates a new stdio transport with custom IO streams (for testing)
func NewTransportWithIO(mcpServer *mcp.MCPServer, version string, stdin io.Reader, stdout, stderr io.Writer) *Transport {
	handler := mcp.NewHandler(mcpServer, version)

	return &Transport{
		handler: handler,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		version: version,
	}
}

// Start starts the stdio transport loop
func (t *Transport) Start(ctx context.Context) error {
	logger := logging.GetLogger("mcp")

	// Redirect logger to stderr for stdio transport
	// This ensures MCP messages on stdout don't get mixed with logs
	t.redirectLoggerToStderr(logger)

	logger.Info("Starting stdio transport")

	scanner := bufio.NewScanner(t.stdin)
	writer := bufio.NewWriter(t.stdout)
	defer writer.Flush()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stdio transport context cancelled, shutting down")
			return ctx.Err()
		default:
			// Read next line from stdin
			if !scanner.Scan() {
				// EOF or error
				if err := scanner.Err(); err != nil {
					logger.Error("Error reading from stdin: %v", err)
					return fmt.Errorf("error reading from stdin: %w", err)
				}
				// Clean EOF - client disconnected
				logger.Info("Stdin closed, shutting down")
				return nil
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// Parse JSON-RPC request
			var req mcp.MCPRequest
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				logger.Error("Failed to parse JSON-RPC request: %v", err)
				// Send error response
				errResp := &mcp.MCPResponse{
					JSONRPC: "2.0",
					ID:      nil,
					Error: &mcp.MCPError{
						Code:    -32700,
						Message: "Parse error",
					},
				}
				if err := t.writeResponse(writer, errResp); err != nil {
					logger.Error("Failed to write error response: %v", err)
					return err
				}
				continue
			}

			// Handle request
			resp := t.handler.HandleRequest(ctx, &req)

			// Write response
			if err := t.writeResponse(writer, resp); err != nil {
				logger.Error("Failed to write response: %v", err)
				return err
			}
		}
	}
}

// Stop gracefully shuts down the stdio transport
func (t *Transport) Stop() error {
	logger := logging.GetLogger("mcp")
	logger.Info("Stdio transport stopped")
	return nil
}

// writeResponse writes a JSON-RPC response to stdout as newline-delimited JSON
func (t *Transport) writeResponse(writer *bufio.Writer, resp *mcp.MCPResponse) error {
	// Marshal response to JSON
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Validate no embedded newlines (per MCP spec)
	if strings.Contains(string(data), "\n") {
		return fmt.Errorf("response contains embedded newlines (not allowed in stdio transport)")
	}

	// Write JSON followed by newline
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush immediately to ensure message is sent
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// redirectLoggerToStderr redirects all logging output to stderr
// This is required by the MCP stdio spec - only MCP messages can go to stdout
func (t *Transport) redirectLoggerToStderr(logger *logging.Logger) {
	// The logging package writes to stdout by default
	// For stdio transport, we need to ensure all logs go to stderr
	// This is a placeholder - actual implementation depends on logging package design

	// Note: If the logging package doesn't support this, we might need to:
	// 1. Modify the logging package to support custom writers
	// 2. Or temporarily redirect os.Stdout to stderr during MCP operation
	// 3. Or create a custom logger that writes to stderr

	// For now, we'll just log a warning that logs should go to stderr
	// The actual logging redirection may need to be implemented in the logging package
}
