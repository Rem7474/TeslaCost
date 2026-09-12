<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import {
  Disc,
  Plus,
  RefreshCw,
  Ruler,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  X,
  Gauge,
  Calendar,
  Layers,
  Sun,
  Snowflake,
  CloudSun,
  History,
  ArrowUpDown,
  Shuffle,
  Trash2,
  Edit2,
  Package,
  Wrench,
  Check,
  ChevronRight,
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const tires = ref<any[]>([])
const loading = ref(false)

// Active tab: 'chassis' (Montés) or 'storage' (Au garage)
const activeTab = ref<'chassis' | 'storage'>('chassis')

// Modals
const showAddTireModal = ref(false)
const showHistoryModal = ref(false)
const showSessionModal = ref(false)
const showLogModal = ref(false)
const showPackSwapModal = ref(false)

// Selected tire for history / session
const selectedTire = ref<any | null>(null)
const selectedTireStats = ref<any | null>(null)
const tireSessions = ref<any[]>([])
const tireLogs = ref<any[]>([])

// Form: Add Tires (Batch / Single)
const addType = ref<'SET_4' | 'SET_4_STORAGE' | 'SET_2_FRONT' | 'SET_2_REAR' | 'SET_2_STORAGE' | 'SINGLE'>('SET_4')
const dimensionPreset = ref('235/40 R19 96W')
const isCustomDimension = ref(false)
const isTotalPrice = ref(true)

const addTireForm = ref({
  brand: 'Michelin',
  model: 'Pilot Sport EV',
  dimension: '235/40 R19 96W',
  season: 'SUMMER',
  purchase_date: new Date().toISOString().split('T')[0],
  total_price: 880,
  unit_price: 220,
  initial_depth_mm: 8.0,
  min_legal_depth_mm: 1.6,
  dot_code: '',
  mounted_odometer: 0,
  accumulated_distance_km: 0,
  estimated_lifespan_km: 45000,
  current_position: 'FL',
})

// Tesla predefined tire dimensions
const teslaDimensionPresets = [
  { group: 'Tesla Model 3', label: '18" Aero — 235/45 R18 98Y', value: '235/45 R18 98Y' },
  { group: 'Tesla Model 3', label: '19" Sport — 235/40 R19 96W', value: '235/40 R19 96W' },
  { group: 'Tesla Model 3', label: '20" Performance — 245/35 R20 95Y', value: '245/35 R20 95Y' },
  { group: 'Tesla Model Y', label: '19" Gemini — 255/45 R19 104W', value: '255/45 R19 104W' },
  { group: 'Tesla Model Y', label: '20" Induction — 255/40 R20 101W', value: '255/40 R20 101W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Av — 255/35 R21 98W', value: '255/35 R21 98W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Ar — 275/35 R21 103W', value: '275/35 R21 103W' },
  { group: 'Tesla Model S', label: '19" Tempest — 255/45 R19 104Y', value: '255/45 R19 104Y' },
  { group: 'Tesla Model S', label: '21" Arachnid Av — 265/35 R21', value: '265/35 R21' },
  { group: 'Tesla Model S', label: '21" Arachnid Ar — 295/30 R21', value: '295/30 R21' },
  { group: 'Tesla Model X', label: '20" Cyberstream — 265/45 R20 / 275/45 R20', value: '265/45 R20' },
  { group: 'Autre', label: 'Dimension personnalisée...', value: 'CUSTOM' },
]

function onDimensionPresetChange() {
  if (dimensionPreset.value === 'CUSTOM') {
    isCustomDimension.value = true
    addTireForm.value.dimension = ''
  } else {
    isCustomDimension.value = false
    addTireForm.value.dimension = dimensionPreset.value
  }
}

// Form: Mount Session (Add / Edit)
const editingSessionId = ref<string | null>(null)
const sessionForm = ref({
  position: 'FL',
  mounted_date: new Date().toISOString().substring(0, 10),
  mounted_odometer: 0,
  is_dismounted: true,
  dismounted_date: new Date().toISOString().substring(0, 10),
  dismounted_odometer: 0,
  distance_km: 0,
  notes: '',
})

// Form: Tread Depth Log
const newLogForm = ref({
  depth_mm: 6.5,
  odometer: 0,
  notes: '',
})

// Form: Seasonal Swap Pack
const packSwapForm = ref({
  odometer: 0,
  tires: {
    FL: '',
    FR: '',
    RL: '',
    RR: '',
  },
})

// Mounted tires mapped by position
const mountedTires = computed(() => {
  const map: Record<string, any> = { FL: null, FR: null, RL: null, RR: null }
  tires.value.forEach((t) => {
    const pos = t.tire.current_position
    if (pos in map) {
      map[pos] = t
    }
  })
  return map
})

const storageTires = computed(() => {
  return tires.value.filter((t) => t.tire.current_position === 'STORAGE')
})

async function loadTires() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    tires.value = await api.getTires(vehicleStore.activeVehicle.id)
    if (vehicleStore.activeVehicle?.current_odometer) {
      const odo = Math.round(vehicleStore.activeVehicle.current_odometer)
      addTireForm.value.mounted_odometer = odo
      newLogForm.value.odometer = odo
      packSwapForm.value.odometer = odo
    }
  } catch (err) {
    console.error('Failed to load tires', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicleId, vehicleStore.lastSyncTimestamp],
  () => {
    loadTires()
  }
)

