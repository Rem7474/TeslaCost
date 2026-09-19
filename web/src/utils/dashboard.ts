export const monthlyRangeOptions = [
  { key: 'ALL', label: 'Tout' },
  { key: '1Y', label: '1 an' },
  { key: '6M', label: '6 mois' },
  { key: '3M', label: '3 mois' },
  { key: '1M', label: '1 mois' },
] as const
export type MonthlyRangeKey = (typeof monthlyRangeOptions)[number]['key']

const monthlyRangeMonths: Record<MonthlyRangeKey, number | null> = {
  ALL: null,
  '1Y': 12,
  '6M': 6,
  '3M': 3,
  '1M': 1,
}

/** The last N months of the monthly costs for a chart range. */
export function filterMonthsByRange<T>(list: T[], range: MonthlyRangeKey): T[] {
  const months = monthlyRangeMonths[range]
  return months ? list.slice(-months) : list
}

export function formatMonthName(monthStr: string) {
  if (!monthStr) return ''
  const parts = monthStr.split('-')
  if (parts.length < 2) return monthStr
  const year = Number.parseInt(parts[0], 10)
  const month = Number.parseInt(parts[1], 10) - 1
  const d = new Date(year, month, 1)
  const formatted = d.toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })
  return formatted.charAt(0).toUpperCase() + formatted.slice(1)
}

/** The current month (or the latest one when it has no row yet) with the previous month's distance. */
export function currentMonthStats(monthlyCosts: any[] | undefined, now = new Date()) {
  if (!monthlyCosts?.length) return null
  const key = now.toISOString().substring(0, 7) // YYYY-MM
  const list = monthlyCosts
  const current = list.find((m: any) => m.month === key) || list[list.length - 1]
  const prevIdx = list.indexOf(current) - 1
  const prev = prevIdx >= 0 ? list[prevIdx] : null

  return {
    raw: current,
    month: current.month,
    distance_km: current.distance_km || 0,
    cost_per_km: current.cost_per_km || 0,
    total: current.total || 0,
    prevDistance: prev ? prev.distance_km : null,
  }
}

/** Reminders that are overdue or due soon, with a short sentence naming the first two. */
export function summarizeUrgentReminders(reminders: { title: string; status: string }[]) {
  const urgent = reminders.filter((r) => r.status === 'OVERDUE' || r.status === 'DUE_SOON')
  const titles = urgent.map((r) => r.title)
  const summary = !titles.length ? '' : titles.length <= 2 ? titles.join(', ') : `${titles.slice(0, 2).join(', ')} et ${titles.length - 2} autre(s)`
  return { urgent, hasOverdue: reminders.some((r) => r.status === 'OVERDUE'), summary }
}

