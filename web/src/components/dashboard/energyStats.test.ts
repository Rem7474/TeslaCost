import { describe, expect, it } from 'vitest'
import { mergeAcDcClasses, type ChargeClass } from './energyStats'

const cls = (over: Partial<ChargeClass>): ChargeClass => ({ class: 'AC', sessions: 0, kwh_added: 0, energy_cost: 0, ...over })

describe('mergeAcDcClasses', () => {
  it('merges the domestic socket into AC, keeps DC separate', () => {
    const slow = cls({ class: 'SLOW', sessions: 10, kwh_added: 99, energy_cost: 6.08, price_per_kwh: 0.132, charge_efficiency: 0.87 })
    const ac = cls({ class: 'AC', sessions: 75, kwh_added: 1452, energy_cost: 8.2 * 75, price_per_kwh: 0.139, charge_efficiency: 0.95 })
    const dc = cls({ class: 'DC', sessions: 30, kwh_added: 832, energy_cost: 832 * 0.273, price_per_kwh: 0.273, charge_efficiency: 0.93 })
    const unknown = cls({ class: 'UNKNOWN', sessions: 5, kwh_added: 0, energy_cost: 0 })

    const { classes, unknownSessions } = mergeAcDcClasses([slow, ac, dc, unknown])

    expect(classes.map((c) => c.class)).toEqual(['AC', 'DC'])
    const mergedAc = classes[0]
    expect(mergedAc.sessions).toBe(85)
    expect(mergedAc.kwh_added).toBeCloseTo(1551, 5)
    expect(mergedAc.energy_cost).toBeCloseTo(slow.energy_cost + ac.energy_cost, 5)
    expect(mergedAc.price_per_kwh).toBeCloseTo(mergedAc.energy_cost / 1551, 5)
    // Weighted by kWh: mostly the AC (wallbox) efficiency, slightly pulled down by the slow one
    expect(mergedAc.charge_efficiency).toBeGreaterThan(0.94)
    expect(mergedAc.charge_efficiency).toBeLessThan(0.95)
    expect(classes[1]).toEqual(dc)
    expect(unknownSessions).toBe(5)
  })

  it('keeps a class missing entirely out of the result, without crashing', () => {
    const dc = cls({ class: 'DC', sessions: 3, kwh_added: 50, energy_cost: 10 })
    expect(mergeAcDcClasses([dc]).classes).toEqual([{ ...dc, class: 'DC' }])
    expect(mergeAcDcClasses([]).classes).toEqual([])
    expect(mergeAcDcClasses([]).unknownSessions).toBe(0)
  })

  it('does not divide by zero when a merged class has no kWh', () => {
    const slow = cls({ class: 'SLOW', sessions: 1, kwh_added: 0, energy_cost: 0 })
    const merged = mergeAcDcClasses([slow]).classes[0]
    expect(merged.price_per_kwh).toBeUndefined()
    expect(merged.charge_efficiency).toBeUndefined()
    expect(merged.cost_per_full_charge).toBeUndefined()
  })
})
