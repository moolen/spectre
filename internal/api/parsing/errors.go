package parsing

import "fmt"

// ParsingError represents an error that occurred during parsing
type ParsingError struct {
	message string
}

// NewParsingError creates a new parsing error
func NewParsingError(message string, args ...interface{}) *ParsingError {
	return &ParsingError{
		message: fmt.Sprintf(message, args...),
	}
}

// Error returns the error message
func (pe *ParsingError) Error() string {
	return pe.message
}

// GetMessage returns the error message for API responses
func (pe *ParsingError) GetMessage() string {
	return pe.message
}
