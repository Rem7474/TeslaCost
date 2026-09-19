import { describe, expect, it } from 'vitest'
import { describeRelativeTime, describeUserAgent } from './userAgent'

describe('describeUserAgent', () => {
  it('names the common browser and system pairs', () => {
    const cases: [string, string][] = [
      ['Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36', 'Chrome sur Linux'],
      ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0', 'Edge sur Windows'],
      ['Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0', 'Firefox sur Linux'],
      ['Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1', 'Safari sur iOS'],
      ['Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36', 'Chrome sur Android'],
      ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15', 'Safari sur macOS'],
      ['Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.153 Mobile/15E148 Safari/604.1', 'Chrome sur iOS'],
    ]
    for (const [ua, want] of cases) expect(describeUserAgent(ua)).toBe(want)
  })

  it('falls back to what it can recognise, then to a generic label', () => {
    expect(describeUserAgent('curl/8.5.0')).toBe('curl')
    expect(describeUserAgent('Something (Linux)')).toBe('Linux')
    expect(describeUserAgent('')).toBe('Appareil inconnu')
    expect(describeUserAgent(null)).toBe('Appareil inconnu')
    expect(describeUserAgent('totally-unknown/1.0')).toBe('Appareil inconnu')
  })
})

describe('describeRelativeTime', () => {
  const now = new Date('2026-09-19T12:00:00Z')

  it('speaks in minutes, hours and days', () => {
    expect(describeRelativeTime('2026-09-19T11:55:00Z', now)).toBe('il y a 5 minutes')
    expect(describeRelativeTime('2026-09-19T09:00:00Z', now)).toBe('il y a 3 heures')
    expect(describeRelativeTime('2026-09-17T12:00:00Z', now)).toBe('il y a 2 jours')
  })

  it('says "à l\'instant" under a minute and writes out old dates', () => {
    expect(describeRelativeTime('2026-09-19T11:59:40Z', now)).toBe("à l'instant")
    expect(describeRelativeTime('2026-08-01T12:00:00Z', now)).toMatch(/2026/)
  })

  it('does not fail on an invalid date', () => {
    expect(describeRelativeTime('not a date', now)).toBe('')
  })
})
