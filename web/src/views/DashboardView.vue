<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
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
} from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const tco = ref<any | null>(null)
const loading = ref(false)

const monthlyChartRef = ref<HTMLCanvasElement | null>(null)
const donutChartRef = ref<HTMLCanvasElement | null>(null)
let monthlyChartInstance: Chart | null = null
let donutChartInstance: Chart | null = null

async function loadTCO() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    tco.value = await api.getTCO(vehicleStore.activeVehicle.id)
    await nextTick()
    renderCharts()
  } catch (err) {
    console.error('Failed to load TCO', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicleId, vehicleStore.lastSyncTimestamp],
  () => {
    loadTCO()
  }
)

onMounted(() => {
  loadTCO()
})

function renderCharts() {
  if (!tco.value) return

  // 1. Monthly Bar Chart
  if (monthlyChartRef.value) {
    if (monthlyChartInstance) monthlyChartInstance.destroy()

    const labels = tco.value.monthly_costs?.map((m: any) => m.month) || []
    const energyData = tco.value.monthly_costs?.map((m: any) => m.energy) || []
    const tollsData = tco.value.monthly_costs?.map((m: any) => m.tolls) || []
    const maintData = tco.value.monthly_costs?.map((m: any) => m.maintenance) || []

    monthlyChartInstance = new Chart(monthlyChartRef.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          { label: 'Énergie (€)', data: energyData, backgroundColor: '#38bdf8' },
          { label: 'Péages & Parkings (€)', data: tollsData, backgroundColor: '#f59e0b' },
          { label: 'Entretien (€)', data: maintData, backgroundColor: '#ec4899' },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { color: '#94a3b8' } },
        },
        scales: {
          x: { stacked: true, grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
          y: { stacked: true, grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
        },
      },
    })
  }

  // 2. Donut Breakdown Chart
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
          legend: { position: 'bottom', labels: { color: '#94a3b8' } },
        },
        cutout: '70%',
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
        <h2 class="text-2xl font-bold tracking-tight text-white">Tableau de bord TCO</h2>
        <p class="text-sm text-slate-400">Coût réel de possession pour {{ vehicleStore.activeVehicle?.name || 'votre véhicule' }}</p>
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

    <!-- TCO Metrics Grid -->
    <div v-else class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
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
          {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km ({{ Math.round(tco?.total_kwh_added || 0) }} kWh)
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

    <!-- Charts Section -->
    <div v-if="vehicleStore.activeVehicle" class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Monthly Evolution Bar Chart -->
      <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
        <h3 class="text-sm font-bold text-white mb-4 flex items-center justify-between">
          <span>Évolution mensuelle des coûts</span>
          <span class="text-xs text-slate-400 font-normal">Derniers 12 mois</span>
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
</template>
