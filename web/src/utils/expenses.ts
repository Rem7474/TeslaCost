export const CURRENCIES = ['EUR', 'CHF', 'GBP', 'USD']

export const CATEGORY_LABELS: Record<string, string> = {
  MAINTENANCE: 'Entretien',
  REPAIR: 'Réparation / sinistre',
  INSURANCE: 'Assurance',
  SUBSCRIPTION: 'Abonnement',
  TAX: 'Taxe',
  FINANCING: 'Financement',
  ACCESSORY: 'Accessoire',
  OTHER: 'Autre',
}

export interface ReminderPreset {
  title: string
  category: string
  interval_km: number | ''
  interval_months: number | ''
  lead_km: number
  lead_days: number
}

export const REMINDER_PRESETS: ReminderPreset[] = [
  {
    title: 'Permutation des pneus',
    category: 'TIRES',
    interval_km: 10000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
  {
    title: 'Filtre d\'habitacle',
    category: 'MAINTENANCE',
    interval_km: 40000,
    interval_months: 24,
    lead_km: 2000,
    lead_days: 30,
  },
  {
    title: 'Contrôle liquide de frein',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: 'Contrôle technique',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: 'Balais d\'essuie-glace',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 12,
    lead_km: 0,
    lead_days: 15,
  },
  {
    title: 'Nettoyage & graissage des étriers',
    category: 'MAINTENANCE',
    interval_km: 20000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
]

/** datetime-local inputs expect local time, not UTC. */
export function toLocalDateTimeInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Currency fields of an expense payload: a foreign currency carries its rate to EUR. */
export function currencyPayload(form: { currency: string; fx_rate: string }) {
  if (form.currency === 'EUR') return { currency: 'EUR', fx_rate: null }
  return { currency: form.currency, fx_rate: form.fx_rate ? Number(form.fx_rate) : null }
}

export function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 o'
  const k = 1024
  const sizes = ['o', 'Ko', 'Mo', 'Go']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

/** Drives the user picked for a toll that are older than the drives the modal loaded. */
export const countUnlistedDrives = (selectedIds: string[], listedDrives: { id: string }[]): number =>
  selectedIds.filter((id) => !listedDrives.some((d) => d.id === id)).length

/** The earlier maintenance a new one can close: amortized, dated on or before the form date, not the one being edited. */
export function findCloseCandidate(maintenances: any[] | null | undefined, editingId: string | null, formDate: string) {
  if (!maintenances || !maintenances.length) return null
  return (
    maintenances.find(
      (m) =>
        m.id !== editingId &&
        m.amortization_mode &&
        m.amortization_mode !== 'NONE' &&
        new Date(m.date).toISOString().substring(0, 10) <= formDate
    ) || null
  )
}
