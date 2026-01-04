package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test helper: captureLogOutput captures log output for testing
type logCapture struct {
	mu     sync.Mutex
	output []string
	stderr []string
}

func newLogCapture() *logCapture {
	return &logCapture{
		output: make([]string, 0),
		stderr: make([]string, 0),
	}
}

func (lc *logCapture) captureStdout(w io.Writer) func() {
	old := log.Writer()
	log.SetOutput(w)
	return func() {
		log.SetOutput(old)
	}
}

func (lc *logCapture) String() string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return strings.Join(lc.output, "\n")
}

func (lc *logCapture) Lines() []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	result := make([]string, len(lc.output))
	copy(result, lc.output)
	return result
}

func (lc *logCapture) StderrLines() []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	result := make([]string, len(lc.stderr))
	copy(result, lc.stderr)
	return result
}

func (lc *logCapture) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.output = lc.output[:0]
	lc.stderr = lc.stderr[:0]
}

// captureOutput captures both stdout and stderr during test execution
func captureOutput(f func()) (stdout, stderr string) {
	// Capture stdout via log package
	oldLogWriter := log.Writer()
	defer log.SetOutput(oldLogWriter)

	var stdoutBuf bytes.Buffer
	log.SetOutput(&stdoutBuf)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run the function
	f()

	// Restore stderr and read captured content
	w.Close()
	os.Stderr = oldStderr
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, r)

	return stdoutBuf.String(), stderrBuf.String()
}

// resetGlobalLogger resets global logger state for test isolation
func resetGlobalLogger() {
	globalLogger = nil
	initOnce = sync.Once{}
}

// TestInitialize tests logger initialization with various levels
func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel LogLevel
	}{
		{"debug level", "debug", DEBUG},
		{"info level", "info", INFO},
		{"warn level", "warn", WARN},
		{"error level", "error", ERROR},
		{"fatal level", "fatal", FATAL},
		{"uppercase debug", "DEBUG", DEBUG},
		{"uppercase info", "INFO", INFO},
		{"mixed case", "WaRn", WARN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalLogger()
			Initialize(tt.level)

			if globalLogger == nil {
				t.Fatal("globalLogger is nil after Initialize")
			}

			if globalLogger.level != tt.wantLevel {
				t.Errorf("Initialize(%q) level = %v, want %v", tt.level, globalLogger.level, tt.wantLevel)
			}

			if globalLogger.name != "spectre" {
				t.Errorf("Initialize(%q) name = %q, want %q", tt.level, globalLogger.name, "spectre")
			}
		})
	}
}

// TestInitializeInvalidLevel tests that invalid levels default to INFO
func TestInitializeInvalidLevel(t *testing.T) {
	resetGlobalLogger()
	Initialize("invalid")

	if globalLogger == nil {
		t.Fatal("globalLogger is nil after Initialize with invalid level")
	}

	// Current implementation defaults to INFO for invalid levels
	if globalLogger.level != INFO {
		t.Errorf("Initialize with invalid level = %v, want %v (default)", globalLogger.level, INFO)
	}
}

// TestGetLogger tests logger creation
func TestGetLogger(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	logger := GetLogger("test-component")

	if logger == nil {
		t.Fatal("GetLogger returned nil")
	}

	if logger.name != "test-component" {
		t.Errorf("GetLogger name = %q, want %q", logger.name, "test-component")
	}

	if logger.level != INFO {
		t.Errorf("GetLogger level = %v, want %v", logger.level, INFO)
	}

	if logger.fields == nil {
		t.Error("GetLogger fields map is nil")
	}
}

// TestGetLoggerLazyInit tests automatic initialization
func TestGetLoggerLazyInit(t *testing.T) {
	resetGlobalLogger()

	// Don't call Initialize - should auto-initialize
	logger := GetLogger("test")

	if logger == nil {
		t.Fatal("GetLogger returned nil with lazy init")
	}

	if logger.level != INFO {
		t.Errorf("Lazy init level = %v, want %v (default)", logger.level, INFO)
	}

	if globalLogger == nil {
		t.Error("Global logger still nil after lazy init")
	}
}

// TestDebugLevel tests debug logging
func TestDebugLevel(t *testing.T) {
	resetGlobalLogger()
	Initialize("debug")

	// Set timestamp for consistent output
	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.Debug("debug message")
	})

	if !strings.Contains(stdout, "[DEBUG]") {
		t.Errorf("Debug log missing [DEBUG] marker: %s", stdout)
	}

	if !strings.Contains(stdout, "debug message") {
		t.Errorf("Debug log missing message: %s", stdout)
	}

	if !strings.Contains(stdout, "test:") {
		t.Errorf("Debug log missing component name: %s", stdout)
	}
}

// TestInfoLevel tests info logging
func TestInfoLevel(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.Info("info message")
	})

	if !strings.Contains(stdout, "[INFO]") {
		t.Errorf("Info log missing [INFO] marker: %s", stdout)
	}

	if !strings.Contains(stdout, "info message") {
		t.Errorf("Info log missing message: %s", stdout)
	}
}

// TestWarnLevel tests warning logging
func TestWarnLevel(t *testing.T) {
	resetGlobalLogger()
	Initialize("warn")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.Warn("warning message")
	})

	if !strings.Contains(stdout, "[WARN]") {
		t.Errorf("Warn log missing [WARN] marker: %s", stdout)
	}

	if !strings.Contains(stdout, "warning message") {
		t.Errorf("Warn log missing message: %s", stdout)
	}
}

