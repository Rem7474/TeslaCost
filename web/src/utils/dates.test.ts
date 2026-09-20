import { describe, expect, it } from 'vitest'
import { formatDayTime, todayIso, toIsoDay } from './dates'

describe('dates', () => {
  it('gives the UTC day of a date', () => {
    expect(toIsoDay('2026-05-02T23:30:00Z')).toBe('2026-05-02')
    expect(toIsoDay(new Date('2026-05-03T00:00:00Z'))).toBe('2026-05-03')
  })

  it('formats today as YYYY-MM-DD', () => {
    expect(todayIso()).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(todayIso()).toBe(new Date().toISOString().slice(0, 10))
  })

  it('formats a short date with the time', () => {
    const s = formatDayTime('2026-05-02T08:30:00')
    expect(s).toContain('02')
    expect(s).toContain('08:30')
  })
})
