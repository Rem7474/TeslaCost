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
	PasswordHash *string   `json:"-"` // nullable: OIDC accounts have no local password
	OIDCSubject  *string   `json:"-"`
	OIDCProvider *string   `json:"-"`
	DisplayName  *string   `json:"display_name,omitempty"` // from IdP "name" claim
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// VehicleRole represents the role of a user on a vehicle.
type VehicleRole string

const (
	RoleOwner  VehicleRole = "OWNER"
	RoleEditor VehicleRole = "EDITOR"
	RoleViewer VehicleRole = "VIEWER"
)

func (r VehicleRole) IsValid() bool {
	return r == RoleOwner || r == RoleEditor || r == RoleViewer
}

func (r VehicleRole) CanEdit() bool {
	return r == RoleOwner || r == RoleEditor
}

// Vehicle represents an automobile owned by a user.
type Vehicle struct {
	ID                       string      `json:"id"`
	UserID                   string      `json:"user_id"`
	Role                     VehicleRole `json:"role,omitempty"`
	Name                     string      `json:"name"`
	Vin                      *string     `json:"vin,omitempty"`
	TeslaMateCarID           *int        `json:"teslamate_car_id,omitempty"`
	CurrentOdometer          float64     `json:"current_odometer"`
	TeslaMateAPIURL          *string     `json:"teslamate_api_url,omitempty"`
	TeslaMateAuthType        AuthMode    `json:"teslamate_auth_type"`
	TeslaMateAPIKeyEncrypted *string     `json:"-"`
	TeslaMateBasicUser       *string     `json:"teslamate_basic_user,omitempty"`
	TeslaMateBasicPassEnc    *string     `json:"-"`
	PreTeslaMateKwh100km     *float64    `json:"pre_teslamate_kwh_100km,omitempty"`
	PreTeslaMateEurPerKwh    *float64    `json:"pre_teslamate_eur_per_kwh,omitempty"`
	Powertrain               string      `json:"powertrain"` // PowertrainEV | PowertrainICE
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
}

// Vehicle powertrains. ICE vehicles are tracked manually (fuel fill-ups) and have no TeslaMate link.
const (
	PowertrainEV  = "EV"
	PowertrainICE = "ICE"
)

// VehicleMember represents a user who has access to a vehicle with a specific role.
type VehicleMember struct {
	VehicleID   string      `json:"vehicle_id"`
	UserID      string      `json:"user_id"`
	Role        VehicleRole `json:"role"`
	UserEmail   string      `json:"user_email"`
	DisplayName *string     `json:"display_name,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type AddVehicleMemberRequest struct {
	Email string      `json:"email"`
	Role  VehicleRole `json:"role"`
}

type UpdateVehicleMemberRoleRequest struct {
	Role VehicleRole `json:"role"`
}

// OdometerCheckpoint represents a manual odometer milestone at a specific date.
type OdometerCheckpoint struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	Date      time.Time `json:"date"`
	Odometer  float64   `json:"odometer"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

// IsHighway returns true if the drive matches the highway detection heuristic:
// either distance >= 40 km and speed_avg >= 70 km/h,
// or distance >= 20 km and speed_max > 125 km/h.
func (d Drive) IsHighway() bool {
	speedAvg := 0.0
	if d.SpeedAvg != nil {
		speedAvg = *d.SpeedAvg
	}
	speedMax := 0
	if d.SpeedMax != nil {
		speedMax = *d.SpeedMax
	}
	return (d.DistanceKm >= 40 && speedAvg >= 70) || (d.DistanceKm >= 20 && speedMax > 125)
}

// TollSegment is a single toll crossing detected on a drive: either a closed-network
// entry/exit pair, a single open barrier, or a closed-network entry with no exit found
// on this drive's trace (end of trace, GPS gap, ...).
type TollSegment struct {
	Network        string       `json:"network,omitempty"` // OpenTollData network_name (closed networks only)
	Operator       string       `json:"operator,omitempty"`
	Type           string       `json:"type"` // "open" or "close"
	Entry          string       `json:"entry"`
	Exit           *string      `json:"exit,omitempty"`
	EstimatedPrice *money.Cents `json:"estimated_price,omitempty"` // class 1 (light vehicle) estimate
}

// TollDetection is the result of matching a drive's GPS trace against the toll station
// reference dataset, cached per drive so the UI doesn't need to re-run detection every time.
type TollDetection struct {
	ID         string        `json:"id"`
	DriveID    string        `json:"drive_id"`
	VehicleID  string        `json:"vehicle_id"`
	Segments   []TollSegment `json:"segments"`
	DetectedAt time.Time     `json:"detected_at"`
}

// TripGroup allows grouping multiple drives (e.g. holiday trip with stops).
type TripGroup struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	Name      string    `json:"name"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Summary (listing only)
	DriveIDs      []string    `json:"drive_ids"`
	DistanceKm    float64     `json:"distance_km"`
	StartTime     *time.Time  `json:"start_time,omitempty"`
	EndTime       *time.Time  `json:"end_time,omitempty"`
	ExpensesTotal money.Cents `json:"expenses_total"` // EUR
	ExpenseCount  int         `json:"expense_count"`
	CarpoolCount  int         `json:"carpool_count"`
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
	DocumentID        *string      `json:"document_id,omitempty"`
	DocumentFilename  *string      `json:"document_filename,omitempty"`
	Source            string       `json:"source"` // ExpenseSourceManual or ExpenseSourceAutoToll
	CreatedAt         time.Time    `json:"created_at"`
}

// Expense provenance: entered by the user, or created from a detected toll estimate.
const (
	ExpenseSourceManual   = "MANUAL"
	ExpenseSourceAutoToll = "AUTO_TOLL"
)

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
	AmortizationMode         string      `json:"amortization_mode"`
	CoverageKm               *float64    `json:"coverage_km,omitempty"`
	CoverageMonths           *int        `json:"coverage_months,omitempty"`
	ClosesMaintenanceID      *string     `json:"closes_maintenance_id,omitempty"`
	Description              string      `json:"description"`
	DocumentID               *string     `json:"document_id,omitempty"`
	DocumentFilename         *string     `json:"document_filename,omitempty"`
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
	DocumentID        *string      `json:"document_id,omitempty"`
	DocumentFilename  *string      `json:"document_filename,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
}