// TestErrorLevel tests error logging
func TestErrorLevel(t *testing.T) {
	resetGlobalLogger()
	Initialize("error")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, stderr := captureOutput(func() {
		logger.Error("error message")
	})

	// Error should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Error should not appear in stdout, got: %s", stdout)
	}

	// Error should appear in stderr only
	if !strings.Contains(stderr, "[ERROR]") {
		t.Errorf("Error log missing [ERROR] marker in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "error message") {
		t.Errorf("Error log missing message in stderr: %s", stderr)
	}
}

// TestErrorWithErr tests error logging with error object
func TestErrorWithErr(t *testing.T) {
	resetGlobalLogger()
	Initialize("error")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")
	testErr := fmt.Errorf("test error")

	stdout, stderr := captureOutput(func() {
		logger.ErrorWithErr("operation failed", testErr)
	})

	// Error should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("ErrorWithErr should not appear in stdout, got: %s", stdout)
	}

	// Error should appear in stderr only
	if !strings.Contains(stderr, "[ERROR]") {
		t.Errorf("ErrorWithErr missing [ERROR] marker in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "operation failed") {
		t.Errorf("ErrorWithErr missing message in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "test error") {
		t.Errorf("ErrorWithErr missing error object in stderr: %s", stderr)
	}
}

// setExitFunc allows tests to override the exit function
// Returns a cleanup function to restore the original behavior
func setExitFunc(f func(int)) func() {
	original := exitFunc
	exitFunc = f
	return func() { exitFunc = original }
}

// TestFatal tests fatal logging with exit behavior
func TestFatal(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	// Track exit calls
	var exitCode int
	exitCalled := false
	cleanup := setExitFunc(func(code int) {
		exitCode = code
		exitCalled = true
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.Fatal("fatal error occurred")
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Fatal should not appear in stdout, got: %s", stdout)
	}

	// Verify message logged to stderr only
	if !strings.Contains(stderr, "[FATAL]") {
		t.Errorf("Fatal log missing [FATAL] marker in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "fatal error occurred") {
		t.Errorf("Fatal log missing message in stderr: %s", stderr)
	}

	// Verify exit was called with code 1
	if !exitCalled {
		t.Error("Fatal did not call exit function")
	}

	if exitCode != 1 {
		t.Errorf("Fatal called exit with code %d, want 1", exitCode)
	}
}

// TestFatalWithFormatting tests fatal logging with printf-style formatting
func TestFatalWithFormatting(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	var exitCode int
	cleanup := setExitFunc(func(code int) {
		exitCode = code
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.Fatal("error code: %d, reason: %s", 500, "internal server error")
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Fatal should not appear in stdout, got: %s", stdout)
	}

	// Verify formatting in stderr
	if !strings.Contains(stderr, "error code: 500, reason: internal server error") {
		t.Errorf("Fatal formatting not working in stderr: %s", stderr)
	}

	if exitCode != 1 {
		t.Errorf("Fatal called exit with code %d, want 1", exitCode)
	}
}

// TestFatalWithFields tests fatal logging with structured fields
func TestFatalWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	var exitCode int
	exitCalled := false
	cleanup := setExitFunc(func(code int) {
		exitCode = code
		exitCalled = true
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.FatalWithFields("critical failure",
			Field("error_code", 500),
			Field("component", "database"),
			Field("retry_count", 3),
		)
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("FatalWithFields should not appear in stdout, got: %s", stdout)
	}

	// Verify message and fields in stderr only
	if !strings.Contains(stderr, "[FATAL]") {
		t.Errorf("FatalWithFields missing [FATAL] marker in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "critical failure") {
		t.Errorf("FatalWithFields missing message in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "error_code=500") {
		t.Errorf("FatalWithFields missing error_code field in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "component=database") {
		t.Errorf("FatalWithFields missing component field in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "retry_count=3") {
		t.Errorf("FatalWithFields missing retry_count field in stderr: %s", stderr)
	}

	// Verify exit was called
	if !exitCalled {
		t.Error("FatalWithFields did not call exit function")
	}

	if exitCode != 1 {
		t.Errorf("FatalWithFields called exit with code %d, want 1", exitCode)
	}
}

// TestFatalWithContextFields tests fatal logging with context fields from WithField
func TestFatalWithContextFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test").WithField("request_id", "req-12345")

	var exitCode int
	cleanup := setExitFunc(func(code int) {
		exitCode = code
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.FatalWithFields("request processing failed",
			Field("status", "timeout"),
		)
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("FatalWithFields should not appear in stdout, got: %s", stdout)
	}

	// Verify both context field and method field are present in stderr
	if !strings.Contains(stderr, "request_id=req-12345") {
		t.Errorf("FatalWithFields missing context field in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "status=timeout") {
		t.Errorf("FatalWithFields missing method field in stderr: %s", stderr)
	}

	if exitCode != 1 {
		t.Errorf("FatalWithFields called exit with code %d, want 1", exitCode)
	}
}

// TestFatalLevelFiltering tests that fatal respects log level
func TestFatalLevelFiltering(t *testing.T) {
	// Fatal level is highest, so setting level above FATAL should filter it
	// However, in practice FATAL is the highest level (value 4)
	// This test verifies the level check works correctly

	resetGlobalLogger()
	Initialize("fatal")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	exitCalled := false
	cleanup := setExitFunc(func(code int) {
		exitCalled = true
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.Fatal("fatal message")
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Fatal should not appear in stdout, got: %s", stdout)
	}

	// At FATAL level, fatal messages should appear in stderr
	if !strings.Contains(stderr, "fatal message") {
		t.Errorf("Fatal message was filtered at FATAL level: %s", stderr)
	}

	if !exitCalled {
		t.Error("Fatal did not call exit at FATAL level")
	}
}

// TestFatalDoesNotExitWhenLevelTooHigh tests level filtering for Fatal
// Note: Since FATAL is the highest level, this is a theoretical test
// In practice, there's no level higher than FATAL that would filter it
func TestFatalRespectLevelCheck(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	// Verify Fatal works at INFO level (INFO <= FATAL)
	exitCalled := false
	cleanup := setExitFunc(func(code int) {
		exitCalled = true
	})
	defer cleanup()

	stdout, stderr := captureOutput(func() {
		logger.Fatal("test fatal")
	})

	// Fatal should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Fatal should not appear in stdout, got: %s", stdout)
	}

	// Verify Fatal works at INFO level and appears in stderr
	if !strings.Contains(stderr, "test fatal") {
		t.Errorf("Fatal was filtered at INFO level (should not be): %s", stderr)
	}

	if !exitCalled {
		t.Error("Fatal did not call exit at INFO level")
	}
}

// TestMultipleFatalCalls tests that exit function is called for each Fatal
func TestMultipleFatalCalls(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	exitCount := 0
	cleanup := setExitFunc(func(code int) {
		exitCount++
	})
	defer cleanup()

	captureOutput(func() {
		logger.Fatal("first fatal")
		logger.Fatal("second fatal")
		logger.Fatal("third fatal")
	})

	if exitCount != 3 {
		t.Errorf("Expected 3 exit calls, got %d", exitCount)
	}
}

// TestConcurrentFatal tests concurrent fatal calls
func TestConcurrentFatal(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	var exitCount int
	var mu sync.Mutex
	cleanup := setExitFunc(func(code int) {
		mu.Lock()
		exitCount++
		mu.Unlock()
	})
	defer cleanup()

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	captureOutput(func() {
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				logger.Fatal("concurrent fatal %d", id)
			}(i)
		}
		wg.Wait()
	})

	if exitCount != numGoroutines {
		t.Errorf("Expected %d exit calls, got %d", numGoroutines, exitCount)
	}
}

// TestFormatting tests printf-style formatting
func TestFormatting(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.Info("formatted message: %s=%d", "count", 42)
	})

	if !strings.Contains(stdout, "formatted message: count=42") {
		t.Errorf("Formatting not working: %s", stdout)
	}
}

// TestLevelFiltering tests that logs below threshold are filtered
func TestLevelFiltering(t *testing.T) {
	tests := []struct {
		name         string
		setLevel     string
		logLevel     func(*Logger)
		shouldAppear bool
		checkStderr  bool // true if this log level writes to stderr instead of stdout
	}{
		{"debug filtered at info", "info", func(l *Logger) { l.Debug("test") }, false, false},
		{"info shown at info", "info", func(l *Logger) { l.Info("test") }, true, false},
		{"warn shown at info", "info", func(l *Logger) { l.Warn("test") }, true, false},
		{"error shown at info", "info", func(l *Logger) { l.Error("test") }, true, true},
		{"info filtered at error", "error", func(l *Logger) { l.Info("test") }, false, false},
		{"warn filtered at error", "error", func(l *Logger) { l.Warn("test") }, false, false},
		{"error shown at error", "error", func(l *Logger) { l.Error("test") }, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalLogger()
			Initialize(tt.setLevel)

			os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
			defer os.Unsetenv("LOG_TIMESTAMP")

			logger := GetLogger("test")

			stdout, stderr := captureOutput(func() {
				tt.logLevel(logger)
			})

			// Check the appropriate stream based on log level
			var hasOutput bool
			if tt.checkStderr {
				hasOutput = len(strings.TrimSpace(stderr)) > 0
			} else {
				hasOutput = len(strings.TrimSpace(stdout)) > 0
			}

			if hasOutput != tt.shouldAppear {
				t.Errorf("Level filtering failed: level=%s, shouldAppear=%v, hasOutput=%v, stdout=%q, stderr=%q",
					tt.setLevel, tt.shouldAppear, hasOutput, stdout, stderr)
			}
		})
	}
}

// TestWithField tests adding a single field
func TestWithField(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")
	loggerWithField := logger.WithField("key", "value")

	stdout, _ := captureOutput(func() {
		loggerWithField.InfoWithFields("message")
	})

	if !strings.Contains(stdout, "key=value") {
		t.Errorf("WithField output missing field: %s", stdout)
	}

	if !strings.Contains(stdout, "message") {
		t.Errorf("WithField output missing message: %s", stdout)
	}
}

// TestWithFields tests adding multiple fields
func TestWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")
	loggerWithFields := logger.WithFields(
		Field("key1", "value1"),
		Field("key2", 42),
		Field("key3", true),
	)

	stdout, _ := captureOutput(func() {
		loggerWithFields.InfoWithFields("message")
	})

	if !strings.Contains(stdout, "key1=value1") {
		t.Errorf("WithFields output missing key1: %s", stdout)
	}

	if !strings.Contains(stdout, "key2=42") {
		t.Errorf("WithFields output missing key2: %s", stdout)
	}

	if !strings.Contains(stdout, "key3=true") {
		t.Errorf("WithFields output missing key3: %s", stdout)
	}
}

// TestInfoWithFields tests structured logging
func TestInfoWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.InfoWithFields("operation completed",
			Field("duration_ms", 123),
			Field("status", "success"),
		)
	})

	if !strings.Contains(stdout, "operation completed") {
		t.Errorf("InfoWithFields missing message: %s", stdout)
	}

	if !strings.Contains(stdout, "duration_ms=123") {
		t.Errorf("InfoWithFields missing duration field: %s", stdout)
	}

	if !strings.Contains(stdout, "status=success") {
		t.Errorf("InfoWithFields missing status field: %s", stdout)
	}
}

