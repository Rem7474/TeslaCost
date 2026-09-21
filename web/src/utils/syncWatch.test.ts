import { describe, expect, it } from 'vitest'
import { checkSyncProgress } from './syncWatch'

const done = (finished_at: string) => ({ status: 'SUCCEEDED', finished_at })

describe('checkSyncProgress', () => {
  it('only records the first look at a vehicle', () => {
    const r = checkSyncProgress(null, 'v1', done('2026-09-21T10:00:00Z'))
    expect(r.refresh).toBe(false)
    expect(r.seen).toEqual({ vehicleId: 'v1', signature: 'SUCCEEDED|2026-09-21T10:00:00Z' })
  })

  it('does not refresh while nothing changed', () => {
    const first = checkSyncProgress(null, 'v1', done('2026-09-21T10:00:00Z'))
    expect(checkSyncProgress(first.seen, 'v1', done('2026-09-21T10:00:00Z')).refresh).toBe(false)
  })

  it('refreshes when a synchronization finished since', () => {
    const first = checkSyncProgress(null, 'v1', done('2026-09-21T10:00:00Z'))
    const r = checkSyncProgress(first.seen, 'v1', done('2026-09-21T10:30:00Z'))
    expect(r.refresh).toBe(true)
    expect(r.seen.signature).toBe('SUCCEEDED|2026-09-21T10:30:00Z')
  })

  it('does not refresh for a failed or still running job', () => {
    const first = checkSyncProgress(null, 'v1', done('2026-09-21T10:00:00Z'))
    expect(checkSyncProgress(first.seen, 'v1', { status: 'FAILED', finished_at: '2026-09-21T10:30:00Z' }).refresh).toBe(false)
    expect(checkSyncProgress(first.seen, 'v1', { status: 'RUNNING' }).refresh).toBe(false)
  })

  it('does not treat another vehicle as a change', () => {
    const first = checkSyncProgress(null, 'v1', done('2026-09-21T10:00:00Z'))
    const r = checkSyncProgress(first.seen, 'v2', done('2026-09-21T10:45:00Z'))
    expect(r.refresh).toBe(false)
    expect(r.seen.vehicleId).toBe('v2')
  })

  it('handles a vehicle that never synchronized', () => {
    const first = checkSyncProgress(null, 'v1', { status: 'NONE' })
    expect(first.seen.signature).toBe('NONE|')
    expect(checkSyncProgress(first.seen, 'v1', { status: 'NONE' }).refresh).toBe(false)
  })
})
