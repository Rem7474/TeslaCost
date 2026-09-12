<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import {
  Receipt,
  Plus,
  Wrench,
  Zap,
  Calendar,
  DollarSign,
  X,
  Repeat,
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const activeTab = ref<'TOLLS' | 'MAINTENANCE' | 'CHARGES'>('TOLLS')

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const loading = ref(false)

// Modals
const showAddTollModal = ref(false)
const showAddMaintModal = ref(false)

const tollForm = ref({
  type: 'TOLL',
  amount: '',
  date: new Date().toISOString(),
  notes: '',
})

const maintForm = ref({
  category: 'MAINTENANCE',
  amount: '',
  date: new Date().toISOString(),
  odometer: 0,
  is_recurring: false,
  recurrence_interval_months: 12,
  description: '',
})

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    if (activeTab.value === 'TOLLS') {
      driveExpenses.value = await api.getDriveExpenses(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'MAINTENANCE') {
      maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'CHARGES') {
      const res = await api.getCharges(vehicleStore.activeVehicle.id)
      charges.value = res.charges
    }
  } catch (err) {
    console.error('Failed to load expenses', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicleId, activeTab.value],
  () => {
    loadData()
  }
)

onMounted(() => {
  loadData()
})

async function handleCreateToll() {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.createDriveExpense(vehicleStore.activeVehicle.id, {
      ...tollForm.value,
      amount: Number(tollForm.value.amount),
    })
    showAddTollModal.value = false
    tollForm.value.amount = ''
    tollForm.value.notes = ''
    await loadData()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleCreateMaint() {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.createMaintenance(vehicleStore.activeVehicle.id, {
      ...maintForm.value,
      amount: Number(maintForm.value.amount),
      odometer: maintForm.value.odometer ? Number(maintForm.value.odometer) : null,
    })
    showAddMaintModal.value = false
    maintForm.value.amount = ''
    maintForm.value.description = ''
    await loadData()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Dépenses & Entretien</h2>
        <p class="text-sm text-slate-400">Péages, parkings, entretien récurrent, assurance et recharges</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          v-if="activeTab === 'TOLLS'"
          @click="showAddTollModal = true"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          + Péage / Parking
        </button>
        <button
          v-if="activeTab === 'MAINTENANCE'"
          @click="showAddMaintModal = true"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          + Entretien / Fixe
        </button>
      </div>
    </div>

    <!-- Sub-tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        @click="activeTab = 'TOLLS'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors"
        :class="activeTab === 'TOLLS' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Receipt class="w-4 h-4" />
        Péages & Parkings
      </button>
      <button
        @click="activeTab = 'MAINTENANCE'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors"
        :class="activeTab === 'MAINTENANCE' ? 'bg-pink-500/20 text-pink-400 border border-pink-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Wrench class="w-4 h-4" />
        Entretien & Coûts Fixes
      </button>
      <button
        @click="activeTab = 'CHARGES'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors"
        :class="activeTab === 'CHARGES' ? 'bg-sky-500/20 text-sky-400 border border-sky-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Zap class="w-4 h-4" />
        Recharges Électriques
      </button>
    </div>

    <!-- Content: Tolls -->
    <div v-if="activeTab === 'TOLLS'">
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!driveExpenses.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucun péage ou parking enregistré.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="e in driveExpenses"
          :key="e.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between"
        >
          <div>
            <div class="flex items-center gap-2">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                {{ e.type }}
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(e.date) }}</span>
            </div>
            <p v-if="e.notes" class="text-sm text-slate-300 mt-1">{{ e.notes }}</p>
          </div>
          <div class="text-lg font-extrabold text-amber-400">
            {{ e.amount.toFixed(2) }} {{ e.currency }}
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Maintenance -->
    <div v-if="activeTab === 'MAINTENANCE'">
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!maintenanceExpenses.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucune dépense d'entretien ou fixe enregistrée.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="m in maintenanceExpenses"
          :key="m.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between"
        >
          <div>
            <div class="flex items-center gap-2">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-pink-500/10 text-pink-400 border border-pink-500/20">
                {{ m.category }}
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(m.date) }}</span>
              <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1">
                <Repeat class="w-3 h-3 text-pink-400" /> tous les {{ m.recurrence_interval_months }} mois
              </span>
            </div>
            <p class="text-sm font-semibold text-slate-200 mt-1">{{ m.description }}</p>
            <p v-if="m.odometer" class="text-xs text-slate-400">À {{ Math.round(m.odometer) }} km</p>
          </div>
          <div class="text-lg font-extrabold text-pink-400">
            {{ m.amount.toFixed(2) }} {{ m.currency }}
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Charges -->
    <div v-if="activeTab === 'CHARGES'">
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!charges.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucune recharge enregistrée. Synchronisez votre véhicule avec TeslaMate !
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="c in charges"
          :key="c.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between"
        >
          <div>
            <div class="flex items-center gap-2">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-sky-500/10 text-sky-400 border border-sky-500/20">
                +{{ c.kwh_added }} kWh
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(c.date) }}</span>
            </div>
            <p class="text-sm text-slate-300 mt-1">{{ c.address || 'Lieu de recharge inconnu' }}</p>
          </div>
          <div class="text-right">
            <span class="text-lg font-extrabold text-sky-400">{{ c.cost.toFixed(2) }} {{ c.currency }}</span>
            <p v-if="c.kwh_added > 0" class="text-[11px] text-slate-400">
              {{ (c.cost / c.kwh_added).toFixed(3) }} €/kWh
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Add Toll/Parking -->
    <div
      v-if="showAddTollModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white">Ajouter un Péage / Parking</h3>
          <button @click="showAddTollModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleCreateToll" class="space-y-3">
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Type</label>
            <select v-model="tollForm.type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
              <option value="TOLL">Péage</option>
              <option value="PARKING">Parking</option>
              <option value="FERRY">Ferry</option>
              <option value="OTHER">Autre</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Montant (€)</label>
            <input v-model="tollForm.amount" type="number" step="0.01" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Notes / Description</label>
            <input v-model="tollForm.notes" placeholder="A10 Paris-Bordeaux..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showAddTollModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add Maintenance/Fixed -->
    <div
      v-if="showAddMaintModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white">Ajouter Entretien / Dépense Fixe</h3>
          <button @click="showAddMaintModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleCreateMaint" class="space-y-3">
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Catégorie</label>
            <select v-model="maintForm.category" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
              <option value="MAINTENANCE">Entretien / Révision</option>
              <option value="INSURANCE">Assurance</option>
              <option value="SUBSCRIPTION">Abonnement (Connectivité...)</option>
              <option value="TAX">Taxe / Carte grise</option>
              <option value="ACCESSORY">Accessoire</option>
              <option value="OTHER">Autre</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Description</label>
            <input v-model="maintForm.description" required placeholder="ex: Remplacement filtre habitacle" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Montant (€)</label>
            <input v-model="maintForm.amount" type="number" step="0.01" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre (optionnel)</label>
            <input v-model.number="maintForm.odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div class="flex items-center gap-2 pt-1">
            <input v-model="maintForm.is_recurring" type="checkbox" id="rec" class="rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500" />
            <label for="rec" class="text-xs text-slate-300 font-medium">Dépense récurrente</label>
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <button type="button" @click="showAddMaintModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
