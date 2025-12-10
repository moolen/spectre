package api

import (
	"encoding/json"
	"io"
	"net/http"
)

// writeJSON writes a JSON response to the response writer
func writeJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

// writeError sends an error response
func writeError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = writeJSON(w, response)
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
