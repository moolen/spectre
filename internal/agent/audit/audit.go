// Package audit provides audit logging for the multi-agent incident response system.
// It captures all agent events (activations, tool calls, responses) to a JSONL file
// for debugging, analysis, and reproducibility.
package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventType represents the type of audit event.
type EventType string

const (
	// EventTypeSessionStart marks the start of a new session.
	EventTypeSessionStart EventType = "session_start"
	// EventTypeUserMessage marks a user input message.
	EventTypeUserMessage EventType = "user_message"
	// EventTypeAgentActivated marks when an agent becomes active.
	EventTypeAgentActivated EventType = "agent_activated"
	// EventTypeToolStart marks the start of a tool call.
	EventTypeToolStart EventType = "tool_start"
	// EventTypeToolComplete marks the completion of a tool call.
	EventTypeToolComplete EventType = "tool_complete"
	// EventTypeAgentText marks text output from an agent.
	EventTypeAgentText EventType = "agent_text"
	// EventTypePipelineComplete marks the completion of the agent pipeline.
	EventTypePipelineComplete EventType = "pipeline_complete"
	// EventTypeError marks an error during processing.
	EventTypeError EventType = "error"
	// EventTypeSessionEnd marks the end of a session.
	EventTypeSessionEnd EventType = "session_end"

	// === LLM Metrics Event Types ===

	// EventTypeLLMRequest logs each LLM request with token usage.
	EventTypeLLMRequest EventType = "llm_request"
	// EventTypeSessionMetrics logs aggregated session metrics.
	EventTypeSessionMetrics EventType = "session_metrics"

	// === Debug/Verbose Event Types ===

	// EventTypeEventReceived logs every raw ADK event received.
	EventTypeEventReceived EventType = "event_received"
	// EventTypeStateDelta logs state changes from an event.
	EventTypeStateDelta EventType = "state_delta"
	// EventTypeFinalResponseCheck logs IsFinalResponse() analysis.
	EventTypeFinalResponseCheck EventType = "final_response_check"
	// EventTypeUserQuestionPending logs when a user question is detected in state.
	EventTypeUserQuestionPending EventType = "user_question_pending"
	// EventTypeUserQuestionDisplayed logs when question is shown to user.
	EventTypeUserQuestionDisplayed EventType = "user_question_displayed"
	// EventTypeUserResponseReceived logs when user responds to a question.
	EventTypeUserResponseReceived EventType = "user_response_received"
	// EventTypeAgentTransfer logs when control transfers between agents.
	EventTypeAgentTransfer EventType = "agent_transfer"
	// EventTypeEscalation logs when an agent escalates.
	EventTypeEscalation EventType = "escalation"
	// EventTypeEventLoopIteration logs each iteration of the event loop.
	EventTypeEventLoopIteration EventType = "event_loop_iteration"
	// EventTypeEventLoopComplete logs when the event loop exits.
	EventTypeEventLoopComplete EventType = "event_loop_complete"
)

// Event represents a single audit log event.
type Event struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Type is the event type.
	Type EventType `json:"type"`
	// SessionID is the session identifier.
	SessionID string `json:"session_id"`
	// Agent is the name of the agent that generated the event (if applicable).
	Agent string `json:"agent,omitempty"`
	// Data contains event-specific data.
	Data map[string]interface{} `json:"data,omitempty"`
}

// Logger writes audit events to a JSONL file.
type Logger struct {
	file      *os.File
	writer    *bufio.Writer
	mutex     sync.Mutex
	sessionID string
}

// NewLogger creates a new audit logger that writes to the specified file path.
// If the file exists, new events are appended.
func NewLogger(filePath, sessionID string) (*Logger, error) {
	// filePath is user-provided configuration for audit log location
	// #nosec G304 -- Audit log path is intentionally configurable by user
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &Logger{
		file:      file,
		writer:    bufio.NewWriter(file),
		sessionID: sessionID,
	}, nil
}

// write writes an event to the audit log.
func (l *Logger) write(event Event) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	if _, err := l.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	if _, err := l.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush immediately for crash safety
	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush audit log: %w", err)
	}

	return nil
}

// LogSessionStart logs the start of a new session.
func (l *Logger) LogSessionStart(model, spectreURL string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeSessionStart,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"model":       model,
			"spectre_url": spectreURL,
		},
	})
}

// LogUserMessage logs a user input message.
func (l *Logger) LogUserMessage(message string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeUserMessage,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"message": message,
		},
	})
}

// LogAgentActivated logs when an agent becomes active.
func (l *Logger) LogAgentActivated(agentName string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeAgentActivated,
		SessionID: l.sessionID,
		Agent:     agentName,
	})
}

// LogToolStart logs the start of a tool call.
func (l *Logger) LogToolStart(agentName, toolName string, args map[string]interface{}) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeToolStart,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"tool_name": toolName,
			"args":      args,
		},
	})
}

// LogToolComplete logs the completion of a tool call.
func (l *Logger) LogToolComplete(agentName, toolName string, success bool, duration time.Duration, result interface{}) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeToolComplete,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"tool_name":   toolName,
			"success":     success,
			"duration_ms": duration.Milliseconds(),
			"result":      result,
		},
	})
}

