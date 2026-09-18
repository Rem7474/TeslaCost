package services

import (
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func strPtr(s string) *string { return &s }

func TestDecideTollAction(t *testing.T) {
	group := "g1"
	tests := []struct {
		name     string
		existing []models.DriveExpense
		want     tollAction
		wantID   string
	}{
		{"no expense creates", nil, tollActionCreate, ""},
		{"non-toll expense ignored", []models.DriveExpense{{Type: "PARKING", Source: models.ExpenseSourceManual}}, tollActionCreate, ""},
		{"manual toll is never overwritten", []models.DriveExpense{{ID: "m", Type: "TOLL", Source: models.ExpenseSourceManual}}, tollActionSkipManual, ""},
		{"empty source is treated as manual", []models.DriveExpense{{ID: "m", Type: "TOLL"}}, tollActionSkipManual, ""},
		{"trip group toll is never touched", []models.DriveExpense{{ID: "g", Type: "TOLL", TripGroupID: &group, Source: models.ExpenseSourceAutoToll}}, tollActionSkipGroup, ""},
		{"earlier auto toll is updated in place", []models.DriveExpense{{ID: "a", Type: "TOLL", Source: models.ExpenseSourceAutoToll}}, tollActionUpdate, "a"},
		{"manual wins over auto", []models.DriveExpense{
			{ID: "a", Type: "TOLL", Source: models.ExpenseSourceAutoToll},
			{ID: "m", Type: "TOLL", Source: models.ExpenseSourceManual},
		}, tollActionSkipManual, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, id := decideTollAction(tt.existing)
			if got != tt.want || id != tt.wantID {
				t.Errorf("got (%v, %q), want (%v, %q)", got, id, tt.want, tt.wantID)
			}
		})
	}
}

func TestSumEstimatedPrice(t *testing.T) {
	p := func(v float64) *money.Cents { c := money.FromFloat(v); return &c }

	total, ok := sumEstimatedPrice([]models.TollSegment{{EstimatedPrice: p(12.30)}, {}, {EstimatedPrice: p(1.10)}})
	if !ok || total != money.FromFloat(13.40) {
		t.Errorf("got (%v, %v), want (13.40, true)", total, ok)
	}
	if _, ok := sumEstimatedPrice([]models.TollSegment{{Entry: "A"}}); ok {
		t.Error("unpriced segments must not be applicable")
	}
	if _, ok := sumEstimatedPrice(nil); ok {
		t.Error("no segments must not be applicable")
	}
}

func TestAutoTollNotes(t *testing.T) {
	notes := autoTollNotes([]models.TollSegment{
		{Type: "close", Entry: "ANNECY CENTRE", Exit: strPtr("LES ABRETS")},
		{Type: "open", Entry: "CHESNES"},
		{Type: "close", Entry: "VIENNE"},
	})
	for _, want := range []string{"Péage auto", "ANNECY CENTRE → LES ABRETS", "Barrière CHESNES", "VIENNE (sortie non identifiée)"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes %q missing %q", notes, want)
		}
	}
}
