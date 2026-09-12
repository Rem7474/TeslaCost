package teslamate

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientAuthentication(t *testing.T) {
	// Test Bearer Auth
	bearerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"cars":[{"car_id":1,"name":"Model 3","car_details":{"model":"3","trim_badging":"Performance","vin":"5YJ3E1EB..."}}]}}`))
	}))
	defer bearerServer.Close()

	client, err := NewClient(Config{
		BaseURL:  bearerServer.URL,
		AuthType: AuthBearer,
		APIToken: "secret-token-123",
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	cars, err := client.GetCars(context.Background())
	if err != nil {
		t.Fatalf("GetCars failed: %v", err)
	}
	if len(cars) != 1 || cars[0].Name != "Model 3" {
		t.Fatalf("Unexpected cars result: %+v", cars)
	}

	// Test Basic Auth
	basicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secretpass"))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"car":{"car_id":1,"car_name":"Model Y"},"status":{"display_name":"Model Y","state":"online","odometer":34521.8},"units":{"unit_of_length":"km"}}}`))
	}))
	defer basicServer.Close()

	basicClient, err := NewClient(Config{
		BaseURL:  basicServer.URL,
		AuthType: AuthBasic,
		Username: "admin",
		Password: "secretpass",
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	status, units, err := basicClient.GetCarStatus(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetCarStatus failed: %v", err)
	}
	if status.Odometer != 34521.8 || units.UnitOfLength != "km" {
		t.Fatalf("Unexpected status result: %+v, units: %+v", status, units)
	}
}

func TestGetDrivesAndCharges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/cars/1/drives":
			if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("show") != "50" {
				t.Errorf("Unexpected query params: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"car": {"car_id": 1, "car_name": "Model 3"},
					"drives": [
						{
							"drive_id": 101,
							"start_date": "2026-09-10T08:30:00Z",
							"end_date": "2026-09-10T09:15:00Z",
							"start_address": "Paris",
							"end_address": "Versailles",
							"odometer_details": {
								"odometer_start": 10000.0,
								"odometer_end": 10025.5,
								"odometer_distance": 25.5
							},
							"duration_min": 45,
							"speed_avg": 34.0,
							"power_max": 120
						}
					],
					"units": {"unit_of_length": "km", "unit_of_temperature": "C"}
				}
			}`))
		case "/api/v1/cars/1/charges":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"car": {"car_id": 1, "car_name": "Model 3"},
					"charges": [
						{
							"charge_id": 201,
							"start_date": "2026-09-10T12:00:00Z",
							"end_date": "2026-09-10T12:40:00Z",
							"address": "Supercharger Orgeval",
							"charge_energy_added": 42.5,
							"charge_energy_used": 45.0,
							"cost": 14.85,
							"odometer": 10025.5
						}
					],
					"units": {"unit_of_length": "km", "unit_of_temperature": "C"}
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Test GetDrives
	drives, units, err := client.GetDrives(context.Background(), 1, DriveFilterOptions{Page: 1, Show: 50})
	if err != nil {
		t.Fatalf("GetDrives failed: %v", err)
	}
	if len(drives) != 1 {
		t.Fatalf("Expected 1 drive, got %d", len(drives))
	}
	if drives[0].DriveID != 101 || drives[0].OdometerDetails.OdometerDistance != 25.5 {
		t.Fatalf("Unexpected drive payload: %+v", drives[0])
	}
	if units.UnitOfLength != "km" {
		t.Fatalf("Expected km unit, got %s", units.UnitOfLength)
	}

	// Test GetCharges
	charges, _, err := client.GetCharges(context.Background(), 1, ChargeFilterOptions{})
	if err != nil {
		t.Fatalf("GetCharges failed: %v", err)
	}
	if len(charges) != 1 {
		t.Fatalf("Expected 1 charge, got %d", len(charges))
	}
	if charges[0].ChargeID != 201 || charges[0].Cost != 14.85 || charges[0].ChargeEnergyAdded != 42.5 {
		t.Fatalf("Unexpected charge payload: %+v", charges[0])
	}
}

func TestDistanceConversion(t *testing.T) {
	km := ConvertDistanceToKm(10.0, "km")
	if km != 10.0 {
		t.Errorf("Expected 10.0 km, got %f", km)
	}

	miInKm := ConvertDistanceToKm(10.0, "mi")
	expected := 16.09344
	if miInKm != expected {
		t.Errorf("Expected %f km, got %f", expected, miInKm)
	}
}
