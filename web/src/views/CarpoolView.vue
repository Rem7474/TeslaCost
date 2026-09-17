<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import AppDatePicker from '@/components/AppDatePicker.vue'
import {
  Users,
  Plus,
  Trash2,
  Edit2,
  Calendar,
  MapPin,
  TrendingUp,
  Zap,
  Disc,
  Wrench,
  Shield,
  CreditCard,
  CheckCircle2,
  X,
  Receipt,
  Sparkles,
  CheckSquare,
  Square,
  Navigation,
  Calculator,
  Lock,
  RotateCw,
} from 'lucide-vue-next'

interface LegForm {
  drive_id: string | null
  start_label: string
  end_label: string
  distance_km: number
  electricity_cost: number
  tolls_cost: number
  tires_cost: number
  maintenance_cost: number
  insurance_cost: number
  other_cost: number
}

interface PassengerForm {
  passenger_name: string
  seats: number
  amount_paid: number
  board_stop_index: number
  alight_stop_index: number
  notes: string
}

const COST_FIELDS: Array<{ key: keyof LegForm; label: string }> = [
  { key: 'electricity_cost', label: 'Électricité' },
  { key: 'tolls_cost', label: 'Péages' },
  { key: 'tires_cost', label: 'Pneus' },
  { key: 'maintenance_cost', label: 'Entretien' },
  { key: 'insurance_cost', label: 'Assurance' },
  { key: 'other_cost', label: 'Divers' },
]

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const route = useRoute()

const loading = ref(true)
const trips = ref<any[]>([])
const summary = ref<any>({
  total_trips: 0,
  total_passengers: 0,
  total_distance_km: 0,
  total_real_cost: 0,
  total_revenue: 0,
  total_net_cost: 0,
  coverage_rate_pct: 0,
  net_cost_per_km: 0,
  total_passengers_share: 0,
  total_driver_share: 0,
})

// Modal state
const showModal = ref(false)
const editingTripId = ref<string | null>(null)
const modalSubmitting = ref(false)
const estimating = ref(false)
const sourceMode = ref<'DRIVES' | 'MANUAL'>('DRIVES')
const recentDrives = ref<any[]>([])
const selectedDriveIds = ref<string[]>([])
const titleTouched = ref(false)
const currentRates = ref<any>(null)

// Batch selection state
const selectedTripIds = ref<string[]>([])
const recalculating = ref(false)

const isAllSelected = computed(() => {
  return trips.value.length > 0 && selectedTripIds.value.length === trips.value.length
})

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedTripIds.value = []
  } else {
    selectedTripIds.value = trips.value.map((t) => t.id)
  }
}

function toggleTripSelection(tripId: string) {
  const idx = selectedTripIds.value.indexOf(tripId)
  if (idx > -1) {
    selectedTripIds.value.splice(idx, 1)
  } else {
    selectedTripIds.value.push(tripId)
  }
}

function clearTripSelection() {
  selectedTripIds.value = []
}

