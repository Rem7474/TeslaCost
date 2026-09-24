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
	nested, ok := err.Params["p1"].(*Error)
	if err.Params["p0"] != 3 || !ok || nested.Code != "inner.code" {
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

func TestNewfMarksDistancesForTheClient(t *testing.T) {
	err := Newf("tire.odometer_below_mount", "The odometer (%.0f km) is lower than %.0f km, %.1f kWh/100km", Km(41999.6), Km(42000), PerKm(16.25))
	if err.Message != "The odometer (42000 km) is lower than 42000 km, 16.2 kWh/100km" {
		t.Fatalf("English message: got %q", err.Message)
	}
	if err.Params["p0"] != "km:41999.6" || err.Params["p1"] != "km:42000" || err.Params["p2"] != "perkm:16.25" {
		t.Fatalf("params: got %v", err.Params)
	}
}
