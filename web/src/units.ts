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

/** Rounded, locale-formatted distance with its unit suffix, e.g. "42 000 km" / "26,097 mi". */
export function formatDistance(km: number): string {
  return `${Math.round(kmToDisplayDistance(km)).toLocaleString(intlLocale())} ${unit.value}`
}
