<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Activity, PieChart } from 'lucide-vue-next'
import { filterMonthsByRange, monthlyRangeOptions, type MonthlyRangeKey } from '@/utils/dashboard'

Chart.register(...registerables)

// Monthly distance and average cost per km on two axes; a click on a month opens its detail
const props = defineProps<{ monthlyCosts: any[] | null }>()
const emit = defineEmits<{ 'open-month': [month: any] }>()

const mileageChartRef = ref<HTMLCanvasElement | null>(null)
const mileageChartRange = ref<MonthlyRangeKey>('ALL')
const filteredMileageCosts = computed(() => filterMonthsByRange(props.monthlyCosts || [], mileageChartRange.value))
let mileageChartInstance: Chart | null = null

function renderChart() {
  if (!mileageChartRef.value || !props.monthlyCosts) return
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
            emit('open-month', monthItem)
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

onMounted(renderChart)
watch([() => props.monthlyCosts, mileageChartRange], renderChart, { flush: 'post' })
onUnmounted(() => {
  if (mileageChartInstance) mileageChartInstance.destroy()
})
</script>

<template>
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
          @click="emit('open-month', filteredMileageCosts[filteredMileageCosts.length - 1])"
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
      <canvas ref="mileageChartRef" role="img" aria-label="Kilométrage mensuel et coût au kilomètre"></canvas>
    </div>
  </div>
</template>
