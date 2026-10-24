// Package apperr holds the error kinds modules return; the HTTP layer maps them to status codes.
package apperr

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindNotFound
	KindUnauthorized
	KindConflict    // state changed underneath the caller
	KindUnavailable // an upstream dependency (payment gateway) failed
)

type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func Invalid(format string, a ...any) error {
	return &Error{KindInvalid, fmt.Sprintf(format, a...)}
}

func NotFound(format string, a ...any) error {
	return &Error{KindNotFound, fmt.Sprintf(format, a...)}
}

func Unauthorized(format string, a ...any) error {
	return &Error{KindUnauthorized, fmt.Sprintf(format, a...)}
}

func Conflict(format string, a ...any) error {
	return &Error{KindConflict, fmt.Sprintf(format, a...)}
}

func Unavailable(format string, a ...any) error {
	return &Error{KindUnavailable, fmt.Sprintf(format, a...)}
}

// KindOf reports the kind of err if it is (or wraps) an *Error.
func KindOf(err error) (Kind, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind, true
	}
	return 0, false
}
