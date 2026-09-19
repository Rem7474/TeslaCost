<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { BatteryMedium } from 'lucide-vue-next'
import { AXIS_TEXT, GRID_COLOR, fmt, type EnergyStats } from './energyStats'

Chart.register(...registerables)

const props = defineProps<{
  stats: EnergyStats
  grafanaUrl?: string | null
}>()

const canvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const latest = computed(() => {
  const snaps = props.stats.battery_health
  return snaps.length ? snaps[snaps.length - 1] : null
})

// One point per month: TeslaMate's reading (last of the month) beside the capacity estimated from the sessions.
const series = computed(() => {
  const estimated = new Map<string, number>()
  for (const m of props.stats.months) {
    if (m.estimated_capacity_kwh !== undefined) estimated.set(m.month, m.estimated_capacity_kwh)
  }
  const teslamate = new Map<string, number>()
  for (const s of props.stats.battery_health) {
    if (s.current_capacity_kwh !== undefined) teslamate.set(s.date.slice(0, 7), s.current_capacity_kwh)
  }
  const months = [...new Set([...estimated.keys(), ...teslamate.keys()])].sort()
  return {
    months,
    estimated: months.map((m) => estimated.get(m) ?? null),
    teslamate: months.map((m) => teslamate.get(m) ?? null),
  }
})

const hasContent = computed(() => latest.value !== null || props.stats.summary.estimated_capacity_kwh !== undefined)
const showChart = computed(() => series.value.months.length >= 2)

function draw() {
  chart?.destroy()
  chart = null
  if (!canvas.value || !showChart.value) return
  const { months, estimated, teslamate } = series.value
  chart = new Chart(canvas.value, {
    type: 'line',
    data: {
      labels: months,
      datasets: [
        { label: 'Capacité TeslaMate', data: teslamate, borderColor: '#34d399', backgroundColor: '#34d399', spanGaps: true, tension: 0.25, pointRadius: 4 },
        { label: 'Estimée d’après les recharges', data: estimated, borderColor: '#a78bfa', backgroundColor: '#a78bfa', borderDash: [5, 4], spanGaps: true, tension: 0.25, pointRadius: 3 },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { position: 'top', labels: { color: AXIS_TEXT, font: { size: 11 } } },
        tooltip: { callbacks: { label: (ctx) => (ctx.raw == null ? '' : `${ctx.dataset.label} : ${fmt(Number(ctx.raw), 1)} kWh`) } },
      },
      scales: {
        x: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT } },
        y: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT }, title: { display: true, text: 'kWh', color: AXIS_TEXT } },
      },
    },
  })
}

watch(() => props.stats, async () => {
  await nextTick()
  draw()
}, { immediate: true, flush: 'post' })

onBeforeUnmount(() => chart?.destroy())
</script>

<template>
  <div v-if="hasContent" class="space-y-3" aria-labelledby="energy-battery-title" role="group">
    <h4 id="energy-battery-title" class="flex items-center gap-2 text-xs font-bold text-slate-200">
      <BatteryMedium class="h-4 w-4 text-emerald-400" aria-hidden="true" />
      Batterie
    </h4>

    <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Santé selon TeslaMate</dt>
        <dd v-if="latest" class="mt-1 text-xl font-bold text-white">
          {{ fmt(latest.health_percent, 1) }} <span class="text-xs font-medium text-slate-400">%</span>
        </dd>
        <dd v-else class="mt-1 text-sm text-slate-400">Pas encore de mesure</dd>
        <p class="mt-0.5 text-[11px] text-slate-400">
          <template v-if="latest">{{ fmt(latest.current_capacity_kwh, 1) }} kWh sur {{ fmt(latest.max_capacity_kwh, 1) }} kWh, la meilleure capacité observée (et non la capacité neuve).</template>
          <template v-else>Relevée à chaque synchronisation si TeslaMateApi expose <code>battery-health</code>.</template>
        </p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Capacité estimée</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmt(stats.summary.estimated_capacity_kwh, 1) }} <span class="text-xs font-medium text-slate-400">kWh</span></dd>
        <p class="mt-0.5 text-[11px] text-slate-400">
          Médiane des {{ stats.summary.capacity_samples ?? 0 }} dernières recharges d'au moins 30 points de batterie : énergie ajoutée sur pourcentage gagné. Simple ordre de grandeur.
        </p>
      </div>
    </dl>

    <div v-if="showChart" class="h-52">
      <canvas ref="canvas" role="img" aria-label="Capacité de la batterie par mois : mesure TeslaMate et estimation d'après les recharges"></canvas>
    </div>
    <table v-if="showChart" class="sr-only">
      <caption>Capacité de la batterie par mois</caption>
      <thead><tr><th>Mois</th><th>TeslaMate (kWh)</th><th>Estimée (kWh)</th></tr></thead>
      <tbody>
        <tr v-for="(m, i) in series.months" :key="m">
          <td>{{ m }}</td><td>{{ fmt(series.teslamate[i] ?? undefined, 1) }}</td><td>{{ fmt(series.estimated[i] ?? undefined, 1) }}</td>
        </tr>
      </tbody>
    </table>

    <p v-if="grafanaUrl" class="text-[11px] text-slate-400">
      Courbes détaillées dans
      <a :href="grafanaUrl" target="_blank" rel="noopener noreferrer" class="font-semibold text-indigo-300 underline">Grafana TeslaMate</a>.
    </p>
  </div>
</template>
