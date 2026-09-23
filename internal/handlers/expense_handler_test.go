package handlers

import (
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

func TestExpenseGroupNameFallsBackToTheAskedLanguage(t *testing.T) {
	notes := "Paris → Lyon"
	if got := expenseGroupName(&notes, "en"); got != notes {
		t.Errorf("notes take priority over the fallback: got %q", got)
	}
	if got := expenseGroupName(nil, "en"); got != "Multi-leg trip" {
		t.Errorf("got %q, want the English fallback", got)
	}
	if got := expenseGroupName(nil, "fr"); got != "Trajet multi-étapes" {
		t.Errorf("got %q, want the French fallback", got)
	}
}

func TestBuildDriveExpenseWithDocumentID(t *testing.T) {
	docID := "123e4567-e89b-12d3-a456-426614174000"
	req := &CreateDriveExpenseRequest{
		Type:       "TOLL",
		Amount:     1550,
		Currency:   "EUR",
		Date:       "2026-09-15T14:30:00Z",
		DocumentID: &docID,
	}

	exp, err := buildDriveExpense("veh-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exp.DocumentID == nil || *exp.DocumentID != docID {
		t.Fatalf("expected DocumentID %q, got %v", docID, exp.DocumentID)
	}
	if exp.Amount != 1550 || exp.Type != "TOLL" {
		t.Fatalf("unexpected exp: %+v", exp)
	}
}

func TestBuildMaintenanceExpenseWithDocumentID(t *testing.T) {
	docID := "123e4567-e89b-12d3-a456-426614174000"
	req := &CreateMaintenanceRequest{
		Category:         "MAINTENANCE",
		Amount:           45000,
		Currency:         "EUR",
		Date:             "2026-09-15",
		Description:      "Changement plaquettes",
		AmortizationMode: "DISTANCE",
		DocumentID:       &docID,
	}

	m, err := buildMaintenanceExpense("veh-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.DocumentID == nil || *m.DocumentID != docID {
		t.Fatalf("expected DocumentID %q, got %v", docID, m.DocumentID)
	}
	if m.Category != "MAINTENANCE" || m.Amount != 45000 {
		t.Fatalf("unexpected m: %+v", m)
	}
}

func TestBuildChargeWithDocumentID(t *testing.T) {
	docID := "123e4567-e89b-12d3-a456-426614174000"
	cost := money.Cents(1240)
	req := &SaveChargeRequest{
		Date:       "2026-09-15T10:00:00Z",
		KwhAdded:   35.5,
		Cost:       &cost,
		Currency:   "EUR",
		DocumentID: &docID,
	}

	c, err := buildCharge("veh-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.DocumentID == nil || *c.DocumentID != docID {
		t.Fatalf("expected DocumentID %q, got %v", docID, c.DocumentID)
	}
	if *c.Cost != 1240 || c.KwhAdded != 35.5 {
		t.Fatalf("unexpected charge: %+v", c)
	}
}
