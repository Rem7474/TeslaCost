import { ref } from 'vue'
import { intlLocale } from '@/i18n'

// Distances are always stored and sent to the API in kilometers (TeslaMate, toll data and the
// existing history are all metric); this module only converts for display and form input,
// driven by the signed-in account's stored preference (see stores/auth.ts).
export const SUPPORTED_DISTANCE_UNITS = ['km', 'mi'] as const
export type DistanceUnit = (typeof SUPPORTED_DISTANCE_UNITS)[number]
export const DEFAULT_DISTANCE_UNIT: DistanceUnit = 'km'

const KM_PER_MILE = 1.609344

const isSupported = (value: string | null | undefined): value is DistanceUnit =>
  !!value && (SUPPORTED_DISTANCE_UNITS as readonly string[]).includes(value)

const unit = ref<DistanceUnit>(DEFAULT_DISTANCE_UNIT)

/** Applies the account's stored choice; an unknown or missing value falls back to km. */
export function setDistanceUnit(value: string | null | undefined) {
  unit.value = isSupported(value) ? value : DEFAULT_DISTANCE_UNIT
}

export const currentDistanceUnit = (): DistanceUnit => unit.value

/** A stored km value, converted to the account's unit. */
export function kmToDisplayDistance(km: number): number {
  return unit.value === 'mi' ? km / KM_PER_MILE : km
}

/** The reverse: a value typed in the account's unit, converted back to km for the API. */
export function displayDistanceToKm(value: number): number {
  return unit.value === 'mi' ? value * KM_PER_MILE : value
}

/** A stored km value in the account's unit, locale-formatted without the unit ("42 000", "26,097.3"). */
export function formatDistanceValue(km: number, digits = 0): string {
  return kmToDisplayDistance(km).toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

/** Locale-formatted distance with its unit suffix, e.g. "42 000 km" / "26,097 mi". */
export function formatDistance(km: number, digits = 0): string {
  return `${formatDistanceValue(km, digits)} ${unit.value}`
}

/**
 * A figure expressed per km (a cost per km, kWh per 100 km, a price per extra km), rescaled to the
 * account's unit: per mile it is 1.609 times larger. Pair it with "/{unit}" in the label.
 */
export function perDistance(valuePerKm: number): number {
  return unit.value === 'mi' ? valuePerKm * KM_PER_MILE : valuePerKm
}

/** A figure per km (or per 100 km) rescaled to the account's unit, locale-formatted without its unit. */
export function formatPerDistanceValue(valuePerKm: number, digits = 1): string {
  return perDistance(valuePerKm).toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

/** The reverse of perDistance, for a per-distance figure typed in a form. */
export function perDistanceToPerKm(value: number): number {
  return unit.value === 'mi' ? value / KM_PER_MILE : value
}

/** Speed unit that goes with the distance unit. */
export const speedUnit = (): string => (unit.value === 'mi' ? 'mph' : 'km/h')

/** A speed stored in km/h, in the account's unit. */
export function formatSpeed(kmh: number): string {
  return `${Math.round(kmToDisplayDistance(kmh)).toLocaleString(intlLocale())} ${speedUnit()}`
}

/** The account's distance unit label, for a catalog's {unit} placeholder ("Distance ({unit})"). */
export const distanceUnit = (): DistanceUnit => unit.value
