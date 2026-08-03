// Package errors defines HermiNas' typed error vocabulary for the Go
// control plane. The Code values mirror (conceptually, not by shared code)
// the taxonomy used in the Rust kernel (rust/kernel/src/errors.rs) and the
// Python kernel (python/src/herminas_kernel/errors.py), so the same class
// of failure carries the same name across languages.
package errors

import (
	stderrors "errors"
	"fmt"
)

type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeNotFound        Code = "not_found"
	CodeUnauthorized    Code = "unauthorized"
	CodeInternal        Code = "internal"
	CodeUnavailable     Code = "unavailable"
	CodeAlreadyExists   Code = "already_exists"
)

// Error is HermiNas' standard error type: a stable code plus a
// human-readable message and an optional wrapped cause.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// Is reports whether err is (or wraps) a HermiNas *Error with the given
// code, so callers can branch on failure class without a type switch.
func Is(err error, code Code) bool {
	var herr *Error
	return stderrors.As(err, &herr) && herr.Code == code
}

func IsNotFound(err error) bool        { return Is(err, CodeNotFound) }
func IsAlreadyExists(err error) bool   { return Is(err, CodeAlreadyExists) }
func IsInvalidArgument(err error) bool { return Is(err, CodeInvalidArgument) }
