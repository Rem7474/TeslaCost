<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api, type MaintenanceReminder } from '@/services/api'
import {
  Coins,
  Gauge,
  Zap,
  Receipt,
  Disc,
  Wrench,
  TrendingUp,
  Briefcase,
  User,
  ArrowRight,
  Activity,
  Calendar,
  AlertTriangle,
  Shield,
  X,
  ChevronLeft,
  ChevronRight,
  PieChart,
  Info,
  FileText,
  CheckCircle2,
  Clock,
  Bell,
} from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const tco = ref<any | null>(null)
const loading = ref(true)
const dashboardReminders = ref<MaintenanceReminder[]>([])

const urgentReminders = computed(() => {
  return dashboardReminders.value.filter(
    (r) => r.status === 'OVERDUE' || r.status === 'DUE_SOON'
  )
})

const hasOverdueReminders = computed(() => {
  return dashboardReminders.value.some((r) => r.status === 'OVERDUE')
})

const urgentRemindersSummary = computed(() => {
  if (!urgentReminders.value.length) return ''
  const titles = urgentReminders.value.map((r) => r.title)
  if (titles.length <= 2) {
    return titles.join(', ')
  }
  return `${titles.slice(0, 2).join(', ')} et ${titles.length - 2} autre(s)`
})

const monthlyChartRef = ref<HTMLCanvasElement | null>(null)
const mileageChartRef = ref<HTMLCanvasElement | null>(null)
const donutChartRef = ref<HTMLCanvasElement | null>(null)

const monthlyRangeOptions = [
  { key: 'ALL', label: 'Tout' },
  { key: '1Y', label: '1 an' },
  { key: '6M', label: '6 mois' },
  { key: '3M', label: '3 mois' },
  { key: '1M', label: '1 mois' },
] as const
type MonthlyRangeKey = (typeof monthlyRangeOptions)[number]['key']
const monthlyChartRange = ref<MonthlyRangeKey>('ALL')

const monthlyRangeMonths: Record<MonthlyRangeKey, number | null> = {
  ALL: null,
  '1Y': 12,
  '6M': 6,
  '3M': 3,
  '1M': 1,
}

const filteredMonthlyCosts = computed(() => {
  const list = tco.value?.monthly_costs || []
  const months = monthlyRangeMonths[monthlyChartRange.value]
  return months ? list.slice(-months) : list
})

const mileageChartRange = ref<MonthlyRangeKey>('ALL')

const filteredMileageCosts = computed(() => {
  const list = tco.value?.monthly_costs || []
  const months = monthlyRangeMonths[mileageChartRange.value]
  return months ? list.slice(-months) : list
})

const dataQuality = ref<any | null>(null)
const showDataQuality = ref(false)

async function toggleDataQuality() {
  showDataQuality.value = !showDataQuality.value
  if (showDataQuality.value && vehicleStore.activeVehicle) {
    try {
      dataQuality.value = await api.getDataQuality(vehicleStore.activeVehicle.id)
    } catch (err) {
      console.error('Failed to load data quality', err)
    }
  }
}

const issueLabels: Record<string, string> = {
  ODOMETER_GAP: 'Trou d\'odomètre avant ce trajet',
  ODOMETER_REGRESSION: 'Odomètre en recul par rapport au trajet précédent',
  DISTANCE_MISMATCH: 'Distance différente du relevé d\'odomètre',
}

const insuranceSourceLabel = computed(() => {
  switch (tco.value?.insurance_source) {
    case 'RECORDED_EXPENSES':
      return 'primes enregistrées'
    case 'INCLUDED_IN_LEASE':
      return 'incluse dans la location'
    default:
      return 'non renseignée'
  }
})

// Contract lease computed metrics
const leaseContract = computed(() => {
  if (!tco.value || !['LOA', 'LLD'].includes(tco.value.acquisition_type)) {
    return null
  }
  const t = tco.value
  const startDateStr = t.contract_start_date
  const endDateStr = t.contract_end_date
  if (!endDateStr && !t.contract_duration_months && !t.lease_duration_months) {
    return null
  }

  const now = new Date()
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
})

let monthlyChartInstance: Chart | null = null
let mileageChartInstance: Chart | null = null
let donutChartInstance: Chart | null = null

const selectedMonth = ref<any | null>(null)
const monthDonutRef = ref<HTMLCanvasElement | null>(null)
let monthDonutChartInstance: Chart | null = null
const monthDetailViewMode = ref<'economic' | 'cash'>('economic')

function formatMonthName(monthStr: string) {
  if (!monthStr) return ''
  const parts = monthStr.split('-')
  if (parts.length < 2) return monthStr
  const year = Number.parseInt(parts[0], 10)
  const month = Number.parseInt(parts[1], 10) - 1
  const d = new Date(year, month, 1)
  const formatted = d.toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })
  return formatted.charAt(0).toUpperCase() + formatted.slice(1)
}

const selectedMonthIndex = computed(() => {
  if (!selectedMonth.value || !tco.value?.monthly_costs) return -1
  return tco.value.monthly_costs.findIndex((m: any) => m.month === selectedMonth.value.month)
})

const hasPrevMonth = computed(() => selectedMonthIndex.value > 0)
const hasNextMonth = computed(() => {
  if (!tco.value?.monthly_costs) return false
  return selectedMonthIndex.value >= 0 && selectedMonthIndex.value < tco.value.monthly_costs.length - 1
})

function selectPrevMonth() {
  if (hasPrevMonth.value && tco.value?.monthly_costs) {
    openMonthDetail(tco.value.monthly_costs[selectedMonthIndex.value - 1])
  }
}

function selectNextMonth() {
  if (hasNextMonth.value && tco.value?.monthly_costs) {
    openMonthDetail(tco.value.monthly_costs[selectedMonthIndex.value + 1])
  }
}

async function openMonthDetail(m: any) {
  if (!m) return
  selectedMonth.value = m
  await nextTick()
  renderMonthDonutChart()
}

function closeMonthDetail() {
  selectedMonth.value = null
  if (monthDonutChartInstance) {
    monthDonutChartInstance.destroy()
    monthDonutChartInstance = null
  }
}

