package http

import (
	"fmt"
)

// ValidationErrors contains field-specific validation errors
type ValidationErrors struct {
	Fields map[string]string
}

// Error returns the error message
func (ve *ValidationErrors) Error() string {
	if len(ve.Fields) == 0 {
		return "validation error"
	}
	return fmt.Sprintf("validation error: %v", ve.Fields)
}

// Add adds a validation error for a field
func (ve *ValidationErrors) Add(field, message string) {
	if ve.Fields == nil {
		ve.Fields = make(map[string]string)
	}
	ve.Fields[field] = message
}

// HasErrors returns true if there are validation errors
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Fields) > 0
}

