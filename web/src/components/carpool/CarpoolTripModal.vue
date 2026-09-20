<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { Users, Plus, Trash2, X, CheckSquare, Square, Navigation, Calculator, Lock, RotateCw, ChevronDown, ChevronUp } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import {
  COST_FIELDS,
  allocate,
  cents,
  clampPassengerStops as clampStops,
  earliestSelectedDriveDate,
  emptyLeg,
  estimateTitle,
  euros,
  fmt,
  formatDriveTime,
  legsFromEstimate,
  legsFromTrip,
  newPassenger,
  passengersFromTrip,
  remapPassengerStops,
  stopNames,
  toDateInputString,
  type LegForm,
  type PassengerForm,
} from '@/utils/carpool'

// Creates a carpool trip, or edits `editing`. A new trip can start from drives or a trip group (createOptions).
// openToken changes every time the page asks to open the modal, so the form is initialised again even if it is already open.
const props = defineProps<{
  vehicleId: string
  editing: any | null
  createOptions: { driveIds?: string[]; tripGroupId?: string }
  openToken: number
}>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const editingTripId = computed<string | null>(() => props.editing?.id ?? null)
const modalSubmitting = ref(false)
const estimating = ref(false)
const vehicleStore = useVehicleStore()
// Trips are built from TeslaMate drives when the vehicle has them, from a typed distance otherwise
const defaultSourceMode = (): 'DRIVES' | 'MANUAL' => (vehicleStore.hasTeslaMate ? 'DRIVES' : 'MANUAL')
const sourceMode = ref<'DRIVES' | 'MANUAL'>(defaultSourceMode())
const recentDrives = ref<any[]>([])
const selectedDriveIds = ref<string[]>([])
const titleTouched = ref(false)
const currentRates = ref<any>(null)
const expandedPassengerIndex = ref<number | null>(null)

function togglePassengerMath(index: number) {
  expandedPassengerIndex.value = expandedPassengerIndex.value === index ? null : index
}

const isDateDisabled = computed(() => sourceMode.value === 'DRIVES')

const form = ref({
  title: '',
  date: toDateInputString(new Date()),
  trip_group_id: null as string | null,
  notes: '',
  legs: [] as LegForm[],
  passengers: [] as PassengerForm[],
})

// ---------- Live form computations ----------
const stops = computed(() => stopNames(form.value.legs))
const live = computed(() => allocate(form.value.legs, form.value.passengers))
const liveDistance = computed(() => form.value.legs.reduce((s, l) => s + (Number(l.distance_km) || 0), 0))
const liveRevenue = computed(() => form.value.passengers.reduce((s, p) => s + cents(p.amount_paid), 0))
const liveNet = computed(() => live.value.total - liveRevenue.value)
const liveCoverage = computed(() => (live.value.total > 0 ? Math.min(100, Math.round((liveRevenue.value / live.value.total) * 1000) / 10) : 0))

