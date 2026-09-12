<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
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
  AlertCircle,
  X,
  ArrowRight,
  Receipt,
  Sparkles,
  HelpCircle,
  Layers,
  CheckSquare,
  Square,
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const route = useRoute()
const router = useRouter()

const loading = ref(true)
const trips = ref<any[]>([])
const summary = ref<any>({
  total_trips: 0,
  total_passengers: 0,
  total_distance_km: 0,
  total_real_cost: 0,
  total_revenue: 0,
  total_net_cost: 0,
  total_saved: 0,
  coverage_rate_pct: 0,
  net_cost_per_km: 0,
})

// Modal state
const showModal = ref(false)
const editingTripId = ref<string | null>(null)
const modalSubmitting = ref(false)
const estimating = ref(false)

// Recent drives for pre-selection
const recentDrives = ref<any[]>([])
const selectedDriveId = ref<string>('')

// Form state
const form = ref({
  title: '',
  date: new Date().toISOString().substring(0, 10),
  distance_km: 0,
  drive_id: null as string | null,
  trip_group_id: null as string | null,
  electricity_cost: 0,
  tolls_cost: 0,
  tires_cost: 0,
  maintenance_cost: 0,
  insurance_cost: 0,
  other_cost: 0,
  notes: '',
  passengers: [] as Array<{
    passenger_name: string
    origin: string
    destination: string
    seats: number
    amount_paid: number
    notes: string
  }>,
})

// Unit rates retrieved during estimation
const currentRates = ref<any>(null)

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
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { limit: 30 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

// Live calculations inside form
const liveTotalCost = computed(() => {
  return (
    Number(form.value.electricity_cost || 0) +
    Number(form.value.tolls_cost || 0) +
    Number(form.value.tires_cost || 0) +
    Number(form.value.maintenance_cost || 0) +
    Number(form.value.insurance_cost || 0) +
    Number(form.value.other_cost || 0)
  )
})

const liveTotalRevenue = computed(() => {
  return form.value.passengers.reduce((sum, p) => sum + Number(p.amount_paid || 0), 0)
})

const liveNetCost = computed(() => {
  return liveTotalCost.value - liveTotalRevenue.value
})

const liveCoveragePct = computed(() => {
  if (liveTotalCost.value <= 0) return 0
  return Math.min(100, Math.round((liveTotalRevenue.value / liveTotalCost.value) * 1000) / 10)
})

const liveNetCostPerKm = computed(() => {
  if (form.value.distance_km <= 0) return 0
  return Math.round((liveNetCost.value / form.value.distance_km) * 1000) / 1000
})

const sourceMode = ref<'SINGLE' | 'MULTI' | 'MANUAL'>('SINGLE')
const selectedMultiDriveIds = ref<string[]>([])
const multiSteps = computed(() => {
  return recentDrives.value
    .filter((d) => selectedMultiDriveIds.value.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
})

function openCreateModal(preselectedDriveId?: string) {
  editingTripId.value = null
  selectedDriveId.value = preselectedDriveId || ''
  selectedMultiDriveIds.value = []
  sourceMode.value = preselectedDriveId ? 'SINGLE' : 'SINGLE'
  form.value = {
    title: '',
    date: new Date().toISOString().substring(0, 10),
    distance_km: 0,
    drive_id: preselectedDriveId || null,
    trip_group_id: null,
    electricity_cost: 0,
    tolls_cost: 0,
    tires_cost: 0,
    maintenance_cost: 0,
    insurance_cost: 0,
    other_cost: 0,
    notes: '',
    passengers: [
      {
        passenger_name: 'Passager 1 (BlaBlaCar)',
        origin: '',
        destination: '',
        seats: 1,
        amount_paid: 0,
        notes: '',
      },
    ],
  }
  showModal.value = true
  loadRecentDrives()

  if (preselectedDriveId) {
    onSelectDrive(preselectedDriveId)
  }
}

async function openCreateModalForTripGroup(tripGroupId: string) {
  editingTripId.value = null
  selectedDriveId.value = ''
  selectedMultiDriveIds.value = []
  sourceMode.value = 'MULTI'
  form.value = {
    title: 'Voyage multi-étapes',
    date: new Date().toISOString().substring(0, 10),
    distance_km: 0,
    drive_id: null,
    trip_group_id: tripGroupId,
    electricity_cost: 0,
    tolls_cost: 0,
    tires_cost: 0,
    maintenance_cost: 0,
    insurance_cost: 0,
    other_cost: 0,
    notes: 'Voyage regroupant plusieurs trajets TeslaMate',
    passengers: [
      {
        passenger_name: 'Passager 1 (BlaBlaCar)',
        origin: '',
        destination: '',
        seats: 1,
        amount_paid: 0,
        notes: '',
      },
    ],
  }
  showModal.value = true
  await loadRecentDrives()

  if (!vehicleStore.activeVehicle) return
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, { trip_group_id: tripGroupId })
    currentRates.value = est
    form.value.distance_km = est.distance_km
    form.value.electricity_cost = est.electricity_cost
    form.value.tolls_cost = est.tolls_cost
    form.value.tires_cost = est.tires_cost
    form.value.maintenance_cost = est.maintenance_cost
    form.value.insurance_cost = est.insurance_cost

    const groups = await api.getTripGroups(vehicleStore.activeVehicle.id)
    const g = (groups || []).find((grp: any) => grp.id === tripGroupId)
    if (g && g.name) {
      form.value.title = g.name
    }
  } catch (err) {
    console.error('Failed to estimate costs for trip group', err)
  } finally {
    estimating.value = false
  }
}

