package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type CarpoolHandler struct {
	repo           *database.Repository
	carpoolService *services.CarpoolService
}

func NewCarpoolHandler(repo *database.Repository, carpoolService *services.CarpoolService) *CarpoolHandler {
	return &CarpoolHandler{
		repo:           repo,
		carpoolService: carpoolService,
	}
}

type PassengerPayload struct {
	PassengerName string  `json:"passenger_name"`
	Origin        *string `json:"origin"`
	Destination   *string `json:"destination"`
	Seats         int     `json:"seats"`
	AmountPaid    float64 `json:"amount_paid"`
	Notes         *string `json:"notes"`
}

type UpsertCarpoolRequest struct {
	Title           string             `json:"title"`
	Date            string             `json:"date"`
	DistanceKm      float64            `json:"distance_km"`
	DriveID         *string            `json:"drive_id"`
	TripGroupID     *string            `json:"trip_group_id"`
	ElectricityCost float64            `json:"electricity_cost"`
	TollsCost       float64            `json:"tolls_cost"`
	TiresCost       float64            `json:"tires_cost"`
	MaintenanceCost float64            `json:"maintenance_cost"`
	InsuranceCost   float64            `json:"insurance_cost"`
	OtherCost       float64            `json:"other_cost"`
	Notes           *string            `json:"notes"`
	Passengers      []PassengerPayload `json:"passengers"`
}

func (h *CarpoolHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	trips, err := h.repo.ListCarpoolTrips(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list carpool trips: "+err.Error())
		return
	}

	summary, err := h.repo.GetCarpoolSummary(r.Context(), vehicleID)
	if err != nil {
		summary = &models.CarpoolSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"trips":   trips,
		"summary": summary,
	})
}

func (h *CarpoolHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	id := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	trip, err := h.repo.GetCarpoolTrip(r.Context(), id, vehicleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Carpool trip not found")
		return
	}

	writeJSON(w, http.StatusOK, trip)
}

func (h *CarpoolHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req UpsertCarpoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "Title is required")
		return
	}

	parsedDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		parsedDate = time.Now().UTC()
	}

	trip := &models.CarpoolTrip{
		VehicleID:       vehicleID,
		DriveID:         req.DriveID,
		TripGroupID:     req.TripGroupID,
		Title:           req.Title,
		Date:            parsedDate,
		DistanceKm:      req.DistanceKm,
		ElectricityCost: req.ElectricityCost,
		TollsCost:       req.TollsCost,
		TiresCost:       req.TiresCost,
		MaintenanceCost: req.MaintenanceCost,
		InsuranceCost:   req.InsuranceCost,
		OtherCost:       req.OtherCost,
		Notes:           req.Notes,
	}

	var passengers []models.CarpoolPassenger
	for _, p := range req.Passengers {
		seats := p.Seats
		if seats <= 0 {
			seats = 1
		}
		name := p.PassengerName
		if name == "" {
			name = "Passager"
		}
		passengers = append(passengers, models.CarpoolPassenger{
			PassengerName: name,
			Origin:        p.Origin,
			Destination:   p.Destination,
			Seats:         seats,
			AmountPaid:    p.AmountPaid,
			Notes:         p.Notes,
		})
	}

	if err := h.repo.CreateCarpoolTrip(r.Context(), trip, passengers); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create carpool trip: "+err.Error())
		return
	}

	tripWithPassengers := &models.CarpoolTripWithPassengers{
		CarpoolTrip: *trip,
		Passengers:  passengers,
	}

	writeJSON(w, http.StatusCreated, tripWithPassengers)
}

func (h *CarpoolHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	id := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req UpsertCarpoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	parsedDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		parsedDate = time.Now().UTC()
	}

	trip := &models.CarpoolTrip{
		ID:              id,
		VehicleID:       vehicleID,
		DriveID:         req.DriveID,
		TripGroupID:     req.TripGroupID,
		Title:           req.Title,
		Date:            parsedDate,
		DistanceKm:      req.DistanceKm,
		ElectricityCost: req.ElectricityCost,
		TollsCost:       req.TollsCost,
		TiresCost:       req.TiresCost,
		MaintenanceCost: req.MaintenanceCost,
		InsuranceCost:   req.InsuranceCost,
		OtherCost:       req.OtherCost,
		Notes:           req.Notes,
	}

	var passengers []models.CarpoolPassenger
	for _, p := range req.Passengers {
		seats := p.Seats
		if seats <= 0 {
			seats = 1
		}
		name := p.PassengerName
		if name == "" {
			name = "Passager"
		}
		passengers = append(passengers, models.CarpoolPassenger{
			PassengerName: name,
			Origin:        p.Origin,
			Destination:   p.Destination,
			Seats:         seats,
			AmountPaid:    p.AmountPaid,
			Notes:         p.Notes,
		})
	}

	if err := h.repo.UpdateCarpoolTrip(r.Context(), trip, passengers); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update carpool trip: "+err.Error())
		return
	}

	tripWithPassengers := &models.CarpoolTripWithPassengers{
		CarpoolTrip: *trip,
		Passengers:  passengers,
	}

	writeJSON(w, http.StatusOK, tripWithPassengers)
}

func (h *CarpoolHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	id := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteCarpoolTrip(r.Context(), id, vehicleID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete carpool trip")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *CarpoolHandler) Estimate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	driveIDParam := r.URL.Query().Get("drive_id")
	var driveID *string
	if driveIDParam != "" {
		driveID = &driveIDParam
	}

	tripGroupIDParam := r.URL.Query().Get("trip_group_id")
	var tripGroupID *string
	if tripGroupIDParam != "" {
		tripGroupID = &tripGroupIDParam
	}

	var driveIDs []string
	driveIDsParam := r.URL.Query().Get("drive_ids")
	if driveIDsParam != "" {
		driveIDs = strings.Split(driveIDsParam, ",")
	}

	distanceKm, _ := strconv.ParseFloat(r.URL.Query().Get("distance_km"), 64)

	estimate, err := h.carpoolService.EstimateCosts(r.Context(), vehicleID, driveID, tripGroupID, driveIDs, distanceKm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to estimate costs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, estimate)
}
