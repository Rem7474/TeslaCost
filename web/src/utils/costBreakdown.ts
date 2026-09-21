import { t } from '@/i18n'

// One color and label per cost category, shared by the monthly breakdown and the drive breakdown.
export const COST_COLORS = {
  energy: '#38bdf8',
  tolls: '#f59e0b',
  tires: '#10b981',
  maintenance: '#ec4899',
  insurance: '#a855f7',
  financing: '#f97316',
  other: '#64748b',
} as const

export type CostCategory = keyof typeof COST_COLORS

export interface DriveBreakdownItem {
  key: CostCategory
  label: string
  color: string
  amount: number
  sharePct: number
  costPerKm: number
}

/** Cost items of a drive or trip (costs as returned by the drives API) with their share of the total and cost per km. */
export function buildDriveBreakdown(costs: any, distanceKm: number) {
  const raw: [CostCategory, number][] = [
    ['energy', Number(costs?.electricity_cost) || 0],
    ['tires', Number(costs?.tires_cost) || 0],
    ['maintenance', Number(costs?.maintenance_cost) || 0],
    ['insurance', Number(costs?.insurance_cost) || 0],
    ['tolls', Number(costs?.tolls_cost) || 0],
  ]
  const total = raw.reduce((sum, [, amount]) => sum + amount, 0)
  const items: DriveBreakdownItem[] = raw.map(([key, amount]) => ({
    key,
    label: t(`dashboard.breakdown.${key}`),
    color: COST_COLORS[key],
    amount,
    sharePct: total > 0 ? (amount / total) * 100 : 0,
    costPerKm: distanceKm > 0 ? amount / distanceKm : 0,
  }))
  return { items, total, byKey: Object.fromEntries(items.map((i) => [i.key, i])) as Record<CostCategory, DriveBreakdownItem> }
}
