import { describe, expect, it } from 'vitest'
import {
  applyBatchTag,
  buildTripCostDrive,
  currentYearMonth,
  driveCsvRows,
  formatMonthLabel,
  formatTripDates,
  isHighwayDrive,
  monthRange,
  needsTollQualification,
  tripNeedsTollQualification,
  paginationPages,
  selectionSummary,
  shiftMonth,
  teslamateDriveUrl,
  toggleTag,
  tollApplyStatusLabel,
  uniqueById,
} from './drives'

describe('isHighwayDrive', () => {
  it('flags long fast drives and short very fast ones', () => {
    expect(isHighwayDrive({ distance_km: 40, speed_avg: 70, speed_max: 100 })).toBe(true)
    expect(isHighwayDrive({ distance_km: 20, speed_avg: 50, speed_max: 126 })).toBe(true)
    expect(isHighwayDrive({ distance_km: 20, speed_avg: 70, speed_max: 110 })).toBe(true)
    expect(isHighwayDrive({ distance_km: 8, speed_avg: 70, speed_max: 105 })).toBe(true)
  })

  it('ignores city drives and drives just under each threshold', () => {
    expect(isHighwayDrive({ distance_km: 39, speed_avg: 90, speed_max: 100 })).toBe(false)
    expect(isHighwayDrive({ distance_km: 40, speed_avg: 69, speed_max: 100 })).toBe(false)
    expect(isHighwayDrive({ distance_km: 20, speed_avg: 50, speed_max: 125 })).toBe(false)
    expect(isHighwayDrive({ distance_km: 7, speed_avg: 90, speed_max: 130 })).toBe(false)
    expect(isHighwayDrive({})).toBe(false)
  })
})

describe('needsTollQualification', () => {
  const highway = { distance_km: 120, speed_avg: 95, speed_max: 130 }

  it('applies to unreviewed highway drives without a toll', () => {
    expect(needsTollQualification(highway)).toBe(true)
  })

  it('does not once reviewed or once a toll is attached', () => {
    expect(needsTollQualification({ ...highway, toll_reviewed_at: '2026-05-01' })).toBe(false)
    expect(needsTollQualification({ ...highway, costs: { tolls_cost: 8 } })).toBe(false)
    expect(needsTollQualification({ distance_km: 10, speed_avg: 30 })).toBe(false)
  })
})

describe('tripNeedsTollQualification', () => {
  it('follows the number of motorway-like drives left to qualify in the trip', () => {
    expect(tripNeedsTollQualification({ unqualified_drive_count: 2 })).toBe(true)
    expect(tripNeedsTollQualification({ unqualified_drive_count: 0 })).toBe(false)
    expect(tripNeedsTollQualification({})).toBe(false)
  })
})

describe('teslamateDriveUrl', () => {
  const vehicle = { teslamate_grafana_url: 'http://grafana.local', teslamate_car_id: 2 }
  const drive = { teslamate_drive_id: 77, start_time: '2026-05-01T08:00:00Z', end_time: '2026-05-01T09:00:00Z' }

  it('builds the Drive Details link with a one minute margin around the drive', () => {
    const url = new URL(teslamateDriveUrl(vehicle, drive)!)
    expect(url.origin + url.pathname).toBe('http://grafana.local/d/zm7wN6Zgz/drive-details')
    expect(url.searchParams.get('var-drive_id')).toBe('77')
    expect(url.searchParams.get('var-car_id')).toBe('2')
    expect(Number(url.searchParams.get('from'))).toBe(new Date(drive.start_time).getTime() - 60_000)
    expect(Number(url.searchParams.get('to'))).toBe(new Date(drive.end_time).getTime() + 60_000)
  })

  it('has no link without Grafana, without a TeslaMate drive, or for a trip group', () => {
    expect(teslamateDriveUrl({}, drive)).toBeNull()
    expect(teslamateDriveUrl(vehicle, { ...drive, teslamate_drive_id: null })).toBeNull()
    expect(teslamateDriveUrl(vehicle, { ...drive, is_trip_group: true })).toBeNull()
    expect(teslamateDriveUrl(null, drive)).toBeNull()
  })
})

