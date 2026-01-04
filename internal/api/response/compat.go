package response

import (
	"io"
	"net/http"
)

// Backward compatibility wrappers for existing code in api package
// These match the function signatures in the original response.go

// WriteJSONCompat is a backward-compatible wrapper for writeJSON
// Exported for use by api package
func WriteJSONCompat(w io.Writer, data interface{}) error {
	return WriteJSON(w, data)
}

// WriteErrorCompat is a backward-compatible wrapper for writeError
// Exported for use by api package
func WriteErrorCompat(w http.ResponseWriter, statusCode int, errorCode, message string) {
	WriteError(w, statusCode, errorCode, message)
}

// ResponseFormatter is an alias for Formatter for backward compatibility
type ResponseFormatter = Formatter

// NewResponseFormatter creates a new response formatter (backward compatible)
func NewResponseFormatter() *ResponseFormatter {
	return NewFormatter()
}
