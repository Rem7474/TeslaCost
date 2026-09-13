<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
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
  Navigation,
  Layers,
  Users,
  CheckSquare,
  Square,
  ArrowRight,
  Pencil,
  Trash2,
  AlertTriangle,
} from 'lucide-vue-next'

const router = useRouter()
const vehicleStore = useVehicleStore()
const activeTab = ref<'TOLLS' | 'MAINTENANCE' | 'CHARGES'>('TOLLS')

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const chargesWithoutCost = ref(0)
const missingCostOnly = ref(false)
const loading = ref(false)

// Modals
const showAddTollModal = ref(false)
const showAddMaintModal = ref(false)
const editingTollId = ref<string | null>(null)
const editingMaintId = ref<string | null>(null)

const recentDrives = ref<any[]>([])
const associationMode = ref<'NONE' | 'SINGLE' | 'MULTI'>('NONE')
const selectedDriveId = ref('')
const selectedDriveIds = ref<string[]>([])

const CURRENCIES = ['EUR', 'CHF', 'GBP', 'USD']

const tollForm = ref({
  type: 'TOLL',
  amount: '',
  currency: 'EUR',
  fx_rate: '',
  date: toLocalDateTimeInput(new Date()),
  notes: '',
})

const maintForm = ref({
  category: 'MAINTENANCE',
  amount: '',
  currency: 'EUR',
  fx_rate: '',
  date: new Date().toISOString().substring(0, 10),
  odometer: 0,
  is_recurring: false,
  recurrence_interval_months: 12,
  recurrence_end_date: '',
  description: '',
})

// Charges: manual entry and cost completion
const showChargeModal = ref(false)
const editingCharge = ref<any | null>(null)
const chargeForm = ref({
  date: toLocalDateTimeInput(new Date()),
  kwh_added: '',
  cost: '',
  currency: 'EUR',
  fx_rate: '',
  address: '',
  odometer: '',
  notes: '',
})

// datetime-local inputs expect local time, not UTC
function toLocalDateTimeInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function currencyPayload(form: { currency: string; fx_rate: string }) {
  if (form.currency === 'EUR') return { currency: 'EUR', fx_rate: null }
  return { currency: form.currency, fx_rate: form.fx_rate ? Number(form.fx_rate) : null }
}

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    if (activeTab.value === 'TOLLS') {
      driveExpenses.value = await api.getDriveExpenses(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'MAINTENANCE') {
      maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'CHARGES') {
      const res = await api.getCharges(vehicleStore.activeVehicle.id, { missingCost: missingCostOnly.value })
      charges.value = res.charges
      chargesWithoutCost.value = res.charges_without_cost || 0
    }
  } catch (err) {
    console.error('Failed to load expenses', err)
  } finally {
    loading.value = false
  }
}

