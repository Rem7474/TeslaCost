<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import {
  Navigation as NavIcon,
  Tag,
  CheckSquare,
  Square,
  Receipt,
  Layers,
  MapPin,
  Clock,
  Zap,
  ChevronLeft,
  ChevronRight,
  X,
  Users,
  Coins,
  Shield,
  Wrench,
  Disc,
  Plus,
  ArrowRight,
  TrendingUp,
} from 'lucide-vue-next'

const router = useRouter()
const vehicleStore = useVehicleStore()
const drives = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const selectedTag = ref('')
const loading = ref(true)

// Multi-selection for trip grouping & tolls & carpooling
const selectedDriveIds = ref<string[]>([])
const showGroupModal = ref(false)
const groupName = ref('')
const tollAmount = ref<number | ''>('')
const expenseType = ref('TOLL')

// Cost breakdown modal state
const selectedCostDrive = ref<any | null>(null)
const showCostModal = ref(false)
const driveExpenses = ref<any[]>([])
const loadingExpenses = ref(false)
const showAddTollInline = ref(false)
const inlineTollAmount = ref<number | ''>('')
const inlineTollType = ref('TOLL')
const inlineTollNotes = ref('')
const addingToll = ref(false)

async function loadDrives() {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, {
      tag: selectedTag.value,
      page: page.value,
      limit: limit.value,
    })
    drives.value = res.drives
    total.value = res.total
  } catch (err) {
    console.error('Failed to load drives', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicleId, selectedTag.value, vehicleStore.lastSyncTimestamp],
  () => {
    page.value = 1
    selectedDriveIds.value = []
    loadDrives()
  }
)

onMounted(() => {
  loadDrives()
})

function toggleSelectDrive(id: string) {
  const idx = selectedDriveIds.value.indexOf(id)
  if (idx > -1) {
    selectedDriveIds.value.splice(idx, 1)
  } else {
    selectedDriveIds.value.push(id)
  }
}

function selectAll() {
  if (selectedDriveIds.value.length === drives.value.length) {
    selectedDriveIds.value = []
  } else {
    selectedDriveIds.value = drives.value.map((d) => d.id)
  }
}

async function toggleDriveTag(drive: any, tagToToggle: string) {
  if (!vehicleStore.activeVehicle) return
  let currentTags = [...(drive.tags || [])]
  const idx = currentTags.indexOf(tagToToggle)

  if (idx > -1) {
    currentTags.splice(idx, 1)
  } else {
    if (tagToToggle === 'Pro') {
      currentTags = currentTags.filter((t) => t !== 'Perso')
    } else if (tagToToggle === 'Perso') {
      currentTags = currentTags.filter((t) => t !== 'Pro')
    }
    currentTags.push(tagToToggle)
  }

  try {
    await api.updateDriveTags(vehicleStore.activeVehicle.id, drive.id, currentTags)
    drive.tags = currentTags
  } catch (err: any) {
    alert(`Erreur de mise à jour du tag : ${err.message}`)
  }
}

async function handleCreateGroupAndExpense() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  if (!groupName.value) {
    alert('Veuillez donner un nom au groupe de trajets (ex: Voyage Paris-Lyon)')
    return
  }

  try {
    const tg = await api.createTripGroup(vehicleStore.activeVehicle.id, {
      name: groupName.value,
      drive_ids: selectedDriveIds.value,
    })

    if (tollAmount.value && Number(tollAmount.value) > 0) {
      await api.createDriveExpense(vehicleStore.activeVehicle.id, {
        trip_group_id: tg.id,
        type: expenseType.value,
        amount: Number(tollAmount.value),
        currency: 'EUR',
        date: new Date().toISOString(),
        notes: `Assigné au groupe ${groupName.value}`,
      })
    }

    alert('Groupe de trajets créé avec succès !')
    showGroupModal.value = false
    selectedDriveIds.value = []
    groupName.value = ''
    tollAmount.value = ''
    loadDrives()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleCarpoolSelectedDrives() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return

  if (selectedDriveIds.value.length === 1) {
    router.push({ path: '/carpools', query: { new_drive_id: selectedDriveIds.value[0] } })
    return
  }

  // Multi-drives: sort chronologically to identify first origin and last destination
  const selectedDrives = drives.value
    .filter((d) => selectedDriveIds.value.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())

  const first = selectedDrives[0]
  const last = selectedDrives[selectedDrives.length - 1]
  const fromCity = (first.start_address || 'Départ').split(',')[0]
  const toCity = (last.end_address || 'Arrivée').split(',')[0]
  const groupTitle = `${fromCity} → ${toCity} (${selectedDrives.length} étapes)`

  try {
    const tg = await api.createTripGroup(vehicleStore.activeVehicle.id, {
      name: groupTitle,
      drive_ids: selectedDrives.map((d) => d.id),
    })
    selectedDriveIds.value = []
    router.push({ path: '/carpools', query: { new_trip_group_id: tg.id } })
  } catch (err: any) {
    alert(`Erreur lors de la préparation du voyage : ${err.message}`)
  }
}