// LogAgentText logs text output from an agent.
func (l *Logger) LogAgentText(agentName, content string, isFinal bool) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeAgentText,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"content":  content,
			"is_final": isFinal,
		},
	})
}

// LogPipelineComplete logs the completion of the agent pipeline.
func (l *Logger) LogPipelineComplete(duration time.Duration) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypePipelineComplete,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
		},
	})
}

// LogError logs an error during processing.
func (l *Logger) LogError(agentName string, err error) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeError,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"error": err.Error(),
		},
	})
}

// LogSessionEnd logs the end of a session.
func (l *Logger) LogSessionEnd() error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeSessionEnd,
		SessionID: l.sessionID,
	})
}

// === LLM Metrics Logging Methods ===

// LogLLMRequest logs an individual LLM request with token usage information.
func (l *Logger) LogLLMRequest(provider, model string, inputTokens, outputTokens int, stopReason string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeLLMRequest,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"provider":      provider,
			"model":         model,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
			"stop_reason":   stopReason,
		},
	})
}

// LogSessionMetrics logs aggregated metrics for the entire session.
func (l *Logger) LogSessionMetrics(totalRequests, totalInputTokens, totalOutputTokens int) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeSessionMetrics,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"total_llm_requests":  totalRequests,
			"total_input_tokens":  totalInputTokens,
			"total_output_tokens": totalOutputTokens,
			"total_tokens":        totalInputTokens + totalOutputTokens,
		},
	})
}

// Close closes the audit logger and flushes any pending writes.
func (l *Logger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	var errs []error

	if err := l.writer.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("failed to flush audit log: %w", err))
	}

	if err := l.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close audit log file: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing audit log: %v", errs)
	}

	return nil
}

// === Verbose Debug Logging Methods ===

// LogEventReceived logs every raw ADK event received from the runner.
func (l *Logger) LogEventReceived(eventID, author string, details map[string]interface{}) error {
	data := map[string]interface{}{
		"event_id": eventID,
		"author":   author,
	}
	for k, v := range details {
		data[k] = v
	}
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeEventReceived,
		SessionID: l.sessionID,
		Agent:     author,
		Data:      data,
	})
}

// LogStateDelta logs state changes from an event.
func (l *Logger) LogStateDelta(agentName string, keys []string, values map[string]string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeStateDelta,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"keys":   keys,
			"values": values,
		},
	})
}

// LogFinalResponseCheck logs the analysis of IsFinalResponse().
func (l *Logger) LogFinalResponseCheck(agentName string, result bool, details map[string]interface{}) error {
	data := map[string]interface{}{
		"is_final_response": result,
	}
	for k, v := range details {
		data[k] = v
	}
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeFinalResponseCheck,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data:      data,
	})
}

// LogUserQuestionPending logs when a user question is detected in state delta.
func (l *Logger) LogUserQuestionPending(agentName, question string, summaryLen int, defaultConfirm bool) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeUserQuestionPending,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"question":        truncateString(question, 200),
			"summary_length":  summaryLen,
			"default_confirm": defaultConfirm,
		},
	})
}

// LogUserQuestionDisplayed logs when the question is shown to the user.
func (l *Logger) LogUserQuestionDisplayed(agentName, mode string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeUserQuestionDisplayed,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"mode": mode,
		},
	})
}

// LogUserResponseReceived logs when user responds to a question.
func (l *Logger) LogUserResponseReceived(response string, confirmed, hasClarification bool) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeUserResponseReceived,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"response":          truncateString(response, 500),
			"confirmed":         confirmed,
			"has_clarification": hasClarification,
		},
	})
}

// LogAgentTransfer logs when control transfers between agents.
func (l *Logger) LogAgentTransfer(fromAgent, toAgent string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeAgentTransfer,
		SessionID: l.sessionID,
		Data: map[string]interface{}{
			"from_agent": fromAgent,
			"to_agent":   toAgent,
		},
	})
}

// LogEscalation logs when an agent escalates.
func (l *Logger) LogEscalation(agentName, reason string) error {
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeEscalation,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data: map[string]interface{}{
			"reason": reason,
		},
	})
}

// LogEventLoopIteration logs each iteration of the event processing loop.
func (l *Logger) LogEventLoopIteration(iteration int, agentName string, details map[string]interface{}) error {
	data := map[string]interface{}{
		"iteration": iteration,
	}
	for k, v := range details {
		data[k] = v
	}
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeEventLoopIteration,
		SessionID: l.sessionID,
		Agent:     agentName,
		Data:      data,
	})
}

// LogEventLoopComplete logs when the event processing loop exits.
func (l *Logger) LogEventLoopComplete(reason string, details map[string]interface{}) error {
	data := map[string]interface{}{
		"reason": reason,
	}
	for k, v := range details {
		data[k] = v
	}
	return l.write(Event{
		Timestamp: time.Now(),
		Type:      EventTypeEventLoopComplete,
		SessionID: l.sessionID,
		Data:      data,
	})
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
