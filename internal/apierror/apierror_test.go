package apierror

import (
	"errors"
	"testing"
)

func TestNewfKeepsMessageParamsAndCause(t *testing.T) {
	inner := New("inner.code", "inner problem")
	err := Newf("outer.code", "leg %d: %w", 3, inner)
	if err.Message != "leg 3: inner problem" || err.Code != "outer.code" {
		t.Fatalf("got %q / %q", err.Code, err.Message)
	}
	if err.Params["p0"] != 3 || err.Params["p1"] != "inner problem" {
		t.Errorf("params = %v", err.Params)
	}
	if !errors.Is(err, inner) {
		t.Error("the wrapped error must stay reachable")
	}
}

func TestAsFindsAWrappedError(t *testing.T) {
	wrapped := errors.Join(errors.New("x"), New("a.b", "msg"))
	got, ok := As(wrapped)
	if !ok || got.Code != "a.b" {
		t.Errorf("As = %v, %v", got, ok)
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Error("a plain error is not an API error")
	}
}
