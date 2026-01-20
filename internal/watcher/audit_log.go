package watcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// AuditLogWriter interface for writing events to an audit log
type AuditLogWriter interface {
	// WriteEvent writes an event to the audit log
	WriteEvent(event *models.Event) error
	// Close closes the audit log writer and flushes any pending writes
	Close() error
}

// FileAuditLogWriter implements AuditLogWriter using a JSONL file
type FileAuditLogWriter struct {
	file   *os.File
	writer *bufio.Writer
	mutex  sync.Mutex
	logger *logging.Logger
}

// NewFileAuditLogWriter creates a new file-based audit log writer
func NewFileAuditLogWriter(filePath string) (*FileAuditLogWriter, error) {
	// Open file for appending (create if doesn't exist)
	// Use restrictive permissions for audit log file
	// filePath is user-provided configuration for watcher audit log
	// #nosec G304 -- Audit log path is intentionally configurable by user
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	writer := &FileAuditLogWriter{
		file:   file,
		writer: bufio.NewWriter(file),
		logger: logging.GetLogger("audit_log"),
	}

	return writer, nil
}

// WriteEvent writes an event to the audit log in JSONL format
func (w *FileAuditLogWriter) WriteEvent(event *models.Event) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Marshal event to JSON
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	// Write JSON line
	if _, err := w.writer.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write event to audit log: %w", err)
	}

	// Write newline
	if err := w.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline to audit log: %w", err)
	}

	// Flush immediately for crash safety
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush audit log: %w", err)
	}

	return nil
}

// Close closes the audit log writer and flushes any pending writes
func (w *FileAuditLogWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	var errs []error

	// Flush any remaining data
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("failed to flush audit log: %w", err))
		}
	}

	// Close file
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close audit log file: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing audit log: %v", errs)
	}

	return nil
}

