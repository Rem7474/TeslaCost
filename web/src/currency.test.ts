import { describe, expect, it } from 'vitest'
import { formatAmount, formatMoney } from './currency'

describe('formatMoney', () => {
  it('divides cents into the main unit before formatting', () => {
    expect(formatMoney(154000, 'EUR')).toBe(new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' }).format(1540))
  })

  it('formats a non-euro currency with its own symbol', () => {
    expect(formatMoney(100, 'USD')).toBe(new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'USD' }).format(1))
  })
})

describe('formatAmount', () => {
  it('respects maximumFractionDigits for a per-km rate', () => {
    const got = formatAmount(0.15, 'EUR', 3)
    expect(got).toContain('0,15')
  })
})
