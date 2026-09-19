// Shape of GET /api/vehicles/{id}/energy-stats, shared by the dashboard sections.

export interface EnergyMonth {
  month: string
  distance_km: number
  consumption_kwh_100km?: number
  price_per_kwh?: number
  cost_per_100km?: number
  cost_per_100km_trailing?: number
  kwh_added: number
  energy_cost: number
  estimated_capacity_kwh?: number
  capacity_samples?: number
}

export interface ChargeClass {
  class: 'SLOW' | 'AC' | 'DC' | 'UNKNOWN'
  sessions: number
  kwh_added: number
  energy_cost: number
  price_per_kwh?: number
  charge_efficiency?: number
  cost_per_full_charge?: number
}

export interface TemperatureBin {
  min_c: number
  max_c: number
  drives: number
  distance_km: number
  consumption_kwh_100km: number
}

export interface BatterySnapshot {
  date: string // YYYY-MM-DD
  max_capacity_kwh?: number
  current_capacity_kwh?: number
  health_percent?: number
}

export interface EnergyStats {
  months: EnergyMonth[]
  charge_classes: ChargeClass[]
  summary: {
    consumption_kwh_100km?: number
    charge_efficiency?: number
    price_per_kwh?: number
    cost_per_100km?: number
    sessions_without_cost: number
    estimated_capacity_kwh?: number
    capacity_samples?: number
    cost_per_full_charge?: number
  }
  temperature_bins: TemperatureBin[]
  temperature: {
    cold_consumption_kwh_100km?: number
    mild_consumption_kwh_100km?: number
    extra_percent?: number
    extra_cost_per_100km?: number
  }
  battery_health: BatterySnapshot[]
}

export const fmt = (v: number | undefined, digits: number) =>
  v === undefined ? '—' : v.toLocaleString('fr-FR', { minimumFractionDigits: digits, maximumFractionDigits: digits })

export const fmtPercent = (v: number | undefined) =>
  v === undefined ? '—' : `${(v * 100).toLocaleString('fr-FR', { maximumFractionDigits: 0 })} %`

export const AXIS_TEXT = '#94a3b8'
export const GRID_COLOR = '#1e293b'