onMounted(() => {
  loadTires()
})

// Quick rotations
async function handleQuickRotate(mode: 'FRONT_BACK' | 'CROSS') {
  if (!vehicleStore.activeVehicle) return
  const odo = Math.round(vehicleStore.activeVehicle.current_odometer || 0)
  const label = mode === 'FRONT_BACK' ? 'Avant ⇄ Arrière (FL ⇄ RL, FR ⇄ RR)' : 'Croisée (FL ⇄ RR, FR ⇄ RL)'
  if (!confirm(`Confirmez-vous la permutation rapide ${label} à ${odo.toLocaleString('fr-FR')} km ?`)) return

  try {
    await api.quickRotateTires(vehicleStore.activeVehicle.id, {
      mode,
      odometer: odo,
    })
    await loadTires()
  } catch (err: any) {
    alert(`Erreur lors de la permutation : ${err.message}`)
  }
}

// Open pack swap modal
function openPackSwapModal() {
  if (!vehicleStore.activeVehicle) return
  packSwapForm.value.odometer = Math.round(vehicleStore.activeVehicle.current_odometer || 0)
  // Pre-fill with first 4 storage tires if available
  const st = storageTires.value
  packSwapForm.value.tires.FL = st[0]?.tire.id || ''
  packSwapForm.value.tires.FR = st[1]?.tire.id || ''
  packSwapForm.value.tires.RL = st[2]?.tire.id || ''
  packSwapForm.value.tires.RR = st[3]?.tire.id || ''
  showPackSwapModal.value = true
}