const selectedMonthBreakdown = computed(() => {
  if (!selectedMonth.value) return null
  const m = selectedMonth.value
  const dist = m.distance_km || 0

  const items = [
    {
      key: 'energy',
      label: 'Énergie',
      subLabel: '',
      icon: Zap,
      color: '#38bdf8',
      amount: m.energy || 0,
      cashAmount: m.energy || 0,
      note: m.smoothed_energy > 0 ? `dont ${m.smoothed_energy.toFixed(2)} € estimés avant TeslaMate` : null,
    },
    {
      key: 'tolls',
      label: 'Péages & Parkings',
      subLabel: '',
      icon: Receipt,
      color: '#f59e0b',
      amount: m.tolls || 0,
      cashAmount: m.tolls || 0,
      note: null,
    },
    {
      key: 'tires',
      label: 'Pneus',
      subLabel: 'usure amortie',
      icon: Disc,
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
      icon: Wrench,
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
      icon: Shield,
      color: '#a855f7',
      amount: m.insurance || 0,
      cashAmount: m.insurance || 0,
      note: null,
    },
    {
      key: 'financing',
      label: 'Financement & Location',
      subLabel: 'lissé',
      icon: Briefcase,
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
      icon: Coins,
      color: '#64748b',
      amount: m.other || 0,
      cashAmount: m.other || 0,
      note: null,
    },
  ]

  const economicTotal = items.reduce((sum, it) => sum + it.amount, 0)
  const cashTotal = typeof m.total === 'number' ? m.total : items.reduce((sum, it) => sum + it.cashAmount, 0)
  const activeTotal = monthDetailViewMode.value === 'economic' ? economicTotal : cashTotal

  const itemsWithStats = items.map((it) => {
    const displayAmount = monthDetailViewMode.value === 'economic' ? it.amount : it.cashAmount
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
})

function renderMonthDonutChart() {
  if (monthDonutChartInstance) {
    monthDonutChartInstance.destroy()
    monthDonutChartInstance = null
  }
  if (!monthDonutRef.value || !selectedMonthBreakdown.value) return

  const breakdown = selectedMonthBreakdown.value
  const activeItems = breakdown.items.filter((it) => it.displayAmount > 0.005)

  let labels: string[]
  let data: number[]
  let backgroundColors: string[]

  if (activeItems.length > 0) {
    labels = activeItems.map((it) => it.label)
    data = activeItems.map((it) => it.displayAmount)
    backgroundColors = activeItems.map((it) => it.color)
  } else {
    labels = ['Aucune dépense']
    data = [1]
    backgroundColors = ['#334155']
  }

  monthDonutChartInstance = new Chart(monthDonutRef.value, {
    type: 'doughnut',
    data: {
      labels,
      datasets: [
        {
          data,
          backgroundColor: backgroundColors,
          borderWidth: 0,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'bottom',
          labels: {
            color: '#94a3b8',
            font: { size: 11 },
            boxWidth: 10,
            padding: 10,
          },
        },
        tooltip: {
          callbacks: {
            label: (ctx) => {
              if (activeItems.length === 0) return ' Aucune dépense enregistrée'
              const val = Number(ctx.raw || 0).toFixed(2)
              const total = breakdown.activeTotal
              const pct = total > 0 ? ((Number(ctx.raw || 0) / total) * 100).toFixed(1) : '0'
              return ` ${ctx.label} : ${val} € (${pct}%)`
            },
          },
        },
      },
      cutout: '68%',
    },
  })
}

watch(monthDetailViewMode, () => {
  renderMonthDonutChart()
})

function onKeydown(e: KeyboardEvent) {
  if (!selectedMonth.value) return
  if (e.key === 'Escape') {
    closeMonthDetail()
  } else if (e.key === 'ArrowLeft') {
    selectPrevMonth()
  } else if (e.key === 'ArrowRight') {
    selectNextMonth()
  }
}

// Current month stats computed
const currentMonthStats = computed(() => {
  if (!tco.value?.monthly_costs?.length) return null
  const now = new Date().toISOString().substring(0, 7) // YYYY-MM
  const list = tco.value.monthly_costs
  const current = list.find((m: any) => m.month === now) || list[list.length - 1]
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
})

async function loadTCO() {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const [tcoData, remindersData] = await Promise.all([
      api.getTCO(vehicleStore.activeVehicle.id),
      api.getReminders(vehicleStore.activeVehicle.id).catch(() => []),
    ])
    tco.value = tcoData
    dashboardReminders.value = remindersData
  } catch (err) {
    console.error('Failed to load TCO or reminders', err)
  } finally {
    loading.value = false
    await nextTick()
    setTimeout(() => {
      renderCharts()
    }, 50)
  }
}

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => {
    loadTCO()
  }
)

watch([monthlyChartRange, mileageChartRange], () => {
  renderCharts()
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  loadTCO()
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  if (monthlyChartInstance) monthlyChartInstance.destroy()
  if (mileageChartInstance) mileageChartInstance.destroy()
  if (donutChartInstance) donutChartInstance.destroy()
  if (monthDonutChartInstance) monthDonutChartInstance.destroy()
})