// TestDebugWithFields tests debug structured logging
func TestDebugWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("debug")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.DebugWithFields("debug info", Field("key", "value"))
	})

	if !strings.Contains(stdout, "[DEBUG]") {
		t.Errorf("DebugWithFields missing DEBUG marker: %s", stdout)
	}

	if !strings.Contains(stdout, "key=value") {
		t.Errorf("DebugWithFields missing field: %s", stdout)
	}
}

// TestWarnWithFields tests warning structured logging
func TestWarnWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("warn")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.WarnWithFields("warning info", Field("reason", "test"))
	})

	if !strings.Contains(stdout, "[WARN]") {
		t.Errorf("WarnWithFields missing WARN marker: %s", stdout)
	}

	if !strings.Contains(stdout, "reason=test") {
		t.Errorf("WarnWithFields missing field: %s", stdout)
	}
}

// TestErrorWithFields tests error structured logging
func TestErrorWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("error")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, stderr := captureOutput(func() {
		logger.ErrorWithFields("error occurred", Field("code", 500))
	})

	// Error should NOT appear in stdout - only stderr
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("ErrorWithFields should not appear in stdout, got: %s", stdout)
	}

	// Should appear in stderr only
	if !strings.Contains(stderr, "[ERROR]") {
		t.Errorf("ErrorWithFields missing ERROR marker in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "code=500") {
		t.Errorf("ErrorWithFields missing field in stderr: %s", stderr)
	}
}

