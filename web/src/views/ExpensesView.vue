<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
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
const { showConfirm, showAlert } = useConfirm()
const activeTab = ref<'TOLLS' | 'MAINTENANCE' | 'CHARGES'>('TOLLS')

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const chargesWithoutCost = ref(0)
const chargesTotal = ref(0)
const chargesPage = ref(1)
const loadingMoreCharges = ref(false)
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

const CATEGORY_LABELS: Record<string, string> = {
  MAINTENANCE: 'Entretien',
  REPAIR: 'Réparation / sinistre',
  INSURANCE: 'Assurance',
  SUBSCRIPTION: 'Abonnement',
  TAX: 'Taxe',
  FINANCING: 'Financement',
  ACCESSORY: 'Accessoire',
  OTHER: 'Autre',
}

const insuranceAnnualPremium = ref<number | ''>('')

// Annual premium paid monthly: amount = premium / 12, recurring every month
function applyMonthlyPremium() {
  const annual = Number(insuranceAnnualPremium.value)
  if (!annual || annual <= 0) {
    showAlert('Veuillez saisir la prime annuelle', 'Champ requis', 'warning')
    return
  }
  maintForm.value.amount = (Math.round((annual / 12) * 100) / 100).toFixed(2)
  maintForm.value.is_recurring = true
  maintForm.value.recurrence_interval_months = 1
  if (!maintForm.value.description) {
    maintForm.value.description = `Prime d'assurance (${annual.toFixed(2)} €/an)`
  }
}

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
  amortization_mode: 'DISTANCE',
  coverage_km: 50000,
  coverage_months: 24,
  closes_maintenance_id: null as string | null,
  description: '',
})

const detectedOdometer = ref<number | null>(null)
const detectingOdometer = ref(false)
const shouldClosePrevious = ref(false)

const closeCandidateMaintenance = computed(() => {
  if (!maintenanceExpenses.value || !maintenanceExpenses.value.length) return null
  const currentId = editingMaintId.value
  const formDate = maintForm.value.date
  return maintenanceExpenses.value.find(
    (m) =>
      m.id !== currentId &&
      m.amortization_mode &&
      m.amortization_mode !== 'NONE' &&
      new Date(m.date).toISOString().substring(0, 10) <= formDate
  ) || null
})

async function checkOdometerForDate(dateVal: string) {
  if (!vehicleStore.activeVehicle || !dateVal) return
  detectingOdometer.value = true
  try {
    const res = await api.getOdometerAt(vehicleStore.activeVehicle.id, dateVal)
    if (res && typeof res.odometer === 'number' && res.odometer > 0) {
      detectedOdometer.value = res.odometer
      if (!maintForm.value.odometer || maintForm.value.odometer === 0) {
        maintForm.value.odometer = Math.round(res.odometer)
      }
    } else {
      detectedOdometer.value = null
    }
  } catch {
    detectedOdometer.value = null
  } finally {
    detectingOdometer.value = false
  }
}

