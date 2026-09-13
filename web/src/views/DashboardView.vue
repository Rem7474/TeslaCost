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
} from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const tco = ref<any | null>(null)
const loading = ref(true)

const monthlyChartRef = ref<HTMLCanvasElement | null>(null)
const mileageChartRef = ref<HTMLCanvasElement | null>(null)
const donutChartRef = ref<HTMLCanvasElement | null>(null)

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

    const energyData = monthlyList.map((m: any) => m.energy)
    const tollsData = monthlyList.map((m: any) => m.tolls)
    const tiresData = monthlyList.map((m: any) => m.tires || 0)
    const maintData = monthlyList.map((m: any) => m.maintenance)

    monthlyChartInstance = new Chart(monthlyChartRef.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          { label: 'Énergie (€)', data: energyData, backgroundColor: '#38bdf8', borderRadius: 4 },
          { label: 'Péages & Parkings (€)', data: tollsData, backgroundColor: '#f59e0b', borderRadius: 4 },
          { label: 'Pneus (€)', data: tiresData, backgroundColor: '#10b981', borderRadius: 4 },
          { label: 'Entretien (€)', data: maintData, backgroundColor: '#ec4899', borderRadius: 4 },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
          tooltip: {
            callbacks: {
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
                if (context.dataset.yAxisID === 'yDistance') {
                  return `Distance : ${Number(context.raw).toLocaleString('fr-FR')} km`
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
        labels: ['Énergie', 'Péages & Parkings', 'Pneus', 'Entretien'],
        datasets: [
          {
            data: [
              tco.value.energy_cost || 0,
              tco.value.tolls_cost || 0,
              tco.value.tires_cost || 0,
              tco.value.maintenance_cost || 0,
            ],
            backgroundColor: ['#38bdf8', '#f59e0b', '#10b981', '#ec4899'],
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
          <p class="text-xs text-slate-400 mt-1">Dépenses globales cumulées</p>
        </div>

        <!-- Cost per km -->
        <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût au km</span>
            <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
              <TrendingUp class="w-5 h-5" />
            </div>
          </div>
          <div class="text-2xl sm:text-3xl font-extrabold text-emerald-400">
            {{ (tco?.total_cost_per_km || 0).toLocaleString('fr-FR', { minimumFractionDigits: 3, maximumFractionDigits: 3 }) }} €<span class="text-xs font-normal text-slate-400">/km</span>
          </div>
          <p class="text-xs text-slate-400 mt-1">Sur {{ Math.round(tco?.total_distance_km || 0).toLocaleString('fr-FR') }} km parcourus</p>
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
          <h3 class="text-sm font-bold text-white mb-4 flex items-center justify-between">
            <span>Évolution mensuelle des dépenses (€)</span>
            <span class="text-xs text-slate-400 font-normal">Historique mensuel</span>
          </h3>
          <div class="h-64 sm:h-72">
            <canvas ref="monthlyChartRef"></canvas>
          </div>
        </div>

        <!-- Donut Cost Breakdown -->
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
          <h3 class="text-sm font-bold text-white mb-4">Répartition des dépenses</h3>
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