// TestFieldPersistence tests that fields persist across logs
func TestFieldPersistence(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")
	loggerWithFields := logger.WithField("request_id", "12345")

	stdout, _ := captureOutput(func() {
		loggerWithFields.InfoWithFields("first log")
		loggerWithFields.InfoWithFields("second log")
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 log lines, got %d", len(lines))
	}

	// Both logs should have the persistent field
	if !strings.Contains(lines[0], "request_id=12345") {
		t.Errorf("First log missing persistent field: %s", lines[0])
	}

	if !strings.Contains(lines[1], "request_id=12345") {
		t.Errorf("Second log missing persistent field: %s", lines[1])
	}
}

// TestWithName tests logger name changes
func TestWithName(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("original")
	renamedLogger := logger.WithName("renamed")

	stdout, _ := captureOutput(func() {
		renamedLogger.Info("test message")
	})

	if !strings.Contains(stdout, "renamed:") {
		t.Errorf("WithName output missing new name: %s", stdout)
	}

	if strings.Contains(stdout, "original:") {
		t.Errorf("WithName output still has old name: %s", stdout)
	}
}

// TestLoggerIsolation tests that loggers don't share state
func TestLoggerIsolation(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger1 := GetLogger("logger1").WithField("id", "1")
	logger2 := GetLogger("logger2").WithField("id", "2")

	stdout, _ := captureOutput(func() {
		logger1.InfoWithFields("from logger1")
		logger2.InfoWithFields("from logger2")
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected 2 log lines, got %d", len(lines))
	}

	// First log should only have id=1
	if !strings.Contains(lines[0], "id=1") {
		t.Errorf("Logger1 output missing id=1: %s", lines[0])
	}

	if strings.Contains(lines[0], "id=2") {
		t.Errorf("Logger1 output has logger2's field: %s", lines[0])
	}

	// Second log should only have id=2
	if !strings.Contains(lines[1], "id=2") {
		t.Errorf("Logger2 output missing id=2: %s", lines[1])
	}

	if strings.Contains(lines[1], "id=1") {
		t.Errorf("Logger2 output has logger1's field: %s", lines[1])
	}
}

// TestGetTimestamp tests timestamp generation
func TestGetTimestamp(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		wantExact  string
		wantFormat bool
	}{
		{"with env var override", "2024-01-01T12:00:00Z", "2024-01-01T12:00:00Z", false},
		{"actual timestamp generation", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("LOG_TIMESTAMP", tt.envValue)
				defer os.Unsetenv("LOG_TIMESTAMP")
			} else {
				os.Unsetenv("LOG_TIMESTAMP")
			}

			got := GetTimestamp()

			if tt.wantFormat {
				// Verify it's a valid RFC3339 timestamp
				_, err := time.Parse(time.RFC3339, got)
				if err != nil {
					t.Errorf("GetTimestamp() returned invalid RFC3339 format: %q, error: %v", got, err)
				}

				// Verify timestamp is recent (within last second)
				parsedTime, _ := time.Parse(time.RFC3339, got)
				now := time.Now()
				diff := now.Sub(parsedTime)
				if diff < 0 || diff > time.Second {
					t.Errorf("GetTimestamp() returned timestamp not within last second: %q (diff: %v)", got, diff)
				}
			} else {
				if got != tt.wantExact {
					t.Errorf("GetTimestamp() = %q, want %q", got, tt.wantExact)
				}
			}
		})
	}
}

