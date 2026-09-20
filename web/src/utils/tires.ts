import { CloudSun, Snowflake, Sun } from 'lucide-vue-next'
import { todayIso, toIsoDay } from '@/utils/dates'

export const MOUNTED_POSITIONS = ['FL', 'FR', 'RL', 'RR'] as const

export const isMountedPosition = (position: string): boolean => (MOUNTED_POSITIONS as readonly string[]).includes(position)

export function getConditionBadge(condition: string) {
  switch (condition) {
    case 'GOOD':
      return { label: 'Bon état', class: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' }
    case 'WARNING':
      return { label: 'À surveiller', class: 'bg-amber-500/10 text-amber-400 border-amber-500/20' }
    case 'CRITICAL':
      return { label: 'Usure critique', class: 'bg-rose-500/10 text-rose-400 border-rose-500/20' }
    default:
      return { label: 'Inconnu', class: 'bg-slate-800 text-slate-400 border-slate-700' }
  }
}

export function getSeasonIcon(season: string) {
  switch (season) {
    case 'WINTER':
      return { icon: Snowflake, color: 'text-sky-400', label: 'Hiver' }
    case 'ALL_SEASON':
      return { icon: CloudSun, color: 'text-amber-400', label: '4 Saisons' }
    default:
      return { icon: Sun, color: 'text-orange-400', label: 'Été' }
  }
}

export function formatDate(d: string) {
  return new Date(d).toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

export function getTireSelectLabel(t: any): string {
  if (!t || !t.tire) return ''
  const pos = t.tire.current_position === 'STORAGE'
    ? 'Au garage'
    : t.tire.current_position === 'DISPOSED'
    ? 'Au rebut'
    : 'Roue ' + t.tire.current_position
  const km = Math.round(t.total_distance_km ?? t.tire.accumulated_distance_km ?? 0)
  const sessionCount = t.sessions?.length ?? 0
  const sessionLabel = sessionCount > 1 ? `${sessionCount} sessions` : `${sessionCount} session`
  const dot = t.tire.dot_code ? ` • DOT ${t.tire.dot_code}` : ''
  return `${t.tire.brand} ${t.tire.model} (${t.tire.dimension}) — ${pos} • ${km.toLocaleString('fr-FR')} km • ${sessionLabel}${dot}`
}

/** Date and odometer of the most recent dismount of a tire, from the sessions carried by the tire list. */
export function getLastDismountInfo(tires: any[], tireId: string) {
  const tireEntry = tires.find((t) => t.tire.id === tireId)
  const sessions = tireEntry?.sessions || []
  const dismountedSessions = sessions
    .filter((s: any) => s.dismounted_date)
    .sort((a: any, b: any) => new Date(b.dismounted_date).getTime() - new Date(a.dismounted_date).getTime())
  if (dismountedSessions.length > 0) {
    const s = dismountedSessions[0]
    return {
      date: toIsoDay(s.dismounted_date),
      odometer: s.dismounted_odometer || null,
    }
  }
  return null
}

/** The tires a copy or duplication targets by default: same brand and model when there are some, otherwise every other tire. */
export function defaultTargetTireIds(tires: any[], source: { id?: string; brand?: string; model?: string } | null | undefined): string[] {
  const others = tires.filter((t) => t.tire.id !== source?.id)
  const sameFamily = others.filter((t) => t.tire.brand === source?.brand && t.tire.model === source?.model)
  return (sameFamily.length > 0 ? sameFamily : others).map((t) => t.tire.id)
}

export interface SessionForm {
  position: string
  mounted_date: string
  mounted_odometer: number
  is_dismounted: boolean
  dismounted_date: string
  dismounted_odometer: number
  distance_km: number
  notes: string
}

export const emptySessionForm = (): SessionForm => ({
  position: 'FL',
  mounted_date: todayIso(),
  mounted_odometer: 0,
  is_dismounted: true,
  dismounted_date: todayIso(),
  dismounted_odometer: 0,
  distance_km: 0,
  notes: '',
})

export const sessionFormFromSession = (s: any): SessionForm => ({
  position: s.position,
  mounted_date: toIsoDay(s.mounted_date),
  mounted_odometer: s.mounted_odometer,
  is_dismounted: !!s.dismounted_date,
  dismounted_date: s.dismounted_date ? toIsoDay(s.dismounted_date) : todayIso(),
  dismounted_odometer: s.dismounted_odometer || 0,
  distance_km: s.distance_km || 0,
  notes: s.notes || '',
})

/** A session snapshot kept by the "copy" button so it can be pasted onto another tire. */
export const copiedSessionFromSession = (s: any): SessionForm => ({
  position: s.position,
  mounted_date: s.mounted_date ? toIsoDay(s.mounted_date) : '',
  mounted_odometer: s.mounted_odometer || 0,
  is_dismounted: !!s.dismounted_date,
  dismounted_date: s.dismounted_date ? toIsoDay(s.dismounted_date) : '',
  dismounted_odometer: s.dismounted_odometer || 0,
  distance_km: s.distance_km || 0,
  notes: s.notes || '',
})

/** Form for pasting a copied session on a tire: a mounted tire keeps its own wheel, otherwise the copied position wins. */
export const sessionFormFromCopy = (copied: SessionForm, tire: { current_position?: string } | null | undefined): SessionForm => ({
  ...copied,
  position:
    tire && tire.current_position !== 'STORAGE' && tire.current_position !== 'DISPOSED'
      ? (tire.current_position as string)
      : copied.position || 'FL',
})

export interface TireLogForm {
  depth_mm: number
  odometer: number
  notes: string
  date: string
}