function toDateInputString(dateVal: string | Date | null | undefined): string {
  if (!dateVal) return ''
  const d = new Date(dateVal)
  if (isNaN(d.getTime())) return ''
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
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

// ---------- Helpers ----------
const cents = (v: number | string) => Math.round((Number(v) || 0) * 100)
const euros = (c: number) => c / 100
const fmt = (v: number) => Number(v || 0).toFixed(2)

function emptyLeg(): LegForm {
  return {
    drive_id: null,
    start_label: '',
    end_label: '',
    distance_km: 0,
    electricity_cost: 0,
    tolls_cost: 0,
    tires_cost: 0,
    maintenance_cost: 0,
    insurance_cost: 0,
    other_cost: 0,
  }
}

function newPassenger(index: number): PassengerForm {
  return {
    passenger_name: `Passager ${index + 1}`,
    seats: 1,
    amount_paid: 0,
    board_stop_index: 0,
    alight_stop_index: Math.max(1, form.value.legs.length),
    notes: '',
  }
}

function legTotalCents(leg: any) {
  return COST_FIELDS.reduce((sum, f) => sum + cents(leg[f.key]), 0)
}

// Stop names: stop i starts leg i, the last stop ends the last leg
function stopNames(legs: any[]) {
  if (!legs.length) return ['Départ', 'Arrivée']
  const names = legs.map((l, i) => l.start_label || (i > 0 && legs[i - 1].end_label) || (i === 0 ? 'Départ' : `Arrêt ${i}`))
  names.push(legs[legs.length - 1].end_label || 'Arrivée')
  return names
}

// Same fair split as the backend: each leg cost is divided between the people on board (driver included)
function allocate(legs: any[], passengers: any[]) {
  const shares = passengers.map(() => 0)
  const legDetails = legs.map((leg, i) => {
    const total = legTotalCents(leg)
    let seats = 0
    passengers.forEach((p) => {
      if (p.board_stop_index <= i && i < p.alight_stop_index) seats += Number(p.seats) || 1
    })
    const perPerson = Math.floor(total / (1 + seats))
    passengers.forEach((p, j) => {
      if (p.board_stop_index <= i && i < p.alight_stop_index) shares[j] += perPerson * (Number(p.seats) || 1)
    })
    return { total, seats, perPerson }
  })
  const total = legDetails.reduce((s, l) => s + l.total, 0)
  const passengersShare = shares.reduce((s, v) => s + v, 0)
  return { legDetails, shares, total, passengersShare, driverShare: total - passengersShare }
}

// ---------- Live form computations ----------
const stops = computed(() => stopNames(form.value.legs))
const live = computed(() => allocate(form.value.legs, form.value.passengers))
const liveDistance = computed(() => form.value.legs.reduce((s, l) => s + (Number(l.distance_km) || 0), 0))
const liveRevenue = computed(() => form.value.passengers.reduce((s, p) => s + cents(p.amount_paid), 0))
const liveNet = computed(() => live.value.total - liveRevenue.value)
const liveCoverage = computed(() => (live.value.total > 0 ? Math.min(100, Math.round((liveRevenue.value / live.value.total) * 1000) / 10) : 0))

// ---------- Data loading ----------
async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    const res = await api.getCarpools(vehicleStore.activeVehicle.id)
    trips.value = res.trips || []
    summary.value = res.summary || summary.value
  } catch (err) {
    console.error('Failed to load carpool trips', err)
  } finally {
    loading.value = false
  }
}

