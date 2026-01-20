// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// MockInputServer listens for external input to control the mock LLM in interactive mode.
// It runs a simple TCP server that accepts JSON messages to inject LLM responses.
type MockInputServer struct {
	port     int
	listener net.Listener
	inputCh  chan *InteractiveInput
	errCh    chan error

	mu      sync.Mutex
	started bool
	closed  bool
}

// InteractiveInput is sent from the CLI client to inject mock LLM responses.
type InteractiveInput struct {
	// Text is the text response from the agent.
	Text string `json:"text,omitempty"`

	// ToolCalls defines tool calls the mock LLM will make.
	ToolCalls []MockToolCall `json:"tool_calls,omitempty"`
}

// NewMockInputServer creates a new mock input server on the specified port.
// If port is 0, a random available port will be assigned.
func NewMockInputServer(port int) (*MockInputServer, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Get the actual port (in case port was 0)
	actualPort := listener.Addr().(*net.TCPAddr).Port

	return &MockInputServer{
		port:     actualPort,
		listener: listener,
		inputCh:  make(chan *InteractiveInput, 10),
		errCh:    make(chan error, 1),
	}, nil
}

// Port returns the port the server is listening on.
func (s *MockInputServer) Port() int {
	return s.port
}

// Address returns the full address the server is listening on.
func (s *MockInputServer) Address() string {
	return fmt.Sprintf("127.0.0.1:%d", s.port)
}

// Start begins accepting connections in the background.
// Call this in a goroutine.
func (s *MockInputServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := s.listener.Accept()
			if err != nil {
				s.mu.Lock()
				if s.closed {
					s.mu.Unlock()
					return
				}
				s.mu.Unlock()
				// Log error but continue accepting
				continue
			}

			// Handle connection in a goroutine
			go s.handleConnection(ctx, conn)
		}
	}()

	return nil
}

// handleConnection processes a single client connection.
func (s *MockInputServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var input InteractiveInput
		if err := json.Unmarshal([]byte(line), &input); err != nil {
			// Send error response back to client
			errResp := map[string]string{"error": fmt.Sprintf("invalid JSON: %v", err)}
			errJSON, _ := json.Marshal(errResp)
			_, _ = fmt.Fprintf(conn, "%s\n", errJSON)
			continue
		}

		// Validate input
		if input.Text == "" && len(input.ToolCalls) == 0 {
			errResp := map[string]string{"error": "input must have either 'text' or 'tool_calls'"}
			errJSON, _ := json.Marshal(errResp)
			_, _ = fmt.Fprintf(conn, "%s\n", errJSON)
			continue
		}

		// Send to input channel
		select {
		case s.inputCh <- &input:
			// Send success response
			okResp := map[string]string{"status": "ok", "message": "input queued"}
			okJSON, _ := json.Marshal(okResp)
			_, _ = fmt.Fprintf(conn, "%s\n", okJSON)
		case <-ctx.Done():
			return
		default:
			// Channel full
			errResp := map[string]string{"error": "input queue full, try again"}
			errJSON, _ := json.Marshal(errResp)
			_, _ = fmt.Fprintf(conn, "%s\n", errJSON)
		}
	}
}

// WaitForInput blocks until input is received from an external client.
func (s *MockInputServer) WaitForInput(ctx context.Context) (*InteractiveInput, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case input := <-s.inputCh:
		return input, nil
	}
}

// SendInput sends input directly (for testing purposes).
func (s *MockInputServer) SendInput(input *InteractiveInput) error {
	select {
	case s.inputCh <- input:
		return nil
	default:
		return fmt.Errorf("input queue full")
	}
}

// Close shuts down the server.
func (s *MockInputServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	close(s.inputCh)
	return s.listener.Close()
}

// MockInputClient is a simple client for sending input to a MockInputServer.
type MockInputClient struct {
	address string
}

// NewMockInputClient creates a client that connects to the mock input server.
func NewMockInputClient(address string) *MockInputClient {
	return &MockInputClient{address: address}
}

// NewMockInputClientWithPort creates a client from a port number.
func NewMockInputClientWithPort(port int) *MockInputClient {
	return &MockInputClient{address: fmt.Sprintf("127.0.0.1:%d", port)}
}

// SendText sends a text response to the mock LLM.
func (c *MockInputClient) SendText(text string) (*ClientResponse, error) {
	return c.Send(&InteractiveInput{Text: text})
}

// SendToolCall sends a tool call to the mock LLM.
func (c *MockInputClient) SendToolCall(name string, args map[string]interface{}) (*ClientResponse, error) {
	return c.Send(&InteractiveInput{
		ToolCalls: []MockToolCall{{Name: name, Args: args}},
	})
}

// SendTextAndToolCalls sends both text and tool calls.
func (c *MockInputClient) SendTextAndToolCalls(text string, toolCalls []MockToolCall) (*ClientResponse, error) {
	return c.Send(&InteractiveInput{Text: text, ToolCalls: toolCalls})
}

// Send sends an arbitrary input to the mock LLM.
func (c *MockInputClient) Send(input *InteractiveInput) (*ClientResponse, error) {
	conn, err := net.Dial("tcp", c.address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", c.address, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Send JSON
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	_, err = fmt.Fprintf(conn, "%s\n", data)
	if err != nil {
		return nil, fmt.Errorf("failed to send input: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		var resp ClientResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &resp, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return nil, fmt.Errorf("no response received")
}

// ClientResponse is the response from the mock input server.
type ClientResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsOK returns true if the request was successful.
func (r *ClientResponse) IsOK() bool {
	return r.Status == "ok"
}
