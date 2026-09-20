import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { apiErrorMessage } from './apiError'

afterEach(() => setLocale('fr'))

describe('apiErrorMessage', () => {
  it('translates a known code in the current language', () => {
    expect(apiErrorMessage({ error: 'Vehicle not found', code: 'vehicle.not_found' }, 'x')).toBe('Véhicule introuvable')
    setLocale('en')
    expect(apiErrorMessage({ error: 'Vehicle not found', code: 'vehicle.not_found' }, 'x')).toBe('Vehicle not found')
  })

  it('fills the parameters of the message', () => {
    expect(apiErrorMessage({ code: 'tire.not_in_storage', params: { p0: 'abc' } }, 'x')).toBe("Le pneu abc n'est pas en stockage")
    setLocale('en')
    expect(apiErrorMessage({ code: 'carpool.too_many_seats', params: { p0: 4, p1: 2 } }, 'x')).toBe('More than 4 seats taken on leg 2')
  })

  it('falls back to the server message for an unknown or missing code', () => {
    expect(apiErrorMessage({ error: 'Something odd', code: 'nope.unknown' }, 'x')).toBe('Something odd')
    expect(apiErrorMessage({ error: 'Plain message' }, 'x')).toBe('Plain message')
    expect(apiErrorMessage({}, 'fallback')).toBe('fallback')
    expect(apiErrorMessage(null, 'fallback')).toBe('fallback')
  })
})
