package domain

import (
	"errors"
	"fmt"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func NewError(code, message string) error { return &Error{Code: code, Message: message} }

func WrapError(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal_error"
}

const (
	ErrInvalidInput        = "invalid_input"
	ErrInvalidState        = "invalid_state"
	ErrRevisionConflict    = "revision_conflict"
	ErrIdempotencyConflict = "idempotency_conflict"
	ErrNotFound            = "not_found"
	ErrIntegrity           = "integrity_error"
	ErrForbidden           = "forbidden"
)
