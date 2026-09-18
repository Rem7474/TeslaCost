package handlers

import (
	"strings"
	"testing"
)

func TestNormalizeGrafanaURL(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		name    string
		in      *string
		want    string // "" means nil
		wantErr string
	}{
		{"absent", nil, "", ""},
		{"blank clears", s("   "), "", ""},
		{"plain http", s("http://192.168.1.50:3000"), "http://192.168.1.50:3000", ""},
		{"trailing slash is dropped", s("https://grafana.example.com/"), "https://grafana.example.com", ""},
		{"sub path is kept", s(" https://home.example.com/grafana/ "), "https://home.example.com/grafana", ""},
		{"no scheme", s("grafana.local:3000"), "", "invalide"},
		{"unsupported scheme", s("ftp://grafana.local"), "", "invalide"},
		{"no host", s("http://"), "", "invalide"},
		{"credentials refused", s("http://admin:secret@grafana.local"), "", "identifiants"},
		{"query refused", s("http://grafana.local/?orgId=1"), "", "sans paramètres"},
		{"fragment refused", s("http://grafana.local/#top"), "", "sans paramètres"},
		{"too long", s("http://" + strings.Repeat("a", 300) + ".com"), "", "trop longue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeGrafanaURL(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == "" {
				if got != nil {
					t.Fatalf("got %q, want nil", *got)
				}
				return
			}
			if got == nil || *got != tt.want {
				t.Fatalf("got %v, want %q", got, tt.want)
			}
		})
	}
}
