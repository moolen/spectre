package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/models"
)

// Validator validates API request parameters
type Validator struct{}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateTimestamps validates timestamp parameters
func (v *Validator) ValidateTimestamps(start, end int64) error {
	if start < 0 {
		return NewValidationError("start timestamp must be non-negative")
	}

	if end < 0 {
		return NewValidationError("end timestamp must be non-negative")
	}

	if start > end {
		return NewValidationError("start timestamp must be less than or equal to end timestamp")
	}

	return nil
}

// ValidateFilters validates filter parameters
func (v *Validator) ValidateFilters(filters models.QueryFilters) error {
	// Validate group
	if filters.Group != "" && len(filters.Group) > 255 {
		return NewValidationError("group filter is too long (max 255 characters)")
	}

	// Validate version
	if filters.Version != "" && len(filters.Version) > 255 {
		return NewValidationError("version filter is too long (max 255 characters)")
	}

	// Validate kind (single value, deprecated)
	if filters.Kind != "" && len(filters.Kind) > 255 {
		return NewValidationError("kind filter is too long (max 255 characters)")
	}

	// Validate kinds (multi-value)
	for _, kind := range filters.Kinds {
		if len(kind) > 255 {
			return NewValidationError("kind filter is too long (max 255 characters)")
		}
	}

	// Validate namespace (single value, deprecated)
	if filters.Namespace != "" {
		if len(filters.Namespace) > 63 {
			return NewValidationError("namespace must be 63 characters or less")
		}
		// Kubernetes namespace naming rules - allow empty string (means all namespaces)
		// Only validate format if namespace is non-empty
		if !isValidNamespace(filters.Namespace) {
			return NewValidationError("invalid namespace format")
		}
	}

	// Validate namespaces (multi-value)
	for _, namespace := range filters.Namespaces {
		if len(namespace) > 63 {
			return NewValidationError("namespace must be 63 characters or less")
		}
		if namespace != "" && !isValidNamespace(namespace) {
			return NewValidationError("invalid namespace format")
		}
	}

	return nil
}

// ValidateQuery validates a complete query request
func (v *Validator) ValidateQuery(query *models.QueryRequest) error {
	if err := v.ValidateTimestamps(query.StartTimestamp, query.EndTimestamp); err != nil {
		return err
	}

	if err := v.ValidateFilters(query.Filters); err != nil {
		return err
	}

	return nil
}

// isValidNamespace checks if a namespace name is valid per Kubernetes naming rules
// Kubernetes namespaces are DNS subdomain names: [a-z0-9]([-a-z0-9]*[a-z0-9])?
// But we'll be more lenient to allow common namespace patterns
func isValidNamespace(namespace string) bool {
	if namespace == "" {
		// Empty namespace means "all namespaces" - this is valid
		return true
	}

	if len(namespace) > 63 {
		return false
	}

	// Kubernetes namespaces must start and end with alphanumeric characters
	// and can contain hyphens, dots, and underscores in the middle
	// We'll allow both lowercase and uppercase for compatibility
	for i, ch := range namespace {
		if i == 0 || i == len(namespace)-1 {
			// Must start and end with alphanumeric
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				return false
			}
		} else {
			// Middle can have hyphens, dots, underscores, and alphanumeric
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_') {
				return false
			}
		}
	}

	return true
}

// ValidateSearchRequest validates a complete search request
func (v *Validator) ValidateSearchRequest(startStr, endStr string, filters models.QueryFilters) error {
	if startStr == "" || endStr == "" {
		return NewValidationError("start and end timestamps are required")
	}

	return v.ValidateFilters(filters)
}

// ValidationError represents a validation error
type ValidationError struct {
	message string
}

// NewValidationError creates a new validation error
func NewValidationError(message string, args ...interface{}) *ValidationError {
	return &ValidationError{
		message: fmt.Sprintf(message, args...),
	}
}

// Error returns the error message
func (ve *ValidationError) Error() string {
	return ve.message
}

// GetMessage returns the error message for API responses
func (ve *ValidationError) GetMessage() string {
	return ve.message
}