// TestTimestampInActualLog verifies timestamps appear in real log output
func TestTimestampInActualLog(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	// Don't set LOG_TIMESTAMP - use real timestamps
	os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	stdout, _ := captureOutput(func() {
		logger.Info("test message")
	})

	// Verify output contains RFC3339 timestamp format markers
	if !strings.Contains(stdout, "T") {
		t.Errorf("Log output missing timestamp 'T' separator: %s", stdout)
	}

	if !strings.Contains(stdout, "[INFO]") {
		t.Errorf("Log output missing [INFO] marker: %s", stdout)
	}

	if !strings.Contains(stdout, "test message") {
		t.Errorf("Log output missing message: %s", stdout)
	}

	// Extract timestamp from log format
	// log.Println adds: "2026/01/04 10:40:59 "
	// Then our format: "[2026-01-04T10:40:59+01:00] [INFO] test: test message"
	// We want to extract just the RFC3339 part between first [ and first ]
	startIdx := strings.Index(stdout, "[")
	endIdx := strings.Index(stdout, "]")

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("Log output doesn't contain [timestamp]: %s", stdout)
	}

	timestamp := stdout[startIdx+1 : endIdx]

	// Verify it's a valid RFC3339 timestamp
	_, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t.Errorf("Timestamp in log is not valid RFC3339: %q, error: %v", timestamp, err)
	}

	t.Logf("Successfully parsed timestamp: %s", timestamp)
}

// TestFieldConstructor tests Field helper function
func TestFieldConstructor(t *testing.T) {
	field := Field("key", "value")

	if field.Key != "key" {
		t.Errorf("Field.Key = %q, want %q", field.Key, "key")
	}

	if field.Value != "value" {
		t.Errorf("Field.Value = %v, want %v", field.Value, "value")
	}
}

// TestConcurrentGetLogger tests thread-safe logger creation
func TestConcurrentGetLogger(t *testing.T) {
	resetGlobalLogger()

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	loggers := make([]*Logger, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			loggers[idx] = GetLogger(fmt.Sprintf("logger-%d", idx))
		}(i)
	}

	wg.Wait()

	// Verify all loggers were created
	for i, logger := range loggers {
		if logger == nil {
			t.Errorf("Logger %d is nil", i)
		}
	}

	// Verify global logger was initialized exactly once
	if globalLogger == nil {
		t.Error("Global logger not initialized after concurrent access")
	}
}

// TestConcurrentLogging tests concurrent logging
func TestConcurrentLogging(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("concurrent-test")

	const numGoroutines = 50
	const logsPerGoroutine = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	stdout, _ := captureOutput(func() {
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < logsPerGoroutine; j++ {
					logger.Info("concurrent log from goroutine %d, iteration %d", id, j)
				}
			}(i)
		}
		wg.Wait()
	})

	// Verify we got all logs
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	expectedLines := numGoroutines * logsPerGoroutine

	if len(lines) != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, len(lines))
	}
}

// TestConcurrentWithFields tests concurrent field manipulation
func TestConcurrentWithFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	stdout, _ := captureOutput(func() {
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				loggerWithField := logger.WithField("goroutine_id", id)
				loggerWithField.InfoWithFields("test log")
			}(i)
		}
		wg.Wait()
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != numGoroutines {
		t.Errorf("Expected %d log lines, got %d", numGoroutines, len(lines))
	}
}

// Benchmarks

// BenchmarkBasicLogging benchmarks simple logging
func BenchmarkBasicLogging(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("bench")

	// Redirect output to discard
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}

// BenchmarkFormattedLogging benchmarks formatted logging
func BenchmarkFormattedLogging(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("bench")

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message: %s=%d", "iteration", i)
	}
}

// BenchmarkStructuredLogging benchmarks structured logging
func BenchmarkStructuredLogging(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("bench")

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("benchmark message",
			Field("iteration", i),
			Field("timestamp", time.Now().Unix()),
		)
	}
}

// BenchmarkStructuredLoggingWithContext benchmarks structured logging with context fields
func BenchmarkStructuredLoggingWithContext(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("bench").
		WithField("request_id", "12345").
		WithField("user_id", "user-123")

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("benchmark message",
			Field("iteration", i),
		)
	}
}

// BenchmarkLoggerCreation benchmarks logger creation
func BenchmarkLoggerCreation(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetLogger("bench")
	}
}

// BenchmarkLoggerCloning benchmarks logger cloning with fields
func BenchmarkLoggerCloning(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	logger := GetLogger("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.WithField("key", "value")
	}
}

// BenchmarkLoggerCloningMultipleFields benchmarks logger cloning with multiple fields
func BenchmarkLoggerCloningMultipleFields(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	logger := GetLogger("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.WithFields(
			Field("key1", "value1"),
			Field("key2", "value2"),
			Field("key3", "value3"),
		)
	}
}

// TestRaceConditionFixed tests that concurrent initialization is thread-safe
// This test specifically validates the sync.Once fix for the race condition
func TestRaceConditionFixed(t *testing.T) {
	resetGlobalLogger()

	const numGoroutines = 200
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Track how many times Initialize was actually called
	initCalls := make(chan bool, numGoroutines)

	// All goroutines call GetLogger simultaneously
	// Without sync.Once, this would trigger a race condition
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			logger := GetLogger(fmt.Sprintf("concurrent-%d", id))
			if logger != nil {
				initCalls <- true
			}
		}(i)
	}

	wg.Wait()
	close(initCalls)

	// Count successful logger creations
	count := 0
	for range initCalls {
		count++
	}

	// All goroutines should successfully get a logger
	if count != numGoroutines {
		t.Errorf("Expected %d successful logger creations, got %d", numGoroutines, count)
	}

	// Global logger should be initialized exactly once
	if globalLogger == nil {
		t.Error("Global logger not initialized after concurrent access")
	}

	// Verify all loggers have the correct default level (INFO from lazy init)
	logger := GetLogger("test")
	if logger.level != INFO {
		t.Errorf("Logger level = %v, want %v (default from lazy init)", logger.level, INFO)
	}
}