// ExpenseDocument represents a file attachment or invoice stored on the filesystem volume.
type ExpenseDocument struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	VehicleID   string    `json:"vehicle_id"`
	Filename    string    `json:"filename"`
	MimeType    string    `json:"mime_type"`
	FileSize    int64     `json:"file_size"`
	StoragePath *string   `json:"-"` // Relative path on the Docker volume (vehicleID/docID)
	Data        []byte    `json:"-"` // Legacy binary data from PostgreSQL (used for migration & fallback)
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExpenseDocumentHeader represents document metadata without binary payload.
type ExpenseDocumentHeader struct {
	ID                  string    `json:"id"`
	VehicleID           string    `json:"vehicle_id"`
	Filename            string    `json:"filename"`
	MimeType            string    `json:"mime_type"`
	FileSize            int64     `json:"file_size"`
	Description         *string   `json:"description,omitempty"`
	LinkedExpensesCount int       `json:"linked_expenses_count"`
	CreatedAt           time.Time `json:"created_at"`
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

	// Stops where the passenger boards and alights (leg i goes from stop i to stop i + 1)
	BoardStopIndex  int `json:"board_stop_index"`
	AlightStopIndex int `json:"alight_stop_index"`

	// Computed: fair share of the costs of the legs ridden, and amount paid minus that share
	CostShare money.Cents `json:"cost_share"`
	Balance   money.Cents `json:"balance"`
}

// CarpoolLeg is one leg of a carpool trip, usually one TeslaMate drive, with its own costs.
type CarpoolLeg struct {
	ID              string      `json:"id"`
	CarpoolTripID   string      `json:"carpool_trip_id"`
	OrderIndex      int         `json:"order_index"`
	DriveID         *string     `json:"drive_id,omitempty"`
	StartLabel      *string     `json:"start_label,omitempty"`
	EndLabel        *string     `json:"end_label,omitempty"`
	DistanceKm      float64     `json:"distance_km"`
	ElectricityCost money.Cents `json:"electricity_cost"`
	TollsCost       money.Cents `json:"tolls_cost"`
	TiresCost       money.Cents `json:"tires_cost"`
	MaintenanceCost money.Cents `json:"maintenance_cost"`
	InsuranceCost   money.Cents `json:"insurance_cost"`
	OtherCost       money.Cents `json:"other_cost"`

	// Computed
	TotalCost      money.Cents `json:"total_cost"`
	PassengerSeats int         `json:"passenger_seats"` // Seats occupied by passengers on this leg
	CostPerPerson  money.Cents `json:"cost_per_person"` // Driver included
}

// Total returns the sum of the leg cost components.
func (l *CarpoolLeg) Total() money.Cents {
	return l.ElectricityCost + l.TollsCost + l.TiresCost + l.MaintenanceCost + l.InsuranceCost + l.OtherCost
}

