<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Gauge } from 'lucide-vue-next'
import { api } from '@/services/api'
import EnergyBatterySection from './EnergyBatterySection.vue'
import EnergyTemperatureSection from './EnergyTemperatureSection.vue'
import { AXIS_TEXT, GRID_COLOR, fmt, fmtPercent, type ChargeClass, type EnergyStats } from './energyStats'

Chart.register(...registerables)

const props = defineProps<{
  vehicleId: string
  grafanaUrl?: string | null
  // Bumped after a synchronization so the figures follow the imported drives and charges
  syncKey?: number
}>()

const stats = ref<EnergyStats | null>(null)
const failed = ref(false)
const range = ref<12 | 0>(12) // 0 = whole history

const consumptionRef = ref<HTMLCanvasElement | null>(null)
const costRef = ref<HTMLCanvasElement | null>(null)
let consumptionChart: Chart | null = null
let costChart: Chart | null = null

const CLASS_LABELS: Record<ChargeClass['class'], { label: string; hint: string }> = {
  SLOW: { label: 'Prise domestique', hint: 'moins de 3,5 kW en moyenne' },
  AC: { label: 'Wallbox / borne AC', hint: '3,5 à 25 kW en moyenne' },
  DC: { label: 'Recharge rapide DC', hint: 'plus de 25 kW en moyenne' },
  UNKNOWN: { label: 'Durée inconnue', hint: 'saisies manuelles' },
}

const visibleMonths = computed(() => {
  const months = stats.value?.months ?? []
  return range.value === 0 ? months : months.slice(-range.value)
})

const hasData = computed(() => (stats.value?.months.length ?? 0) > 0)

const totalKwh = computed(() => (stats.value?.charge_classes ?? []).reduce((sum, c) => sum + c.kwh_added, 0))

const share = (c: ChargeClass) => (totalKwh.value > 0 ? Math.round((c.kwh_added / totalKwh.value) * 100) : 0)

async function load() {
  failed.value = false
  try {
    stats.value = await api.getEnergyStats(props.vehicleId)
  } catch (err) {
    console.error('Failed to load energy statistics', err)
    stats.value = null
    failed.value = true
  }
}


function baseOptions(unit: string) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: { position: 'top' as const, labels: { color: AXIS_TEXT, font: { size: 11 } } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => (ctx.raw === null || ctx.raw === undefined ? '' : `${ctx.dataset.label} : ${fmt(Number(ctx.raw), 2)} ${unit}`),
        },
      },
    },
    scales: {
      x: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT } },
      y: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT, callback: (v: any) => `${v}` }, title: { display: true, text: unit, color: AXIS_TEXT } },
    },
  }
}

function drawCharts() {
  consumptionChart?.destroy()
  costChart?.destroy()
  consumptionChart = null
  costChart = null
  const months = visibleMonths.value
  if (months.length === 0) return
  const labels = months.map((m) => m.month)

  if (consumptionRef.value) {
    consumptionChart = new Chart(consumptionRef.value, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: 'Consommation',
            data: months.map((m) => m.consumption_kwh_100km ?? null),
            borderColor: '#38bdf8',
            backgroundColor: '#38bdf8',
            tension: 0.25,
            spanGaps: true,
            pointRadius: 3,
          },
        ],
      },
      options: baseOptions('kWh/100 km'),
    })
  }

  if (costRef.value) {
    costChart = new Chart(costRef.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            type: 'bar',
            label: 'Mois',
            data: months.map((m) => m.cost_per_100km ?? null),
            backgroundColor: 'rgba(99, 102, 241, 0.35)',
            borderRadius: 4,
          },
          {
            type: 'line',
            label: 'Moyenne sur 3 mois',
            data: months.map((m) => m.cost_per_100km_trailing ?? null),
            borderColor: '#f59e0b',
            backgroundColor: '#f59e0b',
            tension: 0.25,
            spanGaps: true,
            pointRadius: 3,
          },
        ],
      },
      options: baseOptions('€/100 km'),
    })
  }
}

watch(
  () => [props.vehicleId, props.syncKey],
  () => load(),
  { immediate: true },
)

watch([stats, range], async () => {
  await nextTick()
  drawCharts()
})

onBeforeUnmount(() => {
  consumptionChart?.destroy()
  costChart?.destroy()
})
</script>