async function openCostModal(drive: any) {
  selectedCostDrive.value = drive
  showCostModal.value = true
  showAddTollInline.value = false
  inlineTollAmount.value = ''
  inlineTollNotes.value = ''
  loadDriveExpenses(drive.id)
}

async function loadDriveExpenses(driveId: string) {
  if (!vehicleStore.activeVehicle) return
  loadingExpenses.value = true
  try {
    driveExpenses.value = await api.getDriveExpensesForDrive(vehicleStore.activeVehicle.id, driveId)
  } catch (err) {
    console.error('Failed to load drive expenses', err)
    driveExpenses.value = []
  } finally {
    loadingExpenses.value = false
  }
}

async function handleAddTollToDrive() {
  if (!vehicleStore.activeVehicle || !selectedCostDrive.value) return
  if (!inlineTollAmount.value || Number(inlineTollAmount.value) <= 0) {
    alert('Veuillez entrer un montant valide')
    return
  }

  addingToll.value = true
  try {
    const amountNum = Number(inlineTollAmount.value)
    await api.createDriveExpense(vehicleStore.activeVehicle.id, {
      drive_id: selectedCostDrive.value.id,
      type: inlineTollType.value,
      amount: amountNum,
      currency: 'EUR',
      date: selectedCostDrive.value.start_time,
      notes: inlineTollNotes.value || 'Ajouté depuis la vue trajet',
    })

    // Update local costs
    if (selectedCostDrive.value.costs) {
      selectedCostDrive.value.costs.tolls_cost = (selectedCostDrive.value.costs.tolls_cost || 0) + amountNum
      selectedCostDrive.value.costs.total_cost = (selectedCostDrive.value.costs.total_cost || 0) + amountNum
      if (selectedCostDrive.value.distance_km > 0) {
        selectedCostDrive.value.costs.cost_per_km =
          selectedCostDrive.value.costs.total_cost / selectedCostDrive.value.distance_km
      }
    }

    await loadDriveExpenses(selectedCostDrive.value.id)
    showAddTollInline.value = false
    inlineTollAmount.value = ''
    inlineTollNotes.value = ''
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  } finally {
    addingToll.value = false
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Filter Tabs -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Trajets & Voyages</h2>
        <p class="text-sm text-slate-400">
          {{ total }} trajets enregistrés • Coûts réels de revient calculés en temps réel
        </p>
      </div>

      <!-- Tag Filters -->
      <div class="flex items-center gap-2 bg-slate-900 border border-slate-800 p-1 rounded-xl self-start sm:self-auto">
        <button
          @click="selectedTag = ''"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === '' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
        >
          Tous
        </button>
        <button
          @click="selectedTag = 'Pro'"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === 'Pro' ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30' : 'text-slate-400 hover:text-white'"
        >
          Pro
        </button>
        <button
          @click="selectedTag = 'Perso'"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === 'Perso' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'text-slate-400 hover:text-white'"
        >
          Perso
        </button>
      </div>
    </div>

    <!-- Multi-selection Action Bar -->
    <div
      v-if="selectedDriveIds.length"
      class="sticky top-16 z-30 bg-slate-800/95 backdrop-blur-md border border-slate-700 p-3 rounded-2xl flex flex-wrap items-center justify-between gap-3 shadow-2xl"
    >
      <div class="flex items-center gap-2 text-sm text-slate-200">
        <span class="px-2.5 py-0.5 bg-rose-500/20 text-rose-400 font-bold rounded-lg border border-rose-500/30">
          {{ selectedDriveIds.length }}
        </span>
        <span>trajet(s) sélectionné(s)</span>
      </div>

      <div class="flex items-center gap-2">
        <!-- Direct Carpool button for single or multi-drives -->
        <button
          @click="handleCarpoolSelectedDrives"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
        >
          <Users class="w-4 h-4" />
          <span>{{ selectedDriveIds.length > 1 ? `Covoiturer la sélection (${selectedDriveIds.length} étapes)` : 'Covoiturer ce trajet' }}</span>
        </button>

        <!-- Fusion Voyage Group -->
        <button
          @click="showGroupModal = true"
          class="px-3 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
        >
          <Layers class="w-3.5 h-3.5" />
          <span>Fusionner & Péage</span>
        </button>

        <button
          @click="selectedDriveIds = []"
          class="p-2 text-slate-400 hover:text-white hover:bg-slate-700 rounded-xl transition-colors"
          title="Annuler la sélection"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- SKELETON LOADING STATE -->
    <div v-if="loading" class="space-y-3 animate-pulse">
      <div v-for="i in 5" :key="i" class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between gap-4">
        <div class="flex items-start gap-3 w-2/3">
          <div class="w-5 h-5 bg-slate-800 rounded mt-1"></div>
          <div class="space-y-2.5 w-full">
            <div class="flex items-center gap-2">
              <div class="h-3 w-24 bg-slate-800 rounded"></div>
              <div class="h-4 w-16 bg-slate-800 rounded-full"></div>
              <div class="h-3 w-16 bg-slate-800 rounded"></div>
            </div>
            <div class="h-4 w-4/5 bg-slate-800/80 rounded"></div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <div class="h-8 w-24 bg-slate-800 rounded-xl"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
        </div>
      </div>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="!drives.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
      Aucun trajet trouvé pour cette sélection.
    </div>

    <!-- REAL DRIVES LIST -->
    <div v-else class="space-y-3">
      <!-- Select all toggle & Total info -->
      <div class="flex items-center justify-between text-xs text-slate-400 px-2">
        <button @click="selectAll" class="flex items-center gap-2 hover:text-slate-200 transition-colors">
          <component :is="selectedDriveIds.length === drives.length ? CheckSquare : Square" class="w-4 h-4 text-rose-400" />
          <span>{{ selectedDriveIds.length === drives.length ? 'Tout désélectionner' : 'Tout sélectionner' }}</span>
        </button>
        <span>Page {{ page }} sur {{ Math.ceil(total / limit) || 1 }}</span>
      </div>

      <!-- Drive Card -->
      <div
        v-for="d in drives"
        :key="d.id"
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-4"
        :class="{ 'border-rose-500/40 bg-slate-800/40 shadow-lg shadow-rose-950/20': selectedDriveIds.includes(d.id) }"
      >
        <div class="flex items-start gap-3">
          <!-- Selection checkbox -->
          <button @click="toggleSelectDrive(d.id)" class="mt-1 text-slate-500 hover:text-rose-400 transition-colors">
            <component :is="selectedDriveIds.includes(d.id) ? CheckSquare : Square" class="w-5 h-5 text-rose-400" />
          </button>

          <!-- Drive Details -->
          <div>
            <div class="flex items-center gap-2 flex-wrap mb-1.5">
              <span class="text-xs font-semibold text-slate-400">{{ formatDate(d.start_time) }}</span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60">
                {{ d.distance_km }} km
              </span>
              <span v-if="d.duration_min" class="text-xs text-slate-400 flex items-center gap-1">
                <Clock class="w-3 h-3" /> {{ d.duration_min }} min
              </span>
              <span v-if="d.consumption_kwh_100km" class="text-xs text-sky-400 font-mono">
                {{ d.consumption_kwh_100km }} kWh/100km
              </span>
            </div>

            <!-- Route Address -->
            <div class="text-sm text-slate-300 flex items-center gap-1.5 flex-wrap">
              <MapPin class="w-3.5 h-3.5 text-rose-400 shrink-0" />
              <span class="truncate max-w-xs font-medium">{{ d.start_address || 'Départ inconnu' }}</span>
              <span class="text-slate-500">→</span>
              <span class="truncate max-w-xs font-medium">{{ d.end_address || 'Arrivée inconnue' }}</span>
            </div>
          </div>
        </div>

        <!-- Right Side: Cost Badge & Actions -->
        <div class="flex items-center gap-2.5 self-end sm:self-auto flex-wrap sm:flex-nowrap">
          <!-- Real Cost Badge (Clickable for full breakdown) -->
          <button
            @click="openCostModal(d)"
            class="px-3 py-1.5 bg-slate-800/80 hover:bg-slate-700/80 border border-slate-700/70 hover:border-emerald-500/40 rounded-xl flex items-center gap-2 transition-all text-left shadow-sm group"
            title="Cliquez pour voir la décomposition détaillée par poste de dépense"
          >
            <div class="p-1 rounded-lg bg-emerald-500/10 text-emerald-400 group-hover:bg-emerald-500/20">
              <Coins class="w-3.5 h-3.5" />
            </div>
            <div>
              <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
                <span>{{ (d.costs?.total_cost || 0).toFixed(2) }} €</span>
                <span class="text-[10px] font-normal text-emerald-400 font-mono">
                  {{ (d.costs?.cost_per_km || 0).toFixed(3) }} €/km
                </span>
              </div>
            </div>
          </button>

          <!-- Tags -->
          <div class="flex items-center gap-1">
            <button
              @click="toggleDriveTag(d, 'Pro')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                d.tags?.includes('Pro')
                  ? 'bg-blue-500/20 text-blue-400 border-blue-500/40 shadow-sm shadow-blue-500/20'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              Pro
            </button>
            <button
              @click="toggleDriveTag(d, 'Perso')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                d.tags?.includes('Perso')
                  ? 'bg-emerald-500/20 text-emerald-400 border-emerald-500/40 shadow-sm shadow-emerald-500/20'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              Perso
            </button>
          </div>

          <!-- Quick Carpool Button -->
          <button
            @click="router.push({ path: '/carpools', query: { new_drive_id: d.id } })"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
            title="Créer un covoiturage depuis ce trajet"
          >
            <Users class="w-3.5 h-3.5 text-rose-500" />
            <span class="hidden md:inline">Covoiturer</span>
          </button>
        </div>
      </div>

      <!-- Pagination Controls -->
      <div class="flex items-center justify-center gap-4 pt-4">
        <button
          @click="page--; loadDrives()"
          :disabled="page <= 1"
          class="p-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronLeft class="w-5 h-5" />
        </button>
        <span class="text-xs text-slate-400 font-semibold">Page {{ page }}</span>
        <button
          @click="page++; loadDrives()"
          :disabled="page * limit >= total"
          class="p-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronRight class="w-5 h-5" />
        </button>
      </div>
    </div>

    <!-- MODAL : DÉCOMPOSITION COMPLÈTE DU COÛT D'UN TRAJET -->
    <div
      v-if="showCostModal && selectedCostDrive"
      class="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-3xl max-w-lg w-full p-6 space-y-5 shadow-2xl my-8">
        <!-- Header -->
        <div class="flex items-center justify-between pb-3 border-b border-slate-800">
          <div class="flex items-center gap-2.5">
            <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
              <Coins class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-base font-bold text-white">Coût Réel du Trajet</h3>
              <p class="text-xs text-slate-400">{{ formatDate(selectedCostDrive.start_time) }}</p>
            </div>
          </div>
          <button @click="showCostModal = false" class="p-1.5 text-slate-400 hover:text-white rounded-lg">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Trip Summary Route -->
        <div class="bg-slate-800/60 border border-slate-700/60 p-3.5 rounded-2xl space-y-2">
          <div class="text-sm font-semibold text-white flex items-center gap-2">
            <MapPin class="w-4 h-4 text-rose-400 shrink-0" />
            <span class="truncate">{{ selectedCostDrive.start_address || 'Départ' }}</span>
            <span class="text-slate-500">→</span>
            <span class="truncate">{{ selectedCostDrive.end_address || 'Arrivée' }}</span>
          </div>
          <div class="flex items-center gap-3 text-xs text-slate-300">
            <span class="font-bold text-rose-400">{{ selectedCostDrive.distance_km }} km</span>
            <span v-if="selectedCostDrive.duration_min" class="text-slate-400">• {{ selectedCostDrive.duration_min }} min</span>
            <span v-if="selectedCostDrive.speed_avg" class="text-slate-400">• {{ Math.round(selectedCostDrive.speed_avg) }} km/h moy</span>
            <span v-if="selectedCostDrive.costs?.electricity_kwh" class="text-sky-400 font-mono">• {{ selectedCostDrive.costs.electricity_kwh }} kWh</span>
          </div>
        </div>

        <!-- Cost Breakdown List -->
        <div class="space-y-2.5">
          <!-- 1. Électricité -->
          <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-sky-500/10 text-sky-400 rounded-lg">
                <Zap class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Énergie Électrique</div>
                <div class="text-[11px] text-slate-400 font-mono">
                  {{ selectedCostDrive.costs?.electricity_kwh || 0 }} kWh × {{ (selectedCostDrive.costs?.electricity_rate || 0.22).toFixed(3) }} €/kWh
                </div>
              </div>
            </div>
            <div class="text-sm font-bold text-sky-400 font-mono">
              {{ (selectedCostDrive.costs?.electricity_cost || 0).toFixed(2) }} €
            </div>
          </div>

          <!-- 2. Pneus -->
          <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
                <Disc class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Usure des Pneumatiques</div>
                <div class="text-[11px] text-slate-400 font-mono">
                  {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.tires_rate || 0.02).toFixed(3) }} €/km
                </div>
              </div>
            </div>
            <div class="text-sm font-bold text-emerald-400 font-mono">
              {{ (selectedCostDrive.costs?.tires_cost || 0).toFixed(2) }} €
            </div>
          </div>

          <!-- 3. Entretien -->
          <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-pink-500/10 text-pink-400 rounded-lg">
                <Wrench class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Provision Entretien</div>
                <div class="text-[11px] text-slate-400 font-mono">
                  {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.maintenance_rate || 0.015).toFixed(3) }} €/km
                </div>
              </div>
            </div>
            <div class="text-sm font-bold text-pink-400 font-mono">
              {{ (selectedCostDrive.costs?.maintenance_cost || 0).toFixed(2) }} €
            </div>
          </div>

          <!-- 4. Assurance -->
          <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-purple-500/10 text-purple-400 rounded-lg">
                <Shield class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Quote-part Assurance</div>
                <div class="text-[11px] text-slate-400 font-mono">
                  {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.insurance_rate || 0.035).toFixed(3) }} €/km
                </div>
              </div>
            </div>
            <div class="text-sm font-bold text-purple-400 font-mono">
              {{ (selectedCostDrive.costs?.insurance_cost || 0).toFixed(2) }} €
            </div>
          </div>

          <!-- 5. Péages & Frais de route -->
          <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl space-y-2">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="p-2 bg-amber-500/10 text-amber-400 rounded-lg">
                  <Receipt class="w-4 h-4" />
                </div>
                <div>
                  <div class="text-xs font-semibold text-white">Péages & Frais de route</div>
                  <div class="text-[11px] text-slate-400">
                    {{ driveExpenses.length }} frais assigné(s)
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span class="text-sm font-bold text-amber-400 font-mono">
                  {{ (selectedCostDrive.costs?.tolls_cost || 0).toFixed(2) }} €
                </span>
                <button
                  @click="showAddTollInline = !showAddTollInline"
                  class="p-1 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg text-xs"
                  title="Ajouter un péage ou parking à ce trajet"
                >
                  <Plus class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- List of attached expenses -->
            <div v-if="driveExpenses.length" class="space-y-1 pt-1 border-t border-slate-700/50">
              <div
                v-for="exp in driveExpenses"
                :key="exp.id"
                class="flex items-center justify-between text-[11px] text-slate-300 pl-9"
              >
                <span>{{ exp.type === 'TOLL' ? 'Péage' : exp.type }} <span v-if="exp.notes" class="text-slate-500">({{ exp.notes }})</span></span>
                <span class="font-mono text-amber-400">{{ exp.amount.toFixed(2) }} €</span>
              </div>
            </div>

            <!-- Inline add toll form -->
            <div v-if="showAddTollInline" class="p-3 bg-slate-900 border border-slate-700 rounded-xl space-y-2 mt-2">
              <div class="text-xs font-bold text-white">Ajouter un péage / parking</div>
              <div class="grid grid-cols-2 gap-2">
                <select
                  v-model="inlineTollType"
                  class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
                >
                  <option value="TOLL">Péage</option>
                  <option value="PARKING">Parking</option>
                  <option value="FERRY">Ferry</option>
                </select>
                <input
                  v-model="inlineTollAmount"
                  type="number"
                  step="0.01"
                  placeholder="Montant (€)"
                  class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
                />
              </div>
              <input
                v-model="inlineTollNotes"
                type="text"
                placeholder="Notes (ex: Péage A6 Beaune)"
                class="w-full bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
              />
              <div class="flex justify-end gap-2">
                <button
                  @click="showAddTollInline = false"
                  class="px-2.5 py-1 text-xs text-slate-400 hover:text-white"
                >
                  Annuler
                </button>
                <button
                  @click="handleAddTollToDrive"
                  :disabled="addingToll"
                  class="px-3 py-1 bg-amber-600 hover:bg-amber-500 text-white font-semibold text-xs rounded-lg disabled:opacity-50"
                >
                  {{ addingToll ? 'Enregistrement...' : 'Valider' }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Grand Total Card -->
        <div class="bg-gradient-to-r from-slate-800 to-slate-800/80 border border-emerald-500/30 p-4 rounded-2xl flex items-center justify-between shadow-lg">
          <div>
            <span class="text-xs font-semibold text-emerald-400 uppercase tracking-wider">Coût de Revient Total</span>
            <div class="text-2xl font-black text-white">
              {{ (selectedCostDrive.costs?.total_cost || 0).toFixed(2) }} €
            </div>
          </div>
          <div class="text-right">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût au kilomètre</span>
            <div class="text-lg font-extrabold text-emerald-400 font-mono">
              {{ (selectedCostDrive.costs?.cost_per_km || 0).toFixed(3) }} €<span class="text-xs font-normal text-slate-400">/km</span>
            </div>
          </div>
        </div>

        <!-- Footer Actions -->
        <div class="flex items-center justify-between pt-2">
          <button
            @click="showCostModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl"
          >
            Fermer
          </button>
          <button
            @click="showCostModal = false; router.push({ path: '/carpools', query: { new_drive_id: selectedCostDrive.id } })"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
          >
            <Users class="w-4 h-4" />
            <span>Partager en covoiturage</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL : FUSIONNER EN VOYAGE & ASSIGNER PÉAGE -->
    <div
      v-if="showGroupModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-bold text-white flex items-center gap-2">
            <Layers class="w-5 h-5 text-rose-400" />
            Créer un Voyage / Fusion
          </h3>
          <button @click="showGroupModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <p class="text-xs text-slate-400">
          Vous allez fusionner <strong>{{ selectedDriveIds.length }} trajets</strong> consécutifs (ex: trajet segmenté par des arrêts recharge/déjeuner) et lui assigner un péage ou parking global.
        </p>

        <div>
          <label class="block text-xs font-semibold text-slate-300 mb-1">Nom du voyage / groupe</label>
          <input
            v-model="groupName"
            type="text"
            placeholder="ex: Vacances Bretagne - Aller"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
          />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Type de frais</label>
            <select
              v-model="expenseType"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            >
              <option value="TOLL">Péage</option>
              <option value="PARKING">Parking</option>
              <option value="FERRY">Ferry</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Montant (€)</label>
            <input
              v-model="tollAmount"
              type="number"
              step="0.01"
              placeholder="0.00"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            />
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2">
          <button
            @click="showGroupModal = false"
            class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl"
          >
            Annuler
          </button>
          <button
            @click="handleCreateGroupAndExpense"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/20"
          >
            Enregistrer le groupe
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