async function loadRecentDrives() {
  if (!vehicleStore.activeVehicle) return
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { limit: 200 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

// ---------- Estimation ----------
// Identifies a stop by the drive it starts (or ends, for the last stop), so that passengers keep their
// boarding and alighting places when drives are added or removed around them.
function stopKeys(legs: LegForm[]) {
  const keys = legs.map((l, i) => (l.drive_id ? `start:${l.drive_id}` : `index:${i}`))
  keys.push(legs.length && legs[legs.length - 1].drive_id ? `end:${legs[legs.length - 1].drive_id}` : `index:${legs.length}`)
  return keys
}

function applyEstimate(est: any) {
  currentRates.value = est
  const previousLegs = form.value.legs
  const previousLegCount = previousLegs.length
  const previousKeys = stopKeys(previousLegs)
  // Alternative key of a stop: the end of the previous drive is the same place as the start of the next one
  const previousAltKeys = previousKeys.map((_, i) => (i > 0 && previousLegs[i - 1]?.drive_id ? `end:${previousLegs[i - 1].drive_id}` : ''))
  form.value.legs = (est.legs || []).map((l: any) => ({
    drive_id: l.drive_id || null,
    start_label: l.start_label || '',
    end_label: l.end_label || '',
    distance_km: l.distance_km,
    electricity_cost: l.electricity_cost,
    tolls_cost: l.tolls_cost,
    tires_cost: l.tires_cost,
    maintenance_cost: l.maintenance_cost,
    insurance_cost: l.insurance_cost,
    other_cost: l.other_cost || 0,
  }))

  if (previousLegCount > 0) {
    const keys = stopKeys(form.value.legs)
    const altIndex = new Map<string, number>()
    form.value.legs.forEach((l, i) => l.drive_id && altIndex.set(`end:${l.drive_id}`, i + 1))
    const locate = (stop: number) => {
      const direct = keys.indexOf(previousKeys[stop])
      if (direct >= 0) return direct
      if (altIndex.has(previousKeys[stop])) return altIndex.get(previousKeys[stop])!
      const alt = previousAltKeys[stop] && altIndex.get(previousAltKeys[stop])
      return alt === undefined || alt === '' ? -1 : alt
    }
    form.value.passengers.forEach((p) => {
      const ridesToEnd = p.alight_stop_index >= previousLegCount
      const board = locate(p.board_stop_index)
      const alight = locate(p.alight_stop_index)
      p.board_stop_index = board >= 0 ? board : 0
      p.alight_stop_index = ridesToEnd || alight < 0 ? form.value.legs.length : alight
    })
  }
  clampPassengerStops(previousLegCount === 0 ? 0 : -1)
  if (est.start_date) {
    form.value.date = toDateInputString(est.start_date)
  }
  if (!titleTouched.value && form.value.legs.length) {
    const names = stops.value
    form.value.title = `${names[0]} → ${names[names.length - 1]}${form.value.legs.length > 1 ? ` (${form.value.legs.length} étapes)` : ''}`
  }
}

// Keeps passengers' stops inside the current stops. A passenger who rode to the last stop (or any passenger
// entered before the legs were known) still rides to the new last stop.
function clampPassengerStops(previousLegCount: number) {
  const n = form.value.legs.length
  if (!n) return
  form.value.passengers.forEach((p) => {
    if (previousLegCount === 0 || (previousLegCount > 0 && p.alight_stop_index >= previousLegCount)) p.alight_stop_index = n
    p.alight_stop_index = Math.min(Math.max(p.alight_stop_index, 1), n)
    p.board_stop_index = Math.min(Math.max(p.board_stop_index, 0), p.alight_stop_index - 1)
  })
}

async function estimateFromDrives() {
  if (!vehicleStore.activeVehicle) return
  if (!selectedDriveIds.value.length) {
    form.value.legs = []
    form.value.date = toDateInputString(new Date())
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, { drive_ids: selectedDriveIds.value })
    applyEstimate(est)
    if (est.start_date) {
      form.value.date = toDateInputString(est.start_date)
    } else {
      const first = recentDrives.value
        .filter((d) => selectedDriveIds.value.includes(d.id))
        .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())[0]
      if (first) form.value.date = toDateInputString(first.start_time)
    }
  } catch (err: any) {
    alert(`Erreur d'estimation : ${err.message}`)
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
  if (!vehicleStore.activeVehicle || !(Number(leg.distance_km) > 0)) {
    alert("Indiquez d'abord la distance de l'étape")
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, { distance_km: Number(leg.distance_km) })
    currentRates.value = est
    const estimated = est.legs?.[0] || est
    leg.electricity_cost = estimated.electricity_cost
    leg.tires_cost = estimated.tires_cost
    leg.maintenance_cost = estimated.maintenance_cost
    leg.insurance_cost = estimated.insurance_cost
  } catch (err: any) {
    alert(`Erreur d'estimation : ${err.message}`)
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

// ---------- Modal ----------
function resetForm() {
  editingTripId.value = null
  titleTouched.value = false
  currentRates.value = null
  selectedDriveIds.value = []
  sourceMode.value = 'DRIVES'
  form.value = {
    title: '',
    date: toDateInputString(new Date()),
    trip_group_id: null,
    notes: '',
    legs: [],
    passengers: [],
  }
  form.value.passengers.push({ ...newPassenger(0), passenger_name: 'Passager 1 (BlaBlaCar)' })
}

async function openCreateModal(options: { driveIds?: string[]; tripGroupId?: string } = {}) {
  resetForm()
  showModal.value = true
  await loadRecentDrives()
  if (!vehicleStore.activeVehicle) return

  if (options.tripGroupId) {
    form.value.trip_group_id = options.tripGroupId
    estimating.value = true
    try {
      const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, { trip_group_id: options.tripGroupId })
      selectedDriveIds.value = (est.legs || []).map((l: any) => l.drive_id).filter(Boolean)
      applyEstimate(est)
      const groups = await api.getTripGroups(vehicleStore.activeVehicle.id)
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
      alert(`Erreur d'estimation : ${err.message}`)
    } finally {
      estimating.value = false
    }
  } else if (options.driveIds?.length) {
    selectedDriveIds.value = [...options.driveIds]
    await estimateFromDrives()
  }
}

function openEditModal(trip: any) {
  resetForm()
  editingTripId.value = trip.id
  titleTouched.value = true
  const legs: LegForm[] = (trip.legs || []).map((l: any) => ({
    drive_id: l.drive_id || null,
    start_label: l.start_label || '',
    end_label: l.end_label || '',
    distance_km: l.distance_km,
    electricity_cost: l.electricity_cost,
    tolls_cost: l.tolls_cost,
    tires_cost: l.tires_cost,
    maintenance_cost: l.maintenance_cost,
    insurance_cost: l.insurance_cost,
    other_cost: l.other_cost,
  }))
  sourceMode.value = legs.some((l) => l.drive_id) ? 'DRIVES' : 'MANUAL'
  selectedDriveIds.value = legs.map((l) => l.drive_id).filter((id): id is string => !!id)
  form.value = {
    title: trip.title,
    date: toDateInputString(trip.date),
    trip_group_id: trip.trip_group_id || null,
    notes: trip.notes || '',
    legs,
    passengers: (trip.passengers || []).map((p: any) => ({
      passenger_name: p.passenger_name,
      seats: p.seats || 1,
      amount_paid: p.amount_paid || 0,
      board_stop_index: p.board_stop_index ?? 0,
      alight_stop_index: p.alight_stop_index ?? legs.length,
      notes: p.notes || '',
    })),
  }
  if (!form.value.passengers.length) addPassenger()
  showModal.value = true
  loadRecentDrives()
}

function addPassenger() {
  form.value.passengers.push(newPassenger(form.value.passengers.length))
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
  if (!vehicleStore.activeVehicle) return
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
      await api.updateCarpool(vehicleStore.activeVehicle.id, editingTripId.value, payload)
    } else {
      await api.createCarpool(vehicleStore.activeVehicle.id, payload)
    }
    showModal.value = false
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de l'enregistrement : ${err.message}`, 'Erreur', 'danger')
  } finally {
    modalSubmitting.value = false
  }
}

