package servertext

import "testing"

func TestEveryKeyHasBothLanguages(t *testing.T) {
	for key, entry := range catalog {
		if entry["en"] == "" {
			t.Errorf("%q has no English text", key)
		}
		if entry["fr"] == "" {
			t.Errorf("%q has no French text", key)
		}
	}
}

func TestTextFallsBackToEnglishForAnUnknownLanguage(t *testing.T) {
	if got := Text("de", "trip.departure"); got != "Start" {
		t.Errorf("got %q, want the English fallback", got)
	}
}

func TestTextFormatsWithArgs(t *testing.T) {
	if got := Text("en", "reminder.gotify_title", "Oil change", "Due soon"); got != "AutoLedger: Oil change (Due soon)" {
		t.Errorf("got %q", got)
	}
}

func TestTextReturnsTheKeyItselfWhenUnknown(t *testing.T) {
	if got := Text("en", "no.such.key"); got != "no.such.key" {
		t.Errorf("got %q, want the key echoed back", got)
	}
}