describe('tollApplyStatusLabel', () => {
  it('explains each skipped status and falls back to a generic message', () => {
    expect(tollApplyStatusLabel('skipped_manual')).toContain('manuellement')
    expect(tollApplyStatusLabel('skipped_trip_group')).toContain('voyage')
    expect(tollApplyStatusLabel('skipped_no_price')).toContain('tarif')
    expect(tollApplyStatusLabel('skipped_no_gps')).toContain('GPS')
    expect(tollApplyStatusLabel('whatever')).toContain('pas pu être appliqué')
  })
})

describe('formatTripDates', () => {
  it('shows one date for a single-day trip and a range otherwise', () => {
    expect(formatTripDates({})).toBe('Aucun trajet')
    const one = formatTripDates({ start_time: '2026-05-02T08:00:00Z', end_time: '2026-05-02T18:00:00Z' })
    expect(one).not.toContain('→')
    expect(formatTripDates({ start_time: '2026-05-02T08:00:00Z', end_time: '2026-05-04T18:00:00Z' })).toContain(' → ')
    expect(formatTripDates({ start_time: '2026-05-02T08:00:00Z' })).not.toContain('→')
  })
})

describe('month helpers', () => {
  it('formats the current month as YYYY-MM', () => {
    expect(currentYearMonth(new Date(2026, 8, 19))).toBe('2026-09')
    expect(currentYearMonth(new Date(2026, 0, 2))).toBe('2026-01')
  })

  it('shifts across year boundaries', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftMonth('2025-12', 1)).toBe('2026-01')
    expect(shiftMonth('2026-05', 0)).toBe('2026-05')
  })

  it('gives the first and last day, including leap Februaries', () => {
    expect(monthRange('2026-05')).toEqual({ from: '2026-05-01', to: '2026-05-31' })
    expect(monthRange('2026-02')).toEqual({ from: '2026-02-01', to: '2026-02-28' })
    expect(monthRange('2028-02').to).toBe('2028-02-29')
  })

  it('capitalizes the month label', () => {
    expect(formatMonthLabel('2026-05')).toBe('Mai 2026')
    expect(formatMonthLabel('')).toBe('')
  })
})

