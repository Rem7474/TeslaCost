package models

import (
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// FuelLog is a manually entered fuel fill-up of a combustion vehicle.
// Odometer and Amount are the minimum; Liters (or price per liter) enables consumption figures.
type FuelLog struct {
	ID            string      `json:"id"`
	VehicleID     string      `json:"vehicle_id"`
	Date          time.Time   `json:"date"`
	Odometer      float64     `json:"odometer"`
	Amount        money.Cents `json:"amount"`
	Liters        *float64    `json:"liters,omitempty"`
	PricePerLiter *float64    `json:"price_per_liter,omitempty"`
	FuelType      *string     `json:"fuel_type,omitempty"`
	IsFullTank    bool        `json:"is_full_tank"`
	Notes         *string     `json:"notes,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}