/** Follow-up figures of a lease (LOA / LLD) contract: elapsed time, mileage against the allowance, status. */
export function buildLeaseSummary(tco: any, now = new Date()) {
  if (!tco || !['LOA', 'LLD'].includes(tco.acquisition_type)) {
    return null
  }
  const t = tco
  const startDateStr = t.contract_start_date
  const endDateStr = t.contract_end_date
  if (!endDateStr && !t.contract_duration_months && !t.lease_duration_months) {
    return null
  }

  const start = startDateStr ? new Date(startDateStr) : null
  const durationMonths = t.contract_duration_months || t.lease_duration_months || 0
  let end = endDateStr ? new Date(endDateStr) : null
  if (!end && start && durationMonths > 0) {
    end = new Date(start.getFullYear(), start.getMonth() + durationMonths, start.getDate())
  }

  let totalMonths = durationMonths
  if (!totalMonths && start && end) {
    totalMonths = Math.max(1, Math.round((end.getTime() - start.getTime()) / (1000 * 3600 * 24 * 30.4375)))
  }

  let elapsedMonths = 0
  let remainingMonths = 0
  let durationProgressPct = 0
  let isEnded = false
  let isNotStarted = false

  if (start && end) {
    const totalMs = end.getTime() - start.getTime()
    const elapsedMs = now.getTime() - start.getTime()
    if (elapsedMs < 0) {
      isNotStarted = true
      durationProgressPct = 0
      remainingMonths = totalMonths
    } else if (now.getTime() >= end.getTime()) {
      isEnded = true
      durationProgressPct = 100
      elapsedMonths = totalMonths
      remainingMonths = 0
    } else {
      durationProgressPct = Math.min(100, Math.max(0, Math.round((elapsedMs / totalMs) * 100)))
      elapsedMonths = Math.max(0, Math.round(elapsedMs / (1000 * 3600 * 24 * 30.4375)))
      remainingMonths = Math.max(0, Math.round((end.getTime() - now.getTime()) / (1000 * 3600 * 24 * 30.4375)))
    }
  } else if (totalMonths > 0) {
    durationProgressPct = 50
  }

  // Mileage
  const kmDriven = t.lease_km_driven || 0
  const kmAllowanceToDate = t.lease_km_allowance_to_date || 0
  const kmAllowanceTotal = t.lease_km_allowance_total || 0
  const hasMileageAllowance = kmAllowanceToDate > 0 || kmAllowanceTotal > 0

  let mileageProgressPct = 0
  let kmDiff = 0
  let actualPaceKmMonth = 0
  let contractualPaceKmMonth = 0

  if (hasMileageAllowance) {
    if (kmAllowanceTotal > 0) {
      mileageProgressPct = Math.min(100, Math.max(0, Math.round((kmDriven / kmAllowanceTotal) * 100)))
    } else if (kmAllowanceToDate > 0) {
      mileageProgressPct = Math.min(100, Math.max(0, Math.round((kmDriven / kmAllowanceToDate) * 100)))
    }

    kmDiff = Math.round(kmDriven - kmAllowanceToDate)

    if (elapsedMonths > 0) {
      actualPaceKmMonth = Math.round(kmDriven / elapsedMonths)
    }
    if (t.lease_km_allowance_per_year) {
      contractualPaceKmMonth = Math.round(t.lease_km_allowance_per_year / 12)
    } else if (totalMonths > 0 && kmAllowanceTotal > 0) {
      contractualPaceKmMonth = Math.round(kmAllowanceTotal / totalMonths)
    }
  }

  // Status
  let status = { label: 'En cours', class: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30' }
  if (t.option_exercised_date) {
    status = { label: "Option d'achat levée", class: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' }
  } else if (isEnded) {
    status = { label: 'Terminé', class: 'bg-slate-800 text-slate-400 border-slate-700' }
  } else if (remainingMonths <= 3 && !isNotStarted) {
    status = { label: 'Échéance proche', class: 'bg-amber-500/15 text-amber-300 border-amber-500/30' }
  }

  // Color for mileage bar
  let mileageColor = 'bg-gradient-to-r from-emerald-500 to-teal-500'
  if (kmAllowanceToDate > 0 && kmDriven > kmAllowanceToDate) {
    mileageColor = 'bg-gradient-to-r from-rose-500 to-red-500'
  } else if (kmAllowanceToDate > 0 && kmDriven / kmAllowanceToDate > 0.92) {
    mileageColor = 'bg-gradient-to-r from-amber-500 to-orange-500'
  }

  return {
    acquisitionType: t.acquisition_type,
    startDate: start,
    endDate: end,
    totalMonths,
    elapsedMonths,
    remainingMonths,
    durationProgressPct,
    isEnded,
    isNotStarted,
    status,
    // Mileage
    hasMileageAllowance,
    kmDriven,
    kmAllowanceToDate,
    kmAllowanceTotal,
    mileageProgressPct,
    kmDiff,
    actualPaceKmMonth,
    contractualPaceKmMonth,
    mileageColor,
    // Financials
    monthlyRent: t.lease_monthly_rent,
    downPayment: t.lease_down_payment,
    purchaseOptionPrice: t.lease_purchase_option_price,
    excessKmCost: t.lease_excess_km_cost || 0,
    excessKmProjected: t.lease_excess_km_projected || 0,
    excessKmPrice: t.lease_excess_km_price,
    // Included services
    includesMaintenance: t.lease_includes_maintenance,
    includesInsurance: t.lease_includes_insurance,
    includesTires: t.lease_includes_tires,
  }
}

export type MonthDetailMode = 'economic' | 'cash'

/** Cost items of one month, in the economic view (smoothed) or the cash view (what was paid), with their share and cost per km. */
export function buildMonthBreakdown(m: any, mode: MonthDetailMode) {
  const dist = m.distance_km || 0

  const items = [
    {
      key: 'energy',
      label: 'Énergie',
      subLabel: '',
      color: '#38bdf8',
      amount: m.energy || 0,
      cashAmount: m.energy || 0,
      note: m.smoothed_energy > 0 ? `dont ${m.smoothed_energy.toFixed(2)} € estimés avant TeslaMate` : null,
    },
    {
      key: 'tolls',
      label: 'Péages & Parkings',
      subLabel: '',
      color: '#f59e0b',
      amount: m.tolls || 0,
      cashAmount: m.tolls || 0,
      note: null,
    },
    {
      key: 'tires',
      label: 'Pneus',
      subLabel: 'usure amortie',
      color: '#10b981',
      amount: m.tires_amortized || 0,
      cashAmount: m.tires || 0,
      note: (m.tires || 0) > 0
        ? `${Number(m.tires).toFixed(2)} € décaissés ce mois (achat pneus)`
        : (m.tires_amortized > 0 ? `Amorti sur ${Math.round(dist).toLocaleString('fr-FR')} km (0 € décaissé)` : null),
    },
    {
      key: 'maintenance',
      label: 'Entretien & Réparations',
      subLabel: 'lissé',
      color: '#ec4899',
      amount: m.maintenance_amortized || 0,
      cashAmount: m.maintenance || 0,
      note: (m.maintenance || 0) > 0
        ? `${Number(m.maintenance).toFixed(2)} € facturés à l'atelier ce mois`
        : (m.maintenance_amortized > 0 ? `Lissage révisions/pièces sur la période` : null),
    },
    {
      key: 'insurance',
      label: 'Assurance',
      subLabel: '',
      color: '#a855f7',
      amount: m.insurance || 0,
      cashAmount: m.insurance || 0,
      note: null,
    },
    {
      key: 'financing',
      label: 'Financement & Location',
      subLabel: 'lissé',
      color: '#f97316',
      amount: m.financing_amortized || m.financing || 0,
      cashAmount: m.financing || 0,
      note: (m.financing_amortized > 0 && Math.abs(m.financing_amortized - (m.financing || 0)) > 0.01)
        ? `Lissé : ${m.financing_amortized.toFixed(2)} € (mensualité réglée : ${(m.financing || 0).toFixed(2)} €)`
        : null,
    },
    {
      key: 'other',
      label: 'Abonnements, taxes & autres',
      subLabel: '',
      color: '#64748b',
      amount: m.other || 0,
      cashAmount: m.other || 0,
      note: null,
    },
  ]

  const economicTotal = items.reduce((sum, it) => sum + it.amount, 0)
  const cashTotal = typeof m.total === 'number' ? m.total : items.reduce((sum, it) => sum + it.cashAmount, 0)
  const activeTotal = mode === 'economic' ? economicTotal : cashTotal

  const itemsWithStats = items.map((it) => {
    const displayAmount = mode === 'economic' ? it.amount : it.cashAmount
    const costPerKm = dist > 0 ? displayAmount / dist : 0
    const sharePct = activeTotal > 0 ? (displayAmount / activeTotal) * 100 : 0
    return {
      ...it,
      displayAmount,
      costPerKm,
      sharePct,
    }
  })

  return {
    month: m.month,
    distanceKm: dist,
    trackedDistanceKm: m.tracked_distance_km || 0,
    smoothedKm: m.smoothed_km || 0,
    costPerKm: m.cost_per_km || (dist > 0 ? economicTotal / dist : 0),
    economicTotal,
    cashTotal,
    activeTotal,
    items: itemsWithStats,
  }
}
