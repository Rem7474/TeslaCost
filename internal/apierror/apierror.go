// Package apierror defines the errors a client can be told about: a stable code the front end translates,
// an English message for API consumers and logs, and the values interpolated in the translation.
package apierror

import (
	"errors"
	"fmt"
	"strconv"
)

// Error is a user-facing error. Code identifies the message in the front-end catalogs ("vehicle.not_found").
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Params holds the interpolated values, named p0, p1, ... in the order of the format verbs.
	Params map[string]any `json:"params,omitempty"`
	cause  error
}

// Message is what a payload carries when it holds text for the user that is not a failure (a warning, an
// assumption, a label): the same code, English text and parameters as an error, serialized as an object.
type Message = Error

// NewMessage is New under the name that fits a payload text.
func NewMessage(code, text string) *Message { return New(code, text) }

// NewMessagef is Newf under the name that fits a payload text.
func NewMessagef(code, format string, args ...any) *Message { return Newf(code, format, args...) }

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

// Km is a distance in kilometres passed to Newf: the English message formats it like a float64 ("%.0f km"),
// and the parameter reaches the client as "km:<value>" so the front end shows it in the account's unit.
type Km float64

// Format prints the distance as its float64 value, so format verbs keep their meaning.
func (k Km) Format(f fmt.State, verb rune) { fmt.Fprintf(f, fmt.FormatString(f, verb), float64(k)) }

// PerKm is a figure per km or per 100 km (a consumption, a price per km), sent as "perkm:<value>" for the
// front end to rescale to the account's unit; formatted like a float64 in the English message.
type PerKm float64

// Format prints the figure as its float64 value.
func (p PerKm) Format(f fmt.State, verb rune) { fmt.Fprintf(f, fmt.FormatString(f, verb), float64(p)) }

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
			// A nested user-facing error keeps its code so the front end can translate it too.
			if nested, isAPI := As(err); isAPI {
				params[fmt.Sprintf("p%d", i)] = nested
			} else {
				params[fmt.Sprintf("p%d", i)] = err.Error()
			}
		} else if km, ok := arg.(Km); ok {
			params[fmt.Sprintf("p%d", i)] = "km:" + strconv.FormatFloat(float64(km), 'f', -1, 64)
		} else if perKm, ok := arg.(PerKm); ok {
			params[fmt.Sprintf("p%d", i)] = "perkm:" + strconv.FormatFloat(float64(perKm), 'f', -1, 64)
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