function renderCharts() {
  if (!tco.value) return

  const monthlyList = tco.value.monthly_costs || []
  const labels = monthlyList.map((m: any) => m.month)

  // 1. Monthly Cost Breakdown Bar Chart
  if (monthlyChartRef.value) {
    if (monthlyChartInstance) monthlyChartInstance.destroy()

    const filteredList = filteredMonthlyCosts.value
    const filteredLabels = filteredList.map((m: any) => m.month)
    const energyData = filteredList.map((m: any) => m.energy)
    const tollsData = filteredList.map((m: any) => m.tolls)
    const tiresData = filteredList.map((m: any) => m.tires || 0)
    const maintData = filteredList.map((m: any) => m.maintenance)
    const insuranceData = filteredList.map((m: any) => m.insurance || 0)
    const otherData = filteredList.map((m: any) => m.other || 0)
    const financingData = filteredList.map((m: any) => m.financing || 0)

    monthlyChartInstance = new Chart(monthlyChartRef.value, {
      type: 'bar',
      data: {
        labels: filteredLabels,
        datasets: [
          { label: 'Énergie (€)', data: energyData, backgroundColor: '#38bdf8', borderRadius: 4 },
          { label: 'Péages & Parkings (€)', data: tollsData, backgroundColor: '#f59e0b', borderRadius: 4 },
          { label: 'Pneus (€)', data: tiresData, backgroundColor: '#10b981', borderRadius: 4 },
          { label: 'Entretien (€)', data: maintData, backgroundColor: '#ec4899', borderRadius: 4 },
          { label: 'Assurance (€)', data: insuranceData, backgroundColor: '#a855f7', borderRadius: 4 },
          { label: 'Financement (€)', data: financingData, backgroundColor: '#f97316', borderRadius: 4 },
          { label: 'Abonnements, taxes & autres (€)', data: otherData, backgroundColor: '#64748b', borderRadius: 4 },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: 'index',
          intersect: false,
        },
        onClick: (event: any) => {
          if (!monthlyChartInstance) return
          const points = monthlyChartInstance.getElementsAtEventForMode(
            event.native || event,
            'index',
            { intersect: false },
            true
          )
          if (points && points.length > 0) {
            const dataIndex = points[0].index
            const monthItem = filteredList[dataIndex]
            if (monthItem) {
              openMonthDetail(monthItem)
            }
          }
        },
        onHover: (event: any, elements: any[]) => {
          const canvas = monthlyChartRef.value
          if (canvas) {
            canvas.style.cursor = elements && elements.length > 0 ? 'pointer' : 'default'
          }
        },
        plugins: {
          legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
          tooltip: {
            callbacks: {
              label: (context) => {
                const val = Number(context.raw || 0).toFixed(2)
                return `${context.dataset.label} : ${val} €`
              },
              footer: (items) => {
                const total = items.reduce((sum, item) => sum + (Number(item.raw) || 0), 0)
                return `Total mois : ${total.toFixed(2)} €`
              },
            },
          },
        },
        scales: {
          x: { stacked: true, grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
          y: {
            stacked: true,
            grid: { color: '#1e293b' },
            ticks: { color: '#64748b', callback: (v) => `${v} €` },
          },
        },
      },
    })
  }

  // 2. Mileage & Cost/Km Dual-Axis Chart
  if (mileageChartRef.value) {
    if (mileageChartInstance) mileageChartInstance.destroy()

    const filteredMileageList = filteredMileageCosts.value
    const mileageLabels = filteredMileageList.map((m: any) => m.month)
    const distanceData = filteredMileageList.map((m: any) => m.distance_km || 0)
    const costPerKmData = filteredMileageList.map((m: any) => m.cost_per_km || 0)

    mileageChartInstance = new Chart(mileageChartRef.value, {
      type: 'bar',
      data: {
        labels: mileageLabels,
        datasets: [
          {
            type: 'bar',
            label: 'Kilomètres parcourus (km)',
            data: distanceData,
            backgroundColor: 'rgba(99, 102, 241, 0.65)',
            hoverBackgroundColor: 'rgba(99, 102, 241, 0.9)',
            borderRadius: 6,
            yAxisID: 'yDistance',
          },
          {
            type: 'line',
            label: 'Coût moyen au km (€/km)',
            data: costPerKmData,
            borderColor: '#10b981',
            backgroundColor: '#10b981',
            borderWidth: 2.5,
            pointBackgroundColor: '#10b981',
            pointRadius: 4,
            pointHoverRadius: 6,
            tension: 0.3,
            yAxisID: 'yCost',
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: 'index',
          intersect: false,
        },
        onClick: (event: any) => {
          if (!mileageChartInstance) return
          const points = mileageChartInstance.getElementsAtEventForMode(
            event.native || event,
            'index',
            { intersect: false },
            true
          )
          if (points && points.length > 0) {
            const dataIndex = points[0].index
            const monthItem = filteredMileageList[dataIndex]
            if (monthItem) {
              openMonthDetail(monthItem)
            }
          }
        },
        onHover: (event: any, elements: any[]) => {
          const canvas = mileageChartRef.value
          if (canvas) {
            canvas.style.cursor = elements && elements.length > 0 ? 'pointer' : 'default'
          }
        },
        plugins: {
          legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
          tooltip: {
            callbacks: {
              label: (context) => {
                if (context.dataset.yAxisID === 'yDistance') {
                  const dist = Number(context.raw).toLocaleString('fr-FR')
                  return `Distance : ${dist} km`
                }
                const costPerKm = Number(context.raw).toFixed(3)
                return `Coût de revient : ${costPerKm} €/km`
              },
            },
          },
        },
        scales: {
          x: { grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
          yDistance: {
            type: 'linear',
            position: 'left',
            grid: { color: '#1e293b' },
            ticks: {
              color: '#818cf8',
              callback: (v) => `${v} km`,
            },
            title: { display: true, text: 'Distance (km)', color: '#818cf8', font: { size: 11 } },
          },
          yCost: {
            type: 'linear',
            position: 'right',
            grid: { display: false },
            ticks: {
              color: '#10b981',
              callback: (v) => `${Number(v).toFixed(3)} €`,
            },
            title: { display: true, text: 'Coût/km (€)', color: '#10b981', font: { size: 11 } },
          },
        },
      },
    })
  }

  // 3. Donut Breakdown Chart
  if (donutChartRef.value) {
    if (donutChartInstance) donutChartInstance.destroy()

    donutChartInstance = new Chart(donutChartRef.value, {
      type: 'doughnut',
      data: {
        labels: ['Énergie', 'Péages & Parkings', 'Pneus (usure amortie)', 'Entretien & réparations', 'Assurance', 'Financement & location', 'Décote', 'Abonnements, taxes & autres'],
        datasets: [
          {
            data: [
              tco.value.energy_cost || 0,
              tco.value.tolls_cost || 0,
              tco.value.tires_amortized_cost || 0,
              (tco.value.maintenance_cost || 0) + (tco.value.repair_cost || 0),
              tco.value.insurance_cost || 0,
              Math.max(0, tco.value.financing_full_cost || 0),
              tco.value.depreciation_cost || 0,
              (tco.value.subscription_cost || 0) + (tco.value.tax_cost || 0) + (tco.value.other_cost || 0),
            ],
            backgroundColor: ['#38bdf8', '#f59e0b', '#10b981', '#ec4899', '#a855f7', '#f97316', '#e11d48', '#64748b'],
            borderWidth: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { position: 'bottom', labels: { color: '#94a3b8', font: { size: 11 } } },
        },
        cutout: '72%',
      },
    })
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header Summary -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
          Tableau de bord TCO
        </h2>
        <p class="text-sm text-slate-400">
          Coût réel de possession et rendement kilométrique pour {{ vehicleStore.activeVehicle?.name || 'votre véhicule' }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <router-link
          to="/expenses"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 transition-colors"
        >
          + Dépense
        </router-link>
        <router-link
          to="/drives"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/20 transition-colors"
        >
          Voir les trajets
        </router-link>
      </div>
    </div>

    <!-- Empty state if no vehicle -->
    <div v-if="!vehicleStore.activeVehicle" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl">
      <p class="text-slate-400 mb-4">Aucun véhicule configuré.</p>
      <router-link to="/vehicles" class="px-4 py-2 bg-rose-600 text-white text-sm font-semibold rounded-xl">
        Créer un premier véhicule
      </router-link>
    </div>

    <!-- SKELETON LOADING STATE -->
    <div v-else-if="loading" class="space-y-6 animate-pulse">
      <!-- 4 KPI Card Skeletons -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <div v-for="i in 4" :key="i" class="bg-slate-900 border border-slate-800 p-5 rounded-2xl h-32 flex flex-col justify-between">
          <div class="flex justify-between items-center">
            <div class="h-3 w-20 bg-slate-800 rounded"></div>
            <div class="h-8 w-8 bg-slate-800 rounded-xl"></div>
          </div>
          <div class="space-y-2">
            <div class="h-7 w-28 bg-slate-800 rounded"></div>
            <div class="h-3 w-36 bg-slate-800/60 rounded"></div>
          </div>
        </div>
      </div>

      <!-- Charts Skeletons -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
          <div class="h-4 w-44 bg-slate-800 rounded"></div>
          <div class="h-56 w-full bg-slate-800/40 rounded-xl"></div>
        </div>
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
          <div class="h-4 w-32 bg-slate-800 rounded"></div>
          <div class="h-52 w-52 rounded-full border-8 border-slate-800 mx-auto"></div>
        </div>
      </div>

      <!-- Second Chart Skeleton -->
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
        <div class="h-4 w-52 bg-slate-800 rounded"></div>
        <div class="h-56 w-full bg-slate-800/40 rounded-xl"></div>
      </div>
    </div>

    <!-- REAL CONTENT WHEN LOADED -->
    <div v-else class="space-y-6">
      <!-- Maintenance Reminders Urgent Alert Banner -->
      <div
        v-if="urgentReminders.length > 0"
        class="p-4 rounded-2xl border flex flex-col sm:flex-row sm:items-center justify-between gap-3 transition-colors"
        :class="hasOverdueReminders ? 'bg-rose-500/10 border-rose-500/30' : 'bg-amber-500/10 border-amber-500/30'"
      >
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
            :class="hasOverdueReminders ? 'bg-rose-500/20 text-rose-400' : 'bg-amber-500/20 text-amber-400'"
          >
            <AlertTriangle v-if="hasOverdueReminders" class="w-5 h-5" />
            <Clock v-else class="w-5 h-5" />
          </div>
          <div>
            <div class="text-sm font-bold text-white flex items-center gap-2">
              <span>{{ hasOverdueReminders ? 'Entretien(s) en retard' : 'Entretien(s) à prévoir prochainement' }}</span>
              <span
                class="px-2 py-0.5 text-xs rounded-full font-bold"
                :class="hasOverdueReminders ? 'bg-rose-500/20 text-rose-400' : 'bg-amber-500/20 text-amber-400'"
              >
                {{ urgentReminders.length }}
              </span>
            </div>
            <p class="text-xs text-slate-300 mt-0.5">
              {{ urgentRemindersSummary }}
            </p>
          </div>
        </div>
        <router-link
          to="/expenses?tab=REMINDERS"
          class="px-3.5 py-1.5 text-xs font-semibold rounded-xl shrink-0 transition-colors inline-flex items-center gap-1.5 self-start sm:self-auto shadow-sm"
          :class="hasOverdueReminders ? 'bg-rose-600 hover:bg-rose-500 text-white' : 'bg-amber-600 hover:bg-amber-500 text-white'"
        >
          Consulter les rappels
          <ArrowRight class="w-3.5 h-3.5" />
        </router-link>
      </div>

      <!-- Data completeness -->
      <div
        v-if="tco?.completeness"
        class="p-4 rounded-2xl space-y-2 border"
        :class="tco.completeness.is_complete ? 'bg-emerald-500/5 border-emerald-500/20' : 'bg-amber-500/10 border-amber-500/30'"
      >
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div class="flex items-center gap-2 text-sm font-bold" :class="tco.completeness.is_complete ? 'text-emerald-400' : 'text-amber-400'">
            <AlertTriangle v-if="!tco.completeness.is_complete" class="w-4 h-4" />
            TCO consolidé à {{ tco.completeness.score_pct }} %
          </div>
          <div class="w-full sm:w-48 h-2 bg-slate-800 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full"
              :class="tco.completeness.score_pct >= 90 ? 'bg-emerald-500' : tco.completeness.score_pct >= 60 ? 'bg-amber-500' : 'bg-rose-500'"
              :style="{ width: tco.completeness.score_pct + '%' }"
            ></div>
          </div>
        </div>
        <div class="flex flex-wrap gap-2 pt-1">
          <router-link
            v-if="tco.completeness.charges_without_cost > 0"
            to="/expenses"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Compléter les recharges
          </router-link>
          <router-link
            v-if="tco.completeness.unqualified_drives > 0"
            to="/drives"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Qualifier les trajets
          </router-link>
          <router-link
            v-if="tco.completeness.insurance_missing"
            to="/expenses"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Renseigner l'assurance
          </router-link>
          <router-link
            v-if="tco.completeness.acquisition_missing"
            to="/vehicles"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Renseigner l'acquisition
          </router-link>
          <router-link
            v-if="tco.powertrain === 'ICE' && !tco.fuel_fill_ups"
            to="/expenses?tab=CHARGES"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Saisir un plein
          </router-link>
          <router-link
            v-if="tco.powertrain !== 'ICE' && tco.completeness.untracked_distance_km > 0 && !tco.pre_teslamate_cost"
            to="/vehicles"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            Compléter recharges avant TeslaMate
          </router-link>
          <button
            v-if="tco.completeness.odometer_gaps > 0 || tco.completeness.odometer_anomalies > 0"
            @click="toggleDataQuality"
            class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
          >
            {{ showDataQuality ? 'Masquer' : 'Voir' }} les incohérences d'odomètre
          </button>
        </div>
        <div v-if="showDataQuality && dataQuality" class="pt-2 max-h-64 overflow-y-auto space-y-1">
          <div
            v-for="issue in dataQuality.issues"
            :key="issue.type + issue.drive_id"
            class="text-[11px] text-amber-100/90 flex items-center justify-between gap-3 bg-slate-950/40 rounded-lg px-2.5 py-1.5"
          >
            <span>{{ new Date(issue.date).toLocaleString('fr-FR', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) }} — {{ issueLabels[issue.type] || issue.type }}</span>
            <span class="font-mono">{{ issue.km > 0 ? '+' : '' }}{{ issue.km }} km</span>
          </div>
        </div>
      </div>

      <!-- Lease contract follow-up card -->
      <div
        v-if="leaseContract"
        class="bg-gradient-to-br from-slate-900/90 to-indigo-950/20 border border-indigo-500/20 p-4 sm:p-5 rounded-2xl shadow-sm space-y-4"
      >
        <!-- Card Header -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
          <div class="flex items-center gap-2.5 flex-wrap">
            <span class="px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider bg-indigo-500/15 text-indigo-400 border border-indigo-500/30 flex items-center gap-1.5">
              <FileText class="w-3.5 h-3.5" />
              Contrat {{ leaseContract.acquisitionType }}
            </span>
            <span
              class="px-2.5 py-0.5 rounded-full text-xs font-semibold border"
              :class="leaseContract.status.class"
            >
              {{ leaseContract.status.label }}
            </span>
          </div>

          <!-- Monthly rent / downpayment -->
          <div v-if="leaseContract.monthlyRent" class="self-start sm:self-auto text-left sm:text-right">
            <span class="text-base sm:text-lg font-extrabold text-white">
              {{ Number(leaseContract.monthlyRent).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
            </span>
            <span class="text-xs text-slate-400 font-normal"> / mois</span>
            <span v-if="leaseContract.downPayment && leaseContract.downPayment > 0" class="block text-[11px] text-slate-400">
              (Apport : {{ Number(leaseContract.downPayment).toLocaleString('fr-FR', { maximumFractionDigits: 0 }) }} €)
            </span>
          </div>
        </div>

        <!-- Progress Bars Section: 1 col on mobile, 2 cols on desktop if mileage allowance configured -->
        <div
          class="grid gap-4"
          :class="leaseContract.hasMileageAllowance ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1'"
        >
          <!-- Bar 1: Contract Duration Progress -->
          <div class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-semibold text-slate-300 flex items-center gap-1.5">
                <Calendar class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                Durée du contrat
              </span>
              <span class="font-bold text-indigo-300">
                {{ leaseContract.elapsedMonths }} / {{ leaseContract.totalMonths }} mois
                <span class="text-slate-400 font-normal">({{ leaseContract.durationProgressPct }} %)</span>
              </span>
            </div>

            <!-- Progress Bar Track -->
            <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
              <div
                class="h-full bg-gradient-to-r from-indigo-500 to-violet-500 rounded-full transition-all duration-500"
                :style="{ width: `${leaseContract.durationProgressPct}%` }"
              ></div>
            </div>

            <!-- Sub-info: start date, remaining, end date -->
            <div class="flex items-center justify-between text-[11px] text-slate-400 flex-wrap gap-1">
              <span v-if="leaseContract.startDate">
                Début : {{ leaseContract.startDate.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' }) }}
              </span>
              <span class="font-medium text-slate-300">
                {{ leaseContract.isEnded ? 'Contrat terminé' : leaseContract.isNotStarted ? 'Contrat à venir' : `${leaseContract.remainingMonths} mois restants` }}
              </span>
              <span v-if="leaseContract.endDate">
                Fin : {{ leaseContract.endDate.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' }) }}
              </span>
            </div>
          </div>

          <!-- Bar 2: Mileage Allowance Progress (if configured) -->
          <div v-if="leaseContract.hasMileageAllowance" class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-semibold text-slate-300 flex items-center gap-1.5">
                <Gauge class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                Forfait kilométrique
              </span>
              <span class="font-bold text-slate-200">
                {{ Math.round(leaseContract.kmDriven).toLocaleString('fr-FR') }} km
                <span v-if="leaseContract.kmAllowanceTotal" class="text-slate-400 font-normal">
                  / {{ Math.round(leaseContract.kmAllowanceTotal).toLocaleString('fr-FR') }} km
                </span>
                <span v-else class="text-slate-400 font-normal">
                  / {{ Math.round(leaseContract.kmAllowanceToDate).toLocaleString('fr-FR') }} km à date
                </span>
              </span>
            </div>

            <!-- Progress Bar Track -->
            <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="leaseContract.mileageColor"
                :style="{ width: `${Math.min(100, leaseContract.mileageProgressPct)}%` }"
              ></div>
            </div>

            <!-- Sub-info: Pace & Diff -->
            <div class="flex items-center justify-between text-[11px] flex-wrap gap-1">
              <span class="text-slate-400">
                Rythme : <strong>{{ leaseContract.actualPaceKmMonth }} km/mois</strong>
                <template v-if="leaseContract.contractualPaceKmMonth"> (prévu : {{ leaseContract.contractualPaceKmMonth }} km/mois)</template>
              </span>
              <span
                v-if="leaseContract.kmDiff !== undefined"
                :class="leaseContract.kmDiff > 0 ? 'text-rose-400 font-semibold' : 'text-emerald-400 font-semibold'"
              >
                {{ leaseContract.kmDiff > 0 ? `+${leaseContract.kmDiff.toLocaleString('fr-FR')} km de dépassement` : `${Math.abs(leaseContract.kmDiff).toLocaleString('fr-FR')} km d'avance` }}
              </span>
            </div>
          </div>
        </div>

        <!-- Footer: Buyout option, penalties, included services -->
        <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-800/60 text-xs text-slate-400">
          <div class="flex items-center gap-2 flex-wrap">
            <!-- Purchase option (LOA) -->
            <span
              v-if="leaseContract.purchaseOptionPrice"
              class="px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 font-medium border border-slate-700/60 flex items-center gap-1.5"
            >
              <Coins class="w-3.5 h-3.5 text-amber-400" />
              Option d'achat finale : <strong class="text-white">{{ Number(leaseContract.purchaseOptionPrice).toLocaleString('fr-FR', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) }} €</strong>
            </span>

            <!-- Included Services Badges -->
            <span
              v-if="leaseContract.includesMaintenance"
              class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
            >
              <CheckCircle2 class="w-3 h-3" /> Entretien inclus
            </span>
            <span
              v-if="leaseContract.includesInsurance"
              class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
            >
              <CheckCircle2 class="w-3 h-3" /> Assurance incluse
            </span>
            <span
              v-if="leaseContract.includesTires"
              class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
            >
              <CheckCircle2 class="w-3 h-3" /> Pneus inclus
            </span>
          </div>

          <!-- Penalty Warnings if applicable -->
          <div class="flex items-center gap-3">
            <span v-if="leaseContract.excessKmCost > 0" class="text-rose-400 font-semibold">
              Dépassement à date : {{ leaseContract.excessKmCost.toFixed(2) }} €
            </span>
            <span v-if="leaseContract.excessKmProjected > 0" class="text-amber-400 font-semibold">
              Pénalité estimée fin de contrat : {{ leaseContract.excessKmProjected.toFixed(2) }} €
            </span>
          </div>
        </div>
      </div>

      <!-- TCO Metrics Grid -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <!-- Total Cost -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût Total</span>
            <div class="p-2 bg-rose-500/10 text-rose-400 rounded-xl">
              <Coins class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-white">
            {{ (tco?.total_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
          </div>
          <div class="mt-2 space-y-0.5">
            <p class="text-xs text-slate-300">
              Coût complet : {{ (tco?.full_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
              <span v-if="tco?.depreciation_cost" class="text-slate-400 text-[11px]"> (dont décote {{ tco.depreciation_cost.toLocaleString('fr-FR', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) }} €)</span>
            </p>
            <p v-if="tco?.carpool_revenue" class="text-[11px] text-emerald-400">
              Net covoiturage : {{ (tco.full_cost_net || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
            </p>
          </div>
        </div>

        <!-- Cost per km -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût complet au km</span>
            <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
              <TrendingUp class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-emerald-400">
            {{ (tco?.full_cost_per_km || 0).toLocaleString('fr-FR', { minimumFractionDigits: 3, maximumFractionDigits: 3 }) }} €<span class="text-xs font-normal text-slate-400">/km</span>
          </div>
          <div class="mt-2 space-y-0.5">
            <p class="text-xs text-slate-400">
              Usage direct : {{ (tco?.usage_cost_per_km || 0).toFixed(3) }} €/km
            </p>
            <p class="text-[11px] text-slate-500">
              Sur {{ Math.round(tco?.distance_basis_km || 0).toLocaleString('fr-FR') }} km
              <template v-if="tco?.depreciation_cost_per_km"> • décote {{ tco.depreciation_cost_per_km.toFixed(3) }} €/km</template>
            </p>
          </div>
        </div>

        <!-- Energy Cost -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ tco?.powertrain === 'ICE' ? 'Carburant (pleins)' : 'Énergie (Charges)' }}</span>
            <div class="p-2 bg-sky-500/10 text-sky-400 rounded-xl">
              <Zap class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-sky-400">
            {{ (tco?.energy_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
          </div>
          <div class="mt-2 space-y-0.5">
            <p v-if="tco?.powertrain === 'ICE'" class="text-xs text-slate-400">
              {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km • {{ Math.round(tco?.total_liters || 0).toLocaleString('fr-FR') }} L<template v-if="tco?.consumption_l_100km"> • {{ tco.consumption_l_100km.toFixed(2) }} L/100</template><template v-if="tco?.avg_cost_per_liter"> • {{ tco.avg_cost_per_liter.toFixed(3) }} €/L</template>
            </p>
            <p v-else class="text-xs text-slate-400">
              {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km • {{ Math.round(tco?.total_kwh_added || 0).toLocaleString('fr-FR') }} kWh
            </p>
            <p v-if="tco?.completeness?.charges_without_cost" class="text-[11px] text-amber-400">
              {{ tco.completeness.charges_without_cost }} recharge(s) sans coût
            </p>
          </div>
        </div>

        <!-- Tolls & Parkings -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Péages & Parkings</span>
            <div class="p-2 bg-amber-500/10 text-amber-400 rounded-xl">
              <Receipt class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-amber-400">
            {{ (tco?.tolls_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
          </div>
          <div class="mt-2">
            <p class="text-xs text-slate-400">{{ (tco?.tolls_cost_per_km || 0).toFixed(3) }} €/km</p>
          </div>
        </div>
      </div>

      <!-- Current Month Quick Highlight Banner -->
      <div v-if="currentMonthStats" class="bg-slate-900/80 border border-indigo-500/30 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="flex items-center gap-3 min-w-0 flex-1">
          <div class="p-2.5 bg-indigo-500/10 text-indigo-400 rounded-xl shrink-0">
            <Calendar class="w-5 h-5" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs font-semibold text-indigo-400 uppercase tracking-wider">Activité du mois ({{ currentMonthStats.month }})</span>
            </div>
            <div class="text-base sm:text-lg font-bold text-white flex items-center gap-2 sm:gap-3 mt-0.5 flex-wrap">
              <span>{{ Math.round(currentMonthStats.distance_km).toLocaleString('fr-FR') }} km roulés</span>
              <span class="text-slate-500">•</span>
              <span class="text-emerald-400">{{ currentMonthStats.cost_per_km > 0 ? currentMonthStats.cost_per_km.toFixed(3) + ' €/km' : '0.000 €/km' }}</span>
              <span class="text-slate-500">•</span>
              <span class="text-slate-300">{{ currentMonthStats.total.toFixed(2) }} € dépensés</span>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 self-start sm:self-auto">
          <button
            type="button"
            @click="openMonthDetail(currentMonthStats.raw)"
            class="px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-colors"
          >
            <PieChart class="w-3.5 h-3.5" />
            <span>Voir la répartition</span>
          </button>
          <router-link
            to="/drives"
            class="text-xs font-semibold text-slate-400 hover:text-white flex items-center gap-1 px-2 py-1.5"
          >
            Trajets <ArrowRight class="w-3.5 h-3.5" />
          </router-link>
        </div>
      </div>

      <!-- Chart 1 & Donut: Expenses & Breakdown -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Monthly Evolution Bar Chart -->
        <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
            <h3 class="text-sm font-bold text-white flex items-center gap-2">
              <span>Évolution mensuelle des dépenses (€)</span>
            </h3>
            <div class="flex items-center gap-1 bg-slate-800/60 rounded-lg p-0.5 self-start sm:self-auto">
              <button
                v-for="opt in monthlyRangeOptions"
                :key="opt.key"
                type="button"
                @click="monthlyChartRange = opt.key"
                :class="[
                  'px-2.5 py-1 text-[11px] font-semibold rounded-md transition-colors',
                  monthlyChartRange === opt.key ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white',
                ]"
              >
                {{ opt.label }}
              </button>
            </div>
          </div>
          <div class="h-64 sm:h-72">
            <canvas ref="monthlyChartRef"></canvas>
          </div>
        </div>

        <!-- Donut Cost Breakdown -->
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
          <h3 class="text-sm font-bold text-white mb-4">Répartition du coût complet</h3>
          <div class="h-56 sm:h-64">
            <canvas ref="donutChartRef"></canvas>
          </div>
        </div>
      </div>

      <!-- Chart 2: Monthly Distance & Average Cost/Km Dual-Axis Chart -->
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-3">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div>
            <h3 class="text-sm font-bold text-white flex items-center gap-2">
              <Activity class="w-4 h-4 text-indigo-400" />
              <span>Kilométrage Mensuel & Coût de Revient au Km (€/km)</span>
            </h3>
            <p class="text-xs text-slate-400 mt-0.5">Cliquer sur une barre du graphique pour ouvrir le détail chiffré du mois.</p>
          </div>
          <div class="flex flex-wrap items-center gap-2.5 self-start sm:self-auto">
            <button
              v-if="filteredMileageCosts.length"
              type="button"
              @click="openMonthDetail(filteredMileageCosts[filteredMileageCosts.length - 1])"
              class="px-2.5 py-1 bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 rounded-lg text-[11px] font-semibold flex items-center gap-1.5 transition-colors"
            >
              <PieChart class="w-3.5 h-3.5" />
              <span>Détail dernier mois</span>
            </button>
            <div class="flex items-center gap-1 bg-slate-800/60 rounded-lg p-0.5">
              <button
                v-for="opt in monthlyRangeOptions"
                :key="opt.key"
                type="button"
                @click="mileageChartRange = opt.key"
                :class="[
                  'px-2.5 py-1 text-[11px] font-semibold rounded-md transition-colors',
                  mileageChartRange === opt.key ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white',
                ]"
              >
                {{ opt.label }}
              </button>
            </div>
            <div class="flex items-center gap-3 text-xs">
              <span class="flex items-center gap-1.5 text-indigo-300">
                <span class="w-3 h-3 rounded bg-indigo-500/80 inline-block"></span>
                Distance (km)
              </span>
              <span class="flex items-center gap-1.5 text-emerald-400">
                <span class="w-3 h-1 rounded bg-emerald-400 inline-block"></span>
                Coût (€/km)
              </span>
            </div>
          </div>
        </div>

        <div class="h-64 sm:h-72">
          <canvas ref="mileageChartRef"></canvas>
        </div>
      </div>

      <!-- Pro vs Perso Breakdown -->
      <div v-if="tco?.tag_breakdown?.length" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
        <h3 class="text-sm font-bold text-white mb-3">Ventilation par Catégorie de Trajets (Pro / Perso)</h3>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div
            v-for="item in tco.tag_breakdown"
            :key="item.tag"
            class="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex items-center justify-between"
          >
            <div>
              <div class="flex items-center gap-2">
                <Briefcase v-if="item.tag === 'Pro'" class="w-4 h-4 text-blue-400" />
                <User v-else class="w-4 h-4 text-emerald-400" />
                <span class="text-sm font-bold text-white">{{ item.tag }}</span>
              </div>
              <p class="text-xs text-slate-400 mt-1">{{ item.distance_km.toLocaleString('fr-FR') }} km ({{ item.energy_kwh }} kWh)</p>
              <p v-if="item.tolls_amount" class="text-xs text-amber-400">{{ item.tolls_amount.toFixed(2) }} € de péages & parkings</p>
            </div>
            <div class="text-right">
              <span class="text-lg font-extrabold text-white">{{ item.percentage }}%</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Monthly Cost & Donut Detail -->
    <div
      v-if="selectedMonth && selectedMonthBreakdown"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="closeMonthDetail"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-3xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <!-- Header with Month Title, Prev/Next Navigation, and Close Button -->
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between gap-2 shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2 sm:gap-3 min-w-0 pr-2">
            <div class="p-2 bg-indigo-500/10 text-indigo-400 rounded-xl shrink-0">
              <PieChart class="w-5 h-5" />
            </div>
            <div class="min-w-0 truncate">
              <h3 class="text-base sm:text-lg font-bold text-white flex items-center gap-2 truncate">
                <span class="truncate">Détail des coûts — {{ formatMonthName(selectedMonthBreakdown.month) }}</span>
              </h3>
              <p class="text-xs text-slate-400 truncate">
                Ventilation complète des postes de dépenses et coût kilométrique
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1 sm:gap-2 shrink-0">
            <div class="flex items-center bg-slate-800/80 rounded-xl border border-slate-700/60 p-0.5">
              <button
                type="button"
                :disabled="!hasPrevMonth"
                @click="selectPrevMonth"
                class="p-1.5 text-slate-400 hover:text-white disabled:opacity-30 disabled:hover:text-slate-400 rounded-lg hover:bg-slate-700/50 transition-colors"
                title="Mois précédent (←)"
              >
                <ChevronLeft class="w-4 h-4" />
              </button>
              <button
                type="button"
                :disabled="!hasNextMonth"
                @click="selectNextMonth"
                class="p-1.5 text-slate-400 hover:text-white disabled:opacity-30 disabled:hover:text-slate-400 rounded-lg hover:bg-slate-700/50 transition-colors"
                title="Mois suivant (→)"
              >
                <ChevronRight class="w-4 h-4" />
              </button>
            </div>
            <button
              type="button"
              @click="closeMonthDetail"
              class="p-2 text-slate-400 hover:text-white rounded-xl hover:bg-slate-800 transition-colors"
              title="Fermer (Échap)"
            >
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Body -->
        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

        <!-- 4 KPI Summary Cards for the Month -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 sm:gap-3">
          <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
            <span class="text-[11px] font-medium text-slate-400 block">Distance totale</span>
            <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
              {{ Math.round(selectedMonthBreakdown.distanceKm).toLocaleString('fr-FR') }} <span class="text-xs font-normal text-slate-400">km</span>
            </div>
            <span v-if="selectedMonthBreakdown.smoothedKm > 0" class="text-[10px] text-slate-400 block truncate">
              dont {{ Math.round(selectedMonthBreakdown.smoothedKm).toLocaleString('fr-FR') }} km lissés
            </span>
            <span v-else class="text-[10px] text-slate-500 block truncate">100% trajets GPS</span>
          </div>

          <div class="bg-emerald-500/5 border border-emerald-500/30 p-3 rounded-xl">
            <span class="text-[11px] font-medium text-emerald-400 block">Coût kilométrique</span>
            <div class="text-base sm:text-lg font-extrabold text-emerald-400 mt-0.5">
              {{ selectedMonthBreakdown.costPerKm.toFixed(3) }} <span class="text-xs font-normal text-emerald-500/80">€/km</span>
            </div>
            <span class="text-[10px] text-emerald-400/70 block">Coût de revient réel</span>
          </div>

          <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
            <span class="text-[11px] font-medium text-slate-400 block">Coût d'usage calculé</span>
            <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
              {{ selectedMonthBreakdown.economicTotal.toFixed(2) }} <span class="text-xs font-normal text-slate-400">€</span>
            </div>
            <span class="text-[10px] text-slate-400 block">Base coût au km</span>
          </div>

          <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
            <span class="text-[11px] font-medium text-slate-400 block">Total décaissé (cash)</span>
            <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
              {{ selectedMonthBreakdown.cashTotal.toFixed(2) }} <span class="text-xs font-normal text-slate-400">€</span>
            </div>
            <span class="text-[10px] text-slate-400 block">Règlements du mois</span>
          </div>
        </div>

        <!-- Toggle View Mode: Economic Cost vs Cash-Flow -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 bg-slate-800/40 border border-slate-700/50 rounded-xl text-xs">
          <span class="text-slate-300 font-medium flex items-center gap-1.5">
            <Info class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
            <span>Mode de calcul de la répartition :</span>
          </span>
          <div class="flex items-center gap-1 bg-slate-900/80 p-0.5 rounded-lg border border-slate-700/70">
            <button
              type="button"
              @click="monthDetailViewMode = 'economic'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition-colors',
                monthDetailViewMode === 'economic' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-white'
              ]"
            >
              Coût de revient (€/km)
            </button>
            <button
              type="button"
              @click="monthDetailViewMode = 'cash'"
              :class="[
                'px-2.5 py-1 text-[11px] font-semibold rounded-md transition-colors',
                monthDetailViewMode === 'cash' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-white'
              ]"
            >
              Dépenses décaissées (€)
            </button>
          </div>
        </div>

        <!-- Content: Donut Chart (Left) + Itemized Breakdown List (Right) -->
        <div class="grid grid-cols-1 md:grid-cols-5 gap-6 items-start">
          <!-- Left (2 cols): Donut Chart -->
          <div class="md:col-span-2 bg-slate-800/30 border border-slate-800 rounded-xl p-4 flex flex-col items-center justify-center">
            <h4 class="text-xs font-bold text-white mb-2 self-start flex items-center gap-1.5">
              <PieChart class="w-3.5 h-3.5 text-indigo-400" />
              <span>Répartition du mois</span>
            </h4>
            <div class="w-full h-56 sm:h-64 relative">
              <canvas ref="monthDonutRef"></canvas>
            </div>
          </div>

          <!-- Right (3 cols): Itemized Table / List -->
          <div class="md:col-span-3 space-y-2">
            <h4 class="text-xs font-bold text-white mb-2 flex items-center justify-between">
              <span>Détail chiffré par poste de dépense</span>
              <span class="text-[11px] text-slate-400 font-normal">
                {{ monthDetailViewMode === 'economic' ? 'Lissage d\'usage inclus' : 'Montants comptants' }}
              </span>
            </h4>

            <div class="space-y-1.5 max-h-72 sm:max-h-80 overflow-y-auto pr-1">
              <div
                v-for="item in selectedMonthBreakdown.items"
                :key="item.key"
                class="p-2.5 bg-slate-800/50 border border-slate-700/40 rounded-xl flex items-center justify-between gap-3 text-xs hover:border-slate-600 transition-colors"
              >
                <div class="flex items-center gap-2.5 min-w-0">
                  <span
                    class="w-2.5 h-2.5 rounded-full shrink-0"
                    :style="{ backgroundColor: item.color }"
                  ></span>
                  <component :is="item.icon" class="w-4 h-4 shrink-0 text-slate-400" />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="font-semibold text-white truncate">{{ item.label }}</span>
                      <span v-if="item.subLabel" class="text-[10px] px-1.5 py-0.2 bg-slate-700 text-slate-300 rounded font-normal">
                        {{ item.subLabel }}
                      </span>
                    </div>
                    <p v-if="item.note" class="text-[10px] text-slate-400 truncate">
                      {{ item.note }}
                    </p>
                  </div>
                </div>

                <div class="text-right shrink-0">
                  <div class="flex items-baseline justify-end gap-2">
                    <span class="font-bold text-white text-xs sm:text-sm">
                      {{ item.displayAmount.toFixed(2) }} €
                    </span>
                    <span class="text-[10px] text-slate-400 font-medium">
                      ({{ item.sharePct.toFixed(1) }}%)
                    </span>
                  </div>
                  <div class="text-[10px] text-emerald-400 font-medium">
                    {{ item.costPerKm.toFixed(3) }} €/km
                  </div>
                </div>
              </div>
            </div>

            <!-- Total Row -->
            <div class="p-3 bg-indigo-500/10 border border-indigo-500/30 rounded-xl flex items-center justify-between text-xs mt-2">
              <div class="font-bold text-white flex items-center gap-2">
                <span>Total du mois</span>
                <span class="text-[11px] text-slate-400 font-normal">({{ Math.round(selectedMonthBreakdown.distanceKm).toLocaleString('fr-FR') }} km)</span>
              </div>
              <div class="text-right">
                <div class="font-extrabold text-white text-sm sm:text-base">
                  {{ selectedMonthBreakdown.activeTotal.toFixed(2) }} €
                </div>
                <div class="text-[11px] font-bold text-emerald-400">
                  {{ (selectedMonthBreakdown.distanceKm > 0 ? selectedMonthBreakdown.activeTotal / selectedMonthBreakdown.distanceKm : 0).toFixed(3) }} €/km
                </div>
              </div>
            </div>
          </div>
        </div>
        </div>

        <!-- Footer with Quick Links and Close Button -->
        <div class="px-5 py-3.5 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <router-link
              to="/drives"
              class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white rounded-xl text-xs font-semibold border border-slate-700 transition-colors flex items-center gap-1.5"
            >
              <Activity class="w-3.5 h-3.5 text-indigo-400" />
              <span>Trajets du véhicule</span>
            </router-link>
            <router-link
              to="/expenses"
              class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white rounded-xl text-xs font-semibold border border-slate-700 transition-colors flex items-center gap-1.5"
            >
              <Receipt class="w-3.5 h-3.5 text-amber-400" />
              <span>Dépenses & Factures</span>
            </router-link>
          </div>

          <button
            type="button"
            @click="closeMonthDetail"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors self-end sm:self-auto"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
