package data

import (
	"errors"
	"strings"
)

type ErrorCode string

const (
	ErrorUnknown          ErrorCode = "unknown"
	ErrorObjectNotFound   ErrorCode = "object_not_found"
	ErrorFieldNotFound    ErrorCode = "field_not_found"
	ErrorInvalidFilter    ErrorCode = "invalid_filter"
	ErrorPermissionDenied ErrorCode = "permission_denied"
	ErrorMigrationFailed  ErrorCode = "migration_failed"
	ErrorSchemaDrift      ErrorCode = "schema_drift"
	ErrorInvalidCursor    ErrorCode = "invalid_cursor"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Op      string    `json:"op,omitempty"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Message
	}
	return e.Op + ": " + e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var dataErr *Error
	if errors.As(err, &dataErr) {
		return dataErr.Code
	}
	return classifyError(err)
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	var dataErr *Error
	if errors.As(err, &dataErr) {
		return err
	}
	return &Error{
		Code:    classifyError(err),
		Op:      op,
		Message: err.Error(),
		Err:     err,
	}
}

func classifyError(err error) ErrorCode {
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "permission denied"):
		return ErrorPermissionDenied
	case strings.Contains(text, "does not exist on object") && strings.Contains(text, "field"):
		return ErrorFieldNotFound
	case strings.Contains(text, "selected field") || strings.Contains(text, "filter field") || strings.Contains(text, "sort field"):
		return ErrorFieldNotFound
	case strings.Contains(text, "record") && strings.Contains(text, "does not exist"):
		return ErrorObjectNotFound
	case strings.Contains(text, "load data object") || strings.Contains(text, "object") && strings.Contains(text, "does not exist"):
		return ErrorObjectNotFound
	case strings.Contains(text, "filter") || strings.Contains(text, "operator"):
		return ErrorInvalidFilter
	case strings.Contains(text, "cursor"):
		return ErrorInvalidCursor
	case strings.Contains(text, "schema drift") || strings.Contains(text, "physical schema drift"):
		return ErrorSchemaDrift
	case strings.Contains(text, "migration") || strings.Contains(text, "ddl"):
		return ErrorMigrationFailed
	default:
		return ErrorUnknown
	}
}
