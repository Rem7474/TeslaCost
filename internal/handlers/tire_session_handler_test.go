package handlers

import (
	"errors"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

func TestParseSessionPayloadDates(t *testing.T) {
	day := func(offset int) string { return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02") }
	ptr := func(s string) *string { return &s }
	payload := func(mounted string, dismounted *string) *MountSessionPayload {
		return &MountSessionPayload{Position: models.TirePosFL, MountedDate: mounted, DismountedDate: dismounted}
	}

	tests := []struct {
		name string
		req  *MountSessionPayload
		code string // "" when accepted
	}{
		{"ongoing session started today", payload(day(0), nil), ""},
		{"finished session in the past", payload(day(-120), ptr(day(-30))), ""},
		{"removal today", payload(day(-10), ptr(day(0))), ""},
		{"removal tomorrow tolerates the client time zone", payload(day(-10), ptr(day(1))), ""},
		{"removal months ahead", payload(day(-10), ptr(day(90))), "tire.session_in_future"},
		{"fitting months ahead", payload(day(90), nil), "tire.session_in_future"},
		{"removal before fitting", payload(day(-10), ptr(day(-20))), "tire.dismount_before_mount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseSessionPayload(tt.req)
			if tt.code == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var apiErr *apierror.Error
			if !errors.As(err, &apiErr) || apiErr.Code != tt.code {
				t.Fatalf("want code %s, got %v", tt.code, err)
			}
		})
	}
}
