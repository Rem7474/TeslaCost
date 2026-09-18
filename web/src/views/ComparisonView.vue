<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Scale, Plus, Trash2, Pencil, ArrowLeft, ArrowRight, Info, TrendingDown, TrendingUp } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()

type Mode = 'RETROSPECTIVE' | 'PROJECTION'

const scenarios = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const defaults = ref<any | null>(null)

// 'list' | 'edit' (steps 1-2) | 'result'
const view = ref<'list' | 'edit' | 'result'>('list')
const step = ref(1)
const editingId = ref<string | null>(null)
const currentScenario = ref<any | null>(null)
const result = ref<any | null>(null)
const formError = ref('')

const fuelTypes = computed<any[]>(() => defaults.value?.ice || [])

function emptyForm() {
  return {
    mode: (vehicleStore.activeVehicle ? 'RETROSPECTIVE' : 'PROJECTION') as Mode,
    name: '',
    annual_km: 12000,
    years: 5,
    ice: {
      fuel_type: 'SP95_E10',
      l_per_100km: 6.5,
      fuel_price: 1.75,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 700,
      insurance_yearly: 650,
      tax_yearly: 0,
    },
    ev: {
      kwh_per_100km: 16,
      eur_per_kwh: 0.2,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 300,
      insurance_yearly: 800,
      tax_yearly: 0,
    },
  }
}

const form = reactive(emptyForm())

const isRetro = computed(() => form.mode === 'RETROSPECTIVE')

const iceFields = [
  { key: 'purchase_price', label: "Prix d'achat (€)", step: 100 },
  { key: 'resale_value', label: 'Valeur de revente en fin de période (€)', step: 100 },
  { key: 'maintenance_yearly', label: 'Entretien par an (€)', step: 10 },
  { key: 'insurance_yearly', label: 'Assurance par an (€)', step: 10 },
  { key: 'tax_yearly', label: 'Taxes par an (€)', step: 10 },
] as const

const evFields = [
  { key: 'purchase_price', label: "Prix d'achat net des aides (€)", step: 100 },
  { key: 'resale_value', label: 'Valeur de revente en fin de période (€)', step: 100 },
  { key: 'maintenance_yearly', label: 'Entretien par an (€)', step: 10 },
  { key: 'insurance_yearly', label: 'Assurance par an (€)', step: 10 },
] as const

