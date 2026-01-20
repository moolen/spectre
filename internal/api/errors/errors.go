package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
)

// ErrorResponse represents an error response for HTTP APIs
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

	// ErrorCodeValidation represents a validation error
	ErrorCodeValidation ErrorCode = "VALIDATION_ERROR"
)

// APIError represents an API error with status code and message
// It supports both HTTP status codes and gRPC/Connect error codes
type APIError struct {
	Code         ErrorCode
	HTTPStatus   int
	ConnectCode  connect.Code
	Message      string
	Details      map[string]interface{} // Optional additional error details
}

// NewAPIError creates a new API error with HTTP and Connect codes
func NewAPIError(code ErrorCode, httpStatus int, connectCode connect.Code, message string) *APIError {
	return &APIError{
		Code:        code,
		HTTPStatus:  httpStatus,
		ConnectCode: connectCode,
		Message:     message,
		Details:     make(map[string]interface{}),
	}
}

// NewAPIErrorFromHTTP creates a new API error from HTTP status code only
// It automatically maps to the appropriate Connect code
func NewAPIErrorFromHTTP(code ErrorCode, httpStatus int, message string) *APIError {
	connectCode := HTTPToConnectCode(httpStatus)
	return NewAPIError(code, httpStatus, connectCode, message)
}

// Error returns the error message
func (e *APIError) Error() string {
	return e.Message
}

// GetHTTPResponse returns the HTTP error response
func (e *APIError) GetHTTPResponse() ErrorResponse {
	return ErrorResponse{
		Error:   string(e.Code),
		Message: e.Message,
	}
}

// GetHTTPStatusCode returns the HTTP status code
func (e *APIError) GetHTTPStatusCode() int {
	return e.HTTPStatus
}

// GetConnectError returns a Connect error for gRPC/Connect APIs
func (e *APIError) GetConnectError() *connect.Error {
	err := connect.NewError(e.ConnectCode, fmt.Errorf("%s", e.Message))
	// Note: Error details can be added in the future using protobuf messages
	// For now, the error message and code are sufficient
	return err
}

// WithDetail adds additional context to the error
func (e *APIError) WithDetail(key string, value interface{}) *APIError {
	e.Details[key] = value
	return e
}

// HTTPToConnectCode maps HTTP status codes to Connect error codes
func HTTPToConnectCode(httpStatus int) connect.Code {
	switch httpStatus {
	case http.StatusBadRequest:
		return connect.CodeInvalidArgument
	case http.StatusUnauthorized:
		return connect.CodeUnauthenticated
	case http.StatusForbidden:
		return connect.CodePermissionDenied
	case http.StatusNotFound:
		return connect.CodeNotFound
	case http.StatusConflict:
		return connect.CodeAlreadyExists
	case http.StatusTooManyRequests:
		return connect.CodeResourceExhausted
	case http.StatusInternalServerError:
		return connect.CodeInternal
	case http.StatusNotImplemented:
		return connect.CodeUnimplemented
	case http.StatusServiceUnavailable:
		return connect.CodeUnavailable
	default:
		return connect.CodeUnknown
	}
}

// ConnectToHTTPCode maps Connect error codes to HTTP status codes
func ConnectToHTTPCode(code connect.Code) int {
	switch code {
	case connect.CodeInvalidArgument:
		return http.StatusBadRequest
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeNotFound:
		return http.StatusNotFound
	case connect.CodeAlreadyExists:
		return http.StatusConflict
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case connect.CodeInternal:
		return http.StatusInternalServerError
	case connect.CodeUnimplemented:
		return http.StatusNotImplemented
	case connect.CodeUnavailable:
		return http.StatusServiceUnavailable
	case connect.CodeCanceled:
		return http.StatusRequestTimeout
	case connect.CodeUnknown:
		return http.StatusInternalServerError
	case connect.CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case connect.CodeFailedPrecondition:
		return http.StatusBadRequest
	case connect.CodeAborted:
		return http.StatusConflict
	case connect.CodeOutOfRange:
		return http.StatusBadRequest
	case connect.CodeDataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		connect.CodeInvalidArgument,
		fmt.Sprintf(message, args...),
	)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeNotFound,
		http.StatusNotFound,
		connect.CodeNotFound,
		fmt.Sprintf(message, args...),
	)
}

// NewInternalServerError creates an internal server error
func NewInternalServerError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeInternalError,
		http.StatusInternalServerError,
		connect.CodeInternal,
		fmt.Sprintf(message, args...),
	)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeUnauthorized,
		http.StatusUnauthorized,
		connect.CodeUnauthenticated,
		fmt.Sprintf(message, args...),
	)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeForbidden,
		http.StatusForbidden,
		connect.CodePermissionDenied,
		fmt.Sprintf(message, args...),
	)
}

// NewConflictError creates a conflict error
func NewConflictError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeConflict,
		http.StatusConflict,
		connect.CodeAlreadyExists,
		fmt.Sprintf(message, args...),
	)
}

// NewTooManyRequestsError creates a rate limiting error
func NewTooManyRequestsError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeTooManyRequests,
		http.StatusTooManyRequests,
		connect.CodeResourceExhausted,
		fmt.Sprintf(message, args...),
	)
}

// NewValidationError creates a validation error
func NewValidationError(message string, args ...interface{}) *APIError {
	return NewAPIError(
		ErrorCodeValidation,
		http.StatusBadRequest,
		connect.CodeInvalidArgument,
		fmt.Sprintf(message, args...),
	)
}

// WrapError wraps a standard error as an internal server error
func WrapError(err error) *APIError {
	var apiErr *APIError
	if stderrors.As(err, &apiErr) {
		return apiErr
	}
	return NewInternalServerError("Internal server error: %v", err)
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) bool {
	var apiErr *APIError
	return stderrors.As(err, &apiErr)
}

// AsAPIError tries to convert an error to an APIError
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	ok := stderrors.As(err, &apiErr)
	return apiErr, ok
}