// BenchmarkLevelFiltering benchmarks filtered (not logged) messages
func BenchmarkLevelFiltering(b *testing.B) {
	resetGlobalLogger()
	Initialize("error") // Set to ERROR so DEBUG/INFO/WARN are filtered

	logger := GetLogger("bench")

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("this will be filtered")
	}
}

// TestWithContext tests creating a context-aware logger
func TestWithContext(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	// Create context with trace and span IDs
	ctx := context.Background()
	ctx = context.WithValue(ctx, TraceIDKey(), "trace-abc-123")
	ctx = context.WithValue(ctx, SpanIDKey(), "span-xyz-789")

	ctxLogger := logger.WithContext(ctx)

	stdout, _ := captureOutput(func() {
		ctxLogger.InfoWithFields("test message")
	})

	if !strings.Contains(stdout, "trace_id=trace-abc-123") {
		t.Errorf("WithContext output missing trace_id: %s", stdout)
	}

	if !strings.Contains(stdout, "span_id=span-xyz-789") {
		t.Errorf("WithContext output missing span_id: %s", stdout)
	}

	if !strings.Contains(stdout, "test message") {
		t.Errorf("WithContext output missing message: %s", stdout)
	}
}

// TestWithContextNilContext tests creating logger with nil context
func TestWithContextNilContext(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")
	ctxLogger := logger.WithContext(nil)

	stdout, _ := captureOutput(func() {
		ctxLogger.Info("test message")
	})

	// Should work fine without context fields
	if !strings.Contains(stdout, "test message") {
		t.Errorf("WithContext(nil) output missing message: %s", stdout)
	}

	// Should not have trace/span fields
	if strings.Contains(stdout, "trace_id") {
		t.Errorf("WithContext(nil) should not have trace_id: %s", stdout)
	}
}

// TestWithContextPartialFields tests context with only trace ID
func TestWithContextPartialFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	// Context with only trace ID
	ctx := context.WithValue(context.Background(), TraceIDKey(), "trace-only")
	ctxLogger := logger.WithContext(ctx)

	stdout, _ := captureOutput(func() {
		ctxLogger.Info("test message")
	})

	if !strings.Contains(stdout, "trace_id=trace-only") {
		t.Errorf("WithContext output missing trace_id: %s", stdout)
	}

	if strings.Contains(stdout, "span_id") {
		t.Errorf("WithContext should not have span_id: %s", stdout)
	}
}

// TestWithContextAndFields tests combining context with persistent fields
func TestWithContextAndFields(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	ctx := context.WithValue(context.Background(), TraceIDKey(), "trace-123")
	ctxLogger := logger.WithContext(ctx).WithField("user_id", "user-456")

	stdout, _ := captureOutput(func() {
		ctxLogger.InfoWithFields("operation completed",
			Field("duration_ms", 42),
		)
	})

	if !strings.Contains(stdout, "trace_id=trace-123") {
		t.Errorf("Output missing trace_id: %s", stdout)
	}

	if !strings.Contains(stdout, "user_id=user-456") {
		t.Errorf("Output missing user_id: %s", stdout)
	}

	if !strings.Contains(stdout, "duration_ms=42") {
		t.Errorf("Output missing duration_ms: %s", stdout)
	}

	if !strings.Contains(stdout, "operation completed") {
		t.Errorf("Output missing message: %s", stdout)
	}
}

// TestContextFieldPriority tests field priority when same key exists
func TestContextFieldPriority(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	// Context has trace_id
	ctx := context.WithValue(context.Background(), TraceIDKey(), "from-context")

	// Logger field overrides context field (same key)
	ctxLogger := logger.WithContext(ctx).WithField("trace_id", "from-logger")

	stdout, _ := captureOutput(func() {
		ctxLogger.Info("test")
	})

	// Logger field should win over context field
	if !strings.Contains(stdout, "trace_id=from-logger") {
		t.Errorf("Expected logger field to override context field: %s", stdout)
	}

	if strings.Contains(stdout, "from-context") {
		t.Errorf("Context field should be overridden: %s", stdout)
	}
}

// TestContextPreservedThroughChaining tests context persists through method chaining
func TestContextPreservedThroughChaining(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	ctx := context.WithValue(context.Background(), TraceIDKey(), "trace-chain")

	// Chain multiple operations
	chainedLogger := logger.
		WithContext(ctx).
		WithField("field1", "value1").
		WithField("field2", "value2")

	stdout, _ := captureOutput(func() {
		chainedLogger.Info("chained log")
	})

	// All fields should be present
	if !strings.Contains(stdout, "trace_id=trace-chain") {
		t.Errorf("Context not preserved through chaining: %s", stdout)
	}

	if !strings.Contains(stdout, "field1=value1") {
		t.Errorf("Field1 not preserved: %s", stdout)
	}

	if !strings.Contains(stdout, "field2=value2") {
		t.Errorf("Field2 not preserved: %s", stdout)
	}
}

// TestContextWithAllLogLevels tests context support across all log levels
func TestContextWithAllLogLevels(t *testing.T) {
	resetGlobalLogger()
	Initialize("debug")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	ctx := context.WithValue(context.Background(), TraceIDKey(), "trace-all-levels")
	ctxLogger := logger.WithContext(ctx)

	tests := []struct {
		name    string
		logFunc func()
		level   string
	}{
		{"debug", func() { ctxLogger.DebugWithFields("debug msg") }, "DEBUG"},
		{"info", func() { ctxLogger.InfoWithFields("info msg") }, "INFO"},
		{"warn", func() { ctxLogger.WarnWithFields("warn msg") }, "WARN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _ := captureOutput(func() {
				tt.logFunc()
			})

			if !strings.Contains(stdout, "trace_id=trace-all-levels") {
				t.Errorf("%s level missing trace_id: %s", tt.level, stdout)
			}
		})
	}
}

