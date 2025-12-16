// Package helpers provides MCP subprocess utilities for e2e testing.
package helpers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// MCPSubprocess manages an MCP server running as a subprocess
type MCPSubprocess struct {
	t       *testing.T
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	closed  bool
}

// MCPSubprocessRequest represents a JSON-RPC request to send via stdio
type MCPSubprocessRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPSubprocessResponse represents a JSON-RPC response received via stdio
type MCPSubprocessResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPSubprocessError    `json:"error,omitempty"`
}

// MCPSubprocessError represents a JSON-RPC error
type MCPSubprocessError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// StartMCPSubprocess starts an MCP server as a subprocess with stdio transport
func StartMCPSubprocess(t *testing.T, spectreBinary, spectreURL string) (*MCPSubprocess, error) {
	t.Helper()

	t.Logf("Starting MCP subprocess: %s mcp --transport stdio --spectre-url %s", spectreBinary, spectreURL)

	cmd := exec.Command(spectreBinary, "mcp", "--transport", "stdio", "--spectre-url", spectreURL)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	subprocess := &MCPSubprocess{
		t:       t,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		scanner: bufio.NewScanner(stdout),
	}

	// Start stderr logger
	go subprocess.logStderr()

	t.Logf("✓ MCP subprocess started (PID: %d)", cmd.Process.Pid)

	return subprocess, nil
}

// SendRequest sends a JSON-RPC request and returns the response
func (s *MCPSubprocess) SendRequest(ctx context.Context, method string, params map[string]interface{}) (*MCPSubprocessResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("subprocess is closed")
	}

	reqID := time.Now().UnixNano()

	req := MCPSubprocessRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request (newline-delimited JSON)
	if _, err := s.stdin.Write(reqData); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	if _, err := s.stdin.Write([]byte("\n")); err != nil {
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}

	s.t.Logf("→ Sent: %s", method)

	// Read response
	responseCh := make(chan *MCPSubprocessResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				errCh <- fmt.Errorf("scanner error: %w", err)
			} else {
				errCh <- fmt.Errorf("unexpected EOF")
			}
			return
		}

		line := s.scanner.Text()
		s.t.Logf("← Received: %s", truncate(line, 100))

		var resp MCPSubprocessResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			errCh <- fmt.Errorf("failed to unmarshal response: %w", err)
			return
		}

		responseCh <- &resp
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	case err := <-errCh:
		return nil, err
	case resp := <-responseCh:
		if resp.Error != nil {
			return resp, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	}
}

// Initialize sends an initialize request
func (s *MCPSubprocess) Initialize(ctx context.Context) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]interface{}{
			"name":    "spectre-test-client",
			"version": "1.0.0",
		},
	}

	resp, err := s.SendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// ListTools requests the list of available tools
func (s *MCPSubprocess) ListTools(ctx context.Context) ([]interface{}, error) {
	resp, err := s.SendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	tools, ok := resp.Result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected tools format in response")
	}

	return tools, nil
}

// CallTool calls a tool with the given name and arguments
func (s *MCPSubprocess) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	resp, err := s.SendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// ListPrompts requests the list of available prompts
func (s *MCPSubprocess) ListPrompts(ctx context.Context) ([]interface{}, error) {
	resp, err := s.SendRequest(ctx, "prompts/list", nil)
	if err != nil {
		return nil, err
	}

	prompts, ok := resp.Result["prompts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected prompts format in response")
	}

	return prompts, nil
}

// GetPrompt gets a prompt by name with the given arguments
func (s *MCPSubprocess) GetPrompt(ctx context.Context, promptName string, args map[string]interface{}) (map[string]interface{}, error) {
	// Convert all argument values to strings as required by MCP protocol
	// (mcp-go's GetPromptParams expects map[string]string)
	stringArgs := make(map[string]string)
	for k, v := range args {
		stringArgs[k] = fmt.Sprintf("%v", v)
	}

	params := map[string]interface{}{
		"name":      promptName,
		"arguments": stringArgs,
	}

	resp, err := s.SendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// Close closes the subprocess
func (s *MCPSubprocess) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Close stdin to signal EOF to the subprocess
	if err := s.stdin.Close(); err != nil {
		s.t.Logf("Warning: failed to close stdin: %v", err)
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			s.t.Logf("Warning: subprocess exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		s.t.Logf("Warning: subprocess did not exit, killing it")
		if err := s.cmd.Process.Kill(); err != nil {
			s.t.Logf("Warning: failed to kill process: %v", err)
		}
	}

	s.t.Logf("✓ MCP subprocess closed")
	return nil
}

// logStderr logs stderr output from the subprocess
func (s *MCPSubprocess) logStderr() {
	scanner := bufio.NewScanner(s.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		s.t.Logf("[stderr] %s", line)
	}
	if err := scanner.Err(); err != nil {
		s.t.Logf("Warning: stderr scanner error: %v", err)
	}
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BuildSpectreBinary builds the spectre binary for testing
func BuildSpectreBinary(t *testing.T) (string, error) {
	t.Helper()

	repoRoot, err := DetectRepoRoot()
	if err != nil {
		return "", err
	}

	binaryPath := repoRoot + "/bin/spectre-test"

	t.Logf("Building spectre binary: %s", binaryPath)

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/spectre")
	cmd.Dir = repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build binary: %w\n%s", err, string(output))
	}

	t.Logf("✓ Spectre binary built: %s", binaryPath)

	return binaryPath, nil
}

// WaitForMCPReady waits for the MCP subprocess to be ready by sending a ping
func WaitForMCPReady(ctx context.Context, subprocess *MCPSubprocess, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for MCP to be ready: %w", ctx.Err())
		case <-ticker.C:
			// Try to ping
			_, err := subprocess.SendRequest(ctx, "ping", nil)
			if err == nil {
				subprocess.t.Logf("✓ MCP subprocess is ready")
				return nil
			}
			// If error contains "unexpected EOF", subprocess may not be ready yet
			if strings.Contains(err.Error(), "EOF") {
				continue
			}
			// Other errors are likely fatal
			return fmt.Errorf("ping failed: %w", err)
		}
	}
}