function fmtEur(v: number | null | undefined, digits = 0): string {
  return Number(v || 0).toLocaleString('fr-FR', {
    style: 'currency',
    currency: 'EUR',
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

function fmtKm(v: number): string {
  return Math.round(Number(v || 0)).toLocaleString('fr-FR')
}

async function loadScenarios() {
  loading.value = true
  try {
    scenarios.value = await api.getComparisonScenarios()
  } catch (err) {
    console.error('Failed to load comparisons', err)
  } finally {
    loading.value = false
  }
}

async function loadDefaults() {
  const vehicleId = isRetro.value ? vehicleStore.activeVehicle?.id : undefined
  try {
    defaults.value = await api.getComparisonDefaults(vehicleId)
  } catch (err) {
    console.error('Failed to load comparison defaults', err)
  }
}

function applyFuelDefaults() {
  const d = fuelTypes.value.find((f) => f.fuel_type === form.ice.fuel_type)
  if (!d) return
  form.ice.l_per_100km = d.l_per_100km
  form.ice.fuel_price = d.fuel_price
}

async function startNew() {
  Object.assign(form, emptyForm())
  editingId.value = null
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  await loadDefaults()
  applyDefaultsToForm()
}

function applyDefaultsToForm() {
  const d = defaults.value
  if (!d) return
  form.annual_km = d.annual_km
  form.ice.maintenance_yearly = d.maintenance_yearly
  form.ice.insurance_yearly = d.insurance_yearly
  if (d.ev_kwh_per_100km) form.ev.kwh_per_100km = d.ev_kwh_per_100km
  if (d.ev_eur_per_kwh) form.ev.eur_per_kwh = d.ev_eur_per_kwh
  applyFuelDefaults()
}

async function onModeChange() {
  await loadDefaults()
  applyDefaultsToForm()
}

function editScenario(sc: any) {
  Object.assign(form, emptyForm(), {
    mode: sc.mode,
    name: sc.name,
    annual_km: sc.annual_km,
    years: sc.years,
    ice: { ...sc.ice },
    ev: sc.ev ? { ...sc.ev } : emptyForm().ev,
  })
  editingId.value = sc.id
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  loadDefaults()
}

function buildPayload() {
  const payload: any = {
    name: form.name,
    mode: form.mode,
    annual_km: Number(form.annual_km),
    years: Number(form.years),
    ice: {
      ...form.ice,
      l_per_100km: Number(form.ice.l_per_100km),
      fuel_price: Number(form.ice.fuel_price),
    },
    options: {},
  }
  if (isRetro.value) {
    payload.vehicle_id = vehicleStore.activeVehicle?.id
  } else {
    payload.ev = {
      ...form.ev,
      kwh_per_100km: Number(form.ev.kwh_per_100km),
      eur_per_kwh: Number(form.ev.eur_per_kwh),
    }
  }
  for (const side of [payload.ice, payload.ev]) {
    if (!side) continue
    for (const k of ['purchase_price', 'resale_value', 'maintenance_yearly', 'insurance_yearly', 'tax_yearly']) {
      side[k] = Number(side[k] || 0)
    }
  }
  return payload
}

function stepError(): string {
  if (step.value === 1) {
    if (!form.name.trim()) return 'Donnez un nom au comparatif.'
    if (!(Number(form.annual_km) > 0)) return 'Indiquez un kilométrage annuel.'
    if (!(Number(form.years) >= 1 && Number(form.years) <= 15)) return 'La durée doit être comprise entre 1 et 15 ans.'
    if (isRetro.value && !vehicleStore.activeVehicle) return 'Aucun véhicule actif : choisissez le mode projection.'
  }
  if (step.value === 2) {
    if (!(Number(form.ice.l_per_100km) > 0)) return 'Indiquez la consommation du thermique.'
    if (!(Number(form.ice.purchase_price) > 0)) return "Indiquez le prix d'achat du thermique."
    if (!isRetro.value && !(Number(form.ev.purchase_price) > 0)) return "Indiquez le prix d'achat de l'électrique."
  }
  return ''
}

async function nextStep() {
  formError.value = stepError()
  if (formError.value) return
  if (step.value < 2) {
    step.value += 1
    return
  }
  await saveAndCompute()
}

function prevStep() {
  formError.value = ''
  if (step.value > 1) step.value -= 1
  else view.value = 'list'
}

async function saveAndCompute() {
  saving.value = true
  try {
    const payload = buildPayload()
    const saved = editingId.value
      ? await api.updateComparisonScenario(editingId.value, payload)
      : await api.createComparisonScenario(payload)
    await loadScenarios()
    await openResult(saved)
  } catch (err: any) {
    formError.value = err?.message || "Impossible d'enregistrer le comparatif."
  } finally {
    saving.value = false
  }
}

async function openResult(sc: any) {
  currentScenario.value = sc
  result.value = null
  view.value = 'result'
  try {
    result.value = await api.getComparisonResult(sc.id)
  } catch (err: any) {
    await showAlert(err?.message || 'Calcul impossible.', 'Comparatif', 'danger')
    view.value = 'list'
    return
  }
  await nextTick()
  renderChart()
}

async function removeScenario(sc: any) {
  const ok = await showConfirm({
    title: 'Supprimer le comparatif',
    message: `Supprimer « ${sc.name} » ? Vos données réelles ne sont pas affectées.`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteComparisonScenario(sc.id)
    await loadScenarios()
  } catch (err: any) {
    await showAlert(err?.message || 'Suppression impossible.', 'Suppression', 'danger')
  }
}

// --- Result presentation ---

const savings = computed<number>(() => Number(result.value?.ev_savings || 0))

const verdict = computed(() => {
  if (!result.value) return ''
  const n = result.value.years_count
  const abs = fmtEur(Math.abs(savings.value))
  if (Math.abs(savings.value) < 1) return `Sur ${n} an${n > 1 ? 's' : ''}, les deux véhicules coûtent autant.`
  return savings.value > 0
    ? `Sur ${n} an${n > 1 ? 's' : ''}, l'électrique vous coûte ${abs} de moins.`
    : `Sur ${n} an${n > 1 ? 's' : ''}, l'électrique vous coûte ${abs} de plus.`
})

const breakEvenText = computed(() => {
  if (!result.value) return ''
  const be = result.value.break_even_year
  if (be === undefined || be === null) return "Point d'équilibre non atteint sur la période."
  if (be === 0) return "L'électrique est moins chère dès l'achat et le reste."
  return `Point d'équilibre après ${String(be).replace('.', ',')} an(s) : les économies d'usage compensent l'écart d'achat.`
})

const costRows = computed(() => {
  const r = result.value
  if (!r) return []
  return [
    { label: 'Énergie', ev: r.ev.energy, ice: r.ice.energy },
    { label: 'Entretien et pneus', ev: r.ev.maintenance, ice: r.ice.maintenance },
    { label: 'Assurance', ev: r.ev.insurance, ice: r.ice.insurance },
    { label: 'Taxes', ev: r.ev.tax, ice: r.ice.tax },
    { label: 'Dépréciation', ev: r.ev.depreciation, ice: r.ice.depreciation },
  ]
})

const chartRef = ref<HTMLCanvasElement | null>(null)
let chartInstance: Chart | null = null

function destroyChart() {
  if (chartInstance) {
    chartInstance.destroy()
    chartInstance = null
  }
}

function renderChart() {
  destroyChart()
  if (!chartRef.value || !result.value) return
  const points = result.value.cumulative as { year: number; ev: number; ice: number }[]
  chartInstance = new Chart(chartRef.value, {
    type: 'line',
    data: {
      labels: points.map((p) => (p.year === 0 ? 'Achat' : `An ${p.year}`)),
      datasets: [
        { label: 'Électrique', data: points.map((p) => p.ev), borderColor: '#38bdf8', backgroundColor: '#38bdf8', tension: 0.15 },
        { label: 'Thermique', data: points.map((p) => p.ice), borderColor: '#f59e0b', backgroundColor: '#f59e0b', tension: 0.15 },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { labels: { color: '#94a3b8', boxWidth: 12 } },
        tooltip: { callbacks: { label: (ctx) => ` ${ctx.dataset.label} : ${fmtEur(Number(ctx.raw))}` } },
      },
      scales: {
        x: { ticks: { color: '#94a3b8' }, grid: { color: '#1e293b' } },
        y: { ticks: { color: '#94a3b8', callback: (v) => fmtEur(Number(v)) }, grid: { color: '#1e293b' } },
      },
    },
  })
}

watch(view, (v) => {
  if (v !== 'result') destroyChart()
})

onMounted(async () => {
  await loadScenarios()
})

onBeforeUnmount(destroyChart)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center">
          <Scale class="w-5 h-5 text-sky-400" />
        </div>
        <div>
          <h1 class="text-xl font-bold text-white">Comparatif électrique / thermique</h1>
          <p class="text-xs text-slate-400">Estimation informative : aucune de vos données réelles n'est modifiée.</p>
        </div>
      </div>
      <button
        v-if="view === 'list'"
        class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2 transition-colors"
        @click="startNew"
      >
        <Plus class="w-4 h-4" /> Nouveau comparatif
      </button>
    </div>

    <!-- Scenario list -->
    <div v-if="view === 'list'">
      <div v-if="loading" class="text-sm text-slate-400">Chargement…</div>
      <div v-else-if="scenarios.length === 0" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 text-sm text-slate-400">
        Aucun comparatif pour l'instant. Créez-en un pour estimer ce qu'un thermique équivalent vous coûterait
        (ou ce qu'une électrique vous coûterait) sur plusieurs années.
      </div>
      <ul v-else class="grid gap-3 md:grid-cols-2">
        <li v-for="sc in scenarios" :key="sc.id" class="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center justify-between gap-3">
          <button class="text-left min-w-0 flex-1" @click="openResult(sc)">
            <div class="text-sm font-semibold text-white truncate">{{ sc.name }}</div>
            <div class="text-xs text-slate-400">
              {{ sc.mode === 'RETROSPECTIVE' ? 'Véhicule suivi' : 'Projection' }} · {{ fmtKm(sc.annual_km) }} km/an · {{ sc.years }} an(s)
            </div>
          </button>
          <div class="flex items-center gap-1.5 shrink-0">
            <button :aria-label="`Modifier ${sc.name}`" class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300" @click="editScenario(sc)">
              <Pencil class="w-4 h-4" />
            </button>
            <button :aria-label="`Supprimer ${sc.name}`" class="p-2 rounded-lg bg-slate-800 hover:bg-red-900/60 text-red-300" @click="removeScenario(sc)">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </li>
      </ul>
    </div>

    <!-- Editor (steps 1 and 2) -->
    <form v-else-if="view === 'edit'" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 space-y-5" @submit.prevent="nextStep">
      <div class="text-xs text-slate-400">Étape {{ step }} sur 2 — {{ step === 1 ? 'Usage' : isRetro ? 'Thermique équivalent' : 'Véhicules comparés' }}</div>

      <template v-if="step === 1">
        <div>
          <label for="cmp-name" class="block text-xs font-semibold text-slate-300 mb-1">Nom du comparatif</label>
          <input id="cmp-name" v-model="form.name" maxlength="100" placeholder="Ex. Comparé à un SUV essence" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
        </div>
        <div>
          <label for="cmp-mode" class="block text-xs font-semibold text-slate-300 mb-1">Type de comparatif</label>
          <select id="cmp-mode" v-model="form.mode" :disabled="!!editingId" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" @change="onModeChange">
            <option value="RETROSPECTIVE" :disabled="!vehicleStore.activeVehicle">
              Mon électrique suivie{{ vehicleStore.activeVehicle ? ` (${vehicleStore.activeVehicle.name})` : '' }} vs un thermique
            </option>
            <option value="PROJECTION">Projection : je compare deux véhicules que je saisis</option>
          </select>
          <p v-if="isRetro" class="text-[11px] text-slate-500 mt-1">Les coûts de l'électrique sont ceux réellement constatés sur votre véhicule.</p>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="cmp-km" class="block text-xs font-semibold text-slate-300 mb-1">Kilomètres par an</label>
            <input id="cmp-km" v-model.number="form.annual_km" type="number" min="1" step="500" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            <p v-if="defaults && isRetro" class="text-[11px] text-slate-500 mt-1">
              {{ defaults.annual_km_from_data ? 'Estimé depuis votre historique' : 'Valeur par défaut (historique insuffisant)' }}
            </p>
          </div>
          <div>
            <label for="cmp-years" class="block text-xs font-semibold text-slate-300 mb-1">Durée (années)</label>
            <input id="cmp-years" v-model.number="form.years" type="number" min="1" max="15" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
        </div>
      </template>

      <template v-else>
        <div class="space-y-3">
          <h2 class="text-sm font-semibold text-amber-300">Thermique équivalent</h2>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="cmp-fuel" class="block text-xs font-semibold text-slate-300 mb-1">Carburant</label>
              <select id="cmp-fuel" v-model="form.ice.fuel_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" @change="applyFuelDefaults">
                <option v-for="f in fuelTypes" :key="f.fuel_type" :value="f.fuel_type">{{ f.label }}</option>
              </select>
            </div>
            <div>
              <label for="cmp-ice-l100" class="block text-xs font-semibold text-slate-300 mb-1">Consommation (L/100 km)</label>
              <input id="cmp-ice-l100" v-model.number="form.ice.l_per_100km" type="number" min="0.1" step="0.1" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="cmp-ice-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix du carburant (€/L)</label>
              <input id="cmp-ice-price" v-model.number="form.ice.fuel_price" type="number" min="0" step="0.01" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>
          <p class="text-[11px] text-slate-500 flex items-center gap-1"><Info class="w-3 h-3" /> {{ defaults?.source || 'Valeurs indicatives, à ajuster' }}</p>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in iceFields" :key="f.key">
              <label :for="`cmp-ice-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ f.label }}</label>
              <input :id="`cmp-ice-${f.key}`" v-model.number="form.ice[f.key]" type="number" min="0" :step="f.step" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>
        </div>

        <div v-if="!isRetro" class="space-y-3 pt-2 border-t border-slate-800">
          <h2 class="text-sm font-semibold text-sky-300">Véhicule électrique</h2>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="cmp-ev-kwh" class="block text-xs font-semibold text-slate-300 mb-1">Consommation (kWh/100 km)</label>
              <input id="cmp-ev-kwh" v-model.number="form.ev.kwh_per_100km" type="number" min="0.1" step="0.1" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="cmp-ev-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix moyen de l'électricité (€/kWh)</label>
              <input id="cmp-ev-price" v-model.number="form.ev.eur_per_kwh" type="number" min="0" step="0.01" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in evFields" :key="f.key">
              <label :for="`cmp-ev-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ f.label }}</label>
              <input :id="`cmp-ev-${f.key}`" v-model.number="form.ev[f.key]" type="number" min="0" :step="f.step" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>
        </div>
      </template>

      <p v-if="formError" class="text-xs text-red-300" role="alert">{{ formError }}</p>

      <div class="flex items-center justify-between pt-1">
        <button type="button" class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2" @click="prevStep">
          <ArrowLeft class="w-4 h-4" /> {{ step === 1 ? 'Annuler' : 'Retour' }}
        </button>
        <button type="submit" :disabled="saving" class="bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2">
          {{ step === 2 ? (saving ? 'Calcul…' : 'Voir le résultat') : 'Suivant' }} <ArrowRight class="w-4 h-4" />
        </button>
      </div>
    </form>

    <!-- Result -->
    <div v-else-if="view === 'result'" class="space-y-5">
      <button class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5" @click="view = 'list'">
        <ArrowLeft class="w-4 h-4" /> Mes comparatifs
      </button>

      <div v-if="!result" class="text-sm text-slate-400">Calcul en cours…</div>
      <template v-else>
        <div
          class="rounded-2xl border p-5 flex items-center gap-4"
          :class="savings >= 0 ? 'bg-emerald-500/10 border-emerald-500/30' : 'bg-amber-500/10 border-amber-500/30'"
        >
          <component :is="savings >= 0 ? TrendingDown : TrendingUp" class="w-8 h-8 shrink-0" :class="savings >= 0 ? 'text-emerald-400' : 'text-amber-400'" />
          <div>
            <div class="text-lg font-bold text-white">{{ verdict }}</div>
            <div class="text-xs text-slate-400 mt-0.5">{{ currentScenario?.name }} · {{ fmtKm(result.annual_km) }} km/an</div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
            <div class="flex items-center justify-between mb-2">
              <h2 class="text-sm font-semibold text-sky-300">Électrique</h2>
              <span class="text-[10px] px-2 py-0.5 rounded-full border" :class="result.mode === 'RETROSPECTIVE' ? 'border-emerald-500/40 text-emerald-300' : 'border-slate-600 text-slate-400'">
                {{ result.mode === 'RETROSPECTIVE' ? 'réel' : 'estimé' }}
              </span>
            </div>
            <div class="text-2xl font-bold text-white">{{ fmtEur(result.ev.total) }}</div>
            <div class="text-xs text-slate-400 mt-1">{{ fmtEur(result.ev.per_month) }}/mois · {{ result.ev.cost_per_km.toFixed(3).replace('.', ',') }} €/km</div>
          </div>
          <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
            <div class="flex items-center justify-between mb-2">
              <h2 class="text-sm font-semibold text-amber-300">Thermique</h2>
              <span class="text-[10px] px-2 py-0.5 rounded-full border border-slate-600 text-slate-400">estimé</span>
            </div>
            <div class="text-2xl font-bold text-white">{{ fmtEur(result.ice.total) }}</div>
            <div class="text-xs text-slate-400 mt-1">{{ fmtEur(result.ice.per_month) }}/mois · {{ result.ice.cost_per_km.toFixed(3).replace('.', ',') }} €/km</div>
          </div>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4 overflow-x-auto">
          <table class="w-full text-sm">
            <caption class="sr-only">Coût par poste sur {{ result.years_count }} an(s)</caption>
            <thead>
              <tr class="text-xs text-slate-400 text-right">
                <th scope="col" class="text-left font-semibold pb-2">Poste</th>
                <th scope="col" class="font-semibold pb-2">Électrique</th>
                <th scope="col" class="font-semibold pb-2">Thermique</th>
              </tr>
            </thead>
            <tbody class="text-slate-200">
              <tr v-for="row in costRows" :key="row.label" class="border-t border-slate-800 text-right">
                <th scope="row" class="text-left font-normal py-1.5">{{ row.label }}</th>
                <td>{{ fmtEur(row.ev) }}</td>
                <td>{{ fmtEur(row.ice) }}</td>
              </tr>
              <tr class="border-t border-slate-700 text-right font-semibold text-white">
                <th scope="row" class="text-left py-1.5">Total</th>
                <td>{{ fmtEur(result.ev.total) }}</td>
                <td>{{ fmtEur(result.ice.total) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-sm font-semibold text-white mb-1">Coût cumulé</h2>
          <p class="text-xs text-slate-400 mb-3">{{ breakEvenText }}</p>
          <div class="h-64"><canvas ref="chartRef" aria-label="Coût cumulé électrique et thermique" role="img"></canvas></div>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-sm font-semibold text-white mb-2">Sensibilité</h2>
          <ul class="text-xs text-slate-300 space-y-1">
            <li v-for="s in result.sensitivity" :key="s.label" class="flex justify-between">
              <span>{{ s.label }}</span>
              <span>{{ s.ev_savings >= 0 ? `électrique −${fmtEur(s.ev_savings)}` : `électrique +${fmtEur(-s.ev_savings)}` }}</span>
            </li>
          </ul>
        </div>

        <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-xs font-semibold text-slate-300 mb-2 flex items-center gap-1.5"><Info class="w-3.5 h-3.5" /> Hypothèses</h2>
          <ul class="list-disc pl-4 text-xs text-slate-400 space-y-1">
            <li v-for="a in result.assumptions" :key="a">{{ a }}</li>
          </ul>
        </div>

        <div class="flex gap-2">
          <button class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2" @click="editScenario(currentScenario)">
            <Pencil class="w-4 h-4" /> Modifier les hypothèses
          </button>
        </div>
      </template>
    </div>
  </div>
</template>
