import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { apiErrorMessage, apiMessageText } from './apiError'

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

describe('apiMessageText', () => {
  it('translates a payload message and its keyword parameters', () => {
    const warning = { code: 'sync.page_limit', message: 'x', params: { p0: 'kw:drives', p1: 500 } }
    expect(apiMessageText(warning)).toBe('Trajets : limite de 500 pages atteinte, historique partiellement importé')
    setLocale('en')
    expect(apiMessageText(warning)).toBe('Drives: limit of 500 pages reached, history partially imported')
  })

  it('translates an error nested in the parameters of another one', () => {
    const error = {
      code: 'sync.unreachable',
      error: 'x',
      params: { p0: 'http://tm:8080', p1: { code: 'teslamate.timeout', message: 'y', params: { p0: 'dial failed' } } },
    }
    setLocale('en')
    expect(apiErrorMessage(error, 'z')).toBe(
      'Cannot reach TeslaMate (http://tm:8080): dial failed (Timed out: check that the address and port are reachable and that TeslaMate is responding)',
    )
  })

  it('shows the server text of a message without a catalog entry', () => {
    expect(apiMessageText({ code: 'unknown.thing', message: 'Server text' })).toBe('Server text')
  })
})
