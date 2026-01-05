package errors

// This file provides backward compatibility helpers for the api package
// It allows existing code to work with the new error system without changes

// GetResponse returns the error response (backward compatibility)
// This matches the old APIError.GetResponse() method
func (e *APIError) GetResponse() ErrorResponse {
	return e.GetHTTPResponse()
}

// GetStatusCode returns the HTTP status code (backward compatibility)
// This matches the old APIError.GetStatusCode() method
func (e *APIError) GetStatusCode() int {
	return e.GetHTTPStatusCode()
}
