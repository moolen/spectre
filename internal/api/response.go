package api

import (
	"encoding/json"
	"io"
	"net/http"
)

// WriteJSON writes a JSON response to the response writer
func WriteJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

// writeJSON is a compatibility alias for WriteJSON
func writeJSON(w io.Writer, data interface{}) error {
	return WriteJSON(w, data)
}

// WriteError sends an error response
func WriteError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = WriteJSON(w, response)
}

// writeError is a compatibility alias for WriteError
func writeError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	WriteError(w, statusCode, errorCode, message)
}

// ResponseFormatter formats API responses
type ResponseFormatter struct{}

// NewResponseFormatter creates a new response formatter
func NewResponseFormatter() *ResponseFormatter {
	return &ResponseFormatter{}
}

// FormatSearchResponse formats a search response per OpenAPI spec
func (rf *ResponseFormatter) FormatSearchResponse(data interface{}) interface{} {
	return data
}

// FormatErrorResponse formats an error response
func (rf *ResponseFormatter) FormatErrorResponse(errorCode, message string) map[string]string {
	return map[string]string{
		"error":   errorCode,
		"message": message,
	}
}