// CarpoolTripWithPassengers bundles a trip with its passengers.
type CarpoolTripWithPassengers struct {
	CarpoolTrip
	Legs       []CarpoolLeg       `json:"legs"`
	Passengers []CarpoolPassenger `json:"passengers"`

	// Computed: fair split of the trip costs between the driver and the passengers
	DriverCostShare     money.Cents `json:"driver_cost_share"`
	PassengersCostShare money.Cents `json:"passengers_cost_share"`
}

// CarpoolCostEstimate provides suggested real cost breakdown based on vehicle TCO metrics.
type CarpoolCostEstimate struct {
	StartDate             *time.Time   `json:"start_date,omitempty"`
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
	DailyInsuranceCost    *money.Cents `json:"daily_insurance_cost,omitempty"`
	InsuranceSource       string       `json:"insurance_source"` // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	// Insurance share: insurance paid over the reference window divided by the kilometers driven over it.
	InsuranceWindowCost   *money.Cents `json:"insurance_window_cost,omitempty"`
	InsuranceWindowKm     *float64     `json:"insurance_window_km,omitempty"`
	EnergySource          string       `json:"energy_source"`           // MEASURED | CONSUMPTION | DEFAULT
	ElectricityRateSource string       `json:"electricity_rate_source"` // HISTORY | DEFAULT
	TiresRateSource       string       `json:"tires_rate_source"`       // MOUNTED_TIRES | HISTORY | DEFAULT
	MaintenanceRateSource string       `json:"maintenance_rate_source"` // HISTORY | DEFAULT
	// One estimated leg per drive (chronological), or a single leg for a manual distance
	Legs []CarpoolLeg `json:"legs"`
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
	// Fair shares over all trips: what passengers should cover given the legs they rode, and the driver's own share
	TotalPassengersShare money.Cents `json:"total_passengers_share"`
	TotalDriverShare     money.Cents `json:"total_driver_share"`
}

// DataQualityIssue reports an odometer continuity problem on synchronized drives.
type DataQualityIssue struct {
	Type            string    `json:"type"` // ODOMETER_GAP | ODOMETER_REGRESSION | DISTANCE_MISMATCH
	DriveID         string    `json:"drive_id"`
	PreviousDriveID *string   `json:"previous_drive_id,omitempty"`
	Date            time.Time `json:"date"`
	Km              float64   `json:"km"`
}

// Acquisition types of a vehicle.
const (
	AcquisitionCash = "CASH" // Cash purchase
	AcquisitionLoan = "LOAN" // Purchase financed by a loan
	AcquisitionLOA  = "LOA"  // Lease with purchase option
	AcquisitionLLD  = "LLD"  // Long-term rental
)