async function loadRecentDrives() {
  if (!props.vehicleId) return
  try {
    const res = await api.getDrives(props.vehicleId, { limit: 200 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

function clampPassengerStops(previousLegCount: number) {
  clampStops(form.value.passengers, form.value.legs.length, previousLegCount)
}

// ---------- Estimation ----------
function applyEstimate(est: any) {
  currentRates.value = est
  const previousLegs = form.value.legs
  const previousLegCount = previousLegs.length
  form.value.legs = legsFromEstimate(est)
  remapPassengerStops(previousLegs, form.value.legs, form.value.passengers)
  clampPassengerStops(previousLegCount === 0 ? 0 : -1)
  if (est.start_date) {
    form.value.date = toDateInputString(est.start_date)
  }
  if (!titleTouched.value && form.value.legs.length) {
    form.value.title = estimateTitle(stops.value, form.value.legs.length)
  }
}

async function estimateFromDrives() {
  if (!props.vehicleId) return
  if (!selectedDriveIds.value.length) {
    form.value.legs = []
    form.value.date = toDateInputString(new Date())
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(props.vehicleId, { drive_ids: selectedDriveIds.value })
    applyEstimate(est)
    if (est.start_date) {
      form.value.date = toDateInputString(est.start_date)
    } else {
      const first = earliestSelectedDriveDate(recentDrives.value, selectedDriveIds.value)
      if (first) form.value.date = first
    }
  } catch (err: any) {
    showAlert(`Erreur d'estimation : ${err.message}`, 'Erreur', 'danger')
  } finally {
    estimating.value = false
  }
}

async function toggleDrive(driveId: string) {
  const idx = selectedDriveIds.value.indexOf(driveId)
  if (idx > -1) selectedDriveIds.value.splice(idx, 1)
  else selectedDriveIds.value.push(driveId)
  await estimateFromDrives()
}

async function estimateManualLeg(index: number) {
  const leg = form.value.legs[index]
  if (!props.vehicleId || !(Number(leg.distance_km) > 0)) {
    showAlert("Indiquez d'abord la distance de l'étape", 'Champ requis', 'warning')
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(props.vehicleId, { distance_km: Number(leg.distance_km) })
    currentRates.value = est
    const estimated = est.legs?.[0] || est
    leg.electricity_cost = estimated.electricity_cost
    leg.tires_cost = estimated.tires_cost
    leg.maintenance_cost = estimated.maintenance_cost
    leg.insurance_cost = estimated.insurance_cost
  } catch (err: any) {
    showAlert(`Erreur d'estimation : ${err.message}`, 'Erreur', 'danger')
  } finally {
    estimating.value = false
  }
}

function addManualLeg() {
  const previous = form.value.legs.length
  const leg = emptyLeg()
  if (previous) leg.start_label = form.value.legs[previous - 1].end_label
  form.value.legs.push(leg)
  clampPassengerStops(previous)
}

function removeLeg(index: number) {
  if (form.value.legs.length <= 1) return
  const previous = form.value.legs.length
  form.value.legs.splice(index, 1)
  clampPassengerStops(previous)
}

function switchSource(mode: 'DRIVES' | 'MANUAL') {
  if (sourceMode.value === mode) return
  sourceMode.value = mode
  if (mode === 'MANUAL') {
    selectedDriveIds.value = []
    form.value.legs = form.value.legs.map((l) => ({ ...l, drive_id: null }))
    if (!form.value.legs.length) addManualLeg()
  }
}

// ---------- Initialisation ----------
function resetForm() {
  titleTouched.value = false
  currentRates.value = null
  expandedPassengerIndex.value = null
  selectedDriveIds.value = []
  sourceMode.value = defaultSourceMode()
  form.value = {
    title: '',
    date: toDateInputString(new Date()),
    trip_group_id: null,
    notes: '',
    legs: [],
    passengers: [],
  }
  form.value.passengers.push({ ...newPassenger(0, form.value.legs.length), passenger_name: 'Passager 1 (BlaBlaCar)' })
}

async function initCreate(options: { driveIds?: string[]; tripGroupId?: string }) {
  resetForm()
  if (sourceMode.value === 'MANUAL') {
    addManualLeg()
  } else {
    await loadRecentDrives()
  }
  if (!props.vehicleId) return

  if (options.tripGroupId) {
    form.value.trip_group_id = options.tripGroupId
    estimating.value = true
    try {
      const est = await api.estimateCarpoolCosts(props.vehicleId, { trip_group_id: options.tripGroupId })
      selectedDriveIds.value = (est.legs || []).map((l: any) => l.drive_id).filter(Boolean)
      applyEstimate(est)
      const groups = await api.getTripGroups(props.vehicleId)
      const group = (groups || []).find((g: any) => g.id === options.tripGroupId)
      if (group?.name) {
        form.value.title = group.name
        titleTouched.value = true
      }
      if (est.start_date) {
        form.value.date = toDateInputString(est.start_date)
      } else if (group?.start_time) {
        form.value.date = toDateInputString(group.start_time)
      }
    } catch (err: any) {
      showAlert(`Erreur d'estimation : ${err.message}`, 'Erreur', 'danger')
    } finally {
      estimating.value = false
    }
  } else if (options.driveIds?.length) {
    selectedDriveIds.value = [...options.driveIds]
    await estimateFromDrives()
  }
}

function initEdit(trip: any) {
  resetForm()
  titleTouched.value = true
  const legs = legsFromTrip(trip)
  sourceMode.value = legs.some((l) => l.drive_id) ? 'DRIVES' : 'MANUAL'
  selectedDriveIds.value = legs.map((l) => l.drive_id).filter((id): id is string => !!id)
  form.value = {
    title: trip.title,
    date: toDateInputString(trip.date),
    trip_group_id: trip.trip_group_id || null,
    notes: trip.notes || '',
    legs,
    passengers: passengersFromTrip(trip, legs.length),
  }
  if (!form.value.passengers.length) addPassenger()
  loadRecentDrives()
}

watch([open, () => props.openToken], ([isOpen]) => {
  if (!isOpen) return
  if (props.editing) initEdit(props.editing)
  else initCreate(props.createOptions)
})

function addPassenger() {
  form.value.passengers.push(newPassenger(form.value.passengers.length, form.value.legs.length))
}

function removePassenger(index: number) {
  form.value.passengers.splice(index, 1)
}

function onBoardChange(p: PassengerForm) {
  if (p.alight_stop_index <= p.board_stop_index) p.alight_stop_index = p.board_stop_index + 1
}

function applyFairPrice(index: number) {
  form.value.passengers[index].amount_paid = euros(live.value.shares[index])
}

async function handleSave() {
  if (!props.vehicleId) return
  if (!form.value.title.trim()) {
    showAlert('Veuillez indiquer un titre (ex: Paris → Lyon)', 'Champ requis', 'warning')
    return
  }
  if (!form.value.legs.length) {
    showAlert('Sélectionnez au moins un trajet ou ajoutez une étape', 'Champ requis', 'warning')
    return
  }
  modalSubmitting.value = true
  try {
    const payload = {
      title: form.value.title,
      date: new Date(form.value.date).toISOString(),
      trip_group_id: form.value.trip_group_id,
      notes: form.value.notes || null,
      legs: form.value.legs.map((l) => ({
        ...l,
        distance_km: Number(l.distance_km) || 0,
        ...Object.fromEntries(COST_FIELDS.map((f) => [f.key, Number(l[f.key]) || 0])),
      })),
      passengers: form.value.passengers.map((p) => ({
        ...p,
        seats: Number(p.seats) || 1,
        amount_paid: Number(p.amount_paid) || 0,
        notes: p.notes || null,
      })),
    }
    if (editingTripId.value) {
      await api.updateCarpool(props.vehicleId, editingTripId.value, payload)
    } else {
      await api.createCarpool(props.vehicleId, payload)
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(`Erreur lors de l'enregistrement : ${err.message}`, 'Erreur', 'danger')
  } finally {
    modalSubmitting.value = false
  }
}

async function handleModalRecalculate() {
  if (!props.vehicleId) return
  if (sourceMode.value === 'DRIVES') {
    await estimateFromDrives()
  } else {
    for (let i = 0; i < form.value.legs.length; i++) {
      if (Number(form.value.legs[i].distance_km) > 0) {
        await estimateManualLeg(i)
      }
    }
  }
  showAlert('Les coûts ont été recalculés selon les taux et péages actuels.', 'Coûts réestimés', 'info')
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-4xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Users class="w-5 h-5 text-rose-400" />
          {{ editingTripId ? 'Modifier le covoiturage' : 'Nouveau covoiturage' }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

      <!-- Source -->
      <div class="space-y-3">
        <div v-if="vehicleStore.hasTeslaMate || sourceMode === 'DRIVES'" class="flex items-center gap-1 bg-slate-950 border border-slate-800 p-1 rounded-xl w-fit text-xs font-semibold">
          <button
            @click="switchSource('DRIVES')"
            class="px-3 py-1.5 rounded-lg"
            :class="sourceMode === 'DRIVES' ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white'"
          >
            Trajets TeslaMate
          </button>
          <button
            @click="switchSource('MANUAL')"
            class="px-3 py-1.5 rounded-lg"
            :class="sourceMode === 'MANUAL' ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white'"
          >
            Saisie manuelle
          </button>
        </div>

        <div v-if="sourceMode === 'DRIVES'" class="space-y-1.5">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">Cochez les trajets qui composent le covoiturage : chaque trajet devient une étape.</span>
            <span class="text-rose-400 font-semibold">{{ selectedDriveIds.length }} étape(s){{ estimating ? ' • estimation…' : '' }}</span>
          </div>
          <div class="max-h-44 overflow-y-auto space-y-1 pr-1">
            <button
              v-for="d in recentDrives"
              :key="d.id"
              type="button"
              @click="toggleDrive(d.id)"
              class="w-full flex items-center justify-between gap-3 p-2 rounded-lg text-xs border text-left transition-colors"
              :class="selectedDriveIds.includes(d.id) ? 'bg-rose-500/10 border-rose-500/40 text-rose-100' : 'bg-slate-800/60 border-slate-700 text-slate-300 hover:bg-slate-800'"
            >
              <span class="flex items-center gap-2 truncate">
                <component :is="selectedDriveIds.includes(d.id) ? CheckSquare : Square" class="w-4 h-4 shrink-0 text-rose-400" />
                <span class="truncate">{{ formatDriveTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}</span>
              </span>
              <span class="font-mono text-[11px] text-slate-400 shrink-0">{{ Number(d.distance_km).toFixed(0) }} km</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Title & date -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div class="sm:col-span-2">
          <label for="carpool-title" class="block text-xs font-semibold text-slate-400 mb-1">Titre</label>
          <input
            id="carpool-title"
            v-model="form.title"
            @input="titleTouched = true"
            placeholder="ex: Annecy → Valence"
            class="w-full bg-slate-800 text-slate-100 text-sm rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>
        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="carpool-date" class="block text-xs font-semibold text-slate-400">Date</label>
            <span v-if="isDateDisabled" class="text-[10px] text-slate-400 flex items-center gap-1 font-normal" title="La date est automatiquement liée au(x) trajet(s) TeslaMate">
              <Lock class="w-3 h-3 text-slate-400" />
              Date du trajet
            </span>
          </div>
          <AppDatePicker
            id="carpool-date"
            v-model="form.date"
            :disabled="isDateDisabled"
            required
          />
        </div>
      </div>

      <!-- Legs -->
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <Navigation class="w-4 h-4 text-indigo-400" />
            Étapes et coûts réels ({{ liveDistance.toFixed(1) }} km • {{ fmt(euros(live.total)) }} €)
          </h4>
          <div class="flex items-center gap-2">
            <button
              v-if="editingTripId && form.legs.length > 0"
              type="button"
              @click="handleModalRecalculate"
              :disabled="estimating"
              class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1.5 px-2.5 py-1 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 rounded-lg transition-colors disabled:opacity-50"
              title="Mettre à jour les coûts réels selon les taux actuels et péages rattachés"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': estimating }" />
              <span>Recalculer les coûts</span>
            </button>
            <button
              v-if="sourceMode === 'MANUAL'"
              type="button"
              @click="addManualLeg"
              class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1"
            >
              <Plus class="w-3.5 h-3.5" /> Ajouter une étape
            </button>
          </div>
        </div>
        <p v-if="!form.legs.length" class="text-xs text-slate-500">Sélectionnez au moins un trajet.</p>

        <div v-for="(leg, i) in form.legs" :key="i" class="bg-slate-950/50 border border-slate-800 rounded-xl p-3 space-y-2">
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <span class="font-bold text-indigo-300">Étape {{ i + 1 }}</span>
            <label :for="`leg-start-${i}`" class="sr-only">Départ de l'étape {{ i + 1 }}</label>
            <input
              :id="`leg-start-${i}`"
              v-model="leg.start_label"
              placeholder="Départ"
              class="w-36 bg-slate-800 text-slate-100 rounded-lg px-2 py-1 border border-slate-700"
            />
            <span class="text-slate-500">→</span>
            <label :for="`leg-end-${i}`" class="sr-only">Arrivée de l'étape {{ i + 1 }}</label>
            <input
              :id="`leg-end-${i}`"
              v-model="leg.end_label"
              placeholder="Arrivée"
              class="w-36 bg-slate-800 text-slate-100 rounded-lg px-2 py-1 border border-slate-700"
            />
            <label :for="`leg-distance-${i}`" class="text-slate-400">km</label>
            <input
              :id="`leg-distance-${i}`"
              v-model.number="leg.distance_km"
              type="number"
              step="0.1"
              min="0"
              :readonly="!!leg.drive_id"
              class="w-20 bg-slate-800 text-slate-100 rounded-lg px-2 py-1 border border-slate-700"
            />
            <button
              v-if="!leg.drive_id"
              type="button"
              @click="estimateManualLeg(i)"
              class="flex items-center gap-1 text-[11px] text-indigo-400 hover:text-indigo-300"
              title="Estimer électricité, pneus, entretien et assurance à partir de la distance"
            >
              <Calculator class="w-3.5 h-3.5" /> Estimer
            </button>
            <span class="ml-auto text-slate-300">
              {{ fmt(euros(live.legDetails[i]?.total || 0)) }} € •
              {{ 1 + (live.legDetails[i]?.seats || 0) }} à bord •
              <strong>{{ fmt(euros(live.legDetails[i]?.perPerson || 0)) }} €/pers.</strong>
            </span>
            <button v-if="sourceMode === 'MANUAL' && form.legs.length > 1" type="button" @click="removeLeg(i)" class="text-slate-500 hover:text-rose-400" title="Supprimer l'étape">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
          <div class="grid grid-cols-3 sm:grid-cols-6 gap-2">
            <div v-for="f in COST_FIELDS" :key="f.key">
              <label :for="`leg-${f.key}-${i}`" class="block text-[10px] text-slate-500 mb-0.5">{{ f.label }} (€)</label>
              <input
                :id="`leg-${f.key}-${i}`"
                v-model.number="(leg as any)[f.key]"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2 py-1 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- Passengers -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <Users class="w-4 h-4 text-blue-400" />
            Passagers, montée et descente
          </h4>
          <button type="button" @click="addPassenger" class="text-xs text-blue-400 hover:text-blue-300 font-semibold flex items-center gap-1">
            <Plus class="w-3.5 h-3.5" /> Ajouter un passager
          </button>
        </div>

        <div v-for="(p, index) in form.passengers" :key="index" class="bg-slate-950/50 border border-slate-800 rounded-xl p-3 space-y-2">
          <div class="grid grid-cols-2 sm:grid-cols-12 gap-2 items-end text-xs">
            <div class="col-span-2 sm:col-span-3">
              <label :for="`passenger-name-${index}`" class="block text-[10px] text-slate-500 mb-0.5">Nom</label>
              <input
                :id="`passenger-name-${index}`"
                v-model="p.passenger_name"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
            <div class="sm:col-span-3">
              <label :for="`passenger-board-${index}`" class="block text-[10px] text-slate-500 mb-0.5">Monte à</label>
              <select
                :id="`passenger-board-${index}`"
                v-model.number="p.board_stop_index"
                @change="onBoardChange(p)"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              >
                <option v-for="(name, s) in stops.slice(0, -1)" :key="s" :value="s">{{ name }}</option>
              </select>
            </div>
            <div class="sm:col-span-3">
              <label :for="`passenger-alight-${index}`" class="block text-[10px] text-slate-500 mb-0.5">Descend à</label>
              <select
                :id="`passenger-alight-${index}`"
                v-model.number="p.alight_stop_index"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              >
                <option v-for="(name, s) in stops" v-show="s > p.board_stop_index" :key="s" :value="s" :disabled="s <= p.board_stop_index">{{ name }}</option>
              </select>
            </div>
            <div>
              <label :for="`passenger-seats-${index}`" class="block text-[10px] text-slate-500 mb-0.5">Places</label>
              <input
                :id="`passenger-seats-${index}`"
                v-model.number="p.seats"
                type="number"
                min="1"
                max="7"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
            <div class="sm:col-span-2">
              <label :for="`passenger-paid-${index}`" class="block text-[10px] text-slate-500 mb-0.5">Payé (€)</label>
              <input
                :id="`passenger-paid-${index}`"
                v-model.number="p.amount_paid"
                type="number"
                step="0.5"
                min="0"
                class="w-full bg-slate-900 text-emerald-400 font-bold rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2 text-[11px]">
            <span class="text-slate-400">
              Part équitable : <strong class="text-slate-200">{{ fmt(euros(live.shares[index] || 0)) }} €</strong>
              <span :class="cents(p.amount_paid) >= (live.shares[index] || 0) ? 'text-emerald-400' : 'text-amber-400'" class="ml-2">
                {{ cents(p.amount_paid) >= (live.shares[index] || 0)
                  ? `+${fmt(euros(cents(p.amount_paid) - (live.shares[index] || 0)))} € au-dessus`
                  : `${fmt(euros((live.shares[index] || 0) - cents(p.amount_paid)))} € sous sa part` }}
              </span>
            </span>
            <span class="flex items-center gap-3">
              <button
                type="button"
                @click="togglePassengerMath(index)"
                class="text-indigo-400 hover:text-indigo-300 font-medium flex items-center gap-1 transition-colors"
                :title="expandedPassengerIndex === index ? 'Masquer le détail du calcul' : 'Comprendre comment cette part est calculée'"
              >
                <Calculator class="w-3.5 h-3.5" />
                <span>{{ expandedPassengerIndex === index ? 'Masquer le calcul' : 'Détail du calcul' }}</span>
                <ChevronUp v-if="expandedPassengerIndex === index" class="w-3.5 h-3.5" />
                <ChevronDown v-else class="w-3.5 h-3.5" />
              </button>
              <button type="button" @click="applyFairPrice(index)" class="text-indigo-400 hover:text-indigo-300 font-semibold">Appliquer la part équitable</button>
              <button v-if="form.passengers.length > 1" type="button" @click="removePassenger(index)" class="text-slate-500 hover:text-rose-400" title="Retirer ce passager">
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </span>
          </div>

          <!-- Mathematical Breakdown Card -->
          <div
            v-if="expandedPassengerIndex === index"
            class="mt-3 pt-3 border-t border-slate-800 space-y-2.5 bg-slate-950/70 p-3 rounded-xl"
          >
            <div class="flex items-center justify-between text-xs">
              <span class="font-bold text-slate-200 flex items-center gap-1.5">
                <Calculator class="w-3.5 h-3.5 text-indigo-400" />
                Formule de calcul par tronçon — {{ p.passenger_name || `Passager ${index + 1}` }}
              </span>
              <span class="text-[11px] text-slate-400 font-medium">
                {{ p.seats }} place{{ p.seats > 1 ? 's' : '' }} réservée{{ p.seats > 1 ? 's' : '' }}
              </span>
            </div>

            <div class="space-y-2">
              <div
                v-for="(leg, legIdx) in form.legs"
                :key="legIdx"
                class="text-[11px] p-2.5 rounded-lg border transition-colors"
                :class="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index
                  ? 'bg-slate-900/90 border-indigo-500/30 text-slate-200'
                  : 'bg-slate-900/30 border-slate-800/50 text-slate-500 opacity-60'"
              >
                <div class="flex items-center justify-between font-semibold">
                  <span class="flex items-center gap-1.5">
                    <span class="w-4 h-4 rounded-full bg-slate-800 flex items-center justify-center text-[10px] font-mono text-slate-300">
                      {{ legIdx + 1 }}
                    </span>
                    <span>{{ stops[legIdx] }} → {{ stops[legIdx + 1] }}</span>
                    <span v-if="Number(leg.distance_km)" class="text-slate-400 font-normal">({{ leg.distance_km }} km)</span>
                  </span>
                  <span v-if="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index" class="text-indigo-300 font-bold">
                    {{ fmt(euros((live.legDetails[legIdx]?.perPerson || 0) * (Number(p.seats) || 1))) }} €
                  </span>
                  <span v-else class="text-slate-500 italic text-[10px]">
                    Non emprunté
                  </span>
                </div>

                <div v-if="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index" class="mt-1.5 pl-5.5 text-[10px] text-slate-400 flex flex-wrap items-center gap-x-2.5 gap-y-1">
                  <span>Coût réel du tronçon : <strong class="text-slate-200">{{ fmt(euros(live.legDetails[legIdx]?.total || 0)) }} €</strong></span>
                  <span>•</span>
                  <span>Occupants : <strong class="text-slate-200">1 conducteur + {{ live.legDetails[legIdx]?.seats || 0 }} passager(s) = {{ 1 + (live.legDetails[legIdx]?.seats || 0) }}</strong></span>
                  <span>•</span>
                  <span class="text-indigo-300/90">
                    Calcul : {{ fmt(euros(live.legDetails[legIdx]?.total || 0)) }} € ÷ {{ 1 + (live.legDetails[legIdx]?.seats || 0) }} pers.{{ p.seats > 1 ? ` × ${p.seats} pl.` : '' }}
                  </span>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-slate-800/80 flex items-center justify-between text-xs">
              <span class="text-slate-400">Total part équitable due :</span>
              <div class="text-right">
                <span class="font-bold text-emerald-400 text-sm">{{ fmt(euros(live.shares[index] || 0)) }} €</span>
                <span class="text-[10px] text-slate-500 block">Somme exacte des tronçons où ce passager est à bord</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Simulation -->
      <div class="bg-slate-950/80 border border-slate-800 rounded-2xl p-4 grid grid-cols-2 sm:grid-cols-5 gap-3 text-xs">
        <div>
          <div class="text-slate-500">Coût réel</div>
          <div class="text-sm font-bold text-white">{{ fmt(euros(live.total)) }} €</div>
        </div>
        <div>
          <div class="text-slate-500">Parts des passagers</div>
          <div class="text-sm font-bold text-blue-400">{{ fmt(euros(live.passengersShare)) }} €</div>
        </div>
        <div>
          <div class="text-slate-500">Part du conducteur</div>
          <div class="text-sm font-bold text-amber-400">{{ fmt(euros(live.driverShare)) }} €</div>
        </div>
        <div>
          <div class="text-slate-500">Perçu</div>
          <div class="text-sm font-bold text-emerald-400">{{ fmt(euros(liveRevenue)) }} € ({{ liveCoverage }} %)</div>
        </div>
        <div>
          <div class="text-slate-500">{{ liveNet > 0 ? 'Reste à charge' : 'Excédent' }}</div>
          <div class="text-sm font-bold" :class="liveNet > 0 ? 'text-white' : 'text-emerald-400'">{{ fmt(euros(Math.abs(liveNet))) }} €</div>
        </div>
      </div>

      <div>
        <label for="carpool-notes" class="block text-xs font-semibold text-slate-400 mb-1">Notes (optionnel)</label>
        <input id="carpool-notes" v-model="form.notes" class="w-full bg-slate-800 text-slate-100 text-sm rounded-xl px-3 py-2 border border-slate-700" />
      </div>

      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          Annuler
        </button>
        <button
          type="button"
          @click="handleSave"
          :disabled="modalSubmitting || estimating"
          class="bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white text-xs font-semibold px-5 py-2 rounded-xl transition-colors"
        >
          {{ modalSubmitting ? 'Enregistrement…' : 'Enregistrer' }}
        </button>
      </div>
    </div>
  </div>
</template>