describe('paginationPages', () => {
  it('lists every page when there are seven or fewer', () => {
    expect(paginationPages(1, 1)).toEqual([1])
    expect(paginationPages(3, 7)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it('collapses the middle for many pages', () => {
    expect(paginationPages(1, 20)).toEqual([1, 2, 3, 4, 5, '...', 20])
    expect(paginationPages(4, 20)).toEqual([1, 2, 3, 4, 5, '...', 20])
    expect(paginationPages(10, 20)).toEqual([1, '...', 9, 10, 11, '...', 20])
    expect(paginationPages(17, 20)).toEqual([1, '...', 16, 17, 18, 19, 20])
    expect(paginationPages(20, 20)).toEqual([1, '...', 16, 17, 18, 19, 20])
  })
})

describe('tags', () => {
  it('toggles a tag on and off', () => {
    expect(toggleTag([], 'Pro')).toEqual(['Pro'])
    expect(toggleTag(['Pro'], 'Pro')).toEqual([])
    expect(toggleTag(undefined, 'Perso')).toEqual(['Perso'])
  })

  it('keeps Pro and Perso exclusive and leaves other tags alone', () => {
    expect(toggleTag(['Perso', 'Vacances'], 'Pro')).toEqual(['Vacances', 'Pro'])
    expect(toggleTag(['Pro'], 'Perso')).toEqual(['Perso'])
  })

  it('applies a batch tag without duplicating it, or clears both', () => {
    expect(applyBatchTag(['Pro'], 'Pro')).toEqual(['Pro'])
    expect(applyBatchTag(['Perso', 'X'], 'Pro')).toEqual(['X', 'Pro'])
    expect(applyBatchTag(['Pro', 'Perso', 'X'], null)).toEqual(['X'])
  })
})

describe('selectionSummary and CSV rows', () => {
  const list = [
    { id: 'a', distance_km: 100.4, costs: { electricity_kwh: 18.2, total_cost: 5.5, cost_per_km: 0.05 }, tags: ['Pro', 'X'], start_time: '2026-05-01T08:00:00Z', start_address: 'Rue "A", Paris', end_address: 'Lyon' },
    { id: 'b', distance_km: 50, costs: { electricity_kwh: 9, total_cost: 2.25 } },
  ]

  it('sums distance, energy and cost', () => {
    expect(selectionSummary(list)).toBe(`${Math.round(150.4).toLocaleString('fr-FR')} km • 27 kWh • 7.75 €`)
    expect(selectionSummary([])).toBe('')
  })

  it('escapes quotes and joins tags in the CSV rows', () => {
    const rows = driveCsvRows(list)
    expect(rows[0][1]).toBe('2026-05-01T08:00')
    expect(rows[0][2]).toBe('"Rue ""A"", Paris"')
    expect(rows[0][8]).toBe('5.50')
    expect(rows[0][9]).toBe('0.050')
    expect(rows[0][10]).toBe('"Pro, X"')
    expect(rows[1][1]).toBe('')
    expect(rows[1][10]).toBe('""')
  })
})

describe('uniqueById', () => {
  it('keeps the first of each id in order', () => {
    expect(uniqueById([{ id: 'a', v: 1 }, { id: 'b', v: 2 }, { id: 'a', v: 3 }])).toEqual([{ id: 'a', v: 1 }, { id: 'b', v: 2 }])
  })
})

describe('buildTripCostDrive', () => {
  const drives = [
    { start_time: '2026-05-02T08:00:00Z', start_address: 'Paris, France', end_address: 'Dijon, France', distance_km: 300, duration_min: 200, costs: { electricity_kwh: 50, electricity_cost: 11, tires_cost: 3, maintenance_cost: 2, insurance_cost: 1, has_estimates: false } },
    { start_time: '2026-05-03T08:00:00Z', start_address: 'Dijon, France', end_address: 'Lyon, France', distance_km: 200, duration_min: 150, costs: { electricity_kwh: 30, electricity_cost: 7, tires_cost: 2, maintenance_cost: 1, insurance_cost: 1, has_estimates: true } },
  ]

  it('sums the drives and adds the trip expenses to the total', () => {
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes', expenses_total: 20 }, drives)
    expect(t.is_trip_group).toBe(true)
    expect(t.trip_group_name).toBe('Alpes')
    expect(t.drives_count).toBe(2)
    expect(t.distance_km).toBe(500)
    expect(t.duration_min).toBe(350)
    expect(t.start_address).toBe('Paris')
    expect(t.end_address).toBe('Lyon')
    expect(t.costs.electricity_cost).toBe(18)
    expect(t.costs.tolls_cost).toBe(20)
    expect(t.costs.total_cost).toBe(18 + 5 + 3 + 2 + 20)
    expect(t.costs.cost_per_km).toBeCloseTo(48 / 500)
    expect(t.costs.electricity_rate).toBeCloseTo(18 / 80)
    expect(t.costs.has_estimates).toBe(true)
  })

  it('counts the tolls entered on a drive of the trip, not only those attached to the trip itself', () => {
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes', expenses_total: 0, tolls_total: 10.3 }, drives)
    expect(t.costs.tolls_cost).toBe(10.3)
    expect(t.costs.total_cost).toBeCloseTo(18 + 5 + 3 + 2 + 10.3)
  })

  it('falls back to default rates and placeholders for an empty trip', () => {
    const t = buildTripCostDrive({ id: 'tg2', name: 'Vide', created_at: '2026-05-06' }, [])
    expect(t.start_time).toBe('2026-05-06')
    expect(t.start_address).toBe('Départ')
    expect(t.end_address).toBe('Arrivée')
    expect(t.costs.electricity_rate).toBe(0.22)
    expect(t.costs.tires_rate).toBe(0.02)
    expect(t.costs.maintenance_rate).toBe(0.015)
    expect(t.costs.cost_per_km).toBe(0)
  })
})