// VehicleOwnership describes how a vehicle is owned or leased; it generates acquisition and financing costs.
type VehicleOwnership struct {
	VehicleID       string    `json:"vehicle_id"`
	AcquisitionType string    `json:"acquisition_type"`
	StartDate       time.Time `json:"start_date"`
	StartOdometer   *float64  `json:"start_odometer,omitempty"`

	PurchasePrice         *money.Cents `json:"purchase_price,omitempty"`
	PurchaseFees          *money.Cents `json:"purchase_fees,omitempty"`
	Incentives            *money.Cents `json:"incentives,omitempty"`
	ExpectedResaleValue   *money.Cents `json:"expected_resale_value,omitempty"`
	ExpectedHoldingMonths *int         `json:"expected_holding_months,omitempty"`

	LoanAmount           *money.Cents `json:"loan_amount,omitempty"`
	LoanRatePct          *float64     `json:"loan_rate_pct,omitempty"`
	LoanDurationMonths   *int         `json:"loan_duration_months,omitempty"`
	LoanFees             *money.Cents `json:"loan_fees,omitempty"`
	LoanInsuranceMonthly *money.Cents `json:"loan_insurance_monthly,omitempty"`

	LeaseDownPayment         *money.Cents `json:"lease_down_payment,omitempty"`
	LeaseMonthlyRent         *money.Cents `json:"lease_monthly_rent,omitempty"`
	LeaseDurationMonths      *int         `json:"lease_duration_months,omitempty"`
	LeaseFees                *money.Cents `json:"lease_fees,omitempty"`
	LeaseDeposit             *money.Cents `json:"lease_deposit,omitempty"`
	LeaseKmAllowancePerYear  *float64     `json:"lease_km_allowance_per_year,omitempty"`
	LeaseExcessKmPrice       *float64     `json:"lease_excess_km_price,omitempty"`
	LeaseEndFeesEstimate     *money.Cents `json:"lease_end_fees_estimate,omitempty"`
	LeasePurchaseOptionPrice *money.Cents `json:"lease_purchase_option_price,omitempty"`
	LeaseIncludesMaintenance bool         `json:"lease_includes_maintenance"`
	LeaseIncludesInsurance   bool         `json:"lease_includes_insurance"`
	LeaseIncludesTires       bool         `json:"lease_includes_tires"`
	OptionExercisedDate      *time.Time   `json:"option_exercised_date,omitempty"`

	EndDate   *time.Time   `json:"end_date,omitempty"`
	SalePrice *money.Cents `json:"sale_price,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsLease reports whether the vehicle is under a LOA or LLD contract.
func (o *VehicleOwnership) IsLease() bool {
	return o != nil && (o.AcquisitionType == AcquisitionLOA || o.AcquisitionType == AcquisitionLLD)
}

// InLeasePhase reports whether the lease contract is still the ownership mode at a given time
// (not ended, not returned, purchase option not exercised).
func (o *VehicleOwnership) InLeasePhase(at time.Time) bool {
	if !o.IsLease() || o.LeaseDurationMonths == nil {
		return false
	}
	if o.OptionExercisedDate != nil && !at.Before(*o.OptionExercisedDate) {
		return false
	}
	if o.EndDate != nil && !at.Before(*o.EndDate) {
		return false
	}
	return at.Before(o.StartDate.AddDate(0, *o.LeaseDurationMonths, 0))
}

// MaintenanceReminder represents a recurring or scheduled maintenance task.
type MaintenanceReminder struct {
	ID                   string     `json:"id"`
	VehicleID            string     `json:"vehicle_id"`
	Title                string     `json:"title"`
	Category             string     `json:"category"` // MAINTENANCE, TIRES, INSPECTION, OTHER
	IntervalKm           *int       `json:"interval_km,omitempty"`
	IntervalMonths       *int       `json:"interval_months,omitempty"`
	LastServiceOdometer  *float64   `json:"last_service_odometer,omitempty"`
	LastServiceDate      *time.Time `json:"last_service_date,omitempty"`
	LeadKm               int        `json:"lead_km"`
	LeadDays             int        `json:"lead_days"`
	WebhookEnabled       bool       `json:"webhook_enabled"`
	LastNotifiedAt       *time.Time `json:"last_notified_at,omitempty"`
	LastNotifiedOdometer *float64   `json:"last_notified_odometer,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// Computed dynamic status
	Status        string     `json:"status"` // OK, DUE_SOON, OVERDUE
	RemainingKm   *float64   `json:"remaining_km,omitempty"`
	RemainingDays *int       `json:"remaining_days,omitempty"`
	DueOdometer   *float64   `json:"due_odometer,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
}

// ComputeStatus calculates the status (OK, DUE_SOON, OVERDUE) and remaining km/days.
func (r *MaintenanceReminder) ComputeStatus(currentOdometer float64, now time.Time) {
	r.Status = "OK"

	// 1. Kilométrage
	if r.IntervalKm != nil && *r.IntervalKm > 0 {
		baseOdo := 0.0
		if r.LastServiceOdometer != nil {
			baseOdo = *r.LastServiceOdometer
		}
		dueOdo := baseOdo + float64(*r.IntervalKm)
		r.DueOdometer = &dueOdo

		remKm := dueOdo - currentOdometer
		r.RemainingKm = &remKm

		if remKm <= 0 {
			r.Status = "OVERDUE"
		} else if remKm <= float64(r.LeadKm) {
			r.Status = "DUE_SOON"
		}
	}

	// 2. Date
	if r.IntervalMonths != nil && *r.IntervalMonths > 0 {
		baseDate := r.CreatedAt
		if r.LastServiceDate != nil {
			baseDate = *r.LastServiceDate
		}
		dueDate := baseDate.AddDate(0, *r.IntervalMonths, 0)
		r.DueDate = &dueDate

		remDays := int(dueDate.Sub(now).Hours() / 24)
		r.RemainingDays = &remDays

		if remDays <= 0 {
			r.Status = "OVERDUE"
		} else if remDays <= r.LeadDays {
			if r.Status != "OVERDUE" {
				r.Status = "DUE_SOON"
			}
		}
	}
}

// VehicleWebhook holds outgoing homelab webhook settings for notifications.
type VehicleWebhook struct {
	ID        string    `json:"id"`
	VehicleID string    `json:"vehicle_id"`
	URL       string    `json:"url"`
	Type      string    `json:"type"` // DISCORD | TELEGRAM | GOTIFY | GENERIC
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RefreshToken represents a long-lived refresh token session with rotation family tracking.
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	FamilyID  string    `json:"family_id"`
	IsRevoked bool      `json:"is_revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	CreatedIP *string   `json:"created_ip,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
}
