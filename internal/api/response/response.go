package response

import (
	"encoding/json"
	"io"
	"net/http"
)

// WriteJSON writes a JSON response to the response writer
// It disables HTML escaping for better readability of JSON output
func WriteJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

// WriteError sends an error response with the specified status code
// This is a convenience function for HTTP handlers
func WriteError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = WriteJSON(w, response)
}

// WriteSuccess sends a success response with HTTP 200
func WriteSuccess(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return WriteJSON(w, data)
}

// WriteCreated sends a created response with HTTP 201
func WriteCreated(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return WriteJSON(w, data)
}

// WriteNoContent sends a no content response with HTTP 204
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Formatter formats API responses according to specifications
type Formatter struct{}

// NewFormatter creates a new response formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// FormatSearchResponse formats a search response per OpenAPI spec
// Currently a pass-through but can be extended for versioning
func (rf *Formatter) FormatSearchResponse(data interface{}) interface{} {
	return data
}

// FormatErrorResponse formats an error response
func (rf *Formatter) FormatErrorResponse(errorCode, message string) map[string]string {
	return map[string]string{
		"error":   errorCode,
		"message": message,
	}
}

// FormatTimelineResponse formats a timeline response
func (rf *Formatter) FormatTimelineResponse(data interface{}) interface{} {
	return data
}

// FormatMetadataResponse formats a metadata response
func (rf *Formatter) FormatMetadataResponse(data interface{}) interface{} {
	return data
}
