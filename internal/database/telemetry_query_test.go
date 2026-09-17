package database

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestBuildDrivingTelemetryQuery(t *testing.T) {
	t.Run("empty ranges", func(t *testing.T) {
		q, args := buildDrivingTelemetryQuery("veh-1", nil)
		if q != "" || args != nil {
			t.Errorf("expected empty query and nil args, got q=%q, args=%v", q, args)
		}
	})

	t.Run("single closed range", func(t *testing.T) {
		maxOdom := 15000.0
		ranges := []OdometerRange{
			{Min: 10000.0, Max: &maxOdom},
		}
		q, args := buildDrivingTelemetryQuery("veh-1", ranges)
		if len(args) != 3 {
			t.Fatalf("expected 3 arguments (vehicleID, min, max), got %d: %v", len(args), args)
		}
		if args[0] != "veh-1" || args[1] != 10000.0 || args[2] != 15000.0 {
			t.Errorf("unexpected args: %v", args)
		}
		if !strings.Contains(q, "(end_odometer >= $2 AND end_odometer <= $3)") {
			t.Errorf("expected range clause with $2 and $3, got: %s", q)
		}
		assertAllPlaceholdersBound(t, q, len(args))
	})

	t.Run("single open range (mounted tire with active session)", func(t *testing.T) {
		ranges := []OdometerRange{
			{Min: 20000.0, Max: nil},
		}
		q, args := buildDrivingTelemetryQuery("veh-1", ranges)
		if len(args) != 2 {
			t.Fatalf("expected 2 arguments (vehicleID, min), got %d: %v", len(args), args)
		}
		if args[0] != "veh-1" || args[1] != 20000.0 {
			t.Errorf("unexpected args: %v", args)
		}
		if !strings.Contains(q, "(end_odometer >= $2)") {
			t.Errorf("expected clause with $2, got: %s", q)
		}
		assertAllPlaceholdersBound(t, q, len(args))
	})

	t.Run("multiple ranges (closed past session + open active session)", func(t *testing.T) {
		maxOdom := 15000.0
		ranges := []OdometerRange{
			{Min: 10000.0, Max: &maxOdom},
			{Min: 25000.0, Max: nil},
		}
		q, args := buildDrivingTelemetryQuery("veh-1", ranges)
		if len(args) != 4 {
			t.Fatalf("expected 4 arguments, got %d: %v", len(args), args)
		}
		if args[0] != "veh-1" || args[1] != 10000.0 || args[2] != 15000.0 || args[3] != 25000.0 {
			t.Errorf("unexpected args: %v", args)
		}
		if !strings.Contains(q, "(end_odometer >= $2 AND end_odometer <= $3) OR (end_odometer >= $4)") {
			t.Errorf("unexpected clause in query: %s", q)
		}
		assertAllPlaceholdersBound(t, q, len(args))
	})
}

func assertAllPlaceholdersBound(t *testing.T, query string, argCount int) {
	t.Helper()
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(query, -1)
	for _, m := range matches {
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			t.Errorf("failed to parse placeholder %s: %v", m[0], err)
			continue
		}
		if idx < 1 || idx > argCount {
			t.Errorf("query contains placeholder $%d out of range [1, %d]. Query: %s", idx, argCount, query)
		}
	}
}
