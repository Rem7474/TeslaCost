<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
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
} from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const tco = ref<any | null>(null)
const loading = ref(true)

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

let monthlyChartInstance: Chart | null = null
let mileageChartInstance: Chart | null = null
let donutChartInstance: Chart | null = null

// Current month stats computed
const currentMonthStats = computed(() => {
  if (!tco.value?.monthly_costs?.length) return null
  const now = new Date().toISOString().substring(0, 7) // YYYY-MM
  const list = tco.value.monthly_costs
  const current = list.find((m: any) => m.month === now) || list[list.length - 1]
  const prevIdx = list.indexOf(current) - 1
  const prev = prevIdx >= 0 ? list[prevIdx] : null

  return {
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
    tco.value = await api.getTCO(vehicleStore.activeVehicle.id)
  } catch (err) {
    console.error('Failed to load TCO', err)
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

watch(monthlyChartRange, () => {
  renderCharts()
})

onMounted(() => {
  loadTCO()
})

onUnmounted(() => {
  if (monthlyChartInstance) monthlyChartInstance.destroy()
  if (mileageChartInstance) mileageChartInstance.destroy()
  if (donutChartInstance) donutChartInstance.destroy()
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
        plugins: {
          legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
          tooltip: {
            callbacks: {
              label: (context) => {
                const monthItem = monthlyList[context.dataIndex]
                const val = Number(context.raw || 0).toFixed(2)
                if (context.dataset.label?.startsWith('Énergie') && monthItem && monthItem.smoothed_energy > 0) {
                  return `${context.dataset.label} : ${val} € (dont ${monthItem.smoothed_energy.toFixed(2)} € estimés avant TeslaMate)`
                }
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

    const distanceData = monthlyList.map((m: any) => m.distance_km || 0)
    const costPerKmData = monthlyList.map((m: any) => m.cost_per_km || 0)

    mileageChartInstance = new Chart(mileageChartRef.value, {
      type: 'bar',
      data: {
        labels,
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
        plugins: {
          legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
          tooltip: {
            callbacks: {
              label: (context) => {
                const monthItem = monthlyList[context.dataIndex]
                if (context.dataset.yAxisID === 'yDistance') {
                  const dist = Number(context.raw).toLocaleString('fr-FR')
                  if (monthItem && monthItem.smoothed_km > 0) {
                    return `Distance : ${dist} km (dont ${Math.round(monthItem.smoothed_km).toLocaleString('fr-FR')} km lissés)`
                  }
                  return `Distance : ${dist} km`
                }
                return `Coût de revient : ${Number(context.raw).toFixed(3)} €/km`
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
            <span v-if="!tco.completeness.is_complete" class="font-normal text-amber-300/80">— montants partiels, probablement sous-estimés</span>
          </div>
          <div class="w-full sm:w-48 h-2 bg-slate-800 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full"
              :class="tco.completeness.score_pct >= 90 ? 'bg-emerald-500' : tco.completeness.score_pct >= 60 ? 'bg-amber-500' : 'bg-rose-500'"
              :style="{ width: tco.completeness.score_pct + '%' }"
            ></div>
          </div>
        </div>
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-x-4 gap-y-1">
          <div v-for="d in tco.completeness.dimensions" :key="d.key" class="flex items-center justify-between text-[11px] text-slate-400">
            <span class="truncate">{{ d.label }}</span>
            <span class="font-mono" :class="d.score_pct === 100 ? 'text-emerald-400' : 'text-amber-300'">{{ d.score_pct }} %</span>
          </div>
        </div>
        <ul class="text-xs text-amber-200/90 space-y-1 list-disc pl-6">
          <li v-for="w in tco.completeness.warnings" :key="w">{{ w }}</li>
        </ul>
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
            v-if="tco.completeness.untracked_distance_km > 0 && !tco.pre_teslamate_cost"
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

      <!-- Lease contract follow-up -->
      <div
        v-if="tco && ['LOA', 'LLD'].includes(tco.acquisition_type) && (tco.lease_km_allowance_to_date > 0 || tco.contract_end_date)"
        class="bg-slate-900 border border-indigo-500/30 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
      >
        <div class="space-y-1">
          <span class="text-xs font-semibold text-indigo-400 uppercase tracking-wider">Contrat {{ tco.acquisition_type }}</span>
          <p v-if="tco.lease_km_allowance_to_date > 0" class="text-sm text-slate-200">
            {{ Math.round(tco.lease_km_driven).toLocaleString('fr-FR') }} km parcourus pour
            {{ Math.round(tco.lease_km_allowance_to_date).toLocaleString('fr-FR') }} km autorisés à date
            <span :class="tco.lease_km_driven > tco.lease_km_allowance_to_date ? 'text-rose-400' : 'text-emerald-400'">
              ({{ Math.round((tco.lease_km_driven / tco.lease_km_allowance_to_date) * 100) }} %)
            </span>
          </p>
          <p v-if="tco.contract_end_date" class="text-xs text-slate-400">
            Fin de contrat le {{ new Date(tco.contract_end_date).toLocaleDateString('fr-FR') }}
          </p>
        </div>
        <div class="text-right text-xs">
          <p v-if="tco.lease_excess_km_cost > 0" class="text-rose-300">Dépassement à date : {{ tco.lease_excess_km_cost.toFixed(2) }} €</p>
          <p v-if="tco.lease_excess_km_projected > 0" class="text-amber-300">Pénalité projetée en fin de contrat : {{ tco.lease_excess_km_projected.toFixed(2) }} €</p>
          <p v-if="!tco.lease_excess_km_projected && tco.lease_km_allowance_to_date > 0" class="text-emerald-400">Forfait kilométrique respecté au rythme actuel</p>
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
          <p class="text-xs text-slate-400 mt-1">
            Dépenses courantes décaissées (hors achat du véhicule)
          </p>
          <p class="text-xs text-slate-300 mt-1">
            Coût complet : {{ (tco?.full_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
            <span v-if="tco?.depreciation_cost" class="text-slate-400">dont décote {{ tco.depreciation_cost.toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €</span>
          </p>
          <p v-if="tco?.carpool_revenue" class="text-[11px] text-emerald-400 mt-0.5">
            Net des covoiturages : {{ (tco.full_cost_net || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} € ({{ (tco.full_cost_net_per_km || 0).toFixed(3) }} €/km)
          </p>
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
          <p class="text-xs text-slate-400 mt-1">
            Usage direct (énergie + péages) : {{ (tco?.usage_cost_per_km || 0).toFixed(3) }} €/km
            • sur {{ Math.round(tco?.distance_basis_km || 0).toLocaleString('fr-FR') }} km
            <template v-if="tco?.smoothed_distance_km"> (dont {{ Math.round(tco.smoothed_distance_km).toLocaleString('fr-FR') }} km lissés)</template>
          </p>
          <p class="text-[11px] text-slate-500 mt-0.5">
            Assurance : {{ insuranceSourceLabel }}
            <template v-if="tco?.depreciation_cost_per_km"> • décote {{ tco.depreciation_cost_per_km.toFixed(3) }} €/km</template>
          </p>
        </div>

        <!-- Energy Cost -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Énergie (Charges)</span>
            <div class="p-2 bg-sky-500/10 text-sky-400 rounded-xl">
              <Zap class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-sky-400">
            {{ (tco?.energy_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
          </div>
          <p class="text-xs text-slate-400 mt-1">
            {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km ({{ Math.round(tco?.total_kwh_added || 0).toLocaleString('fr-FR') }} kWh)
          </p>
          <p v-if="tco?.pre_teslamate_cost" class="text-[11px] text-sky-300/90 mt-0.5">
            dont {{ tco.pre_teslamate_cost.toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} € estimés avant TeslaMate ({{ Math.round(tco.pre_teslamate_kwh || 0).toLocaleString('fr-FR') }} kWh)
          </p>
          <p v-if="tco?.completeness?.charges_without_cost" class="text-[11px] text-amber-400 mt-0.5">
            {{ tco.completeness.charges_without_cost }} recharge(s) sans coût
          </p>
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
          <p class="text-xs text-slate-400 mt-1">{{ (tco?.tolls_cost_per_km || 0).toFixed(3) }} €/km</p>
        </div>
      </div>

      <!-- Current Month Quick Highlight Banner -->
      <div v-if="currentMonthStats" class="bg-slate-900/80 border border-indigo-500/30 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <div class="p-2.5 bg-indigo-500/10 text-indigo-400 rounded-xl">
            <Calendar class="w-5 h-5" />
          </div>
          <div>
            <span class="text-xs font-semibold text-indigo-400 uppercase tracking-wider">Activité du mois ({{ currentMonthStats.month }})</span>
            <div class="text-lg font-bold text-white flex items-center gap-3">
              <span>{{ Math.round(currentMonthStats.distance_km).toLocaleString('fr-FR') }} km roulés</span>
              <span class="text-slate-500">•</span>
              <span class="text-emerald-400">{{ currentMonthStats.cost_per_km > 0 ? currentMonthStats.cost_per_km.toFixed(3) + ' €/km' : '0.000 €/km' }}</span>
              <span class="text-slate-500">•</span>
              <span class="text-slate-300">{{ currentMonthStats.total.toFixed(2) }} € dépensés</span>
            </div>
          </div>
        </div>
        <router-link
          to="/drives"
          class="text-xs font-semibold text-indigo-400 hover:text-indigo-300 flex items-center gap-1 self-start sm:self-auto"
        >
          Analyser les trajets du mois <ArrowRight class="w-3.5 h-3.5" />
        </router-link>
      </div>

      <!-- Chart 1 & Donut: Expenses & Breakdown -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Monthly Evolution Bar Chart -->
        <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
            <h3 class="text-sm font-bold text-white flex items-center gap-2">
              <span>Évolution mensuelle des dépenses (€)</span>
              <span class="text-xs text-slate-400 font-normal">Montants décaissés par mois</span>
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
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
          <div>
            <h3 class="text-sm font-bold text-white flex items-center gap-2">
              <Activity class="w-4 h-4 text-indigo-400" />
              Kilométrage Mensuel & Coût de Revient au Km (€/km)
            </h3>
            <p class="text-xs text-slate-400">
              Histogramme des kilomètres parcourus chaque mois et courbe du coût de revient au km
            </p>
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
  </div>
</template>