async function handlePackSwapSubmit() {
  if (!vehicleStore.activeVehicle) return
  const selectedIDs = [
    packSwapForm.value.tires.FL,
    packSwapForm.value.tires.FR,
    packSwapForm.value.tires.RL,
    packSwapForm.value.tires.RR,
  ].filter(Boolean)

  if (selectedIDs.length !== 4) {
    alert('Veuillez sélectionner 4 pneus distincts du garage pour remplacer les pneus montés.')
    return
  }

  try {
    await api.quickRotateTires(vehicleStore.activeVehicle.id, {
      mode: 'SWAP_PACK',
      odometer: packSwapForm.value.odometer,
      swap_with_pack_tire_ids: selectedIDs,
    })
    showPackSwapModal.value = false
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

// Open Add modal
function openAddModal() {
  if (vehicleStore.activeVehicle?.current_odometer) {
    addTireForm.value.mounted_odometer = Math.round(vehicleStore.activeVehicle.current_odometer)
  }
  showAddTireModal.value = true
}

async function handleCreateTires() {
  if (!vehicleStore.activeVehicle) return
  if (!addTireForm.value.brand || !addTireForm.value.model || !addTireForm.value.dimension) {
    alert('Veuillez renseigner la marque, le modèle et la dimension')
    return
  }

  try {
    if (addType.value === 'SINGLE') {
      await api.createTire(vehicleStore.activeVehicle.id, {
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        purchase_price: addTireForm.value.unit_price,
        current_position: addTireForm.value.current_position,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addTireForm.value.current_position !== 'STORAGE' ? addTireForm.value.mounted_odometer : null,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    } else {
      await api.batchCreateTires(vehicleStore.activeVehicle.id, {
        type: addType.value,
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        total_price: isTotalPrice.value ? addTireForm.value.total_price : 0,
        unit_price: !isTotalPrice.value ? addTireForm.value.unit_price : 0,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addType.value.includes('STORAGE') ? null : addTireForm.value.mounted_odometer,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    }

    showAddTireModal.value = false
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

// History & Timeline Modal
async function openHistoryModal(t: any) {
  selectedTire.value = t.tire
  selectedTireStats.value = t
  try {
    const res = await api.getTireHistory(vehicleStore.activeVehicle!.id, t.tire.id)
    selectedTire.value = res.tire
    selectedTireStats.value = res.stats
    tireSessions.value = res.sessions || []
    tireLogs.value = res.logs || []
    showHistoryModal.value = true
  } catch (err: any) {
    alert(`Erreur de chargement : ${err.message}`)
  }
}

// Open manual session modal (add / edit)
function openAddSessionModal() {
  editingSessionId.value = null
  sessionForm.value = {
    position: 'FL',
    mounted_date: new Date().toISOString().substring(0, 10),
    mounted_odometer: 0,
    is_dismounted: true,
    dismounted_date: new Date().toISOString().substring(0, 10),
    dismounted_odometer: 0,
    distance_km: 0,
    notes: '',
  }
  showSessionModal.value = true
}

function openEditSessionModal(s: any) {
  editingSessionId.value = s.id
  sessionForm.value = {
    position: s.position,
    mounted_date: new Date(s.mounted_date).toISOString().substring(0, 10),
    mounted_odometer: s.mounted_odometer,
    is_dismounted: !!s.dismounted_date,
    dismounted_date: s.dismounted_date ? new Date(s.dismounted_date).toISOString().substring(0, 10) : new Date().toISOString().substring(0, 10),
    dismounted_odometer: s.dismounted_odometer || 0,
    distance_km: s.distance_km || 0,
    notes: s.notes || '',
  }
  showSessionModal.value = true
}

async function handleSaveSession() {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return

  try {
    const payload: any = {
      position: sessionForm.value.position,
      mounted_date: new Date(sessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(sessionForm.value.mounted_odometer),
      distance_km: Number(sessionForm.value.distance_km),
      notes: sessionForm.value.notes ? sessionForm.value.notes : null,
    }

    if (sessionForm.value.is_dismounted) {
      payload.dismounted_date = new Date(sessionForm.value.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(sessionForm.value.dismounted_odometer)
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    if (editingSessionId.value) {
      await api.updateTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, editingSessionId.value, payload)
    } else {
      await api.createTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, payload)
    }

    showSessionModal.value = false
    // Refresh history
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleDeleteSession(session: any) {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  if (!confirm('Confirmez-vous la suppression de cette session de montage ?')) return

  try {
    await api.deleteTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, session.id)
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    alert(`Erreur lors de la suppression : ${err.message}`)
  }
}

// Open log modal (mesure de gomme)
function openLogModal(t: any) {
  selectedTire.value = t.tire
  newLogForm.value.depth_mm = t.current_depth_mm || 6.5
  newLogForm.value.odometer = Math.round(vehicleStore.activeVehicle?.current_odometer || 0)
  showLogModal.value = true
}

async function handleAddLog() {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  try {
    await api.addTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, {
      depth_mm: Number(newLogForm.value.depth_mm),
      odometer: Number(newLogForm.value.odometer),
      notes: newLogForm.value.notes ? newLogForm.value.notes : null,
      date: new Date().toISOString(),
    })
    showLogModal.value = false
    await loadTires()
    if (showHistoryModal.value) {
      await openHistoryModal({ tire: selectedTire.value })
    }
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function getConditionBadge(condition: string) {
  switch (condition) {
    case 'GOOD':
      return { label: 'Bon état', class: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' }
    case 'WARNING':
      return { label: 'À surveiller', class: 'bg-amber-500/10 text-amber-400 border-amber-500/20' }
    case 'CRITICAL':
      return { label: 'Usure critique', class: 'bg-rose-500/10 text-rose-400 border-rose-500/20' }
    default:
      return { label: 'Inconnu', class: 'bg-slate-800 text-slate-400 border-slate-700' }
  }
}

function getSeasonIcon(season: string) {
  switch (season) {
    case 'WINTER':
      return { icon: Snowflake, color: 'text-sky-400', label: 'Hiver' }
    case 'ALL_SEASON':
      return { icon: CloudSun, color: 'text-amber-400', label: '4 Saisons' }
    default:
      return { icon: Sun, color: 'text-orange-400', label: 'Été' }
  }
}

function formatDate(d: string) {
  return new Date(d).toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Actions -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white flex items-center gap-2.5">
          <Disc class="w-6 h-6 text-rose-500" />
          Pneumatiques & Cycles de vie
        </h2>
        <p class="text-sm text-slate-400">
          Suivi de l'usure en mm, durée de vie estimée, permutations en 1 clic et historique complet des montages
        </p>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2 flex-wrap">
        <button
          @click="openPackSwapModal()"
          :disabled="storageTires.length === 0"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2 transition-colors disabled:opacity-40"
          title="Permuter le pack complet monté avec un pack de réserve (ex: Hiver / Été)"
        >
          <Snowflake class="w-4 h-4 text-sky-400" />
          <span class="hidden md:inline">Changer de pack</span>
        </button>

        <button
          @click="openAddModal()"
          class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all"
        >
          <Plus class="w-4 h-4" />
          Ajouter des pneus
        </button>
      </div>
    </div>

    <!-- Quick Permutations Bar -->
    <div class="bg-slate-900 border border-slate-800 p-3 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
      <div class="flex items-center gap-2 text-slate-300 font-semibold">
        <RefreshCw class="w-4 h-4 text-rose-400" />
        <span>Permutations rapides du véhicule en 1 clic :</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button
          @click="handleQuickRotate('FRONT_BACK')"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
        >
          <ArrowUpDown class="w-3.5 h-3.5 text-blue-400" />
          Avant ⇄ Arrière
        </button>
        <button
          @click="handleQuickRotate('CROSS')"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
        >
          <Shuffle class="w-3.5 h-3.5 text-indigo-400" />
          Permutation Croisée
        </button>
      </div>
    </div>

    <!-- View Switcher Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        @click="activeTab = 'chassis'"
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
        :class="
          activeTab === 'chassis'
            ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40'
        "
      >
        <Disc class="w-4 h-4" />
        Pneus montés sur la Tesla (4)
      </button>

      <button
        @click="activeTab = 'storage'"
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
        :class="
          activeTab === 'storage'
            ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40'
        "
      >
        <Package class="w-4 h-4" />
        Catalogue & Stock au garage ({{ storageTires.length }})
      </button>
    </div>

    <!-- TAB 1: CHASSIS INTERACTIF (PNEUS MONTÉS) -->
    <div v-if="activeTab === 'chassis'" class="space-y-6">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- Wheel Card: FL (Avant Gauche) -->
        <div
          v-if="mountedTires.FL"
          class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-3xl p-5 space-y-4 shadow-sm transition-all"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="text-[11px] font-bold uppercase tracking-wider text-rose-400">Avant Gauche (FL)</div>
              <h3 class="text-base font-bold text-white">{{ mountedTires.FL.tire.brand }} {{ mountedTires.FL.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.FL.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.FL.condition).class"
            >
              {{ getConditionBadge(mountedTires.FL.condition).label }}
            </span>
          </div>

          <!-- Metrics Row -->
          <div class="grid grid-cols-3 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Sculpture</div>
              <div class="text-sm font-bold text-white">{{ mountedTires.FL.current_depth_mm }} mm</div>
              <div class="text-[10px] text-slate-400">Témoin: {{ mountedTires.FL.min_legal_depth_mm }} mm</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.FL.total_distance_km).toLocaleString('fr-FR') }} km</div>
              <div class="text-[10px] text-slate-400">Vie: {{ mountedTires.FL.life_progress_pct }}%</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.FL.cost_per_km).toFixed(4) }} €</div>
              <div class="text-[10px] text-slate-400">/ pneu</div>
            </div>
          </div>

          <!-- Lifespan progress bar -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.FL.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.FL.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.FL.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.FL.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center justify-between pt-2 border-t border-slate-800">
            <button
              @click="openLogModal(mountedTires.FL)"
              class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 font-medium transition-colors"
            >
              <Ruler class="w-3.5 h-3.5 text-rose-400" />
              Mesurer gomme
            </button>
            <button
              @click="openHistoryModal(mountedTires.FL)"
              class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
            >
              <History class="w-3.5 h-3.5" />
              Historique & Sessions
            </button>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Avant Gauche (FL)</span>
        </div>

        <!-- Wheel Card: FR (Avant Droit) -->
        <div
          v-if="mountedTires.FR"
          class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-3xl p-5 space-y-4 shadow-sm transition-all"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="text-[11px] font-bold uppercase tracking-wider text-rose-400">Avant Droit (FR)</div>
              <h3 class="text-base font-bold text-white">{{ mountedTires.FR.tire.brand }} {{ mountedTires.FR.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.FR.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.FR.condition).class"
            >
              {{ getConditionBadge(mountedTires.FR.condition).label }}
            </span>
          </div>

          <div class="grid grid-cols-3 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Sculpture</div>
              <div class="text-sm font-bold text-white">{{ mountedTires.FR.current_depth_mm }} mm</div>
              <div class="text-[10px] text-slate-400">Témoin: {{ mountedTires.FR.min_legal_depth_mm }} mm</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.FR.total_distance_km).toLocaleString('fr-FR') }} km</div>
              <div class="text-[10px] text-slate-400">Vie: {{ mountedTires.FR.life_progress_pct }}%</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.FR.cost_per_km).toFixed(4) }} €</div>
              <div class="text-[10px] text-slate-400">/ pneu</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.FR.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.FR.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.FR.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.FR.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-slate-800">
            <button
              @click="openLogModal(mountedTires.FR)"
              class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 font-medium transition-colors"
            >
              <Ruler class="w-3.5 h-3.5 text-rose-400" />
              Mesurer gomme
            </button>
            <button
              @click="openHistoryModal(mountedTires.FR)"
              class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
            >
              <History class="w-3.5 h-3.5" />
              Historique & Sessions
            </button>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Avant Droit (FR)</span>
        </div>

        <!-- Wheel Card: RL (Arrière Gauche) -->
        <div
          v-if="mountedTires.RL"
          class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-3xl p-5 space-y-4 shadow-sm transition-all"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="text-[11px] font-bold uppercase tracking-wider text-rose-400">Arrière Gauche (RL)</div>
              <h3 class="text-base font-bold text-white">{{ mountedTires.RL.tire.brand }} {{ mountedTires.RL.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.RL.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.RL.condition).class"
            >
              {{ getConditionBadge(mountedTires.RL.condition).label }}
            </span>
          </div>

          <div class="grid grid-cols-3 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Sculpture</div>
              <div class="text-sm font-bold text-white">{{ mountedTires.RL.current_depth_mm }} mm</div>
              <div class="text-[10px] text-slate-400">Témoin: {{ mountedTires.RL.min_legal_depth_mm }} mm</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.RL.total_distance_km).toLocaleString('fr-FR') }} km</div>
              <div class="text-[10px] text-slate-400">Vie: {{ mountedTires.RL.life_progress_pct }}%</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.RL.cost_per_km).toFixed(4) }} €</div>
              <div class="text-[10px] text-slate-400">/ pneu</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.RL.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.RL.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.RL.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.RL.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-slate-800">
            <button
              @click="openLogModal(mountedTires.RL)"
              class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 font-medium transition-colors"
            >
              <Ruler class="w-3.5 h-3.5 text-rose-400" />
              Mesurer gomme
            </button>
            <button
              @click="openHistoryModal(mountedTires.RL)"
              class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
            >
              <History class="w-3.5 h-3.5" />
              Historique & Sessions
            </button>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Arrière Gauche (RL)</span>
        </div>

        <!-- Wheel Card: RR (Arrière Droit) -->
        <div
          v-if="mountedTires.RR"
          class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-3xl p-5 space-y-4 shadow-sm transition-all"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="text-[11px] font-bold uppercase tracking-wider text-rose-400">Arrière Droit (RR)</div>
              <h3 class="text-base font-bold text-white">{{ mountedTires.RR.tire.brand }} {{ mountedTires.RR.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.RR.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.RR.condition).class"
            >
              {{ getConditionBadge(mountedTires.RR.condition).label }}
            </span>
          </div>

          <div class="grid grid-cols-3 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Sculpture</div>
              <div class="text-sm font-bold text-white">{{ mountedTires.RR.current_depth_mm }} mm</div>
              <div class="text-[10px] text-slate-400">Témoin: {{ mountedTires.RR.min_legal_depth_mm }} mm</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.RR.total_distance_km).toLocaleString('fr-FR') }} km</div>
              <div class="text-[10px] text-slate-400">Vie: {{ mountedTires.RR.life_progress_pct }}%</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.RR.cost_per_km).toFixed(4) }} €</div>
              <div class="text-[10px] text-slate-400">/ pneu</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.RR.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.RR.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.RR.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.RR.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-slate-800">
            <button
              @click="openLogModal(mountedTires.RR)"
              class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 font-medium transition-colors"
            >
              <Ruler class="w-3.5 h-3.5 text-rose-400" />
              Mesurer gomme
            </button>
            <button
              @click="openHistoryModal(mountedTires.RR)"
              class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
            >
              <History class="w-3.5 h-3.5" />
              Historique & Sessions
            </button>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Arrière Droit (RR)</span>
        </div>
      </div>
    </div>

    <!-- TAB 2: CATALOGUE & STOCK AU GARAGE -->
    <div v-if="activeTab === 'storage'" class="space-y-4">
      <div v-if="storageTires.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center text-slate-400 space-y-3">
        <Package class="w-10 h-10 mx-auto text-slate-600" />
        <h3 class="text-base font-bold text-white">Aucun pneu stocké au garage</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          Vous pouvez enregistrer vos packs de pneus hiver ou de rechange pour suivre précisément leur kilométrage même démontés.
        </p>
      </div>

      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="t in storageTires"
          :key="t.tire.id"
          class="bg-slate-900 border border-slate-800 hover:border-slate-700 rounded-2xl p-4 space-y-3 shadow-sm transition-all"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="flex items-center gap-1.5 text-xs font-semibold">
                <component :is="getSeasonIcon(t.tire.season).icon" class="w-3.5 h-3.5" :class="getSeasonIcon(t.tire.season).color" />
                <span class="text-slate-300">{{ getSeasonIcon(t.tire.season).label }}</span>
              </div>
              <h4 class="text-sm font-bold text-white mt-1">{{ t.tire.brand }} {{ t.tire.model }}</h4>
              <div class="text-[11px] text-slate-400 font-mono">{{ t.tire.dimension }}</div>
            </div>
            <span class="bg-slate-800 text-slate-400 text-[10px] px-2 py-0.5 rounded-full border border-slate-700 font-medium">
              Au garage
            </span>
          </div>

          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80 text-center text-xs">
            <div>
              <div class="text-[10px] text-slate-500">Kilométrage total</div>
              <div class="font-bold text-white">{{ Math.round(t.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500">Sculpture actuelle</div>
              <div class="font-bold text-emerald-400">{{ t.current_depth_mm }} mm</div>
            </div>
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
            <button
              @click="openHistoryModal(t)"
              class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
            >
              <History class="w-3.5 h-3.5" />
              Historique
            </button>
            <button
              @click="openLogModal(t)"
              class="text-xs text-slate-400 hover:text-white flex items-center gap-1 font-medium transition-colors"
            >
              <Ruler class="w-3.5 h-3.5" />
              Mesurer
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- MODAL: AJOUTER DES PNEUS (BATCH / SINGLE) -->
    <div
      v-if="showAddTireModal"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-xl w-full p-6 space-y-5 max-h-[90vh] overflow-y-auto shadow-2xl">
        <div class="flex items-center justify-between pb-3 border-b border-slate-800">
          <h3 class="text-lg font-bold text-white flex items-center gap-2">
            <Plus class="w-5 h-5 text-rose-500" />
            Ajouter des pneus
          </h3>
          <button @click="showAddTireModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Add Type Selection -->
        <div class="space-y-1.5">
          <label class="block text-xs font-semibold text-slate-300">Format d'enregistrement :</label>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
            <button
              type="button"
              @click="addType = 'SET_4'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_4' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Train complet monté (4 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SET_4_STORAGE'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_4_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Pack au garage (4 pneus hiver/été)
            </button>
            <button
              type="button"
              @click="addType = 'SET_2_FRONT'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_2_FRONT' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Essieu avant monté (2 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SET_2_REAR'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_2_REAR' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Essieu arrière monté (2 pneus)
            </button>
          </div>
        </div>

        <!-- Dimension Dropdown -->
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-400">Dimension homologuée :</label>
          <select
            v-model="dimensionPreset"
            @change="onDimensionPresetChange"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
          >
            <option v-for="p in teslaDimensionPresets" :key="p.value" :value="p.value">
              {{ p.label }}
            </option>
          </select>
          <div v-if="isCustomDimension" class="pt-1.5">
            <input
              v-model="addTireForm.dimension"
              type="text"
              placeholder="Ex: 245/40 R19 98Y"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
            />
          </div>
        </div>

        <!-- Brand, Model, Season -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Marque</label>
            <input
              v-model="addTireForm.brand"
              type="text"
              placeholder="Michelin, Pirelli, Hankook..."
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Modèle</label>
            <input
              v-model="addTireForm.model"
              type="text"
              placeholder="Pilot Sport EV, Winter Sottozero..."
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Saison</label>
            <select
              v-model="addTireForm.season"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            >
              <option value="SUMMER">☀️ Été</option>
              <option value="WINTER">❄️ Hiver</option>
              <option value="ALL_SEASON">🌦️ 4 Saisons</option>
            </select>
          </div>
        </div>

        <!-- Pricing & Lifespan -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-950/60 p-3.5 rounded-2xl border border-slate-800">
          <div>
            <div class="flex items-center justify-between mb-1">
              <label class="text-xs font-semibold text-slate-400">
                {{ isTotalPrice ? 'Prix total du lot (€)' : 'Prix par pneu (€)' }}
              </label>
              <button
                type="button"
                @click="isTotalPrice = !isTotalPrice"
                class="text-[10px] text-rose-400 hover:text-rose-300 underline"
              >
                Passer en {{ isTotalPrice ? 'prix unitaire' : 'prix total' }}
              </button>
            </div>
            <input
              v-if="isTotalPrice"
              v-model.number="addTireForm.total_price"
              type="number"
              step="10"
              class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
            <input
              v-else
              v-model.number="addTireForm.unit_price"
              type="number"
              step="5"
              class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Durée de vie estimée (km)</label>
            <input
              v-model.number="addTireForm.estimated_lifespan_km"
              type="number"
              step="5000"
              class="w-full bg-slate-900 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
        </div>

        <!-- Odometers: Mounted Odo & Accumulated -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Odomètre de montage (km)</label>
            <input
              v-model.number="addTireForm.mounted_odometer"
              type="number"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-400 mb-1">Km déjà parcourus (si occasion)</label>
            <input
              v-model.number="addTireForm.accumulated_distance_km"
              type="number"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
        </div>

        <!-- Date & Sculptures -->
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="block text-[11px] text-slate-400 mb-1">Date d'achat</label>
            <input
              v-model="addTireForm.purchase_date"
              type="date"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
          <div>
            <label class="block text-[11px] text-slate-400 mb-1">Gomme neuve (mm)</label>
            <input
              v-model.number="addTireForm.initial_depth_mm"
              type="number"
              step="0.1"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
          <div>
            <label class="block text-[11px] text-slate-400 mb-1">Témoin légal (mm)</label>
            <input
              v-model.number="addTireForm.min_legal_depth_mm"
              type="number"
              step="0.1"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
          <button
            type="button"
            @click="showAddTireModal = false"
            class="px-4 py-2 rounded-xl text-xs font-semibold text-slate-400 hover:text-white"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleCreateTires"
            class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2.5 rounded-xl shadow-lg shadow-rose-600/20"
          >
            Créer les pneus
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: HISTORIQUE COMPLET & TIMELINE D'UN PNEU -->
    <div
      v-if="showHistoryModal && selectedTire"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-2xl w-full p-6 space-y-6 max-h-[90vh] overflow-y-auto shadow-2xl">
        <!-- Header -->
        <div class="flex items-start justify-between pb-3 border-b border-slate-800">
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-lg font-bold text-white">{{ selectedTire.brand }} {{ selectedTire.model }}</h3>
              <span class="text-xs font-mono bg-slate-800 px-2 py-0.5 rounded text-slate-300">
                {{ selectedTire.dimension }}
              </span>
            </div>
            <div class="text-xs text-slate-400 mt-0.5">
              Acheté le {{ formatDate(selectedTire.purchase_date) }} • {{ selectedTire.purchase_price }} €
            </div>
          </div>
          <button @click="showHistoryModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Life KPI Card -->
        <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-2xl space-y-3">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">Kilométrage total de vie :</span>
            <span class="text-base font-bold text-white">
              {{ Math.round(selectedTireStats?.total_distance_km || 0).toLocaleString('fr-FR') }} km
              <span class="text-xs text-slate-400 font-normal">/ {{ (selectedTire.estimated_lifespan_km || 45000).toLocaleString('fr-FR') }} km estimés</span>
            </span>
          </div>

          <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all"
              :class="(selectedTireStats?.life_progress_pct || 0) > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
              :style="{ width: `${Math.min(100, selectedTireStats?.life_progress_pct || 0)}%` }"
            ></div>
          </div>

          <div class="grid grid-cols-3 gap-2 text-center text-xs pt-1">
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Sculpture actuelle</div>
              <div class="font-bold text-emerald-400">{{ selectedTireStats?.current_depth_mm }} mm</div>
            </div>
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Durée de vie consommée</div>
              <div class="font-bold text-slate-200">{{ selectedTireStats?.life_progress_pct }}%</div>
            </div>
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Coût réel / km</div>
              <div class="font-bold text-amber-400">{{ Number(selectedTireStats?.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>
        </div>

        <!-- Timeline: Mount/Dismount Sessions -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
              <History class="w-4 h-4 text-rose-500" />
              Historique des montages, démontages & permutations
            </h4>
            <button
              @click="openAddSessionModal()"
              class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              Ajouter une session passée
            </button>
          </div>

          <div v-if="tireSessions.length === 0" class="p-6 text-center bg-slate-950/40 rounded-2xl text-xs text-slate-500">
            Aucune session enregistrée pour ce pneu.
          </div>

          <div v-else class="space-y-2.5">
            <div
              v-for="s in tireSessions"
              :key="s.id"
              class="bg-slate-950/80 border border-slate-800/80 rounded-2xl p-3.5 text-xs space-y-2 hover:border-slate-700 transition-all"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span
                    class="px-2 py-0.5 rounded-md font-bold text-[10px]"
                    :class="s.dismounted_date ? 'bg-slate-800 text-slate-300' : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'"
                  >
                    {{ s.dismounted_date ? 'Session terminée' : '🟢 Montage en cours' }}
                  </span>
                  <span class="font-bold text-white">Roue : {{ s.position }}</span>
                </div>

                <div class="flex items-center gap-1.5">
                  <button
                    @click="openEditSessionModal(s)"
                    class="p-1 text-slate-400 hover:text-white rounded"
                    title="Modifier la session"
                  >
                    <Edit2 class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="handleDeleteSession(s)"
                    class="p-1 text-slate-400 hover:text-rose-400 rounded"
                    title="Supprimer la session"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <!-- Session Details -->
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] text-slate-400 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/60">
                <div>
                  <div class="text-slate-500">Montage :</div>
                  <div class="text-slate-200 font-medium">{{ formatDate(s.mounted_date) }} à {{ Math.round(s.mounted_odometer).toLocaleString('fr-FR') }} km</div>
                </div>
                <div>
                  <div class="text-slate-500">Démontage :</div>
                  <div class="text-slate-200 font-medium">
                    {{ s.dismounted_date ? `${formatDate(s.dismounted_date)} à ${Math.round(s.dismounted_odometer).toLocaleString('fr-FR')} km` : 'Actuellement sur le véhicule' }}
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-[11px] pt-1">
                <span v-if="s.notes" class="text-slate-400 italic">"{{ s.notes }}"</span>
                <span v-else></span>
                <span class="font-bold text-rose-400">+{{ Math.round(s.distance_km).toLocaleString('fr-FR') }} km parcourus</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Tread Logs (Mesures de gomme) -->
        <div class="space-y-2 pt-3 border-t border-slate-800">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
              <Ruler class="w-4 h-4 text-emerald-400" />
              Relevés de profondeur de gomme ({{ tireLogs.length }})
            </h4>
            <button
              @click="openLogModal(selectedTireStats)"
              class="text-xs text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              Ajouter un relevé
            </button>
          </div>

          <div v-if="tireLogs.length > 0" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
            <div
              v-for="l in tireLogs"
              :key="l.id"
              class="bg-slate-950/60 border border-slate-800 p-2.5 rounded-xl text-xs space-y-0.5"
            >
              <div class="flex items-center justify-between">
                <span class="font-bold text-emerald-400">{{ l.depth_mm }} mm</span>
                <span class="text-[10px] text-slate-500">{{ formatDate(l.date) }}</span>
              </div>
              <div class="text-[10px] text-slate-400">à {{ Math.round(l.odometer).toLocaleString('fr-FR') }} km</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- MODAL: AJOUTER / MODIFIER UNE SESSION DE MONTAGE -->
    <div
      v-if="showSessionModal"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between pb-2 border-b border-slate-800">
          <h3 class="text-sm font-bold text-white">
            {{ editingSessionId ? 'Modifier la session de montage' : 'Ajouter une session de montage passée' }}
          </h3>
          <button @click="showSessionModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Position occupée</label>
            <select
              v-model="sessionForm.position"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            >
              <option value="FL">Avant Gauche (FL)</option>
              <option value="FR">Avant Droit (FR)</option>
              <option value="RL">Arrière Gauche (RL)</option>
              <option value="RR">Arrière Droit (RR)</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Date de montage</label>
              <input
                v-model="sessionForm.mounted_date"
                type="date"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Odomètre montage (km)</label>
              <input
                v-model.number="sessionForm.mounted_odometer"
                type="number"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              />
            </div>
          </div>

          <div class="pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300">
              <input type="checkbox" v-model="sessionForm.is_dismounted" class="rounded accent-rose-500" />
              <span>Cette session est terminée (pneu démonté)</span>
            </label>
          </div>

          <div v-if="sessionForm.is_dismounted" class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-xl border border-slate-800">
            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Date démontage</label>
              <input
                v-model="sessionForm.dismounted_date"
                type="date"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Odomètre démontage (km)</label>
              <input
                v-model.number="sessionForm.dismounted_odometer"
                type="number"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Distance de la session (km)</label>
            <input
              v-model.number="sessionForm.distance_km"
              type="number"
              placeholder="Auto-calculé ou forcé"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>

          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Commentaire / Notes</label>
            <input
              v-model="sessionForm.notes"
              type="text"
              placeholder="Ex: Saison hivernale 2024"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
          <button
            type="button"
            @click="showSessionModal = false"
            class="px-3 py-1.5 text-xs text-slate-400 hover:text-white"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleSaveSession"
            class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2 rounded-xl"
          >
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: MESURE DE GOMME -->
    <div
      v-if="showLogModal"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-sm w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between pb-2 border-b border-slate-800">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Ruler class="w-4 h-4 text-emerald-400" />
            Relevé de sculpture
          </h3>
          <button @click="showLogModal = false" class="text-slate-400 hover:text-white p-1">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Profondeur mesurée (mm)</label>
            <input
              v-model.number="newLogForm.depth_mm"
              type="number"
              step="0.1"
              min="1.0"
              max="10.0"
              class="w-full bg-slate-800 text-slate-100 font-bold rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 text-sm"
            />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Odomètre actuel (km)</label>
            <input
              v-model.number="newLogForm.odometer"
              type="number"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Notes (optionnel)</label>
            <input
              v-model="newLogForm.notes"
              type="text"
              placeholder="Ex: Contrôle avant vacances"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
          <button type="button" @click="showLogModal = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">
            Annuler
          </button>
          <button
            type="button"
            @click="handleAddLog"
            class="bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold px-4 py-2 rounded-xl"
          >
            Enregistrer le relevé
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: CHANGEMENT DE PACK COMPLET (ÉTÉ ⇄ HIVER) -->
    <div
      v-if="showPackSwapModal"
      class="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-lg w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between pb-2 border-b border-slate-800">
          <h3 class="text-sm font-bold text-white flex items-center gap-2">
            <Snowflake class="w-4 h-4 text-sky-400" />
            Permutation saisonnière (Changement de pack complet)
          </h3>
          <button @click="showPackSwapModal = false" class="text-slate-400 hover:text-white p-1">
            <X class="w-4 h-4" />
          </button>
        </div>

        <p class="text-xs text-slate-400">
          Les 4 pneus actuellement montés vont être envoyés au garage avec leur kilométrage figé. Sélectionnez les 4 pneus du garage à monter sur la Tesla :
        </p>

        <div class="space-y-3 text-xs">
          <div>
            <label class="block text-slate-400 mb-1 font-semibold">Odomètre de la permutation (km)</label>
            <input
              v-model.number="packSwapForm.odometer"
              type="number"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Avant Gauche (FL)</label>
              <select
                v-model="packSwapForm.tires.FL"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Avant Droit (FR)</label>
              <select
                v-model="packSwapForm.tires.FR"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Arrière Gauche (RL)</label>
              <select
                v-model="packSwapForm.tires.RL"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label class="block text-slate-400 mb-1 font-semibold">Arrière Droit (RR)</label>
              <select
                v-model="packSwapForm.tires.RR"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
          <button type="button" @click="showPackSwapModal = false" class="px-3 py-1.5 text-xs text-slate-400 hover:text-white">
            Annuler
          </button>
          <button
            type="button"
            @click="handlePackSwapSubmit"
            class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-4 py-2 rounded-xl"
          >
            Confirmer la permutation
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
