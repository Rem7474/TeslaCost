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
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const tires = ref<any[]>([])
const loading = ref(false)

// Modals
const showAddTireModal = ref(false)
const showLogModal = ref(false)
const showRotationModal = ref(false)
const selectedTireForLog = ref<any | null>(null)

// Forms
const newTireForm = ref({
  brand: 'Michelin',
  model: 'Pilot Sport EV',
  dimension: '235/40 R19 96W',
  season: 'SUMMER',
  purchase_date: new Date().toISOString().split('T')[0],
  purchase_price: 220,
  current_position: 'FL',
  initial_depth_mm: 8.0,
  min_legal_depth_mm: 1.6,
  dot_code: '',
})

const newLogForm = ref({
  depth_mm: 6.5,
  odometer: 0,
  notes: '',
})

const rotationForm = ref({
  odometer: 0,
  mapping: {
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
      newLogForm.value.odometer = Math.round(vehicleStore.activeVehicle.current_odometer)
      rotationForm.value.odometer = Math.round(vehicleStore.activeVehicle.current_odometer)
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

function openLogModal(t: any) {
  selectedTireForLog.value = t
  newLogForm.value.depth_mm = t.current_depth_mm || 6.5
  newLogForm.value.odometer = Math.round(vehicleStore.activeVehicle?.current_odometer || 0)
  showLogModal.value = true
}

async function handleCreateTire() {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.createTire(vehicleStore.activeVehicle.id, newTireForm.value)
    showAddTireModal.value = false
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleAddLog() {
  if (!vehicleStore.activeVehicle || !selectedTireForLog.value) return
  try {
    await api.addTireLog(vehicleStore.activeVehicle.id, selectedTireForLog.value.tire.id, {
      depth_mm: Number(newLogForm.value.depth_mm),
      odometer: Number(newLogForm.value.odometer),
      notes: newLogForm.value.notes,
      date: new Date().toISOString(),
    })
    showLogModal.value = false
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function openRotationModal() {
  // Pre-fill with current positions swapped (front to back cross)
  rotationForm.value.odometer = Math.round(vehicleStore.activeVehicle?.current_odometer || 0)
  rotationForm.value.mapping = {
    FL: mountedTires.value.RL?.tire.id || '',
    FR: mountedTires.value.RR?.tire.id || '',
    RL: mountedTires.value.FR?.tire.id || '',
    RR: mountedTires.value.FL?.tire.id || '',
  }
  showRotationModal.value = true
}

async function handleApplyRotation() {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.rotateTires(vehicleStore.activeVehicle.id, {
      date: new Date().toISOString(),
      odometer: Number(rotationForm.value.odometer),
      mapping_json: rotationForm.value.mapping,
      notes: 'Permutation enregistrée depuis le gestionnaire de pneus',
    })
    showRotationModal.value = false
    await loadTires()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function getConditionBadge(cond: string) {
  if (cond === 'GOOD') {
    return { text: 'Bon état', class: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30' }
  } else if (cond === 'WARNING') {
    return { text: 'Usure modérée', class: 'bg-amber-500/20 text-amber-400 border-amber-500/30' }
  }
  return { text: 'À remplacer', class: 'bg-rose-500/20 text-rose-400 border-rose-500/30' }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Gestion du Cycle de Vie des Pneus</h2>
        <p class="text-sm text-slate-400">Position 4-roues, relevés de sculpture (mm) et projection d'usure</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="openRotationModal"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 flex items-center gap-2 transition-colors"
        >
          <RefreshCw class="w-3.5 h-3.5" />
          Permutation
        </button>
        <button
          @click="showAddTireModal = true"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-colors"
        >
          <Plus class="w-3.5 h-3.5" />
          Ajouter un pneu
        </button>
      </div>
    </div>

    <!-- Interactive Car Chassis Diagram (4 Wheels) -->
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
      <h3 class="text-sm font-bold text-white mb-6 text-center">Disposition des Roues Actuelles</h3>

      <div class="max-w-xl mx-auto grid grid-cols-2 gap-x-8 gap-y-12 relative">
        <!-- Chassis Visual Line -->
        <div class="absolute inset-x-1/2 top-4 bottom-4 w-1 -translate-x-1/2 bg-slate-800 rounded-full hidden sm:block"></div>

        <!-- FL: Avant Gauche -->
        <div class="relative bg-slate-800/80 border border-slate-700/80 p-4 rounded-2xl">
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-extrabold text-slate-400 uppercase tracking-wider">Avant Gauche (FL)</span>
            <span
              v-if="mountedTires.FL"
              class="text-[10px] px-2 py-0.5 rounded-full border font-bold"
              :class="getConditionBadge(mountedTires.FL.condition).class"
            >
              {{ getConditionBadge(mountedTires.FL.condition).text }}
            </span>
          </div>

          <div v-if="mountedTires.FL">
            <p class="text-sm font-bold text-white">{{ mountedTires.FL.tire.brand }} {{ mountedTires.FL.tire.model }}</p>
            <p class="text-xs text-slate-400">{{ mountedTires.FL.tire.dimension }}</p>

            <div class="mt-3 flex items-center justify-between">
              <div>
                <span class="text-xl font-extrabold text-white">{{ mountedTires.FL.current_depth_mm }}</span>
                <span class="text-xs text-slate-400 ml-1">mm (min 1.6)</span>
              </div>
              <button
                @click="openLogModal(mountedTires.FL)"
                class="px-2.5 py-1 text-[11px] bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg flex items-center gap-1"
              >
                <Ruler class="w-3 h-3 text-rose-400" />
                Relever
              </button>
            </div>

            <!-- Wear Projection -->
            <p class="text-[11px] text-slate-400 mt-2">
              Reste est. : <strong class="text-slate-200">{{ Math.round(mountedTires.FL.estimated_remaining_km).toLocaleString('fr-FR') }} km</strong>
            </p>
          </div>
          <div v-else class="text-xs text-slate-500 py-4 text-center">Aucun pneu monté</div>
        </div>

        <!-- FR: Avant Droit -->
        <div class="relative bg-slate-800/80 border border-slate-700/80 p-4 rounded-2xl">
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-extrabold text-slate-400 uppercase tracking-wider">Avant Droit (FR)</span>
            <span
              v-if="mountedTires.FR"
              class="text-[10px] px-2 py-0.5 rounded-full border font-bold"
              :class="getConditionBadge(mountedTires.FR.condition).class"
            >
              {{ getConditionBadge(mountedTires.FR.condition).text }}
            </span>
          </div>

          <div v-if="mountedTires.FR">
            <p class="text-sm font-bold text-white">{{ mountedTires.FR.tire.brand }} {{ mountedTires.FR.tire.model }}</p>
            <p class="text-xs text-slate-400">{{ mountedTires.FR.tire.dimension }}</p>

            <div class="mt-3 flex items-center justify-between">
              <div>
                <span class="text-xl font-extrabold text-white">{{ mountedTires.FR.current_depth_mm }}</span>
                <span class="text-xs text-slate-400 ml-1">mm (min 1.6)</span>
              </div>
              <button
                @click="openLogModal(mountedTires.FR)"
                class="px-2.5 py-1 text-[11px] bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg flex items-center gap-1"
              >
                <Ruler class="w-3 h-3 text-rose-400" />
                Relever
              </button>
            </div>

            <p class="text-[11px] text-slate-400 mt-2">
              Reste est. : <strong class="text-slate-200">{{ Math.round(mountedTires.FR.estimated_remaining_km).toLocaleString('fr-FR') }} km</strong>
            </p>
          </div>
          <div v-else class="text-xs text-slate-500 py-4 text-center">Aucun pneu monté</div>
        </div>

        <!-- RL: Arrière Gauche -->
        <div class="relative bg-slate-800/80 border border-slate-700/80 p-4 rounded-2xl">
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-extrabold text-slate-400 uppercase tracking-wider">Arrière Gauche (RL)</span>
            <span
              v-if="mountedTires.RL"
              class="text-[10px] px-2 py-0.5 rounded-full border font-bold"
              :class="getConditionBadge(mountedTires.RL.condition).class"
            >
              {{ getConditionBadge(mountedTires.RL.condition).text }}
            </span>
          </div>

          <div v-if="mountedTires.RL">
            <p class="text-sm font-bold text-white">{{ mountedTires.RL.tire.brand }} {{ mountedTires.RL.tire.model }}</p>
            <p class="text-xs text-slate-400">{{ mountedTires.RL.tire.dimension }}</p>

            <div class="mt-3 flex items-center justify-between">
              <div>
                <span class="text-xl font-extrabold text-white">{{ mountedTires.RL.current_depth_mm }}</span>
                <span class="text-xs text-slate-400 ml-1">mm (min 1.6)</span>
              </div>
              <button
                @click="openLogModal(mountedTires.RL)"
                class="px-2.5 py-1 text-[11px] bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg flex items-center gap-1"
              >
                <Ruler class="w-3 h-3 text-rose-400" />
                Relever
              </button>
            </div>

            <p class="text-[11px] text-slate-400 mt-2">
              Reste est. : <strong class="text-slate-200">{{ Math.round(mountedTires.RL.estimated_remaining_km).toLocaleString('fr-FR') }} km</strong>
            </p>
          </div>
          <div v-else class="text-xs text-slate-500 py-4 text-center">Aucun pneu monté</div>
        </div>

        <!-- RR: Arrière Droit -->
        <div class="relative bg-slate-800/80 border border-slate-700/80 p-4 rounded-2xl">
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-extrabold text-slate-400 uppercase tracking-wider">Arrière Droit (RR)</span>
            <span
              v-if="mountedTires.RR"
              class="text-[10px] px-2 py-0.5 rounded-full border font-bold"
              :class="getConditionBadge(mountedTires.RR.condition).class"
            >
              {{ getConditionBadge(mountedTires.RR.condition).text }}
            </span>
          </div>

          <div v-if="mountedTires.RR">
            <p class="text-sm font-bold text-white">{{ mountedTires.RR.tire.brand }} {{ mountedTires.RR.tire.model }}</p>
            <p class="text-xs text-slate-400">{{ mountedTires.RR.tire.dimension }}</p>

            <div class="mt-3 flex items-center justify-between">
              <div>
                <span class="text-xl font-extrabold text-white">{{ mountedTires.RR.current_depth_mm }}</span>
                <span class="text-xs text-slate-400 ml-1">mm (min 1.6)</span>
              </div>
              <button
                @click="openLogModal(mountedTires.RR)"
                class="px-2.5 py-1 text-[11px] bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg flex items-center gap-1"
              >
                <Ruler class="w-3 h-3 text-rose-400" />
                Relever
              </button>
            </div>

            <p class="text-[11px] text-slate-400 mt-2">
              Reste est. : <strong class="text-slate-200">{{ Math.round(mountedTires.RR.estimated_remaining_km).toLocaleString('fr-FR') }} km</strong>
            </p>
          </div>
          <div v-else class="text-xs text-slate-500 py-4 text-center">Aucun pneu monté</div>
        </div>
      </div>
    </div>

    <!-- Storage / Archived Tires -->
    <div v-if="storageTires.length" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
      <h3 class="text-sm font-bold text-white mb-3">Pneus au garage / Stockage (Saison précédente)</h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div
          v-for="st in storageTires"
          :key="st.tire.id"
          class="bg-slate-800/60 p-3 rounded-xl border border-slate-700 flex items-center justify-between"
        >
          <div>
            <p class="text-sm font-bold text-white">{{ st.tire.brand }} {{ st.tire.model }}</p>
            <p class="text-xs text-slate-400">{{ st.tire.dimension }} • {{ st.current_depth_mm }} mm</p>
          </div>
          <button
            @click="openLogModal(st)"
            class="px-2.5 py-1 text-xs bg-slate-700 text-slate-200 rounded-lg"
          >
            Relever
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Add Tire -->
    <div
      v-if="showAddTireModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-bold text-white">Ajouter un pneu</h3>
          <button @click="showAddTireModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleCreateTire" class="space-y-3">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Marque</label>
              <input v-model="newTireForm.brand" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Modèle</label>
              <input v-model="newTireForm.model" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Dimensions</label>
            <input v-model="newTireForm.dimension" placeholder="235/40 R19 96W" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Saison</label>
              <select v-model="newTireForm.season" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                <option value="SUMMER">Été</option>
                <option value="WINTER">Hiver</option>
                <option value="ALL_SEASON">4 Saisons</option>
              </select>
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Position</label>
              <select v-model="newTireForm.current_position" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                <option value="FL">Avant Gauche (FL)</option>
                <option value="FR">Avant Droit (FR)</option>
                <option value="RL">Arrière Gauche (RL)</option>
                <option value="RR">Arrière Droit (RR)</option>
                <option value="STORAGE">Stockage (Garage)</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Prix d'achat (€)</label>
              <input v-model.number="newTireForm.purchase_price" type="number" step="0.01" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Profondeur neuve (mm)</label>
              <input v-model.number="newTireForm.initial_depth_mm" type="number" step="0.1" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showAddTireModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add Depth Log -->
    <div
      v-if="showLogModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white">Relevé de sculpture</h3>
          <button @click="showLogModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleAddLog" class="space-y-3">
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Profondeur mesurée (mm)</label>
            <input v-model.number="newLogForm.depth_mm" type="number" step="0.1" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre du véhicule (km)</label>
            <input v-model.number="newLogForm.odometer" type="number" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Notes (optionnel)</label>
            <input v-model="newLogForm.notes" placeholder="Contrôle technique, usure régulière..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showLogModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Permutation (Rotation) -->
    <div
      v-if="showRotationModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <RefreshCw class="w-4 h-4 text-rose-400" />
            Permutation de roues
          </h3>
          <button @click="showRotationModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <p class="text-xs text-slate-400">
          Enregistrez la permutation pour mettre à jour automatiquement les positions montées sur le véhicule.
        </p>

        <form @submit.prevent="handleApplyRotation" class="space-y-3">
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre lors de la permutation (km)</label>
            <input v-model.number="rotationForm.odometer" type="number" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">Nouvel Avant Gauche (FL)</label>
              <select v-model="rotationForm.mapping.FL" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-white">
                <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">{{ t.tire.brand }} ({{ t.tire.current_position }})</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">Nouvel Avant Droit (FR)</label>
              <select v-model="rotationForm.mapping.FR" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-white">
                <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">{{ t.tire.brand }} ({{ t.tire.current_position }})</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">Nouvel Arrière Gauche (RL)</label>
              <select v-model="rotationForm.mapping.RL" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-white">
                <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">{{ t.tire.brand }} ({{ t.tire.current_position }})</option>
              </select>
            </div>
            <div>
              <label class="block text-[11px] text-slate-400 mb-1">Nouvel Arrière Droit (RR)</label>
              <select v-model="rotationForm.mapping.RR" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-white">
                <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">{{ t.tire.brand }} ({{ t.tire.current_position }})</option>
              </select>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showRotationModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Valider la permutation
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
