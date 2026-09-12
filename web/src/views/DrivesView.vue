<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
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
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const drives = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const selectedTag = ref('')
const loading = ref(false)

// Multi-selection for trip grouping & tolls
const selectedDriveIds = ref<string[]>([])
const showGroupModal = ref(false)
const groupName = ref('')
const tollAmount = ref<number | ''>('')
const expenseType = ref('TOLL')

async function loadDrives() {
  if (!vehicleStore.activeVehicle) return
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
    // If setting Pro, remove Perso and vice versa
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
    // 1. Create Trip Group
    const tg = await api.createTripGroup(vehicleStore.activeVehicle.id, {
      name: groupName.value,
      drive_ids: selectedDriveIds.value,
    })

    // 2. Attach expense if amount provided
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

    alert('Groupe de trajets et dépense créés avec succès !')
    showGroupModal.value = false
    selectedDriveIds.value = []
    groupName.value = ''
    tollAmount.value = ''
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
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
          {{ total }} trajets enregistrés • Taguez en Pro/Perso et assignez les péages
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
      class="sticky top-16 z-30 bg-slate-800 border border-slate-700 p-3 rounded-xl flex items-center justify-between shadow-xl"
    >
      <div class="flex items-center gap-3 text-sm text-slate-200">
        <span class="font-bold text-rose-400">{{ selectedDriveIds.length }}</span> trajet(s) sélectionné(s)
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="showGroupModal = true"
          class="px-3 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-colors"
        >
          <Layers class="w-3.5 h-3.5" />
          Fusionner en Voyage & Péage
        </button>
        <button
          @click="selectedDriveIds = []"
          class="p-1.5 text-slate-400 hover:text-white rounded-lg"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Drives List -->
    <div v-if="loading" class="text-center py-12 text-slate-400">
      Chargement des trajets...
    </div>

    <div v-else-if="!drives.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
      Aucun trajet trouvé.
    </div>

    <div v-else class="space-y-3">
      <!-- Select all toggle -->
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
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/80 p-4 rounded-2xl transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-4"
        :class="{ 'border-rose-500/40 bg-slate-800/40': selectedDriveIds.includes(d.id) }"
      >
        <div class="flex items-start gap-3">
          <!-- Selection checkbox -->
          <button @click="toggleSelectDrive(d.id)" class="mt-1 text-slate-500 hover:text-rose-400">
            <component :is="selectedDriveIds.includes(d.id) ? CheckSquare : Square" class="w-5 h-5 text-rose-400" />
          </button>

          <!-- Drive Details -->
          <div>
            <div class="flex items-center gap-2 flex-wrap mb-1">
              <span class="text-xs font-semibold text-slate-400">{{ formatDate(d.start_time) }}</span>
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200">
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
              <span class="truncate max-w-xs">{{ d.start_address || 'Départ inconnu' }}</span>
              <span class="text-slate-500">→</span>
              <span class="truncate max-w-xs">{{ d.end_address || 'Arrivée inconnue' }}</span>
            </div>
          </div>
        </div>

        <!-- Tag Actions -->
        <div class="flex items-center gap-2 self-end sm:self-auto">
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
      </div>

      <!-- Pagination Controls -->
      <div class="flex items-center justify-center gap-4 pt-4">
        <button
          @click="page--; loadDrives()"
          :disabled="page <= 1"
          class="p-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <ChevronLeft class="w-5 h-5" />
        </button>
        <span class="text-xs text-slate-400 font-semibold">Page {{ page }}</span>
        <button
          @click="page++; loadDrives()"
          :disabled="page * limit >= total"
          class="p-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <ChevronRight class="w-5 h-5" />
        </button>
      </div>
    </div>

    <!-- Modal : Group consecutive drives & Assign Toll -->
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