// TestContextWithError tests context support for error logging
func TestContextWithError(t *testing.T) {
	resetGlobalLogger()
	Initialize("error")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	ctx := context.WithValue(context.Background(), TraceIDKey(), "trace-error")
	ctxLogger := logger.WithContext(ctx)

	stdout, stderr := captureOutput(func() {
		ctxLogger.ErrorWithFields("error occurred", Field("error_code", 500))
	})

	// Error should be in stderr only
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("Error should not appear in stdout: %s", stdout)
	}

	if !strings.Contains(stderr, "trace_id=trace-error") {
		t.Errorf("Error missing trace_id in stderr: %s", stderr)
	}

	if !strings.Contains(stderr, "error_code=500") {
		t.Errorf("Error missing error_code in stderr: %s", stderr)
	}
}

// TestContextIsolation tests that loggers with different contexts are isolated
func TestContextIsolation(t *testing.T) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("test")

	ctx1 := context.WithValue(context.Background(), TraceIDKey(), "trace-1")
	ctx2 := context.WithValue(context.Background(), TraceIDKey(), "trace-2")

	logger1 := logger.WithContext(ctx1)
	logger2 := logger.WithContext(ctx2)

	stdout, _ := captureOutput(func() {
		logger1.Info("from logger1")
		logger2.Info("from logger2")
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected 2 log lines, got %d", len(lines))
	}

	// First log should have trace-1
	if !strings.Contains(lines[0], "trace_id=trace-1") {
		t.Errorf("Logger1 missing trace-1: %s", lines[0])
	}

	if strings.Contains(lines[0], "trace-2") {
		t.Errorf("Logger1 should not have trace-2: %s", lines[0])
	}

	// Second log should have trace-2
	if !strings.Contains(lines[1], "trace_id=trace-2") {
		t.Errorf("Logger2 missing trace-2: %s", lines[1])
	}

	if strings.Contains(lines[1], "trace-1") {
		t.Errorf("Logger2 should not have trace-1: %s", lines[1])
	}
}

// TestExtractContextFields tests the extractContextFields helper
func TestExtractContextFields(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		wantNil  bool
		expected map[string]interface{}
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantNil: true,
		},
		{
			name:    "empty context",
			ctx:     context.Background(),
			wantNil: true,
		},
		{
			name: "only trace ID",
			ctx:  context.WithValue(context.Background(), TraceIDKey(), "trace-123"),
			expected: map[string]interface{}{
				"trace_id": "trace-123",
			},
		},
		{
			name: "only span ID",
			ctx:  context.WithValue(context.Background(), SpanIDKey(), "span-456"),
			expected: map[string]interface{}{
				"span_id": "span-456",
			},
		},
		{
			name: "both trace and span",
			ctx: context.WithValue(
				context.WithValue(context.Background(), TraceIDKey(), "trace-abc"),
				SpanIDKey(), "span-xyz",
			),
			expected: map[string]interface{}{
				"trace_id": "trace-abc",
				"span_id":  "span-xyz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContextFields(tt.ctx)

			if tt.wantNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Field %s: expected %v, got %v", k, v, result[k])
				}
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}
		})
	}
}

// BenchmarkContextLogging benchmarks logging with context
func BenchmarkContextLogging(b *testing.B) {
	resetGlobalLogger()
	Initialize("info")

	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := GetLogger("bench")
	ctx := context.WithValue(
		context.WithValue(context.Background(), TraceIDKey(), "trace-bench"),
		SpanIDKey(), "span-bench",
	)
	ctxLogger := logger.WithContext(ctx)

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctxLogger.Info("benchmark message")
	}
}

