// Package apierror defines the errors a client can be told about: a stable code the front end translates,
// an English message for API consumers and logs, and the values interpolated in the translation.
package apierror

import (
	"errors"
	"fmt"
)

// Error is a user-facing error. Code identifies the message in the front-end catalogs ("vehicle.not_found").
type Error struct {
	Code    string
	Message string
	// Params holds the interpolated values, named p0, p1, ... in the order of the format verbs.
	Params map[string]any
	cause  error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

// New creates an error without parameters.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf creates an error whose message and parameters come from a format and its arguments. An error argument
// wrapped with %w stays reachable through errors.Is / errors.As.
func Newf(code, format string, args ...any) *Error {
	wrapped := fmt.Errorf(format, args...)
	params := make(map[string]any, len(args))
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			params[fmt.Sprintf("p%d", i)] = err.Error()
		} else {
			params[fmt.Sprintf("p%d", i)] = arg
		}
	}
	return &Error{Code: code, Message: wrapped.Error(), Params: params, cause: errors.Unwrap(wrapped)}
}

// As reports whether err carries an *Error and returns it.
func As(err error) (*Error, bool) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