async function toggleMultiDrive(driveId: string) {
  const idx = selectedMultiDriveIds.value.indexOf(driveId)
  if (idx > -1) {
    selectedMultiDriveIds.value.splice(idx, 1)
  } else {
    selectedMultiDriveIds.value.push(driveId)
  }

  if (!selectedMultiDriveIds.value.length) {
    form.value.distance_km = 0
    form.value.electricity_cost = 0
    form.value.tolls_cost = 0
    form.value.tires_cost = 0
    form.value.maintenance_cost = 0
    form.value.insurance_cost = 0
    return
  }

  const steps = recentDrives.value
    .filter((d) => selectedMultiDriveIds.value.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())

  if (steps.length > 0) {
    const firstCity = (steps[0].start_address || 'Départ').split(',')[0]
    const lastCity = (steps[steps.length - 1].end_address || 'Arrivée').split(',')[0]
    form.value.title = `${firstCity} → ${lastCity} (${steps.length} étapes)`
    form.value.date = new Date(steps[0].start_time).toISOString().substring(0, 10)
  }

  if (!vehicleStore.activeVehicle) return
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, {
      drive_ids: selectedMultiDriveIds.value,
    })
    currentRates.value = est
    form.value.distance_km = est.distance_km
    form.value.electricity_cost = est.electricity_cost
    form.value.tolls_cost = est.tolls_cost
    form.value.tires_cost = est.tires_cost
    form.value.maintenance_cost = est.maintenance_cost
    form.value.insurance_cost = est.insurance_cost
  } catch (err) {
    console.error('Failed to estimate costs for multi drives', err)
  } finally {
    estimating.value = false
  }
}

function openEditModal(trip: any) {
  editingTripId.value = trip.id
  selectedDriveId.value = trip.drive_id || ''
  selectedMultiDriveIds.value = []
  sourceMode.value = trip.trip_group_id ? 'MULTI' : trip.drive_id ? 'SINGLE' : 'MANUAL'
  form.value = {
    title: trip.title,
    date: new Date(trip.date).toISOString().substring(0, 10),
    distance_km: trip.distance_km,
    drive_id: trip.drive_id || null,
    trip_group_id: trip.trip_group_id || null,
    electricity_cost: trip.electricity_cost,
    tolls_cost: trip.tolls_cost,
    tires_cost: trip.tires_cost,
    maintenance_cost: trip.maintenance_cost,
    insurance_cost: trip.insurance_cost,
    other_cost: trip.other_cost,
    notes: trip.notes || '',
    passengers: (trip.passengers || []).map((p: any) => ({
      passenger_name: p.passenger_name,
      origin: p.origin || '',
      destination: p.destination || '',
      seats: p.seats || 1,
      amount_paid: p.amount_paid || 0,
      notes: p.notes || '',
    })),
  }
  if (form.value.passengers.length === 0) {
    addPassenger()
  }
  showModal.value = true
}

