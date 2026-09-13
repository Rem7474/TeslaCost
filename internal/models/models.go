package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
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
	ID                       string       `json:"id"`
	UserID                   string       `json:"user_id"`
	Name                     string       `json:"name"`
	Vin                      *string      `json:"vin,omitempty"`
	TeslaMateCarID           *int         `json:"teslamate_car_id,omitempty"`
	CurrentOdometer          float64      `json:"current_odometer"`
	TeslaMateAPIURL          *string      `json:"teslamate_api_url,omitempty"`
	TeslaMateAuthType        AuthMode     `json:"teslamate_auth_type"`
	TeslaMateAPIKeyEncrypted *string      `json:"-"`
	TeslaMateBasicUser       *string      `json:"teslamate_basic_user,omitempty"`
	TeslaMateBasicPassEnc    *string      `json:"-"`
	AnnualInsuranceCost      *money.Cents `json:"annual_insurance_cost,omitempty"`
	AnnualExpectedMileage    *float64     `json:"annual_expected_mileage,omitempty"`
	// Acquisition: PURCHASE (depreciation from price, incentives, resale value and holding period) or LEASE
	AcquisitionType       *string      `json:"acquisition_type,omitempty"`
	PurchasePrice         *money.Cents `json:"purchase_price,omitempty"`
	PurchaseDate          *time.Time   `json:"purchase_date,omitempty"`
	PurchaseOdometer      *float64     `json:"purchase_odometer,omitempty"`
	PurchaseIncentives    *money.Cents `json:"purchase_incentives,omitempty"`
	ExpectedResaleValue   *money.Cents `json:"expected_resale_value,omitempty"`
	ExpectedHoldingMonths *int         `json:"expected_holding_months,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

// Drive represents a single vehicle trip.
type Drive struct {
	ID                  string     `json:"id"`
	VehicleID           string     `json:"vehicle_id"`
	TeslaMateDriveID    *int       `json:"teslamate_drive_id,omitempty"`
	StartTime           time.Time  `json:"start_time"`
	EndTime             time.Time  `json:"end_time"`
	StartOdometer       *float64   `json:"start_odometer,omitempty"`
	EndOdometer         *float64   `json:"end_odometer,omitempty"`
	DistanceKm          float64    `json:"distance_km"`
	DurationMin         int        `json:"duration_min"`
	SpeedAvg            *float64   `json:"speed_avg,omitempty"`
	SpeedMax            *int       `json:"speed_max,omitempty"`
	PowerMax            *int       `json:"power_max,omitempty"`
	PowerMin            *int       `json:"power_min,omitempty"`
	StartAddress        *string    `json:"start_address,omitempty"`
	EndAddress          *string    `json:"end_address,omitempty"`
	EnergyConsumedKwh   *float64   `json:"energy_consumed_kwh,omitempty"`
	ConsumptionKwh100km *float64   `json:"consumption_kwh_100km,omitempty"`
	Tags                []string   `json:"tags"`
	IsManual            bool       `json:"is_manual"`
	TollReviewedAt      *time.Time `json:"toll_reviewed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
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
	ID                string       `json:"id"`
	VehicleID         string       `json:"vehicle_id"`
	TripGroupID       *string      `json:"trip_group_id,omitempty"`
	TripGroupName     *string      `json:"trip_group_name,omitempty"`
	DriveID           *string      `json:"drive_id,omitempty"`
	DriveTitle        *string      `json:"drive_title,omitempty"`
	TripGroupDriveIDs []string     `json:"trip_group_drive_ids,omitempty"`
	Type              string       `json:"type"` // TOLL, PARKING, etc.
	Amount            money.Cents  `json:"amount"`
	Currency          string       `json:"currency"`
	FxRate            *float64     `json:"fx_rate,omitempty"`          // Conversion rate to EUR when Currency != EUR
	AllocatedAmount   *money.Cents `json:"allocated_amount,omitempty"` // Share allocated to a given drive (EUR)
	Date              time.Time    `json:"date"`
	Notes             *string      `json:"notes,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
}

// Tire represents an individual tire or set entry.
type Tire struct {
	ID                    string       `json:"id"`
	VehicleID             *string      `json:"vehicle_id,omitempty"`
	Brand                 string       `json:"brand"`
	Model                 string       `json:"model"`
	Dimension             string       `json:"dimension"`
	Season                TireSeason   `json:"season"`
	PurchaseDate          time.Time    `json:"purchase_date"`
	PurchasePrice         money.Cents  `json:"purchase_price"`
	CurrentPosition       TirePosition `json:"current_position"`
	InitialDepthMm        float64      `json:"initial_depth_mm"`
	MinLegalDepthMm       float64      `json:"min_legal_depth_mm"`
	DotCode               *string      `json:"dot_code,omitempty"`
	IsArchived            bool         `json:"is_archived"`
	MountedOdometer       *float64     `json:"mounted_odometer,omitempty"`
	InitialDistanceKm     float64      `json:"initial_distance_km"`
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
	ID                       string      `json:"id"`
	VehicleID                string      `json:"vehicle_id"`
	Category                 string      `json:"category"`
	Amount                   money.Cents `json:"amount"`
	Currency                 string      `json:"currency"`
	FxRate                   *float64    `json:"fx_rate,omitempty"`
	Date                     time.Time   `json:"date"`
	Odometer                 *float64    `json:"odometer,omitempty"`
	IsRecurring              bool        `json:"is_recurring"`
	RecurrenceIntervalMonths *int        `json:"recurrence_interval_months,omitempty"`
	RecurrenceEndDate        *time.Time  `json:"recurrence_end_date,omitempty"`
	Description              string      `json:"description"`
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
}

// ChargeLog records an EV charging event with costs and kWh.
type ChargeLog struct {
	ID                string       `json:"id"`
	VehicleID         string       `json:"vehicle_id"`
	TeslaMateChargeID *int         `json:"teslamate_charge_id,omitempty"`
	Date              time.Time    `json:"date"`
	EndDate           *time.Time   `json:"end_date,omitempty"`
	Address           *string      `json:"address,omitempty"`
	KwhAdded          float64      `json:"kwh_added"`
	KwhUsed           *float64     `json:"kwh_used,omitempty"`
	Cost              *money.Cents `json:"cost"`        // nil = unknown cost (to be completed)
	CostSource        string       `json:"cost_source"` // TESLAMATE | MANUAL
	Currency          string       `json:"currency"`
	FxRate            *float64     `json:"fx_rate,omitempty"`
	Odometer          *float64     `json:"odometer,omitempty"`
	IsManual          bool         `json:"is_manual"`
	Notes             *string      `json:"notes,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
}

