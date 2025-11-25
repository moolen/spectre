package api

import (
	"fmt"
	"net/http"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ErrorCode represents error codes used in API responses
type ErrorCode string

const (
	// ErrorCodeInvalidRequest represents invalid request parameters
	ErrorCodeInvalidRequest ErrorCode = "INVALID_REQUEST"

	// ErrorCodeNotFound represents a not found error
	ErrorCodeNotFound ErrorCode = "NOT_FOUND"

	// ErrorCodeInternalError represents an internal server error
	ErrorCodeInternalError ErrorCode = "INTERNAL_ERROR"

	// ErrorCodeUnauthorized represents unauthorized access
	ErrorCodeUnauthorized ErrorCode = "UNAUTHORIZED"

	// ErrorCodeForbidden represents forbidden access
	ErrorCodeForbidden ErrorCode = "FORBIDDEN"

	// ErrorCodeConflict represents a conflict
	ErrorCodeConflict ErrorCode = "CONFLICT"

	// ErrorCodeTooManyRequests represents rate limiting
	ErrorCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"
)

// APIError represents an API error with status code and message
type APIError struct {
	Code       ErrorCode
	StatusCode int
	Message    string
}

// NewAPIError creates a new API error
func NewAPIError(code ErrorCode, statusCode int, message string) *APIError {
	return &APIError{
		Code:       code,
		StatusCode: statusCode,
		Message:    message,
	}
}

// Error returns the error message
func (e *APIError) Error() string {
	return e.Message
}

// GetResponse returns the error response
func (e *APIError) GetResponse() ErrorResponse {
	return ErrorResponse{
		Error:   string(e.Code),
		Message: e.Message,
	}
}

// GetStatusCode returns the HTTP status code
func (e *APIError) GetStatusCode() int {
	return e.StatusCode
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		fmt.Sprintf(message, args...),
	)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeNotFound,
		http.StatusNotFound,
		fmt.Sprintf(message, args...),
	)
}

// NewInternalServerError creates an internal server error
func NewInternalServerError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeInternalError,
		http.StatusInternalServerError,
		fmt.Sprintf(message, args...),
	)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeUnauthorized,
		http.StatusUnauthorized,
		fmt.Sprintf(message, args...),
	)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeForbidden,
		http.StatusForbidden,
		fmt.Sprintf(message, args...),
	)
}

// NewTooManyRequestsError creates a rate limiting error
func NewTooManyRequestsError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeTooManyRequests,
		http.StatusTooManyRequests,
		fmt.Sprintf(message, args...),
	)
}