// Test per-package log levels

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		packageName string
		pattern     string
		expected    bool
	}{
		// Exact matches
		{"graph.sync", "graph.sync", true},
		{"controller", "controller", true},
		{"app", "app", true},

		// Wildcard matches
		{"graph.sync", "graph.*", true},
		{"graph.analyze", "graph.*", true},
		{"graph", "graph.*", false}, // "graph" doesn't have a dot after "graph"
		{"graphme", "graph.*", false},

		// No matches
		{"controller", "graph.*", false},
		{"graph.sync", "controller", false},
		{"foo.bar", "baz.*", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.packageName, tt.pattern), func(t *testing.T) {
			result := matchesPattern(tt.packageName, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.packageName, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestSetPackageLogLevels(t *testing.T) {
	// Save original state
	resetGlobalLogger()

	tests := []struct {
		name        string
		levels      map[string]string
		shouldError bool
	}{
		{
			name: "valid levels",
			levels: map[string]string{
				"graph.sync":  "DEBUG",
				"controller":  "WARN",
				"graph.*":     "INFO",
			},
			shouldError: false,
		},
		{
			name: "invalid level",
			levels: map[string]string{
				"graph.sync": "INVALID",
			},
			shouldError: true,
		},
		{
			name:        "nil levels",
			levels:      nil,
			shouldError: false,
		},
		{
			name:        "empty levels",
			levels:      map[string]string{},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalLogger()
			err := SetPackageLogLevels(tt.levels)
			if (err != nil) != tt.shouldError {
				t.Errorf("SetPackageLogLevels() error = %v, want error = %v", err, tt.shouldError)
			}
		})
	}
}

func TestGetPackageLogLevel(t *testing.T) {
	resetGlobalLogger()

	// Set up package log levels
	levels := map[string]string{
		"graph.sync":   "DEBUG",
		"graph.*":      "INFO",
		"controller":   "WARN",
		"service.auth": "ERROR",
	}
	if err := SetPackageLogLevels(levels); err != nil {
		t.Fatalf("SetPackageLogLevels() error = %v", err)
	}

	tests := []struct {
		packageName  string
		expectedLevel LogLevel
	}{
		// Exact matches (highest priority)
		{"graph.sync", DEBUG},
		{"controller", WARN},
		{"service.auth", ERROR},

		// Wildcard matches
		{"graph.analyze", INFO},
		{"graph.extract", INFO},

		// Not found (should return -1)
		{"unknown", LogLevel(-1)},
		{"app", LogLevel(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.packageName, func(t *testing.T) {
			level := GetPackageLogLevel(tt.packageName)
			if level != tt.expectedLevel {
				t.Errorf("GetPackageLogLevel(%q) = %v, want %v", tt.packageName, level, tt.expectedLevel)
			}
		})
	}
}

func TestPackageLogLevelPrecedence(t *testing.T) {
	resetGlobalLogger()

	// More specific patterns should take precedence
	levels := map[string]string{
		"graph.*":       "INFO",
		"graph.sync.*":  "WARN",
		"graph.sync":    "DEBUG",
	}
	if err := SetPackageLogLevels(levels); err != nil {
		t.Fatalf("SetPackageLogLevels() error = %v", err)
	}

	tests := []struct {
		packageName   string
		expectedLevel LogLevel
	}{
		{"graph.sync", DEBUG}, // Exact match wins
		{"graph.sync.worker", WARN}, // More specific wildcard wins
		{"graph.analyze", INFO}, // Wildcard match
	}

	for _, tt := range tests {
		t.Run(tt.packageName, func(t *testing.T) {
			level := GetPackageLogLevel(tt.packageName)
			if level != tt.expectedLevel {
				t.Errorf("GetPackageLogLevel(%q) = %v, want %v", tt.packageName, level, tt.expectedLevel)
			}
		})
	}
}

func TestPerPackageLogLevelFiltering(t *testing.T) {
	resetGlobalLogger()

	// Set default level to INFO but graph.sync to DEBUG
	if err := Initialize("info", map[string]string{"graph.sync": "debug"}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify shouldLog works correctly for graph.sync (DEBUG enabled)
	syncLogger := GetLogger("graph.sync")
	if !syncLogger.shouldLog(DEBUG) {
		t.Error("graph.sync logger should log DEBUG messages")
	}
	if !syncLogger.shouldLog(INFO) {
		t.Error("graph.sync logger should log INFO messages")
	}

	// Verify shouldLog works correctly for controller (DEBUG disabled)
	controllerLogger := GetLogger("controller")
	if controllerLogger.shouldLog(DEBUG) {
		t.Error("controller logger should NOT log DEBUG messages")
	}
	if !controllerLogger.shouldLog(INFO) {
		t.Error("controller logger should log INFO messages")
	}

	// Test with wildcard patterns
	if err := SetPackageLogLevels(map[string]string{"graph.*": "warn"}); err != nil {
		t.Fatalf("SetPackageLogLevels() error = %v", err)
	}

	graphAnalyzeLogger := GetLogger("graph.analyze")
	// Should NOT log DEBUG or INFO (WARN is minimum)
	if graphAnalyzeLogger.shouldLog(DEBUG) {
		t.Error("graph.analyze should NOT log DEBUG (pattern: graph.* = WARN)")
	}
	if graphAnalyzeLogger.shouldLog(INFO) {
		t.Error("graph.analyze should NOT log INFO (pattern: graph.* = WARN)")
	}
	// Should log WARN and above
	if !graphAnalyzeLogger.shouldLog(WARN) {
		t.Error("graph.analyze should log WARN (pattern: graph.* = WARN)")
	}
	if !graphAnalyzeLogger.shouldLog(ERROR) {
		t.Error("graph.analyze should log ERROR (pattern: graph.* = WARN)")
	}
}

func TestInitializeWithPackageLevels(t *testing.T) {
	resetGlobalLogger()

	packageLevels := map[string]string{
		"graph.sync": "debug",
		"controller": "warn",
	}

	// Initialize with default "info" and package overrides
	err := Initialize("info", packageLevels)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify global logger has INFO level
	if globalLogger.level != INFO {
		t.Errorf("globalLogger.level = %v, want %v", globalLogger.level, INFO)
	}

	// Verify package levels were set correctly
	if level := GetPackageLogLevel("graph.sync"); level != DEBUG {
		t.Errorf("GetPackageLogLevel(graph.sync) = %v, want %v", level, DEBUG)
	}
	if level := GetPackageLogLevel("controller"); level != WARN {
		t.Errorf("GetPackageLogLevel(controller) = %v, want %v", level, WARN)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		levelStr string
		expected LogLevel
		wantErr  bool
	}{
		{"DEBUG", DEBUG, false},
		{"debug", DEBUG, false},
		{"Info", INFO, false},
		{"WARN", WARN, false},
		{"ERROR", ERROR, false},
		{"error", ERROR, false},
		{"FATAL", FATAL, false},
		{"INVALID", -1, true},
		{"", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.levelStr, func(t *testing.T) {
			level, err := parseLevel(tt.levelStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLevel(%q) error = %v, want error = %v", tt.levelStr, err, tt.wantErr)
			}
			if err == nil && level != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.levelStr, level, tt.expected)
			}
		})
	}
}