async function onSelectDrive(driveId: string) {
  if (!driveId || !vehicleStore.activeVehicle) return
  estimating.value = true
  try {
    const drive = recentDrives.value.find((d) => d.id === driveId)
    if (drive) {
      const from = drive.start_address ? drive.start_address.split(',')[0] : 'Départ'
      const to = drive.end_address ? drive.end_address.split(',')[0] : 'Arrivée'
      form.value.title = `${from} → ${to}`
      form.value.date = new Date(drive.start_time).toISOString().substring(0, 10)
      form.value.distance_km = drive.distance_km
      form.value.drive_id = drive.id
    }

    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, { drive_id: driveId })
    currentRates.value = est
    form.value.distance_km = est.distance_km
    form.value.electricity_cost = est.electricity_cost
    form.value.tolls_cost = est.tolls_cost
    form.value.tires_cost = est.tires_cost
    form.value.maintenance_cost = est.maintenance_cost
    form.value.insurance_cost = est.insurance_cost
  } catch (err) {
    console.error('Failed to estimate costs for drive', err)
  } finally {
    estimating.value = false
  }
}

async function reestimateFromDistance() {
  if (!vehicleStore.activeVehicle || form.value.distance_km <= 0) return
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(vehicleStore.activeVehicle.id, {
      distance_km: Number(form.value.distance_km),
    })
    currentRates.value = est
    form.value.electricity_cost = est.electricity_cost
    form.value.tires_cost = est.tires_cost
    form.value.maintenance_cost = est.maintenance_cost
    form.value.insurance_cost = est.insurance_cost
  } catch (err) {
    console.error('Failed to estimate costs from distance', err)
  } finally {
    estimating.value = false
  }
}

function addPassenger() {
  form.value.passengers.push({
    passenger_name: `Passager ${form.value.passengers.length + 1}`,
    origin: '',
    destination: '',
    seats: 1,
    amount_paid: 0,
    notes: '',
  })
}

function removePassenger(index: number) {
  form.value.passengers.splice(index, 1)
}

async function handleSave() {
  if (!vehicleStore.activeVehicle) return
  if (!form.value.title.trim()) {
    alert('Veuillez indiquer un titre ou trajet (ex: Paris → Lyon)')
    return
  }

  modalSubmitting.value = true
  try {
    // If multi-drives selected directly without existing trip_group_id, create trip group first
    if (sourceMode.value === 'MULTI' && !form.value.trip_group_id && selectedMultiDriveIds.value.length > 1) {
      const tg = await api.createTripGroup(vehicleStore.activeVehicle.id, {
        name: form.value.title,
        drive_ids: selectedMultiDriveIds.value,
      })
      form.value.trip_group_id = tg.id
      form.value.drive_id = null
    }

    const payload = {
      ...form.value,
      date: new Date(form.value.date).toISOString(),
    }

    if (editingTripId.value) {
      await api.updateCarpool(vehicleStore.activeVehicle.id, editingTripId.value, payload)
    } else {
      await api.createCarpool(vehicleStore.activeVehicle.id, payload)
    }

    showModal.value = false
    await loadData()
  } catch (err: any) {
    alert(`Erreur lors de l'enregistrement : ${err.message}`)
  } finally {
    modalSubmitting.value = false
  }
}