async function loadRecentDrives() {
  if (!vehicleStore.activeVehicle) return
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { limit: 40 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

function openAddTollModal() {
  editingTollId.value = null
  tollForm.value = {
    type: 'TOLL',
    amount: '',
    currency: 'EUR',
    fx_rate: '',
    date: toLocalDateTimeInput(new Date()),
    notes: '',
  }
  associationMode.value = 'NONE'
  selectedDriveId.value = ''
  selectedDriveIds.value = []
  showAddTollModal.value = true
  loadRecentDrives()
}

function openEditTollModal(e: any) {
  editingTollId.value = e.id
  tollForm.value = {
    type: e.type || 'TOLL',
    amount: String(e.amount),
    currency: e.currency || 'EUR',
    fx_rate: e.fx_rate ? String(e.fx_rate) : '',
    date: toLocalDateTimeInput(new Date(e.date)),
    notes: e.notes || '',
  }
  if (e.trip_group_id) {
    // Keep the trip group link: its drives are preselected in multi-step mode
    associationMode.value = 'MULTI'
    selectedDriveId.value = ''
    selectedDriveIds.value = [...(e.trip_group_drive_ids || [])]
  } else if (e.drive_id) {
    associationMode.value = 'SINGLE'
    selectedDriveId.value = e.drive_id
    selectedDriveIds.value = []
  } else {
    associationMode.value = 'NONE'
    selectedDriveId.value = ''
    selectedDriveIds.value = []
  }
  showAddTollModal.value = true
  loadRecentDrives()
}

async function handleDeleteToll(e: any) {
  if (!vehicleStore.activeVehicle) return
  if (!confirm(`Supprimer ce péage / parking de ${Number(e.amount).toFixed(2)} € ?`)) return
  try {
    await api.deleteDriveExpense(vehicleStore.activeVehicle.id, e.id)
    await loadData()
  } catch (err: any) {
    alert(`Erreur lors de la suppression : ${err.message}`)
  }
}

function onSingleDriveChange() {
  const d = recentDrives.value.find((dr) => dr.id === selectedDriveId.value)
  if (d) {
    tollForm.value.date = new Date(d.start_time).toISOString().substring(0, 16)
    if (!tollForm.value.notes) {
      const from = (d.start_address || 'Départ').split(',')[0]
      const to = (d.end_address || 'Arrivée').split(',')[0]
      tollForm.value.notes = `Péage ${from} → ${to}`
    }
  }
}

function toggleMultiDrive(id: string) {
  const idx = selectedDriveIds.value.indexOf(id)
  if (idx > -1) {
    selectedDriveIds.value.splice(idx, 1)
  } else {
    selectedDriveIds.value.push(id)
  }

  const selected = recentDrives.value
    .filter((d) => selectedDriveIds.value.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())

  if (selected.length > 0) {
    tollForm.value.date = new Date(selected[0].start_time).toISOString().substring(0, 16)
    if (!tollForm.value.notes || tollForm.value.notes.startsWith('Péage ')) {
      const first = (selected[0].start_address || 'Départ').split(',')[0]
      const last = (selected[selected.length - 1].end_address || 'Arrivée').split(',')[0]
      tollForm.value.notes = `Péage ${first} → ${last} (${selected.length} étapes)`
    }
  }
}

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value, vehicleStore.lastSyncTimestamp],
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
    const payload: any = {
      type: tollForm.value.type,
      amount: Number(tollForm.value.amount),
      ...currencyPayload(tollForm.value),
      date: new Date(tollForm.value.date).toISOString(),
      notes: tollForm.value.notes,
    }

    if (associationMode.value === 'SINGLE' && selectedDriveId.value) {
      payload.drive_id = selectedDriveId.value
    } else if (associationMode.value === 'MULTI' && selectedDriveIds.value.length === 1) {
      payload.drive_id = selectedDriveIds.value[0]
    } else if (associationMode.value === 'MULTI' && selectedDriveIds.value.length > 1) {
      payload.drive_ids = selectedDriveIds.value
    }

    if (editingTollId.value) {
      await api.updateDriveExpense(vehicleStore.activeVehicle.id, editingTollId.value, payload)
    } else {
      await api.createDriveExpense(vehicleStore.activeVehicle.id, payload)
    }
    showAddTollModal.value = false
    await loadData()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function openAddMaintModal() {
  editingMaintId.value = null
  maintForm.value = {
    category: 'MAINTENANCE',
    amount: '',
    currency: 'EUR',
    fx_rate: '',
    date: new Date().toISOString().substring(0, 10),
    odometer: 0,
    is_recurring: false,
    recurrence_interval_months: 12,
    recurrence_end_date: '',
    description: '',
  }
  showAddMaintModal.value = true
}

function openEditMaintModal(m: any) {
  editingMaintId.value = m.id
  maintForm.value = {
    category: m.category || 'MAINTENANCE',
    amount: String(m.amount),
    currency: m.currency || 'EUR',
    fx_rate: m.fx_rate ? String(m.fx_rate) : '',
    date: new Date(m.date).toISOString().substring(0, 10),
    odometer: m.odometer ? Math.round(m.odometer) : 0,
    is_recurring: Boolean(m.is_recurring),
    recurrence_interval_months: m.recurrence_interval_months || 12,
    recurrence_end_date: m.recurrence_end_date ? new Date(m.recurrence_end_date).toISOString().substring(0, 10) : '',
    description: m.description || '',
  }
  showAddMaintModal.value = true
}

async function handleDeleteMaint(m: any) {
  if (!vehicleStore.activeVehicle) return
  if (!confirm(`Supprimer la dépense "${m.description}" de ${Number(m.amount).toFixed(2)} € ?`)) return
  try {
    await api.deleteMaintenance(vehicleStore.activeVehicle.id, m.id)
    await loadData()
  } catch (err: any) {
    alert(`Erreur lors de la suppression : ${err.message}`)
  }
}

async function handleCreateMaint() {
  if (!vehicleStore.activeVehicle) return
  try {
    const payload = {
      ...maintForm.value,
      ...currencyPayload(maintForm.value),
      amount: Number(maintForm.value.amount),
      odometer: maintForm.value.odometer ? Number(maintForm.value.odometer) : null,
      date: new Date(maintForm.value.date).toISOString(),
      recurrence_end_date:
        maintForm.value.is_recurring && maintForm.value.recurrence_end_date
          ? new Date(maintForm.value.recurrence_end_date).toISOString()
          : null,
    }
    if (editingMaintId.value) {
      await api.updateMaintenance(vehicleStore.activeVehicle.id, editingMaintId.value, payload)
    } else {
      await api.createMaintenance(vehicleStore.activeVehicle.id, payload)
    }
    showAddMaintModal.value = false
    await loadData()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function toggleMissingCostFilter() {
  missingCostOnly.value = !missingCostOnly.value
  loadData()
}

function openAddChargeModal() {
  editingCharge.value = null
  chargeForm.value = {
    date: toLocalDateTimeInput(new Date()),
    kwh_added: '',
    cost: '',
    currency: 'EUR',
    fx_rate: '',
    address: '',
    odometer: vehicleStore.activeVehicle?.current_odometer ? String(Math.round(vehicleStore.activeVehicle.current_odometer)) : '',
    notes: '',
  }
  showChargeModal.value = true
}

function openEditChargeModal(c: any) {
  editingCharge.value = c
  chargeForm.value = {
    date: toLocalDateTimeInput(new Date(c.date)),
    kwh_added: String(c.kwh_added),
    cost: c.cost !== null && c.cost !== undefined ? String(c.cost) : '',
    currency: c.currency || 'EUR',
    fx_rate: c.fx_rate ? String(c.fx_rate) : '',
    address: c.address || '',
    odometer: c.odometer ? String(Math.round(c.odometer)) : '',
    notes: c.notes || '',
  }
  showChargeModal.value = true
}

async function handleSaveCharge() {
  if (!vehicleStore.activeVehicle) return
  const payload = {
    date: new Date(chargeForm.value.date).toISOString(),
    kwh_added: Number(chargeForm.value.kwh_added),
    cost: Number(chargeForm.value.cost),
    ...currencyPayload(chargeForm.value),
    address: chargeForm.value.address || null,
    odometer: chargeForm.value.odometer ? Number(chargeForm.value.odometer) : null,
    notes: chargeForm.value.notes || null,
  }
  try {
    if (editingCharge.value) {
      await api.updateCharge(vehicleStore.activeVehicle.id, editingCharge.value.id, payload)
    } else {
      await api.createCharge(vehicleStore.activeVehicle.id, payload)
    }
    showChargeModal.value = false
    await loadData()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleDeleteCharge(c: any) {
  if (!vehicleStore.activeVehicle) return
  if (!confirm(`Supprimer cette recharge manuelle de ${c.kwh_added} kWh ?`)) return
  try {
    await api.deleteCharge(vehicleStore.activeVehicle.id, c.id)
    await loadData()
  } catch (err: any) {
    alert(`Erreur lors de la suppression : ${err.message}`)
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

function formatDriveTime(dateStr: string) {
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
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Dépenses & Entretien</h2>
        <p class="text-sm text-slate-400">Péages, parkings, entretien récurrent, assurance et recharges</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          v-if="activeTab === 'TOLLS'"
          @click="openAddTollModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Péage / Parking
        </button>
        <button
          v-if="activeTab === 'MAINTENANCE'"
          @click="openAddMaintModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Entretien / Fixe
        </button>
        <button
          v-if="activeTab === 'CHARGES'"
          @click="openAddChargeModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Recharge hors TeslaMate
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
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        >
          <div class="space-y-1.5">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                {{ e.type }}
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(e.date) }}</span>
              <span v-if="e.drive_title" class="text-xs px-2.5 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                <Navigation class="w-3 h-3" /> {{ e.drive_title }}
              </span>
              <span v-else-if="e.trip_group_name" class="text-xs px-2.5 py-0.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 flex items-center gap-1">
                <Layers class="w-3 h-3" /> {{ e.trip_group_name }}
              </span>
            </div>
            <p v-if="e.notes" class="text-sm text-slate-300">{{ e.notes }}</p>
          </div>
          <div class="flex items-center justify-between sm:justify-end gap-3">
            <div class="text-lg font-extrabold text-amber-400">
              {{ e.amount.toFixed(2) }} {{ e.currency }}
            </div>
            <div class="flex items-center gap-1.5">
              <button
                v-if="e.drive_id || e.trip_group_id"
                @click="router.push({ path: '/carpools', query: e.drive_id ? { new_drive_id: e.drive_id } : { new_trip_group_id: e.trip_group_id } })"
                class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
                title="Créer un covoiturage pour ce trajet"
              >
                <Users class="w-3.5 h-3.5 text-cyan-400" />
                <span>Covoiturer</span>
              </button>
              <button
                @click="openEditTollModal(e)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-amber-400 rounded-xl transition-colors border border-slate-700/60"
                title="Modifier ce péage"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                @click="handleDeleteToll(e)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer ce péage"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
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
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        >
          <div class="space-y-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-pink-500/10 text-pink-400 border border-pink-500/20">
                {{ m.category }}
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(m.date) }}</span>
              <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1">
                <Repeat class="w-3 h-3 text-pink-400" /> tous les {{ m.recurrence_interval_months }} mois
                <template v-if="m.recurrence_end_date">jusqu'au {{ formatDate(m.recurrence_end_date) }}</template>
              </span>
            </div>
            <p class="text-sm font-semibold text-slate-200">{{ m.description }}</p>
            <p v-if="m.odometer" class="text-xs text-slate-400">À {{ Math.round(m.odometer) }} km</p>
          </div>
          <div class="flex items-center justify-between sm:justify-end gap-3">
            <div class="text-lg font-extrabold text-pink-400">
              {{ m.amount.toFixed(2) }} {{ m.currency }}
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="openEditMaintModal(m)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-pink-400 rounded-xl transition-colors border border-slate-700/60"
                title="Modifier cette dépense"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                @click="handleDeleteMaint(m)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer cette dépense"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Charges -->
    <div v-if="activeTab === 'CHARGES'" class="space-y-3">
      <button
        v-if="chargesWithoutCost > 0 || missingCostOnly"
        @click="toggleMissingCostFilter"
        class="w-full p-3 rounded-2xl text-left text-xs font-semibold flex items-center gap-2 border transition-colors"
        :class="missingCostOnly ? 'bg-amber-500/20 border-amber-500/40 text-amber-300' : 'bg-amber-500/10 border-amber-500/20 text-amber-400 hover:bg-amber-500/15'"
      >
        <AlertTriangle class="w-4 h-4 shrink-0" />
        <span v-if="missingCostOnly">Affichage des recharges sans coût uniquement — cliquer pour tout afficher</span>
        <span v-else>{{ chargesWithoutCost }} recharge(s) sans coût : le TCO est sous-estimé. Cliquer pour les compléter.</span>
      </button>
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!charges.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucune recharge enregistrée. Synchronisez votre véhicule avec TeslaMate ou ajoutez une recharge manuelle.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="c in charges"
          :key="c.id"
          class="bg-slate-900 border p-4 rounded-2xl flex items-center justify-between gap-3"
          :class="c.cost === null ? 'border-amber-500/40' : 'border-slate-800'"
        >
          <div>
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-sky-500/10 text-sky-400 border border-sky-500/20">
                +{{ c.kwh_added }} kWh
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(c.date) }}</span>
              <span v-if="c.is_manual" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">Manuelle</span>
              <span v-else-if="c.cost_source === 'MANUAL'" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">Coût corrigé</span>
            </div>
            <p class="text-sm text-slate-300 mt-1">{{ c.address || 'Lieu de recharge inconnu' }}</p>
          </div>
          <div class="flex items-center gap-3">
            <div class="text-right">
              <template v-if="c.cost !== null">
                <span class="text-lg font-extrabold text-sky-400">{{ c.cost.toFixed(2) }} {{ c.currency }}</span>
                <p v-if="c.kwh_added > 0" class="text-[11px] text-slate-400">
                  {{ (c.cost / c.kwh_added).toFixed(3) }} {{ c.currency }}/kWh
                </p>
              </template>
              <span v-else class="text-xs font-bold text-amber-400 flex items-center gap-1">
                <AlertTriangle class="w-3.5 h-3.5" /> Coût manquant
              </span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="openEditChargeModal(c)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-sky-400 rounded-xl transition-colors border border-slate-700/60"
                :title="c.is_manual ? 'Modifier cette recharge' : 'Renseigner / corriger le coût'"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                v-if="c.is_manual"
                @click="handleDeleteCharge(c)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer cette recharge"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Add Toll/Parking -->
    <div
      v-if="showAddTollModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Receipt class="w-5 h-5 text-amber-400" />
            {{ editingTollId ? 'Modifier le Péage / Parking' : 'Ajouter un Péage / Parking' }}
          </h3>
          <button @click="showAddTollModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleCreateToll" class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
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
              <label class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
              <div class="flex gap-1.5">
                <input v-model="tollForm.amount" type="number" step="0.01" min="0.01" required placeholder="0.00" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
                <select v-model="tollForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                  <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
                </select>
              </div>
            </div>
          </div>
          <div v-if="tollForm.currency !== 'EUR'">
            <label class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ tollForm.currency }} = ? €)</label>
            <input v-model="tollForm.fx_rate" type="number" step="0.000001" min="0.000001" required placeholder="ex: 1.05" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <!-- Association à un/des trajets TeslaMate -->
          <div class="space-y-2 bg-slate-800/50 p-3.5 rounded-xl border border-slate-700/60">
            <label class="block text-xs font-semibold text-slate-200">
              Associer à un trajet TeslaMate
            </label>
            <p class="text-[11px] text-slate-400">
              Permet de récupérer automatiquement ce montant pour le covoiturage (BlaBlaCar) et le coût du trajet.
            </p>

            <div class="grid grid-cols-3 gap-1.5 pt-1">
              <button
                type="button"
                @click="associationMode = 'NONE'"
                class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="associationMode === 'NONE' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Sans trajet
              </button>
              <button
                type="button"
                @click="associationMode = 'SINGLE'"
                class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="associationMode === 'SINGLE' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Trajet unique
              </button>
              <button
                type="button"
                @click="associationMode = 'MULTI'"
                class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="associationMode === 'MULTI' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Multi-étapes
              </button>
            </div>

            <!-- Single drive selection -->
            <div v-if="associationMode === 'SINGLE'" class="pt-2 space-y-1.5">
              <label class="block text-xs text-slate-400">Choisir le trajet :</label>
              <select
                v-model="selectedDriveId"
                @change="onSingleDriveChange"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
              >
                <option value="">-- Sélectionner un trajet récent --</option>
                <option v-for="d in recentDrives" :key="d.id" :value="d.id">
                  {{ formatDriveTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }} ({{ d.distance_km.toFixed(1) }} km)
                </option>
              </select>
            </div>

            <!-- Multi drives selection -->
            <div v-if="associationMode === 'MULTI'" class="pt-2 space-y-1.5">
              <div class="flex items-center justify-between">
                <label class="text-xs text-slate-400">Cocher les étapes composant le voyage :</label>
                <span class="text-[11px] text-amber-400 font-semibold">{{ selectedDriveIds.length }} étape(s)</span>
              </div>
              <div class="max-h-40 overflow-y-auto space-y-1.5 pr-1">
                <div
                  v-for="d in recentDrives"
                  :key="d.id"
                  @click="toggleMultiDrive(d.id)"
                  class="flex items-center justify-between p-2 rounded-lg cursor-pointer text-xs border transition-colors"
                  :class="selectedDriveIds.includes(d.id) ? 'bg-amber-500/10 border-amber-500/40 text-amber-200' : 'bg-slate-800/80 border-slate-700 text-slate-300 hover:bg-slate-800'"
                >
                  <div class="flex items-center gap-2">
                    <CheckSquare v-if="selectedDriveIds.includes(d.id)" class="w-4 h-4 text-amber-400" />
                    <Square v-else class="w-4 h-4 text-slate-500" />
                    <span>{{ formatDriveTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}</span>
                  </div>
                  <span class="font-mono text-[11px] text-slate-400">{{ d.distance_km.toFixed(0) }} km</span>
                </div>
              </div>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
              <input v-model="tollForm.date" type="datetime-local" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Notes / Description</label>
              <input v-model="tollForm.notes" placeholder="A10 Paris-Bordeaux..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" @click="showAddTollModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl font-medium">
              {{ editingTollId ? 'Mettre à jour' : 'Enregistrer' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Add / Edit Maintenance/Fixed -->
    <div
      v-if="showAddMaintModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Wrench class="w-5 h-5 text-pink-400" />
            {{ editingMaintId ? 'Modifier Entretien / Dépense Fixe' : 'Ajouter Entretien / Dépense Fixe' }}
          </h3>
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
              <option value="FINANCING">Financement (loyer, intérêts de crédit)</option>
              <option value="ACCESSORY">Accessoire</option>
              <option value="OTHER">Autre</option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Description</label>
            <input v-model="maintForm.description" required placeholder="ex: Remplacement filtre habitacle" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Date</label>
              <input v-model="maintForm.date" type="date" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
              <div class="flex gap-1.5">
                <input v-model="maintForm.amount" type="number" step="0.01" min="0.01" required class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
                <select v-model="maintForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                  <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
                </select>
              </div>
            </div>
          </div>
          <div v-if="maintForm.currency !== 'EUR'">
            <label class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ maintForm.currency }} = ? €)</label>
            <input v-model="maintForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre (optionnel)</label>
            <input v-model.number="maintForm.odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="space-y-2 pt-1">
            <div class="flex items-center gap-2">
              <input v-model="maintForm.is_recurring" type="checkbox" id="rec" class="rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500" />
              <label for="rec" class="text-xs text-slate-300 font-medium">Dépense récurrente</label>
            </div>
            <div v-if="maintForm.is_recurring" class="pt-1 grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Intervalle (mois)</label>
                <input v-model.number="maintForm.recurrence_interval_months" type="number" min="1" max="120" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Fin (optionnelle)</label>
                <input v-model="maintForm.recurrence_end_date" type="date" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <p class="col-span-2 text-[11px] text-slate-400">
                Chaque échéance est comptée dans le TCO jusqu'à aujourd'hui (ou jusqu'à la date de fin).
              </p>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" @click="showAddMaintModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl font-medium">
              {{ editingMaintId ? 'Mettre à jour' : 'Enregistrer' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Manual charge / cost completion -->
    <div
      v-if="showChargeModal"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Zap class="w-5 h-5 text-sky-400" />
            {{ !editingCharge ? 'Recharge hors TeslaMate' : editingCharge.is_manual ? 'Modifier la recharge' : 'Coût de la recharge' }}
          </h3>
          <button @click="showChargeModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <p v-if="editingCharge && !editingCharge.is_manual" class="text-[11px] text-slate-400">
          Recharge TeslaMate du {{ formatDate(editingCharge.date) }} (+{{ editingCharge.kwh_added }} kWh). Le coût saisi ici ne sera pas écrasé par les synchronisations.
        </p>

        <form @submit.prevent="handleSaveCharge" class="space-y-3">
          <template v-if="!editingCharge || editingCharge.is_manual">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
                <input v-model="chargeForm.date" type="datetime-local" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
              </div>
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Énergie ajoutée (kWh)</label>
                <input v-model="chargeForm.kwh_added" type="number" step="0.001" min="0.001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Lieu (optionnel)</label>
                <input v-model="chargeForm.address" placeholder="Borne, domicile..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre (optionnel)</label>
                <input v-model="chargeForm.odometer" type="number" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
          </template>

          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Coût</label>
            <div class="flex gap-1.5">
              <input v-model="chargeForm.cost" type="number" step="0.01" min="0" required placeholder="0.00 si gratuite" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <select v-model="chargeForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
          <div v-if="chargeForm.currency !== 'EUR'">
            <label class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ chargeForm.currency }} = ? €)</label>
            <input v-model="chargeForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-slate-300 mb-1">Notes (optionnel)</label>
            <input v-model="chargeForm.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" @click="showChargeModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl font-medium">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
