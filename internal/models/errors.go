package models

import "fmt"

// ValidationError represents a validation error in models
type ValidationError struct {
	message string
}

// NewValidationError creates a new validation error
func NewValidationError(format string, args ...interface{}) *ValidationError {
	return &ValidationError{
		message: fmt.Sprintf(format, args...),
	}
}

// Error returns the error message
func (e *ValidationError) Error() string {
	return e.message
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