async function handleDelete(trip: any) {
  if (!vehicleStore.activeVehicle) return
  if (!confirm(`Confirmez-vous la suppression du covoiturage "${trip.title}" ?`)) return

  try {
    await api.deleteCarpool(vehicleStore.activeVehicle.id, trip.id)
    await loadData()
  } catch (err: any) {
    alert(`Erreur lors de la suppression : ${err.message}`)
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

watch(
  () => [vehicleStore.activeVehicleId, vehicleStore.lastSyncTimestamp],
  () => {
    loadData()
  }
)

onMounted(() => {
  loadData()
  const newDriveId = route.query.new_drive_id as string
  const newTripGroupId = route.query.new_trip_group_id as string
  if (newDriveId) {
    openCreateModal(newDriveId)
  } else if (newTripGroupId) {
    openCreateModalForTripGroup(newTripGroupId)
  }
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
          Suivi financier précis par trajet • Recettes reçues, usure réelle, électricité, péages et amortissement des frais
        </p>
      </div>

      <button
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all self-start sm:self-auto"
      >
        <Plus class="w-4 h-4" />
        Nouveau covoiturage
      </button>
    </div>

    <!-- KPI Summary Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
      <!-- Trajets partagés -->
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Trajets covoiturés</span>
          <div class="p-2 bg-slate-800 rounded-xl text-slate-300">
            <Users class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-white">{{ summary.total_trips }}</span>
          <span class="text-xs text-slate-400">voyages</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">
          {{ Math.round(summary.total_distance_km || 0).toLocaleString('fr-FR') }} km partagés
        </div>
      </div>

      <!-- Passagers transportés -->
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Passagers transportés</span>
          <div class="p-2 bg-blue-500/10 rounded-xl text-blue-400">
            <Users class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-blue-400">{{ summary.total_passengers }}</span>
          <span class="text-xs text-slate-400">personnes</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">
          BlaBlaCar & directs
        </div>
      </div>

      <!-- Recettes perçues -->
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Total perçu passagers</span>
          <div class="p-2 bg-emerald-500/10 rounded-xl text-emerald-400">
            <CreditCard class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-emerald-400">{{ Number(summary.total_revenue || 0).toFixed(2) }} €</span>
        </div>
        <div class="mt-1 text-[11px] text-emerald-500/80">
          Argent directement encaissé
        </div>
      </div>

      <!-- Taux d'amortissement / Économies -->
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Taux d'amortissement</span>
          <div class="p-2 bg-rose-500/10 rounded-xl text-rose-400">
            <TrendingUp class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-rose-400">{{ summary.coverage_rate_pct || 0 }} %</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">
          {{ Number(summary.total_saved || 0).toFixed(2) }} € économisés sur les coûts réels
        </div>
      </div>

      <!-- Coût net conducteur au km -->
      <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">Coût net conducteur</span>
          <div class="p-2 bg-amber-500/10 rounded-xl text-amber-400">
            <Receipt class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-amber-400">{{ Number(summary.net_cost_per_km || 0).toFixed(3) }} €</span>
          <span class="text-xs text-slate-400">/ km</span>
        </div>
        <div class="mt-1 text-[11px] text-slate-400">
          Reste à charge réel
        </div>
      </div>
    </div>

    <!-- SKELETON LOADING STATE -->
    <div v-if="loading" class="space-y-6 animate-pulse">
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
        <div v-for="i in 5" :key="i" class="bg-slate-900 border border-slate-800 p-4 rounded-2xl h-24 flex flex-col justify-between">
          <div class="h-3 w-20 bg-slate-800 rounded"></div>
          <div class="h-6 w-24 bg-slate-800 rounded"></div>
        </div>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-3xl p-6 h-64 flex flex-col justify-between">
        <div class="h-4 w-40 bg-slate-800 rounded"></div>
        <div class="space-y-3">
          <div v-for="j in 3" :key="j" class="h-12 bg-slate-800/50 rounded-xl"></div>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-else-if="trips.length === 0"
      class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center max-w-2xl mx-auto space-y-4"
    >
      <div class="w-16 h-16 bg-rose-500/10 border border-rose-500/20 text-rose-400 rounded-2xl flex items-center justify-center mx-auto">
        <Users class="w-8 h-8" />
      </div>
      <div>
        <h3 class="text-lg font-bold text-white">Aucun trajet covoituré pour le moment</h3>
        <p class="text-sm text-slate-400 mt-1 max-w-md mx-auto">
          Enregistrez vos trajets BlaBlaCar pour découvrir le coût de revient exact de vos voyages (électricité, usure pneus, péages, assurance) et combien vos passagers vous ont fait économiser !
        </p>
      </div>
      <button
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2.5 rounded-xl inline-flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all"
      >
        <Plus class="w-4 h-4" />
        Créer mon premier covoiturage
      </button>
    </div>

    <!-- Trips List -->
    <div v-else class="space-y-4">
      <div
        v-for="trip in trips"
        :key="trip.id"
        class="bg-slate-900 border border-slate-800 rounded-2xl p-5 hover:border-slate-700 transition-all shadow-sm space-y-4"
      >
        <!-- Trip Header -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
          <div class="space-y-1">
            <div class="flex items-center gap-2.5 flex-wrap">
              <h3 class="text-base font-bold text-white">{{ trip.title }}</h3>
              <span class="text-xs bg-slate-800 text-slate-300 px-2.5 py-0.5 rounded-full border border-slate-700/60 font-medium">
                {{ trip.distance_km }} km
              </span>
              <span
                class="text-xs px-2.5 py-0.5 rounded-full font-semibold border"
                :class="
                  trip.net_cost <= 0
                    ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                    : 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                "
              >
                {{ trip.net_cost <= 0 ? 'Trajet 100% rentabilisé !' : `Amorti à ${Math.min(100, Math.round((trip.total_revenue / trip.total_cost) * 100))}%` }}
              </span>
            </div>
            <div class="flex items-center gap-2 text-xs text-slate-400">
              <Calendar class="w-3.5 h-3.5" />
              <span>{{ formatDate(trip.date) }}</span>
              <span v-if="trip.notes" class="text-slate-500">• {{ trip.notes }}</span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-2 self-end sm:self-auto">
            <button
              @click="openEditModal(trip)"
              class="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
              title="Modifier"
            >
              <Edit2 class="w-4 h-4" />
            </button>
            <button
              @click="handleDelete(trip)"
              class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
              title="Supprimer"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Passengers & Bouts de trajet -->
        <div class="space-y-2">
          <div class="text-xs font-semibold text-slate-300 flex items-center justify-between">
            <span class="flex items-center gap-1.5">
              <Users class="w-3.5 h-3.5 text-blue-400" />
              Passagers & Tronçons ({{ trip.passengers?.length || 0 }})
            </span>
            <span class="text-emerald-400 font-bold">Total perçu : +{{ Number(trip.total_revenue).toFixed(2) }} €</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5">
            <div
              v-for="p in trip.passengers"
              :key="p.id"
              class="bg-slate-950/60 border border-slate-800/80 rounded-xl p-2.5 flex items-center justify-between text-xs"
            >
              <div class="space-y-0.5 truncate pr-2">
                <div class="font-semibold text-slate-200 truncate">{{ p.passenger_name }}</div>
                <div class="text-[11px] text-slate-400 flex items-center gap-1 truncate">
                  <MapPin class="w-3 h-3 shrink-0 text-slate-500" />
                  <span class="truncate">
                    {{ p.origin || 'Départ' }} → {{ p.destination || 'Arrivée' }}
                  </span>
                </div>
              </div>
              <div class="text-right shrink-0">
                <div class="font-bold text-emerald-400">+{{ Number(p.amount_paid).toFixed(2) }} €</div>
                <div class="text-[10px] text-slate-500">{{ p.seats }} place{{ p.seats > 1 ? 's' : '' }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Cost Breakdown Badges -->
        <div class="pt-3 border-t border-slate-800/60 space-y-2">
          <div class="text-xs font-semibold text-slate-400">Coûts réels de revient du trajet :</div>
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <!-- Electricity -->
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Zap class="w-3.5 h-3.5 text-amber-400" />
              <span>Électricité : <strong>{{ Number(trip.electricity_cost).toFixed(2) }} €</strong></span>
            </div>

            <!-- Tolls -->
            <div
              v-if="trip.tolls_cost > 0"
              class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300"
            >
              <CreditCard class="w-3.5 h-3.5 text-blue-400" />
              <span>Péages : <strong>{{ Number(trip.tolls_cost).toFixed(2) }} €</strong></span>
            </div>

            <!-- Tires -->
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Disc class="w-3.5 h-3.5 text-rose-400" />
              <span>Usure pneus : <strong>{{ Number(trip.tires_cost).toFixed(2) }} €</strong></span>
            </div>

            <!-- Maintenance -->
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Wrench class="w-3.5 h-3.5 text-indigo-400" />
              <span>Entretien : <strong>{{ Number(trip.maintenance_cost).toFixed(2) }} €</strong></span>
            </div>

            <!-- Insurance -->
            <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
              <Shield class="w-3.5 h-3.5 text-emerald-400" />
              <span>Assurance : <strong>{{ Number(trip.insurance_cost).toFixed(2) }} €</strong></span>
            </div>

            <!-- Other -->
            <div
              v-if="trip.other_cost > 0"
              class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300"
            >
              <Receipt class="w-3.5 h-3.5 text-slate-400" />
              <span>Divers : <strong>{{ Number(trip.other_cost).toFixed(2) }} €</strong></span>
            </div>
          </div>
        </div>

        <!-- Financial Bottom Line Banner -->
        <div
          class="rounded-xl p-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs"
          :class="
            trip.net_cost <= 0
              ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-300'
              : 'bg-slate-950/80 border border-slate-800 text-slate-300'
          "
        >
          <div class="flex items-center gap-2">
            <CheckCircle2 v-if="trip.net_cost <= 0" class="w-4 h-4 text-emerald-400 shrink-0" />
            <Sparkles v-else class="w-4 h-4 text-amber-400 shrink-0" />
            <div>
              <span>Coût réel total : <strong>{{ Number(trip.total_cost).toFixed(2) }} €</strong></span>
              <span class="mx-2">•</span>
              <span>Total perçu : <strong class="text-emerald-400">+{{ Number(trip.total_revenue).toFixed(2) }} €</strong></span>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <div v-if="trip.net_cost > 0">
              Reste à charge conducteur : <strong class="text-white text-sm">{{ Number(trip.net_cost).toFixed(2) }} €</strong>
              <span class="text-slate-400 text-[11px] ml-1">({{ (trip.net_cost / trip.distance_km).toFixed(3) }} €/km)</span>
            </div>
            <div v-else class="font-bold text-emerald-400 text-sm">
              Excédent net : +{{ Math.abs(trip.net_cost).toFixed(2) }} €
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create / Edit Carpool Modal -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-2xl w-full p-6 space-y-6 max-h-[90vh] overflow-y-auto shadow-2xl">
        <!-- Modal Header -->
        <div class="flex items-center justify-between pb-4 border-b border-slate-800">
          <div>
            <h3 class="text-lg font-bold text-white flex items-center gap-2">
              <Users class="w-5 h-5 text-rose-500" />
              {{ editingTripId ? 'Modifier le covoiturage' : 'Nouveau trajet covoituré' }}
            </h3>
            <p class="text-xs text-slate-400">Renseignez le trajet, ses passagers et ses frais réels</p>
          </div>
          <button @click="showModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Trip Origin Selection (Single Drive, Multi-Drive Journey, or Manual) -->
        <div v-if="!editingTripId" class="bg-slate-950/60 border border-slate-800 p-3.5 rounded-2xl space-y-3">
          <div class="flex items-center justify-between flex-wrap gap-2">
            <label class="text-xs font-semibold text-slate-300">Origine des données du trajet :</label>
            <!-- Tabs -->
            <div class="flex items-center gap-1 bg-slate-900 border border-slate-800 p-0.5 rounded-lg text-xs">
              <button
                type="button"
                @click="sourceMode = 'SINGLE'"
                class="px-2.5 py-1 rounded-md transition-all font-semibold"
                :class="sourceMode === 'SINGLE' ? 'bg-rose-600 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
              >
                Trajet unique
              </button>
              <button
                type="button"
                @click="sourceMode = 'MULTI'"
                class="px-2.5 py-1 rounded-md transition-all font-semibold flex items-center gap-1"
                :class="sourceMode === 'MULTI' ? 'bg-rose-600 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
              >
                <Layers class="w-3 h-3" />
                Multi-étapes (arrêts)
              </button>
              <button
                type="button"
                @click="sourceMode = 'MANUAL'"
                class="px-2.5 py-1 rounded-md transition-all font-semibold"
                :class="sourceMode === 'MANUAL' ? 'bg-rose-600 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
              >
                Saisie libre
              </button>
            </div>
          </div>

          <!-- Mode 1: Single Drive -->
          <div v-if="sourceMode === 'SINGLE'" class="space-y-2">
            <select
              v-model="selectedDriveId"
              @change="onSelectDrive(selectedDriveId)"
              class="w-full bg-slate-900 text-slate-200 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            >
              <option value="">-- Choisir un trajet récent dans la liste --</option>
              <option v-for="d in recentDrives" :key="d.id" :value="d.id">
                {{ formatDate(d.start_time) }} : {{ d.start_address?.split(',')[0] || 'Départ' }} → {{ d.end_address?.split(',')[0] || 'Arrivée' }} ({{ d.distance_km }} km, {{ d.energy_consumed_kwh ? d.energy_consumed_kwh + ' kWh' : '' }})
              </option>
            </select>
          </div>

          <!-- Mode 2: Multi-stage Drive (Arrêts recharge / pauses) -->
          <div v-else-if="sourceMode === 'MULTI'" class="space-y-2.5">
            <p class="text-[11px] text-slate-400">
              Cochez les trajets consécutifs composant votre voyage (ex: Paris → Beaune puis Beaune → Lyon) :
            </p>
            <div class="max-h-44 overflow-y-auto space-y-1.5 pr-1">
              <div
                v-for="d in recentDrives"
                :key="d.id"
                @click="toggleMultiDrive(d.id)"
                class="flex items-center justify-between p-2 rounded-xl text-xs cursor-pointer border transition-all"
                :class="selectedMultiDriveIds.includes(d.id) ? 'bg-rose-500/15 border-rose-500/40 text-white' : 'bg-slate-900/60 border-slate-800 text-slate-300 hover:bg-slate-800/60'"
              >
                <div class="flex items-center gap-2 truncate pr-2">
                  <component :is="selectedMultiDriveIds.includes(d.id) ? CheckSquare : Square" class="w-4 h-4 text-rose-400 shrink-0" />
                  <span class="text-slate-400 font-mono text-[11px]">{{ formatDate(d.start_time) }}</span>
                  <span class="truncate">{{ d.start_address?.split(',')[0] || 'Départ' }} → {{ d.end_address?.split(',')[0] || 'Arrivée' }}</span>
                </div>
                <div class="shrink-0 text-right">
                  <span class="font-bold text-rose-400">{{ d.distance_km }} km</span>
                  <span v-if="d.energy_consumed_kwh" class="text-sky-400 ml-1.5 font-mono text-[10px]">({{ d.energy_consumed_kwh }} kWh)</span>
                </div>
              </div>
            </div>

            <!-- Steps summary -->
            <div v-if="multiSteps.length" class="bg-slate-900 border border-slate-800 p-2.5 rounded-xl space-y-1.5">
              <div class="text-[11px] font-bold text-rose-400 flex items-center gap-1.5">
                <Layers class="w-3.5 h-3.5" />
                <span>{{ multiSteps.length }} étape(s) combinée(s) : {{ form.distance_km }} km au total</span>
              </div>
              <div class="flex flex-wrap items-center gap-1.5">
                <span
                  v-for="(step, idx) in multiSteps"
                  :key="step.id"
                  class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded-lg text-[10px] text-slate-300 flex items-center gap-1"
                >
                  <span class="text-rose-400 font-bold">Étape {{ idx + 1 }}:</span>
                  <span>{{ step.start_address?.split(',')[0] || 'Dép' }} → {{ step.end_address?.split(',')[0] || 'Arr' }}</span>
                  <span class="text-slate-500">({{ step.distance_km }} km)</span>
                </span>
              </div>
            </div>
          </div>

          <!-- Mode 3: Manual Free Input -->
          <div v-else class="text-[11px] text-slate-400">
            Saisissez manuellement le titre, la distance et vos estimations de frais ci-dessous.
          </div>

          <div v-if="estimating" class="text-[11px] text-rose-400 flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5 animate-spin" />
            <span>Calcul automatique des coûts réels selon les taux de votre Tesla...</span>
          </div>
        </div>

        <!-- Trip Details -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div class="sm:col-span-2">
            <label class="block text-xs font-semibold text-slate-400 mb-1">Titre / Trajet</label>
            <input
              v-model="form.title"
              type="text"
              placeholder="Ex: Paris → Lyon (via Auxerre)"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Date</label>
            <input
              v-model="form.date"
              type="date"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
        </div>

        <!-- Distance & Re-estimate Button -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 items-end">
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Distance parcourue (km)</label>
            <input
              v-model.number="form.distance_km"
              type="number"
              step="0.1"
              min="0"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <button
              type="button"
              @click="reestimateFromDistance"
              :disabled="estimating || form.distance_km <= 0"
              class="w-full bg-slate-800 hover:bg-slate-700 text-rose-400 border border-slate-700 text-xs font-medium px-3 py-2 rounded-xl flex items-center justify-center gap-1.5 transition-colors disabled:opacity-50"
            >
              <Sparkles class="w-3.5 h-3.5" />
              Réestimer coûts réels au km
            </button>
          </div>
        </div>

        <!-- Detailed Real Cost Breakdown (Editable) -->
        <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-2xl space-y-3">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-white flex items-center gap-1.5">
              <Receipt class="w-4 h-4 text-rose-500" />
              Décomposition des coûts réels du véhicule
            </span>
            <span class="text-xs text-slate-300 font-bold">Total coût : {{ liveTotalCost.toFixed(2) }} €</span>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">⚡ Électricité (€)</label>
              <input
                v-model.number="form.electricity_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">🛣️ Péages (€)</label>
              <input
                v-model.number="form.tolls_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">🛞 Usure pneus (€)</label>
              <input
                v-model.number="form.tires_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">🔧 Entretien (€)</label>
              <input
                v-model.number="form.maintenance_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">🛡️ Assurance (€)</label>
              <input
                v-model.number="form.insurance_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">📦 Divers (€)</label>
              <input
                v-model.number="form.other_cost"
                type="number"
                step="0.01"
                min="0"
                class="w-full bg-slate-900 text-slate-100 text-xs rounded-lg px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
              />
            </div>
          </div>
        </div>

        <!-- Passengers / Bouts de trajet section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="text-xs font-bold text-white flex items-center gap-1.5">
              <Users class="w-4 h-4 text-blue-400" />
              Passagers & Tronçons ("Bouts de trajet")
            </label>
            <button
              type="button"
              @click="addPassenger"
              class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              Ajouter un passager
            </button>
          </div>

          <div class="space-y-2.5">
            <div
              v-for="(p, index) in form.passengers"
              :key="index"
              class="bg-slate-950/80 border border-slate-800 p-3 rounded-2xl space-y-2.5"
            >
              <div class="flex items-center justify-between">
                <span class="text-xs font-semibold text-slate-300">Passager #{{ index + 1 }}</span>
                <button
                  v-if="form.passengers.length > 1"
                  type="button"
                  @click="removePassenger(index)"
                  class="text-slate-500 hover:text-rose-400 transition-colors p-1"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-4 gap-2">
                <div class="sm:col-span-2">
                  <input
                    v-model="p.passenger_name"
                    type="text"
                    placeholder="Nom du passager (ex: Sophie - BlaBlaCar)"
                    class="w-full bg-slate-900 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
                  />
                </div>
                <div>
                  <input
                    v-model.number="p.amount_paid"
                    type="number"
                    step="0.5"
                    min="0"
                    placeholder="Montant (€)"
                    class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
                  />
                </div>
                <div>
                  <input
                    v-model.number="p.seats"
                    type="number"
                    min="1"
                    placeholder="Places (1)"
                    class="w-full bg-slate-900 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500"
                  />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-2">
                <input
                  v-model="p.origin"
                  type="text"
                  placeholder="Départ tronçon (ex: Paris)"
                  class="w-full bg-slate-900 text-slate-200 text-[11px] rounded-lg px-2 py-1 border border-slate-800 focus:outline-none focus:border-slate-600"
                />
                <input
                  v-model="p.destination"
                  type="text"
                  placeholder="Arrivée tronçon (ex: Auxerre)"
                  class="w-full bg-slate-900 text-slate-200 text-[11px] rounded-lg px-2 py-1 border border-slate-800 focus:outline-none focus:border-slate-600"
                />
              </div>
            </div>
          </div>
        </div>

        <!-- Real-time Financial Simulation Box -->
        <div class="bg-gradient-to-r from-slate-950 to-slate-900 border border-slate-700/80 p-4 rounded-2xl space-y-2">
          <div class="text-xs font-bold text-slate-300 flex items-center justify-between">
            <span>Bilan financier en direct :</span>
            <span class="text-emerald-400 font-extrabold text-sm">+{{ liveTotalRevenue.toFixed(2) }} € perçus</span>
          </div>

          <div class="flex items-center justify-between text-xs pt-1">
            <span class="text-slate-400">Coût total réel : {{ liveTotalCost.toFixed(2) }} €</span>
            <span class="font-semibold" :class="liveNetCost <= 0 ? 'text-emerald-400' : 'text-rose-400'">
              {{ liveNetCost <= 0 ? `Bénéfice : +${Math.abs(liveNetCost).toFixed(2)} €` : `Reste à charge : ${liveNetCost.toFixed(2)} €` }}
            </span>
          </div>

          <!-- Progress bar -->
          <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
            <div
              class="bg-gradient-to-r from-emerald-500 to-rose-500 h-full rounded-full transition-all duration-300"
              :style="{ width: `${Math.min(100, liveCoveragePct)}%` }"
            ></div>
          </div>

          <div class="flex items-center justify-between text-[11px] text-slate-400">
            <span>Taux de couverture des frais : <strong>{{ liveCoveragePct }}%</strong></span>
            <span v-if="form.distance_km > 0">Coût net : <strong>{{ liveNetCostPerKm.toFixed(3) }} €/km</strong></span>
          </div>
        </div>

        <!-- Notes -->
        <div>
          <label class="block text-xs font-semibold text-slate-400 mb-1">Notes ou commentaires (optionnel)</label>
          <input
            v-model="form.notes"
            type="text"
            placeholder="Ex: Aller-retour week-end, super covoitureurs"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
          <button
            type="button"
            @click="showModal = false"
            class="px-4 py-2 rounded-xl text-xs font-semibold text-slate-400 hover:text-white transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleSave"
            :disabled="modalSubmitting"
            class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2.5 rounded-xl transition-all shadow-lg shadow-rose-600/20 disabled:opacity-50"
          >
            {{ modalSubmitting ? 'Enregistrement...' : editingTripId ? 'Mettre à jour' : 'Créer le covoiturage' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
