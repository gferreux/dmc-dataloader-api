package model

import "errors"

var (
	// ErrNotFound is returned when a load_config document does not exist.
	ErrNotFound = errors.New("load config not found")
	// ErrConflict is returned when a load_config document id already exists.
	ErrConflict = errors.New("load config already exists")
	// ErrUnauthenticated is returned when the request has no accepted credentials.
	ErrUnauthenticated = errors.New("unauthenticated")
)

// FieldIssue is one validation error or warning attached to a field path.
type FieldIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationReport is the body of POST /api/v1/load-configs/validate.
type ValidationReport struct {
	Errors   []FieldIssue `json:"errors"`
	Warnings []FieldIssue `json:"warnings"`
}

// EmptyReport returns a report with non-nil slices so JSON encodes [].
func EmptyReport() ValidationReport {
	return ValidationReport{
		Errors:   []FieldIssue{},
		Warnings: []FieldIssue{},
	}
}

// ValidationError is returned by write operations when the document is rejected.
type ValidationError struct {
	Issues []FieldIssue
}

func (e *ValidationError) Error() string {
	return "validation failed"
}