async function handleDelete(trip: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer le covoiturage',
    message: `Confirmez-vous la suppression du covoiturage "${trip.title}" ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteCarpool(vehicleStore.activeVehicle.id, trip.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleModalRecalculate() {
  if (!vehicleStore.activeVehicle) return
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

async function handleRecalculateSingle(trip: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Recalculer le covoiturage',
    message: `Voulez-vous recalculer les coûts réels de "${trip.title}" selon les tarifs et péages actuels ?\n(Les montants perçus des passagers restent inchangés)`,
    confirmText: 'Recalculer',
    type: 'primary',
  })
  if (!ok) return

  recalculating.value = true
  try {
    await api.recalculateCarpools(vehicleStore.activeVehicle.id, [trip.id])
    showAlert(`Le covoiturage "${trip.title}" a été recalculé avec succès.`, 'Recalcul terminé', 'success')
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors du recalcul : ${err.message}`, 'Erreur', 'danger')
  } finally {
    recalculating.value = false
  }
}

async function handleBatchRecalculate() {
  if (!vehicleStore.activeVehicle || !selectedTripIds.value.length) return
  const count = selectedTripIds.value.length
  const ok = await showConfirm({
    title: 'Recalculer les covoiturages sélectionnés',
    message: `Voulez-vous recalculer les coûts réels de ${count} covoiturage(s) selon les tarifs d'électricité, péages, pneus et entretien actuels ?\n(Les montants perçus des passagers restent inchangés)`,
    confirmText: 'Recalculer',
    type: 'primary',
  })
  if (!ok) return

  recalculating.value = true
  try {
    const res = await api.recalculateCarpools(vehicleStore.activeVehicle.id, selectedTripIds.value)
    showAlert(`${res.updated_count || count} covoiturage(s) recalculé(s) avec succès.`, 'Recalcul terminé', 'success')
    clearTripSelection()
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors du recalcul : ${err.message}`, 'Erreur', 'danger')
  } finally {
    recalculating.value = false
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' })
}

function formatDriveTime(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => loadData()
)

function checkRouteQueryForCarpool() {
  const driveIds = [route.query.new_drive_id, ...String(route.query.new_drive_ids || '').split(',')]
    .map((v) => String(v || '').trim())
    .filter(Boolean)
  const tripGroupId = route.query.new_trip_group_id as string
  if (tripGroupId) openCreateModal({ tripGroupId })
  else if (driveIds.length) openCreateModal({ driveIds })
}

watch(
  () => [route.query.new_drive_id, route.query.new_drive_ids, route.query.new_trip_group_id],
  () => checkRouteQueryForCarpool()
)

onMounted(() => {
  loadData()
  checkRouteQueryForCarpool()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white flex items-center gap-2.5">
          <Users class="w-6 h-6 text-rose-500" />
          Covoiturage & BlaBlaCar
        </h2>
        <p class="text-sm text-slate-400">
          Trajets en plusieurs étapes, passagers qui montent et descendent en route : chacun paie sa part des étapes parcourues
        </p>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all self-start sm:self-auto"
      >
        <Plus class="w-4 h-4" />
        Nouveau covoiturage
      </button>
    </div>

    <!-- Viewer mode banner -->
    <div
      v-if="!vehicleStore.canEdit"
      class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex items-center gap-3 text-xs text-slate-400"
    >
      <Users class="w-4 h-4 text-slate-400 shrink-0" />
      <span>Vous consultez ce véhicule en mode <strong>Lecteur seul</strong>. La création et modification de covoiturages sont désactivées.</span>
    </div>

    <!-- KPI Summary Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 sm:gap-4">
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Trajets covoiturés</span>
          <div class="p-2 bg-slate-800 rounded-xl text-slate-300"><Users class="w-4 h-4" /></div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ summary.total_trips }}</span>
          <span class="text-xs text-slate-400">voyages</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">{{ Math.round(summary.total_distance_km || 0).toLocaleString('fr-FR') }} km partagés</div>
      </div>

      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Passagers transportés</span>
          <div class="p-2 bg-blue-500/10 rounded-xl text-blue-400"><Users class="w-4 h-4" /></div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-blue-400">{{ summary.total_passengers }}</span>
          <span class="text-xs text-slate-400">personnes</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">Part équitable due : {{ fmt(summary.total_passengers_share) }} €</div>
      </div>

      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Total perçu passagers</span>
          <div class="p-2 bg-emerald-500/10 rounded-xl text-emerald-400"><CreditCard class="w-4 h-4" /></div>
        </div>
        <div class="mt-2"><span class="text-2xl font-bold text-emerald-400">{{ fmt(summary.total_revenue) }} €</span></div>
        <div class="mt-1 text-[11px]" :class="summary.total_revenue >= summary.total_passengers_share ? 'text-emerald-500/80' : 'text-amber-400'">
          {{ summary.total_revenue >= summary.total_passengers_share ? 'Parts des passagers couvertes' : `${fmt(summary.total_passengers_share - summary.total_revenue)} € sous leur part` }}
        </div>
      </div>

      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Taux d'amortissement</span>
          <div class="p-2 bg-rose-500/10 rounded-xl text-rose-400"><TrendingUp class="w-4 h-4" /></div>
        </div>
        <div class="mt-2"><span class="text-2xl font-bold text-rose-400">{{ summary.coverage_rate_pct || 0 }} %</span></div>
        <div class="mt-1 text-[11px] text-slate-400">du coût réel total ({{ fmt(summary.total_real_cost) }} €)</div>
      </div>

      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Coût net conducteur</span>
          <div class="p-2 bg-amber-500/10 rounded-xl text-amber-400"><Receipt class="w-4 h-4" /></div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-amber-400">{{ Number(summary.net_cost_per_km || 0).toFixed(3) }} €</span>
          <span class="text-xs text-slate-400">/ km</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">Part équitable conducteur : {{ fmt(summary.total_driver_share) }} €</div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-4 animate-pulse">
      <div v-for="i in 2" :key="i" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 h-48"></div>
    </div>

    <!-- Empty State -->
    <div v-else-if="trips.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center max-w-2xl mx-auto space-y-4">
      <div class="w-16 h-16 bg-rose-500/10 border border-rose-500/20 text-rose-400 rounded-2xl flex items-center justify-center mx-auto">
        <Users class="w-8 h-8" />
      </div>
      <div>
        <h3 class="text-lg font-bold text-white">Aucun trajet covoituré pour le moment</h3>
        <p class="text-sm text-slate-400 mt-1 max-w-md mx-auto">
          Sélectionnez les étapes de votre trajet, indiquez où chaque passager monte et descend : TeslaCost calcule la part réelle de chacun.
        </p>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2.5 rounded-xl inline-flex items-center gap-2 shadow-lg shadow-rose-600/20"
      >
        <Plus class="w-4 h-4" />
        Créer mon premier covoiturage
      </button>
    </div>

    <!-- Trips List -->
    <div v-else class="space-y-4">
      <!-- Batch Selection & Actions Toolbar -->
      <div
        v-if="vehicleStore.canEdit"
        class="flex flex-wrap items-center justify-between gap-3 bg-slate-900 border border-slate-800 px-4 py-3 rounded-2xl shadow-sm"
      >
        <div class="flex items-center gap-3">
          <label
            v-if="vehicleStore.canEdit"
            class="flex items-center gap-2 text-xs font-semibold text-slate-300 hover:text-white cursor-pointer select-none"
          >
            <input
              type="checkbox"
              :checked="isAllSelected"
              @change="toggleSelectAll"
              class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer shrink-0"
            />
            <span>{{ isAllSelected ? 'Tout désélectionner' : 'Tout sélectionner' }} ({{ trips.length }})</span>
          </label>
          <span v-if="selectedTripIds.length > 0" class="text-xs text-rose-400 font-semibold bg-rose-500/10 border border-rose-500/20 px-2.5 py-0.5 rounded-lg">
            {{ selectedTripIds.length }} sélectionné{{ selectedTripIds.length > 1 ? 's' : '' }}
          </span>
        </div>

        <div class="flex items-center gap-2">
          <button
            v-if="selectedTripIds.length > 0"
            type="button"
            @click="handleBatchRecalculate"
            :disabled="recalculating"
            class="px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-md shadow-rose-600/20 disabled:opacity-50 transition-all"
            title="Recalculer les coûts réels des covoiturages sélectionnés"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': recalculating }" />
            <span>Recalculer les coûts réels ({{ selectedTripIds.length }})</span>
          </button>
          <button
            v-if="selectedTripIds.length > 0"
            type="button"
            @click="clearTripSelection"
            class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white text-xs font-medium rounded-xl transition-colors"
          >
            Annuler
          </button>
        </div>
      </div>

      <div
        v-for="trip in trips"
        :key="trip.id"
        class="bg-slate-900 border rounded-2xl p-5 transition-all shadow-sm space-y-4"
        :class="selectedTripIds.includes(trip.id) ? 'border-rose-500/50 bg-rose-500/[0.02]' : 'border-slate-800 hover:border-slate-700'"
      >
        <!-- Trip Header -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
          <div class="flex items-start gap-3 min-w-0 flex-1">
            <!-- Select Checkbox -->
            <input
              v-if="vehicleStore.canEdit"
              type="checkbox"
              :checked="selectedTripIds.includes(trip.id)"
              @change="toggleTripSelection(trip.id)"
              class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer mt-1 shrink-0"
              title="Sélectionner ce covoiturage"
            />

            <div class="space-y-1 min-w-0 flex-1">
              <div class="flex items-center gap-2.5 flex-wrap min-w-0">
                <h3 class="text-base font-bold text-white truncate max-w-sm sm:max-w-md" :title="trip.title">{{ trip.title }}</h3>
                <span class="text-xs bg-slate-800 text-slate-300 px-2.5 py-0.5 rounded-full border border-slate-700/60 font-medium shrink-0">{{ trip.distance_km }} km</span>
                <span
                  class="text-xs px-2.5 py-0.5 rounded-full font-semibold border shrink-0"
                  :class="trip.net_cost <= 0 ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
                >
                  {{ trip.net_cost <= 0 ? 'Trajet 100% rentabilisé !' : `Amorti à ${trip.total_cost > 0 ? Math.min(100, Math.round((trip.total_revenue / trip.total_cost) * 100)) : 0}%` }}
                </span>
              </div>
              <div class="flex items-center gap-2 text-xs text-slate-400 min-w-0">
                <Calendar class="w-3.5 h-3.5 shrink-0" />
                <span class="shrink-0">{{ formatDate(trip.date) }}</span>
                <span v-if="trip.notes" class="text-slate-500 truncate">• {{ trip.notes }}</span>
              </div>
            </div>
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1 sm:gap-2 shrink-0 self-end sm:self-auto">
            <button
              @click="handleRecalculateSingle(trip)"
              :disabled="recalculating"
              class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
              title="Recalculer les coûts réels de ce covoiturage"
            >
              <RotateCw class="w-4 h-4" />
            </button>
            <button @click="openEditModal(trip)" class="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors" title="Modifier">
              <Edit2 class="w-4 h-4" />
            </button>
            <button @click="handleDelete(trip)" class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors" title="Supprimer">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Legs -->
        <div class="space-y-2">
          <div class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
            <Navigation class="w-3.5 h-3.5 text-indigo-400" />
            Étapes ({{ trip.legs.length }})
          </div>
          <div class="flex flex-wrap gap-2">
            <div
              v-for="(leg, i) in trip.legs"
              :key="leg.id"
              class="bg-slate-950/60 border border-slate-800/80 rounded-xl px-2.5 py-1.5 text-[11px] text-slate-300"
            >
              <div class="font-semibold text-slate-200">{{ stopNames(trip.legs)[i] }} → {{ stopNames(trip.legs)[i + 1] }}</div>
              <div class="text-slate-400">
                {{ leg.distance_km }} km • {{ fmt(leg.total_cost) }} € •
                {{ 1 + leg.passenger_seats }} à bord • {{ fmt(leg.cost_per_person) }} €/pers.
              </div>
            </div>
          </div>
        </div>

        <!-- Passengers -->
        <div class="space-y-2">
          <div class="text-xs font-semibold text-slate-300 flex items-center justify-between">
            <span class="flex items-center gap-1.5">
              <Users class="w-3.5 h-3.5 text-blue-400" />
              Passagers ({{ trip.passengers?.length || 0 }})
            </span>
            <span class="text-emerald-400 font-bold">Total perçu : +{{ fmt(trip.total_revenue) }} €</span>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5">
            <div v-for="p in trip.passengers" :key="p.id" class="bg-slate-950/60 border border-slate-800/80 rounded-xl p-2.5 text-xs space-y-1">
              <div class="flex items-center justify-between gap-2">
                <span class="font-semibold text-slate-200 truncate">{{ p.passenger_name }}</span>
                <span class="font-bold text-emerald-400 shrink-0">+{{ fmt(p.amount_paid) }} €</span>
              </div>
              <div class="text-[11px] text-slate-400 flex items-center gap-1 truncate">
                <MapPin class="w-3 h-3 shrink-0 text-slate-500" />
                <span class="truncate">
                  {{ stopNames(trip.legs)[p.board_stop_index] }} → {{ stopNames(trip.legs)[p.alight_stop_index] }}
                  • {{ p.seats }} place{{ p.seats > 1 ? 's' : '' }}
                </span>
              </div>
              <div class="flex items-center justify-between text-[11px]">
                <span class="text-slate-400">Part : {{ fmt(p.cost_share) }} €</span>
                <span :class="p.balance >= 0 ? 'text-emerald-400' : 'text-amber-400'">
                  {{ p.balance >= 0 ? `+${fmt(p.balance)} € au-dessus` : `${fmt(-p.balance)} € sous sa part` }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Cost Breakdown -->
        <div class="pt-3 border-t border-slate-800/60 space-y-2">
          <div class="text-xs font-semibold text-slate-400">Coûts réels du trajet :</div>
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Zap class="w-3.5 h-3.5 text-amber-400" /> Électricité : <strong>{{ fmt(trip.electricity_cost) }} €</strong>
            </div>
            <div v-if="trip.tolls_cost > 0" class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <CreditCard class="w-3.5 h-3.5 text-blue-400" /> Péages : <strong>{{ fmt(trip.tolls_cost) }} €</strong>
            </div>
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Disc class="w-3.5 h-3.5 text-rose-400" /> Usure pneus : <strong>{{ fmt(trip.tires_cost) }} €</strong>
            </div>
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Wrench class="w-3.5 h-3.5 text-indigo-400" /> Entretien : <strong>{{ fmt(trip.maintenance_cost) }} €</strong>
            </div>
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Shield class="w-3.5 h-3.5 text-emerald-400" /> Assurance : <strong>{{ fmt(trip.insurance_cost) }} €</strong>
            </div>
            <div v-if="trip.other_cost > 0" class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Receipt class="w-3.5 h-3.5 text-slate-400" /> Divers : <strong>{{ fmt(trip.other_cost) }} €</strong>
            </div>
          </div>
        </div>

        <!-- Bottom Line -->
        <div
          class="rounded-xl p-3 flex flex-col lg:flex-row lg:items-center justify-between gap-3 text-xs"
          :class="trip.net_cost <= 0 ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-300' : 'bg-slate-950/80 border border-slate-800 text-slate-300'"
        >
          <div class="flex items-center gap-2 flex-wrap">
            <CheckCircle2 v-if="trip.net_cost <= 0" class="w-4 h-4 text-emerald-400 shrink-0" />
            <Sparkles v-else class="w-4 h-4 text-amber-400 shrink-0" />
            <span>Coût réel : <strong>{{ fmt(trip.total_cost) }} €</strong></span>
            <span>•</span>
            <span>Part des passagers : <strong>{{ fmt(trip.passengers_cost_share) }} €</strong></span>
            <span>•</span>
            <span>Part du conducteur : <strong>{{ fmt(trip.driver_cost_share) }} €</strong></span>
          </div>
          <div>
            <template v-if="trip.net_cost > 0">
              Reste à charge conducteur : <strong class="text-white text-sm">{{ fmt(trip.net_cost) }} €</strong>
              <span v-if="trip.distance_km > 0" class="text-slate-400 text-[11px] ml-1">({{ (trip.net_cost / trip.distance_km).toFixed(3) }} €/km)</span>
            </template>
            <span v-else class="font-bold text-emerald-400 text-sm">Excédent net : +{{ fmt(Math.abs(trip.net_cost)) }} €</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Create / Edit Modal -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-4xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Users class="w-5 h-5 text-rose-400" />
            {{ editingTripId ? 'Modifier le covoiturage' : 'Nouveau covoiturage' }}
          </h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

        <!-- Source -->
        <div class="space-y-3">
          <div class="flex items-center gap-1 bg-slate-950 border border-slate-800 p-1 rounded-xl w-fit text-xs font-semibold">
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
                <button type="button" @click="applyFairPrice(index)" class="text-indigo-400 hover:text-indigo-300 font-semibold">Appliquer la part équitable</button>
                <button v-if="form.passengers.length > 1" type="button" @click="removePassenger(index)" class="text-slate-500 hover:text-rose-400" title="Retirer ce passager">
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </span>
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
            @click="showModal = false"
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
  </div>
</template>
