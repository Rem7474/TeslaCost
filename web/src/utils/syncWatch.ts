/** What was last seen of a vehicle's synchronization, to notice a background one that finished since. */
export interface SyncSeen {
  vehicleId: string
  signature: string
}

function signatureOf(job: any): string {
  return `${job?.status ?? 'NONE'}|${job?.finished_at ?? ''}`
}

/**
 * Compares the current synchronization job with the last one seen. The first look at a vehicle only records it;
 * afterwards, a changed successful job means new data arrived (the scheduled synchronization ran).
 */
export function checkSyncProgress(seen: SyncSeen | null, vehicleId: string, job: any): { refresh: boolean; seen: SyncSeen } {
  const signature = signatureOf(job)
  const next = { vehicleId, signature }
  if (!seen || seen.vehicleId !== vehicleId) return { refresh: false, seen: next }
  return { refresh: seen.signature !== signature && job?.status === 'SUCCEEDED', seen: next }
}

/** Coming back to the tab after this long, the pages reload whatever the synchronization says (other devices, other users). */
export const STALE_AFTER_HIDDEN_MS = 5 * 60 * 1000
export const POLL_INTERVAL_MS = 60 * 1000