<template>
  <section v-if="hasData" class="space-y-4 rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-sm" aria-labelledby="energy-efficiency-title">
    <div class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
      <div>
        <h3 id="energy-efficiency-title" class="flex items-center gap-2 text-sm font-bold text-white">
          <Gauge class="h-4 w-4 text-sky-400" aria-hidden="true" />
          Efficacité énergétique
        </h3>
        <p class="mt-0.5 text-xs text-slate-400">Ce que consomme la voiture et ce que coûte réellement l'énergie.</p>
      </div>
      <div class="flex gap-1 self-start rounded-lg border border-slate-800 bg-slate-950 p-0.5" role="group" aria-label="Période">
        <button
          type="button"
          class="min-h-9 rounded-md px-3 text-[11px] font-semibold transition-colors"
          :class="range === 12 ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white'"
          :aria-pressed="range === 12"
          @click="range = 12"
        >
          12 mois
        </button>
        <button
          type="button"
          class="min-h-9 rounded-md px-3 text-[11px] font-semibold transition-colors"
          :class="range === 0 ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white'"
          :aria-pressed="range === 0"
          @click="range = 0"
        >
          Tout
        </button>
      </div>
    </div>

    <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Consommation réelle</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmt(stats?.summary.consumption_kwh_100km, 1) }} <span class="text-xs font-medium text-slate-400">kWh/100 km</span></dd>
        <p class="mt-0.5 text-[11px] text-slate-400">Énergie consommée en roulant, mesurée par TeslaMate.</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Coût de l'énergie</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmt(stats?.summary.cost_per_100km, 2) }} <span class="text-xs font-medium text-slate-400">€/100 km</span></dd>
        <p class="mt-0.5 text-[11px] text-slate-400">Soit {{ fmt(stats?.summary.price_per_kwh, 3) }} €/kWh en moyenne à la recharge.</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Rendement de charge</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmtPercent(stats?.summary.charge_efficiency) }}</dd>
        <p class="mt-0.5 text-[11px] text-slate-400">Énergie stockée dans la batterie sur énergie tirée du réseau.</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Charge complète</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmt(stats?.summary.cost_per_full_charge, 2) }} <span class="text-xs font-medium text-slate-400">€ (0 → 100 %)</span></dd>
        <p class="mt-0.5 text-[11px] text-slate-400">Extrapolé des recharges dont le coût et le niveau de batterie sont connus.</p>
      </div>
    </dl>

    <p v-if="(stats?.summary.sessions_without_cost ?? 0) > 0" class="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs text-amber-300">
      {{ stats?.summary.sessions_without_cost }} recharge(s) sans coût connu : elles ne sont pas comptées dans le coût aux 100 km.
      <router-link to="/expenses?tab=CHARGES" class="font-semibold underline">Les compléter</router-link>
    </p>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div>
        <h4 class="mb-2 text-xs font-bold text-slate-200">Consommation mensuelle</h4>
        <div class="h-56"><canvas ref="consumptionRef" role="img" aria-label="Consommation mensuelle en kWh aux 100 km"></canvas></div>
      </div>
      <div>
        <h4 class="mb-2 text-xs font-bold text-slate-200">Coût de l'énergie aux 100 km</h4>
        <div class="h-56"><canvas ref="costRef" role="img" aria-label="Coût mensuel de l'énergie aux 100 km, avec moyenne sur 3 mois"></canvas></div>
      </div>
    </div>

    <!-- A table is not clipped by sr-only (its width ignores the 1px), so the class goes on a wrapper -->
    <div class="sr-only">
      <table>
        <caption>Consommation et coût de l'énergie par mois</caption>
        <thead>
          <tr><th>Mois</th><th>kWh/100 km</th><th>€/100 km</th><th>Moyenne 3 mois €/100 km</th></tr>
        </thead>
        <tbody>
          <tr v-for="m in visibleMonths" :key="m.month">
            <td>{{ m.month }}</td>
            <td>{{ fmt(m.consumption_kwh_100km, 1) }}</td>
            <td>{{ fmt(m.cost_per_100km, 2) }}</td>
            <td>{{ fmt(m.cost_per_100km_trailing, 2) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="stats && stats.charge_classes.length > 0">
      <h4 class="mb-2 text-xs font-bold text-slate-200">Comment la voiture est rechargée</h4>
      <ul class="space-y-2">
        <li v-for="c in stats.charge_classes" :key="c.class" class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
          <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
            <span class="text-sm font-semibold text-white">
              {{ CLASS_LABELS[c.class].label }}
              <span class="text-[11px] font-normal text-slate-400">({{ CLASS_LABELS[c.class].hint }})</span>
            </span>
            <span class="text-xs text-slate-300">{{ share(c) }} % de l'énergie · {{ c.sessions }} recharge(s)</span>
          </div>
          <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-800" aria-hidden="true">
            <div class="h-full rounded-full" :class="c.class === 'DC' ? 'bg-amber-400' : c.class === 'AC' ? 'bg-sky-400' : 'bg-emerald-400'" :style="{ width: `${share(c)}%` }"></div>
          </div>
          <p class="mt-2 text-xs text-slate-400">
            {{ fmt(c.kwh_added, 0) }} kWh · {{ fmt(c.price_per_kwh, 3) }} €/kWh · rendement {{ fmtPercent(c.charge_efficiency) }}<template v-if="c.cost_per_full_charge !== undefined"> · 0 → 100 % : {{ fmt(c.cost_per_full_charge, 2) }} €</template>
          </p>
        </li>
      </ul>
      <p class="mt-2 text-[11px] text-slate-500">Classement d'après la puissance moyenne de chaque recharge (énergie ajoutée sur sa durée).</p>
    </div>

    <template v-if="stats">
      <EnergyTemperatureSection :stats="stats" />
      <EnergyBatterySection :stats="stats" :grafana-url="grafanaUrl" />
    </template>
  </section>
  <p v-else-if="failed" role="alert" class="rounded-xl border border-slate-800 bg-slate-900 p-3 text-xs text-slate-400">
    Les statistiques d'efficacité énergétique n'ont pas pu être chargées.
  </p>
</template>