// CarpoolTrip represents a shared trip (e.g. BlaBlaCar) with detailed real cost breakdown and passenger revenues.
type CarpoolTrip struct {
	ID              string      `json:"id"`
	VehicleID       string      `json:"vehicle_id"`
	DriveID         *string     `json:"drive_id,omitempty"`
	TripGroupID     *string     `json:"trip_group_id,omitempty"`
	Title           string      `json:"title"`
	Date            time.Time   `json:"date"`
	DistanceKm      float64     `json:"distance_km"`
	ElectricityCost money.Cents `json:"electricity_cost"`
	TollsCost       money.Cents `json:"tolls_cost"`
	TiresCost       money.Cents `json:"tires_cost"`
	MaintenanceCost money.Cents `json:"maintenance_cost"`
	InsuranceCost   money.Cents `json:"insurance_cost"`
	OtherCost       money.Cents `json:"other_cost"`
	TotalCost       money.Cents `json:"total_cost"`
	TotalRevenue    money.Cents `json:"total_revenue"`
	NetCost         money.Cents `json:"net_cost"`
	Notes           *string     `json:"notes,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// CarpoolPassenger represents a booking / passenger contribution for a full trip or sub-leg ("bout de trajet").
type CarpoolPassenger struct {
	ID            string      `json:"id"`
	CarpoolTripID string      `json:"carpool_trip_id"`
	PassengerName string      `json:"passenger_name"`
	Origin        *string     `json:"origin,omitempty"`
	Destination   *string     `json:"destination,omitempty"`
	Seats         int         `json:"seats"`
	AmountPaid    money.Cents `json:"amount_paid"`
	Notes         *string     `json:"notes,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

// CarpoolTripWithPassengers bundles a trip with its passengers.
type CarpoolTripWithPassengers struct {
	CarpoolTrip
	Passengers []CarpoolPassenger `json:"passengers"`
}

// CarpoolCostEstimate provides suggested real cost breakdown based on vehicle TCO metrics.
type CarpoolCostEstimate struct {
	DistanceKm            float64      `json:"distance_km"`
	ElectricityCost       money.Cents  `json:"electricity_cost"`
	TollsCost             money.Cents  `json:"tolls_cost"`
	TiresCost             money.Cents  `json:"tires_cost"`
	MaintenanceCost       money.Cents  `json:"maintenance_cost"`
	InsuranceCost         money.Cents  `json:"insurance_cost"`
	OtherCost             money.Cents  `json:"other_cost"`
	TotalCost             money.Cents  `json:"total_cost"`
	ElectricityRatePerKwh float64      `json:"electricity_rate_per_kwh"`
	TiresRatePerKm        float64      `json:"tires_rate_per_km"`
	MaintenanceRatePerKm  float64      `json:"maintenance_rate_per_km"`
	InsuranceRatePerKm    float64      `json:"insurance_rate_per_km"`
	InsuranceSource       string       `json:"insurance_source"` // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	AnnualInsuranceCost   *money.Cents `json:"annual_insurance_cost,omitempty"`
	AnnualExpectedMileage *float64     `json:"annual_expected_mileage,omitempty"`
	EnergySource          string       `json:"energy_source"`           // MEASURED | CONSUMPTION | DEFAULT
	ElectricityRateSource string       `json:"electricity_rate_source"` // HISTORY | DEFAULT
	TiresRateSource       string       `json:"tires_rate_source"`       // MOUNTED_TIRES | HISTORY | DEFAULT
	MaintenanceRateSource string       `json:"maintenance_rate_source"` // HISTORY | DEFAULT
}

// CarpoolSummary aggregates global carpooling KPIs for the vehicle.
type CarpoolSummary struct {
	TotalTrips      int         `json:"total_trips"`
	TotalPassengers int         `json:"total_passengers"`
	TotalDistanceKm float64     `json:"total_distance_km"`
	TotalRealCost   money.Cents `json:"total_real_cost"`
	TotalRevenue    money.Cents `json:"total_revenue"`
	TotalNetCost    money.Cents `json:"total_net_cost"`
	TotalSaved      money.Cents `json:"total_saved"`
	CoverageRatePct float64     `json:"coverage_rate_pct"`
	NetCostPerKm    float64     `json:"net_cost_per_km"`
}

// DataQualityIssue reports an odometer continuity problem on synchronized drives.
type DataQualityIssue struct {
	Type            string    `json:"type"` // ODOMETER_GAP | ODOMETER_REGRESSION | DISTANCE_MISMATCH
	DriveID         string    `json:"drive_id"`
	PreviousDriveID *string   `json:"previous_drive_id,omitempty"`
	Date            time.Time `json:"date"`
	Km              float64   `json:"km"`
}