watch(() => maintForm.value.date, (newDate) => {
  if (showAddMaintModal.value && newDate) {
    checkOdometerForDate(newDate)
  }
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
      chargesPage.value = 1
      const res = await api.getCharges(vehicleStore.activeVehicle.id, { missingCost: missingCostOnly.value })
      charges.value = res.charges
      chargesTotal.value = res.total || 0
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
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { limit: 200 })
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
  const ok = await showConfirm({
    title: 'Supprimer la dépense',
    message: `Supprimer ce péage / parking de ${Number(e.amount).toFixed(2)} € ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(vehicleStore.activeVehicle.id, e.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
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
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openAddMaintModal() {
  editingMaintId.value = null
  detectedOdometer.value = null
  shouldClosePrevious.value = false
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
    amortization_mode: 'DISTANCE',
    coverage_km: 50000,
    coverage_months: 24,
    closes_maintenance_id: null,
    description: '',
  }
  showAddMaintModal.value = true
  checkOdometerForDate(maintForm.value.date)
}

function openEditMaintModal(m: any) {
  editingMaintId.value = m.id
  detectedOdometer.value = null
  shouldClosePrevious.value = Boolean(m.closes_maintenance_id)
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
    amortization_mode: m.amortization_mode || 'NONE',
    coverage_km: m.coverage_km ? Number(m.coverage_km) : 50000,
    coverage_months: m.coverage_months ? Number(m.coverage_months) : 24,
    closes_maintenance_id: m.closes_maintenance_id || null,
    description: m.description || '',
  }
  showAddMaintModal.value = true
}

async function handleDeleteMaint(m: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer la dépense',
    message: `Supprimer la dépense "${m.description}" de ${Number(m.amount).toFixed(2)} € ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteMaintenance(vehicleStore.activeVehicle.id, m.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleCreateMaint() {
  if (!vehicleStore.activeVehicle) return
  try {
    let closesId: string | null = null
    if (shouldClosePrevious.value && closeCandidateMaintenance.value) {
      closesId = closeCandidateMaintenance.value.id
    }

    const payload = {
      ...maintForm.value,
      ...currencyPayload(maintForm.value),
      amount: Number(maintForm.value.amount),
      odometer: maintForm.value.odometer ? Number(maintForm.value.odometer) : null,
      coverage_km:
        maintForm.value.amortization_mode === 'DISTANCE' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_km || 50000)
          : null,
      coverage_months:
        maintForm.value.amortization_mode === 'DURATION' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_months || 24)
          : null,
      closes_maintenance_id: closesId,
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
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// Next page of charges, so that older charges can also be completed or corrected
async function loadMoreCharges() {
  if (!vehicleStore.activeVehicle) return
  loadingMoreCharges.value = true
  try {
    const res = await api.getCharges(vehicleStore.activeVehicle.id, { page: chargesPage.value + 1, missingCost: missingCostOnly.value })
    chargesPage.value += 1
    charges.value = [...charges.value, ...res.charges]
    chargesTotal.value = res.total || 0
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    loadingMoreCharges.value = false
  }
}

// Drives selected for a toll that are older than the loaded recent drives
const selectedDrivesNotListed = computed(
  () => selectedDriveIds.value.filter((id) => !recentDrives.value.some((d) => d.id === id)).length
)

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
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteCharge(c: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer la recharge',
    message: `Supprimer cette recharge manuelle de ${c.kwh_added} kWh ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteCharge(vehicleStore.activeVehicle.id, c.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
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
                {{ CATEGORY_LABELS[m.category] || m.category }}
              </span>
              <span class="text-xs text-slate-400">{{ formatDate(m.date) }}</span>
              <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1">
                <Repeat class="w-3 h-3 text-pink-400" /> tous les {{ m.recurrence_interval_months }} mois
                <template v-if="m.recurrence_end_date">jusqu'au {{ formatDate(m.recurrence_end_date) }}</template>
              </span>
              <span v-else-if="m.amortization_mode === 'DISTANCE'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                Lissé sur {{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km
              </span>
              <span v-else-if="m.amortization_mode === 'DURATION'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20">
                Lissé sur {{ m.coverage_months || 24 }} mois
              </span>
              <span v-else-if="m.amortization_mode === 'HYBRID'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
                Lissé mixte ({{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km / {{ m.coverage_months || 24 }} mois)
              </span>
              <span v-if="m.closes_maintenance_id" class="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                Clôture la révision précédente
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
      <!-- Pre-TeslaMate Charges Banner if configured -->
      <div
        v-if="vehicleStore.activeVehicle?.pre_teslamate_kwh_100km && vehicleStore.activeVehicle?.pre_teslamate_eur_per_kwh"
        class="p-3 bg-sky-500/10 border border-sky-500/20 rounded-2xl flex items-center justify-between text-xs text-sky-300"
      >
        <div class="flex items-center gap-2">
          <Zap class="w-4 h-4 shrink-0 text-sky-400" />
          <span>
            Estimation avant TeslaMate active :
            <strong>{{ vehicleStore.activeVehicle.pre_teslamate_kwh_100km }} kWh/100km</strong> à
            <strong>{{ Number(vehicleStore.activeVehicle.pre_teslamate_eur_per_kwh).toFixed(4) }} €/kWh</strong>
            (intégrée automatiquement dans le TCO).
          </span>
        </div>
        <button
          @click="router.push('/vehicles')"
          class="shrink-0 font-medium underline hover:text-sky-200 transition-colors ml-2"
        >
          Modifier
        </button>
      </div>

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
        <div class="flex items-center justify-between text-xs text-slate-400 px-1">
          <span>{{ charges.length }} recharge(s) affichée(s) sur {{ chargesTotal }}</span>
          <button
            v-if="charges.length < chargesTotal"
            @click="loadMoreCharges"
            :disabled="loadingMoreCharges"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 disabled:opacity-50"
          >
            {{ loadingMoreCharges ? 'Chargement...' : 'Charger plus' }}
          </button>
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
              <label for="expense-toll-type" class="block text-xs font-semibold text-slate-300 mb-1">Type</label>
              <select id="expense-toll-type" v-model="tollForm.type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                <option value="TOLL">Péage</option>
                <option value="PARKING">Parking</option>
                <option value="FERRY">Ferry</option>
                <option value="OTHER">Autre</option>
              </select>
            </div>
            <div>
              <label for="toll-form-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
              <div class="flex gap-1.5">
                <input id="toll-form-amount" v-model="tollForm.amount" type="number" step="0.01" min="0.01" required placeholder="0.00" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
                <label for="toll-form-currency" class="sr-only">Devise</label>
                <select id="toll-form-currency" v-model="tollForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                  <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
                </select>
              </div>
            </div>
          </div>
          <div v-if="tollForm.currency !== 'EUR'">
            <label for="toll-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ tollForm.currency }} = ? €)</label>
            <input id="toll-form-fx-rate" v-model="tollForm.fx_rate" type="number" step="0.000001" min="0.000001" required placeholder="ex: 1.05" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <!-- Association à un/des trajets TeslaMate -->
          <div class="space-y-2 bg-slate-800/50 p-3.5 rounded-xl border border-slate-700/60">
            <span class="block text-xs font-semibold text-slate-200">
              Associer à un trajet TeslaMate
            </span>
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
              <label for="expense-selected-drive-id" class="block text-xs text-slate-400">Choisir le trajet :</label>
              <select id="expense-selected-drive-id"
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
                <span class="text-xs text-slate-400">Cocher les étapes composant le voyage :</span>
                <span class="text-[11px] text-amber-400 font-semibold">
                  {{ selectedDriveIds.length }} étape(s)<template v-if="selectedDrivesNotListed"> dont {{ selectedDrivesNotListed }} plus ancienne(s) que les 200 derniers trajets</template>
                </span>
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
              <label for="expense-toll-date" class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
              <input id="expense-toll-date" v-model="tollForm.date" type="datetime-local" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
            </div>
            <div>
              <label for="expense-toll-notes" class="block text-xs font-semibold text-slate-300 mb-1">Notes / Description</label>
              <input id="expense-toll-notes" v-model="tollForm.notes" placeholder="A10 Paris-Bordeaux..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
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
            <label for="expense-maint-category" class="block text-xs font-semibold text-slate-300 mb-1">Catégorie</label>
            <select id="expense-maint-category" v-model="maintForm.category" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
              <option value="MAINTENANCE">Entretien / Révision</option>
              <option value="REPAIR">Réparation / sinistre (franchise)</option>
              <option value="INSURANCE">Assurance (prime)</option>
              <option value="SUBSCRIPTION">Abonnement (Connectivité...)</option>
              <option value="TAX">Taxe / Carte grise</option>
              <option value="FINANCING">Autre financement (hors contrat du véhicule)</option>
              <option value="ACCESSORY">Accessoire</option>
              <option value="OTHER">Autre</option>
            </select>
          </div>
          <!-- Insurance: annual premium paid monthly -->
          <div v-if="maintForm.category === 'INSURANCE'" class="bg-indigo-500/10 border border-indigo-500/20 rounded-xl p-3 space-y-2">
            <div class="flex items-end gap-2">
              <div class="flex-1">
                <label for="expense-insurance-annual" class="block text-xs font-semibold text-indigo-200 mb-1">Prime annuelle (€)</label>
                <input
                  id="expense-insurance-annual"
                  v-model="insuranceAnnualPremium"
                  type="number"
                  step="0.01"
                  min="0"
                  placeholder="ex: 850"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
              <button type="button" @click="applyMonthlyPremium" class="px-3 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl">
                Mensualiser
              </button>
            </div>
            <p class="text-[11px] text-indigo-200/80">
              Crée une dépense récurrente mensuelle de prime / 12, à compter de la date de début de la couverture.
              L'assurance est un coût fixe dans le temps : elle n'est répartie au kilomètre que pour le coût d'un trajet, sur les kilomètres réellement parcourus.
            </p>
          </div>
          <p v-else-if="maintForm.category === 'FINANCING'" class="text-[11px] text-amber-300/90">
            Les loyers de LOA/LLD, l'apport et les intérêts d'un crédit sont générés automatiquement depuis « Acquisition & financement » du véhicule : ne les saisissez pas ici.
          </p>

          <div>
            <label for="expense-maint-description" class="block text-xs font-semibold text-slate-300 mb-1">Description</label>
            <input id="expense-maint-description" v-model="maintForm.description" required placeholder="ex: Remplacement filtre habitacle" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="expense-maint-date" class="block text-xs font-semibold text-slate-300 mb-1">Date</label>
              <input id="expense-maint-date" v-model="maintForm.date" type="date" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="maint-form-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
              <div class="flex gap-1.5">
                <input id="maint-form-amount" v-model="maintForm.amount" type="number" step="0.01" min="0.01" required class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
                <label for="maint-form-currency" class="sr-only">Devise</label>
                <select id="maint-form-currency" v-model="maintForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                  <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
                </select>
              </div>
            </div>
          </div>
          <div v-if="maintForm.currency !== 'EUR'">
            <label for="maint-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ maintForm.currency }} = ? €)</label>
            <input id="maint-form-fx-rate" v-model="maintForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <div class="flex items-center justify-between mb-1">
              <label for="expense-maint-odometer" class="block text-xs font-semibold text-slate-300">Odomètre (km)</label>
              <span v-if="detectingOdometer" class="text-[11px] text-slate-400">Détection TeslaMate...</span>
            </div>
            <input id="expense-maint-odometer" v-model.number="maintForm.odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            <div v-if="detectedOdometer !== null && detectedOdometer > 0" class="flex items-center justify-between text-[11px] text-emerald-400 mt-1">
              <span>✓ Détecté via TeslaMate : {{ Math.round(detectedOdometer) }} km</span>
              <button
                type="button"
                v-if="maintForm.odometer !== Math.round(detectedOdometer)"
                @click="maintForm.odometer = Math.round(detectedOdometer)"
                class="underline hover:text-emerald-300 transition-colors ml-2"
              >
                Appliquer
              </button>
            </div>
          </div>

          <!-- Lissage du coût pour dépenses non-récurrentes -->
          <div v-if="!maintForm.is_recurring" class="space-y-2.5 bg-slate-800/40 p-3.5 rounded-xl border border-slate-700/60">
            <span class="block text-xs font-semibold text-slate-200">
              Lissage du coût de revient au km
            </span>
            <p class="text-[11px] text-slate-400">
              Évite les pics artificiels sur la courbe mensuelle (€/km).
            </p>

            <div class="grid grid-cols-4 gap-1.5 pt-1">
              <button
                type="button"
                @click="maintForm.amortization_mode = 'NONE'"
                class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="maintForm.amortization_mode === 'NONE' ? 'bg-pink-500/20 text-pink-300 border-pink-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Immédiat
              </button>
              <button
                type="button"
                @click="maintForm.amortization_mode = 'DISTANCE'"
                class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="maintForm.amortization_mode === 'DISTANCE' ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Au km
              </button>
              <button
                type="button"
                @click="maintForm.amortization_mode = 'DURATION'"
                class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="maintForm.amortization_mode === 'DURATION' ? 'bg-purple-500/20 text-purple-300 border-purple-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                À la durée
              </button>
              <button
                type="button"
                @click="maintForm.amortization_mode = 'HYBRID'"
                class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
                :class="maintForm.amortization_mode === 'HYBRID' ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
              >
                Mixte
              </button>
            </div>

            <!-- Distance parameter -->
            <div v-if="maintForm.amortization_mode === 'DISTANCE' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
              <label for="maint-coverage-km" class="block text-xs text-slate-300 mb-1">Kilométrage couvert (km)</label>
              <input
                id="maint-coverage-km"
                v-model.number="maintForm.coverage_km"
                type="number"
                min="1000"
                step="1000"
                placeholder="50000"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
              />
            </div>

            <!-- Duration parameter -->
            <div v-if="maintForm.amortization_mode === 'DURATION' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
              <label for="maint-coverage-months" class="block text-xs text-slate-300 mb-1">Durée couverte (mois)</label>
              <input
                id="maint-coverage-months"
                v-model.number="maintForm.coverage_months"
                type="number"
                min="1"
                max="120"
                placeholder="24"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
              />
            </div>

            <!-- Clôture de la maintenance précédente -->
            <div v-if="closeCandidateMaintenance && maintForm.amortization_mode !== 'NONE'" class="pt-2 border-t border-slate-700/60">
              <div class="flex items-start gap-2">
                <input
                  id="close-candidate"
                  v-model="shouldClosePrevious"
                  type="checkbox"
                  class="mt-0.5 rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500"
                />
                <label for="close-candidate" class="text-xs text-slate-300 leading-snug cursor-pointer">
                  Clôturer la révision précédente en cours
                  <span class="block text-[11px] text-amber-400 font-normal">
                    {{ closeCandidateMaintenance.description }} ({{ formatDate(closeCandidateMaintenance.date) }} — {{ Number(closeCandidateMaintenance.amount).toFixed(2) }} €)
                  </span>
                </label>
              </div>
            </div>
          </div>

          <div class="space-y-2 pt-1">
            <div class="flex items-center gap-2">
              <input v-model="maintForm.is_recurring" type="checkbox" id="rec" class="rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500" />
              <label for="rec" class="text-xs text-slate-300 font-medium">Dépense récurrente</label>
            </div>
            <div v-if="maintForm.is_recurring" class="pt-1 grid grid-cols-2 gap-3">
              <div>
                <label for="maint-form-recurrence-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">Intervalle (mois)</label>
                <input id="maint-form-recurrence-interval-months" v-model.number="maintForm.recurrence_interval_months" type="number" min="1" max="120" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="maint-form-recurrence-end-date" class="block text-xs font-semibold text-slate-300 mb-1">Fin (optionnelle)</label>
                <input id="maint-form-recurrence-end-date" v-model="maintForm.recurrence_end_date" type="date" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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
                <label for="charge-form-date" class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
                <input id="charge-form-date" v-model="chargeForm.date" type="datetime-local" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
              </div>
              <div>
                <label for="charge-form-kwh-added" class="block text-xs font-semibold text-slate-300 mb-1">Énergie ajoutée (kWh)</label>
                <input id="charge-form-kwh-added" v-model="chargeForm.kwh_added" type="number" step="0.001" min="0.001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="charge-form-address" class="block text-xs font-semibold text-slate-300 mb-1">Lieu (optionnel)</label>
                <input id="charge-form-address" v-model="chargeForm.address" placeholder="Borne, domicile..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="charge-form-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre (optionnel)</label>
                <input id="charge-form-odometer" v-model="chargeForm.odometer" type="number" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
          </template>

          <div>
            <label for="charge-form-cost" class="block text-xs font-semibold text-slate-300 mb-1">Coût</label>
            <div class="flex gap-1.5">
              <input id="charge-form-cost" v-model="chargeForm.cost" type="number" step="0.01" min="0" required placeholder="0.00 si gratuite" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <label for="charge-form-currency" class="sr-only">Devise</label>
              <select id="charge-form-currency" v-model="chargeForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
          <div v-if="chargeForm.currency !== 'EUR'">
            <label for="charge-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ chargeForm.currency }} = ? €)</label>
            <input id="charge-form-fx-rate" v-model="chargeForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label for="charge-form-notes" class="block text-xs font-semibold text-slate-300 mb-1">Notes (optionnel)</label>
            <input id="charge-form-notes" v-model="chargeForm.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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
