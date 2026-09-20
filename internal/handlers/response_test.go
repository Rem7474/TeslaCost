package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/apierror"
)

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestWriteErrSendsTheCodeAndParamsOfAnAPIError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErr(rr, http.StatusBadRequest, apierror.Newf("tire.not_in_storage", "The tire %s is not in storage", "abc"))
	body := decodeBody(t, rr)
	if rr.Code != http.StatusBadRequest || body["code"] != "tire.not_in_storage" || body["error"] != "The tire abc is not in storage" {
		t.Fatalf("got %d %v", rr.Code, body)
	}
	if params, _ := body["params"].(map[string]any); params["p0"] != "abc" {
		t.Errorf("params = %v", body["params"])
	}
}

func TestWriteErrKeepsAPlainErrorWithoutCode(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErr(rr, http.StatusBadRequest, errors.New("plain"))
	body := decodeBody(t, rr)
	if body["error"] != "plain" || body["code"] != nil {
		t.Errorf("got %v", body)
	}
}

func TestWriteErrFindsAnAPIErrorInsideAWrappedError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErr(rr, http.StatusNotFound, errors.Join(errors.New("ctx"), apierror.New("vehicle.not_found", "Vehicle not found")))
	if body := decodeBody(t, rr); body["code"] != "vehicle.not_found" {
		t.Errorf("got %v", body)
	}
}

// errorCode is the code of an API error, or the empty string for any other error.
func errorCode(err error) string {
	if apiErr, ok := apierror.As(err); ok {
		return apiErr.Code
	}
	return ""
}
