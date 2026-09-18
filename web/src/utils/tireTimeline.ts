export interface TimelineTireInfo {
  tireId: string
  label: string
  dimension: string
  position: string
  startKm: number
  endKm: number
  ongoing: boolean
}

export interface TimelineSegment {
  id: string
  startKm: number
  endKm: number
  colorKey: string
  color: string
  tires: TimelineTireInfo[]
}

export interface TimelineGap {
  startKm: number
  endKm: number
}

export interface TireTimeline {
  maxKm: number
  segments: TimelineSegment[]
  gaps: TimelineGap[]
  legend: { key: string; color: string }[]
}

// Sessions whose bounds differ by less than this are the same physical mounting (a set of tires).
const GROUP_TOLERANCE_KM = 50

const PALETTE = ['#f43f5e', '#38bdf8', '#34d399', '#fbbf24', '#a78bfa', '#fb923c', '#2dd4bf', '#f472b6', '#a3e635', '#60a5fa']

interface FlatSession extends TimelineTireInfo {
  colorKey: string
}

function flattenSessions(tires: any[], currentKm: number): FlatSession[] {
  const flat: FlatSession[] = []
  for (const t of tires) {
    for (const s of t.sessions || []) {
      const start = Number(s.mounted_odometer)
      if (!Number.isFinite(start)) continue
      const ongoing = !s.dismounted_date
      let end = ongoing ? currentKm : Number(s.dismounted_odometer)
      if (!ongoing && !(end > start) && s.distance_km > 0) end = start + Number(s.distance_km)
      if (!(end > start)) continue
      const label = `${t.tire.brand} ${t.tire.model}`.trim()
      flat.push({
        tireId: t.tire.id,
        label,
        dimension: t.tire.dimension,
        position: s.position,
        startKm: start,
        endKm: end,
        ongoing,
        colorKey: `${label} ${t.tire.dimension}`,
      })
    }
  }
  return flat.sort((a, b) => a.startKm - b.startKm || a.endKm - b.endKm)
}

export function buildTireTimeline(tires: any[], currentOdometer: number): TireTimeline {
  const flat = flattenSessions(tires, currentOdometer)
  const maxKm = Math.max(currentOdometer, ...flat.map((s) => s.endKm), 0)

  const colorByKey = new Map<string, string>()
  const colorFor = (key: string) => {
    if (!colorByKey.has(key)) colorByKey.set(key, PALETTE[colorByKey.size % PALETTE.length])
    return colorByKey.get(key)!
  }

  const segments: TimelineSegment[] = []
  for (const s of flat) {
    const last = segments[segments.length - 1]
    const sameSet =
      last &&
      Math.abs(last.startKm - s.startKm) <= GROUP_TOLERANCE_KM &&
      Math.abs(last.endKm - s.endKm) <= GROUP_TOLERANCE_KM
    if (sameSet) {
      last.tires.push(s)
      last.endKm = Math.max(last.endKm, s.endKm)
      continue
    }
    segments.push({
      id: `${s.tireId}-${s.startKm}`,
      startKm: s.startKm,
      endKm: s.endKm,
      colorKey: s.colorKey,
      color: colorFor(s.colorKey),
      tires: [s],
    })
  }

  // Overlapping segments (a lone tire on a partially covered range) are clipped so the bar stays a single line.
  const clipped: TimelineSegment[] = []
  const gaps: TimelineGap[] = []
  let cursor = 0
  for (const seg of segments) {
    if (seg.startKm > cursor) gaps.push({ startKm: cursor, endKm: seg.startKm })
    const start = Math.max(seg.startKm, cursor)
    if (seg.endKm > start) {
      clipped.push({ ...seg, startKm: start })
      cursor = seg.endKm
    }
  }
  if (cursor < maxKm) gaps.push({ startKm: cursor, endKm: maxKm })

  const legend = [...colorByKey.entries()].map(([key, color]) => ({ key, color }))
  return { maxKm, segments: clipped, gaps, legend }
}

export function tickStep(maxKm: number): number {
  if (maxKm <= 20000) return 5000
  if (maxKm <= 60000) return 10000
  return 20000
}
