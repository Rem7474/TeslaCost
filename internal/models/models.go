package models

import (
	"time"
)

// TirePosition represents wheel locations or storage status.
type TirePosition string

const (
	TirePosFL       TirePosition = "FL"
	TirePosFR       TirePosition = "FR"
	TirePosRL       TirePosition = "RL"
	TirePosRR       TirePosition = "RR"
	TirePosStorage  TirePosition = "STORAGE"
	TirePosDisposed TirePosition = "DISPOSED"
)

// TireSeason represents the tire category.
type TireSeason string

const (
	TireSeasonSummer    TireSeason = "SUMMER"
	TireSeasonWinter    TireSeason = "WINTER"
	TireSeasonAllSeason TireSeason = "ALL_SEASON"
)

// AuthMode represents authentication type for TeslaMate API.
type AuthMode string

const (
	AuthModeNone   AuthMode = "NONE"
	AuthModeBearer AuthMode = "BEARER"
	AuthModeBasic  AuthMode = "BASIC"
)

// User represents a registered user.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Vehicle represents an automobile owned by a user.
type Vehicle struct {
	ID                       string    `json:"id"`
	UserID                   string    `json:"user_id"`
	Name                     string    `json:"name"`
	Vin                      *string   `json:"vin,omitempty"`
	TeslaMateCarID           *int      `json:"teslamate_car_id,omitempty"`
	CurrentOdometer          float64   `json:"current_odometer"`
	TeslaMateAPIURL          *string   `json:"teslamate_api_url,omitempty"`
	TeslaMateAuthType        AuthMode  `json:"teslamate_auth_type"`
	TeslaMateAPIKeyEncrypted *string   `json:"-"`
	TeslaMateBasicUser       *string   `json:"teslamate_basic_user,omitempty"`
	TeslaMateBasicPassEnc    *string   `json:"-"`
	AnnualInsuranceCost      *float64  `json:"annual_insurance_cost,omitempty"`
	AnnualExpectedMileage    *float64  `json:"annual_expected_mileage,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// Drive represents a single vehicle trip.
type Drive struct {
	ID                  string    `json:"id"`
	VehicleID           string    `json:"vehicle_id"`
	TeslaMateDriveID    *int      `json:"teslamate_drive_id,omitempty"`
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	StartOdometer       *float64  `json:"start_odometer,omitempty"`
	EndOdometer         *float64  `json:"end_odometer,omitempty"`
	DistanceKm          float64   `json:"distance_km"`
	DurationMin         int       `json:"duration_min"`
	SpeedAvg            *float64  `json:"speed_avg,omitempty"`
	SpeedMax            *int      `json:"speed_max,omitempty"`
	PowerMax            *int      `json:"power_max,omitempty"`
	PowerMin            *int      `json:"power_min,omitempty"`
	StartAddress        *string   `json:"start_address,omitempty"`
	EndAddress          *string   `json:"end_address,omitempty"`
	EnergyConsumedKwh   *float64  `json:"energy_consumed_kwh,omitempty"`
	ConsumptionKwh100km *float64  `json:"consumption_kwh_100km,omitempty"`
	Tags                []string  `json:"tags"`
	IsManual            bool      `json:"is_manual"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// TripGroup allows grouping multiple drives (e.g. holiday trip with stops).
type TripGroup struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	Name      string    `json:"name"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DriveExpense holds costs directly attached to a drive or a trip group (tolls, parking).
type DriveExpense struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	TripGroupID   *string   `json:"trip_group_id,omitempty"`
	TripGroupName *string   `json:"trip_group_name,omitempty"`
	DriveID       *string   `json:"drive_id,omitempty"`
	DriveTitle    *string   `json:"drive_title,omitempty"`
	Type          string    `json:"type"` // TOLL, PARKING, etc.
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Date          time.Time `json:"date"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Tire represents an individual tire or set entry.
type Tire struct {
	ID              string       `json:"id"`
	VehicleID       *string      `json:"vehicle_id,omitempty"`
	Brand           string       `json:"brand"`
	Model           string       `json:"model"`
	Dimension       string       `json:"dimension"`
	Season          TireSeason   `json:"season"`
	PurchaseDate    time.Time    `json:"purchase_date"`
	PurchasePrice   float64      `json:"purchase_price"`
	CurrentPosition TirePosition `json:"current_position"`
	InitialDepthMm  float64      `json:"initial_depth_mm"`
	MinLegalDepthMm float64      `json:"min_legal_depth_mm"`
	DotCode               *string      `json:"dot_code,omitempty"`
	IsArchived            bool         `json:"is_archived"`
	MountedOdometer       *float64     `json:"mounted_odometer,omitempty"`
	AccumulatedDistanceKm float64      `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int          `json:"estimated_lifespan_km"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

// TireMountSession logs a specific period where a tire was mounted on a vehicle wheel.
type TireMountSession struct {
	ID                 string       `json:"id"`
	TireID             string       `json:"tire_id"`
	VehicleID          string       `json:"vehicle_id"`
	Position           TirePosition `json:"position"`
	MountedDate        time.Time    `json:"mounted_date"`
	MountedOdometer    float64      `json:"mounted_odometer"`
	DismountedDate     *time.Time   `json:"dismounted_date,omitempty"`
	DismountedOdometer *float64     `json:"dismounted_odometer,omitempty"`
	DistanceKm         float64      `json:"distance_km"`
	Notes              *string      `json:"notes,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// TireLog records a tread depth measurement.
type TireLog struct {
	ID        string    `json:"id"`
	TireID    string    `json:"tire_id"`
	Date      time.Time `json:"date"`
	Odometer  float64   `json:"odometer"`
	DepthMm   float64   `json:"depth_mm"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TireRotation stores wheel swap events across the vehicle.
type TireRotation struct {
	ID          string         `json:"id"`
	VehicleID   string         `json:"vehicle_id"`
	Date        time.Time      `json:"date"`
	Odometer    float64        `json:"odometer"`
	MappingJSON map[string]any `json:"mapping_json"` // e.g. {"FL": "<uuid>", "FR": "<uuid>"}
	Notes       *string        `json:"notes,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// MaintenanceExpense represents vehicle maintenance, insurance, subscriptions, etc.
type MaintenanceExpense struct {
	ID                       string    `json:"id"`
	VehicleID                string    `json:"vehicle_id"`
	Category                 string    `json:"category"`
	Amount                   float64   `json:"amount"`
	Currency                 string    `json:"currency"`
	Date                     time.Time `json:"date"`
	Odometer                 *float64  `json:"odometer,omitempty"`
	IsRecurring              bool      `json:"is_recurring"`
	RecurrenceIntervalMonths *int      `json:"recurrence_interval_months,omitempty"`
	Description              string    `json:"description"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// ChargeLog records an EV charging event with costs and kWh.
type ChargeLog struct {
	ID                string     `json:"id"`
	VehicleID         string     `json:"vehicle_id"`
	TeslaMateChargeID *int       `json:"teslamate_charge_id,omitempty"`
	Date              time.Time  `json:"date"`
	EndDate           *time.Time `json:"end_date,omitempty"`
	Address           *string    `json:"address,omitempty"`
	KwhAdded          float64    `json:"kwh_added"`
	KwhUsed           *float64   `json:"kwh_used,omitempty"`
	Cost              float64    `json:"cost"`
	Currency          string     `json:"currency"`
	Odometer          *float64   `json:"odometer,omitempty"`
	IsManual          bool       `json:"is_manual"`
	CreatedAt         time.Time  `json:"created_at"`
}

// CarpoolTrip represents a shared trip (e.g. BlaBlaCar) with detailed real cost breakdown and passenger revenues.
type CarpoolTrip struct {
	ID              string    `json:"id"`
	VehicleID       string    `json:"vehicle_id"`
	DriveID         *string   `json:"drive_id,omitempty"`
	TripGroupID     *string   `json:"trip_group_id,omitempty"`
	Title           string    `json:"title"`
	Date            time.Time `json:"date"`
	DistanceKm      float64   `json:"distance_km"`
	ElectricityCost float64   `json:"electricity_cost"`
	TollsCost       float64   `json:"tolls_cost"`
	TiresCost       float64   `json:"tires_cost"`
	MaintenanceCost float64   `json:"maintenance_cost"`
	InsuranceCost   float64   `json:"insurance_cost"`
	OtherCost       float64   `json:"other_cost"`
	TotalCost       float64   `json:"total_cost"`
	TotalRevenue    float64   `json:"total_revenue"`
	NetCost         float64   `json:"net_cost"`
	Notes           *string   `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CarpoolPassenger represents a booking / passenger contribution for a full trip or sub-leg ("bout de trajet").
type CarpoolPassenger struct {
	ID            string    `json:"id"`
	CarpoolTripID string    `json:"carpool_trip_id"`
	PassengerName string    `json:"passenger_name"`
	Origin        *string   `json:"origin,omitempty"`
	Destination   *string   `json:"destination,omitempty"`
	Seats         int       `json:"seats"`
	AmountPaid    float64   `json:"amount_paid"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CarpoolTripWithPassengers bundles a trip with its passengers.
type CarpoolTripWithPassengers struct {
	CarpoolTrip
	Passengers []CarpoolPassenger `json:"passengers"`
}

// CarpoolCostEstimate provides suggested real cost breakdown based on vehicle TCO metrics.
type CarpoolCostEstimate struct {
	DistanceKm            float64 `json:"distance_km"`
	ElectricityCost       float64 `json:"electricity_cost"`
	TollsCost             float64 `json:"tolls_cost"`
	TiresCost             float64 `json:"tires_cost"`
	MaintenanceCost       float64 `json:"maintenance_cost"`
	InsuranceCost         float64 `json:"insurance_cost"`
	OtherCost             float64 `json:"other_cost"`
	TotalCost             float64 `json:"total_cost"`
	ElectricityRatePerKwh float64  `json:"electricity_rate_per_kwh"`
	TiresRatePerKm        float64  `json:"tires_rate_per_km"`
	MaintenanceRatePerKm  float64  `json:"maintenance_rate_per_km"`
	InsuranceRatePerKm    float64  `json:"insurance_rate_per_km"`
	InsuranceSource       string   `json:"insurance_source"` // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	AnnualInsuranceCost   *float64 `json:"annual_insurance_cost,omitempty"`
	AnnualExpectedMileage *float64 `json:"annual_expected_mileage,omitempty"`
}

// CarpoolSummary aggregates global carpooling KPIs for the vehicle.
type CarpoolSummary struct {
	TotalTrips      int     `json:"total_trips"`
	TotalPassengers int     `json:"total_passengers"`
	TotalDistanceKm float64 `json:"total_distance_km"`
	TotalRealCost   float64 `json:"total_real_cost"`
	TotalRevenue    float64 `json:"total_revenue"`
	TotalNetCost    float64 `json:"total_net_cost"`
	TotalSaved      float64 `json:"total_saved"`
	CoverageRatePct float64 `json:"coverage_rate_pct"`
	NetCostPerKm    float64 `json:"net_cost_per_km"`
}

