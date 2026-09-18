<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useAuthStore } from '@/stores/auth'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import AppDatePicker from '@/components/AppDatePicker.vue'
import {
  Car,
  Plus,
  Trash2,
  Edit2,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  X,
  Gauge,
  Link2,
  FileText,
  Zap,
  Users,
  UserPlus,
  ShieldCheck,
  Eye,
  LogOut,
  ChevronLeft,
  ChevronRight,
  Check,
  Wallet,
  CreditCard,
  KeyRound,
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const authStore = useAuthStore()
const { showConfirm, showAlert } = useConfirm()
const showModal = ref(false)
const isEditing = ref(false)
const modalTestLoading = ref(false)
const modalTestResult = ref<{ success: boolean; status?: any; error?: string } | null>(null)
const cardTestResults = ref<Record<string, { loading: boolean; success?: boolean; status?: any; error?: string }>>({})
const editingId = ref<string | null>(null)

const form = ref({
  name: 'Tesla Model 3',
  powertrain: 'EV',
  vin: '',
  current_odometer: 0,
  teslamate_car_id: 1,
  teslamate_api_url: '',
  teslamate_auth_type: 'NONE',
  teslamate_api_key: '',
  teslamate_basic_user: '',
  teslamate_basic_pass: '',
  pre_teslamate_kwh_100km: null as number | null,
  pre_teslamate_eur_per_kwh: null as number | null,
})

// Ownership contracts (purchase, loan, LOA, LLD) by vehicle id
const ownerships = ref<Record<string, any | null>>({})
const showOwnershipModal = ref(false)
const ownershipVehicle = ref<any | null>(null)
const ownershipForm = ref(emptyOwnership())

// Odometer Checkpoints & Smoothing state
const showCheckpointsModal = ref(false)
const checkpointsVehicle = ref<any | null>(null)
const checkpoints = ref<any[]>([])
const loadingCheckpoints = ref(false)
const editingCheckpointId = ref<string | null>(null)
const checkpointForm = ref({
  date: new Date().toISOString().substring(0, 10),
  odometer: '',
  notes: '',
})

const ACQUISITION_LABELS: Record<string, string> = {
  CASH: 'Achat comptant',
  LOAN: 'Achat à crédit',
  LOA: 'LOA (location avec option d\'achat)',
  LLD: 'LLD (location longue durée)',
}

// Shared Vehicle Members state
const showMembersModal = ref(false)
const membersVehicle = ref<any | null>(null)
const members = ref<any[]>([])
const loadingMembers = ref(false)
const newMemberEmail = ref('')
const newMemberRole = ref<'EDITOR' | 'VIEWER'>('EDITOR')
const addingMember = ref(false)
const updatingMemberId = ref<string | null>(null)

async function openMembersModal(v: any) {
  membersVehicle.value = v
  newMemberEmail.value = ''
  newMemberRole.value = 'EDITOR'
  showMembersModal.value = true
  await loadMembers(v.id)
}

async function loadMembers(vehicleId: string) {
  loadingMembers.value = true
  try {
    members.value = await api.getVehicleMembers(vehicleId)
  } catch (err: any) {
    showAlert(`Erreur lors du chargement des membres : ${err.message}`, 'Erreur', 'danger')
  } finally {
    loadingMembers.value = false
  }
}

async function handleAddMember() {
  if (!membersVehicle.value || !newMemberEmail.value.trim()) return
  addingMember.value = true
  try {
    await api.addVehicleMember(membersVehicle.value.id, {
      email: newMemberEmail.value.trim(),
      role: newMemberRole.value,
    })
    newMemberEmail.value = ''
    await loadMembers(membersVehicle.value.id)
    showAlert('Membre ajouté avec succès !', 'Succès', 'info')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    addingMember.value = false
  }
}

async function handleUpdateMemberRole(m: any, newRole: string) {
  if (!membersVehicle.value || m.role === newRole) return
  updatingMemberId.value = m.user_id
  try {
    await api.updateVehicleMemberRole(membersVehicle.value.id, m.user_id, { role: newRole })
    await loadMembers(membersVehicle.value.id)
    showAlert('Rôle mis à jour avec succès !', 'Succès', 'info')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    updatingMemberId.value = null
  }
}

async function handleRemoveMember(m: any) {
  if (!membersVehicle.value) return
  const isSelf = authStore.user?.id === m.user_id
  const ok = await showConfirm({
    title: isSelf ? 'Quitter le véhicule partagé' : 'Retirer l\'accès au véhicule',
    message: isSelf
      ? `Êtes-vous sûr de vouloir quitter le véhicule ${membersVehicle.value.name} ? Vous n'aurez plus accès à ses données.`
      : `Retirer l'accès de ${m.user_email} à ce véhicule ?`,
    confirmText: isSelf ? 'Quitter' : 'Retirer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.removeVehicleMember(membersVehicle.value.id, m.user_id)
    if (isSelf) {
      showMembersModal.value = false
      await vehicleStore.fetchVehicles()
    } else {
      await loadMembers(membersVehicle.value.id)
    }
    showAlert(isSelf ? 'Vous avez quitté le véhicule' : 'Accès révoqué avec succès', 'Succès', 'info')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function emptyOwnership() {
  return {
    acquisition_type: 'CASH',
    start_date: new Date().toISOString().substring(0, 10),
    start_odometer: null as number | null,
    purchase_price: null as number | null,
    purchase_fees: null as number | null,
    incentives: null as number | null,
    expected_resale_value: null as number | null,
    expected_holding_months: null as number | null,
    loan_amount: null as number | null,
    loan_rate_pct: null as number | null,
    loan_duration_months: null as number | null,
    loan_fees: null as number | null,
    loan_insurance_monthly: null as number | null,
    lease_down_payment: null as number | null,
    lease_monthly_rent: null as number | null,
    lease_duration_months: null as number | null,
    lease_fees: null as number | null,
    lease_deposit: null as number | null,
    lease_km_allowance_per_year: null as number | null,
    lease_excess_km_price: null as number | null,
    lease_end_fees_estimate: null as number | null,
    lease_purchase_option_price: null as number | null,
    lease_includes_maintenance: false,
    lease_includes_insurance: false,
    lease_includes_tires: false,
    option_exercised_date: '',
    end_date: '',
    sale_price: null as number | null,
  }
}

// Empty numeric inputs are sent as null, never as ""
function nullIfEmpty(v: any) {
  return v === '' || v === undefined ? null : v
}

function toDateInput(v?: string | null) {
  return v ? new Date(v).toISOString().substring(0, 10) : ''
}

async function loadOwnerships() {
  const entries = await Promise.all(
    vehicleStore.vehicles.map(async (v) => {
      try {
        if (v.role && v.role !== 'OWNER') return [v.id, null] as const
        return [v.id, await api.getOwnership(v.id)] as const
      } catch {
        return [v.id, null] as const
      }
    })
  )
  ownerships.value = Object.fromEntries(entries)
}

onMounted(async () => {
  await vehicleStore.fetchVehicles()
  await loadOwnerships()
})

const isLease = computed(() => ['LOA', 'LLD'].includes(ownershipForm.value.acquisition_type))
const isPurchase = computed(() => ['CASH', 'LOAN'].includes(ownershipForm.value.acquisition_type))
const isOwnedPhase = computed(() => isPurchase.value || (ownershipForm.value.acquisition_type === 'LOA' && !!ownershipForm.value.option_exercised_date))

// Loan annuity preview
const loanPreview = computed(() => {
  const f = ownershipForm.value
  const p = Number(f.loan_amount) || 0
  const n = Number(f.loan_duration_months) || 0
  const r = (Number(f.loan_rate_pct) || 0) / 1200
  if (!p || !n) return null
  const payment = r === 0 ? p / n : (p * r) / (1 - Math.pow(1 + r, -n))
  const insurance = Number(f.loan_insurance_monthly) || 0
  return { payment, totalInterest: payment * n - p, totalCost: payment * n - p + insurance * n + (Number(f.loan_fees) || 0) }
})

// Lease total preview (cash paid over the contract, purchase option excluded)
const leasePreview = computed(() => {
  const f = ownershipForm.value
  const rent = Number(f.lease_monthly_rent) || 0
  const n = Number(f.lease_duration_months) || 0
  if (!rent || !n) return null
  const total = rent * n + (Number(f.lease_down_payment) || 0) + (Number(f.lease_fees) || 0) + (Number(f.lease_end_fees_estimate) || 0)
  const allowance = Number(f.lease_km_allowance_per_year) || 0
  return { total, perMonth: total / n, totalKm: (allowance * n) / 12 }
})

function ownershipSummary(o: any) {
  if (!o) return null
  const fmt = (v: number) => Number(v).toLocaleString('fr-FR', { maximumFractionDigits: 0 })
  if (o.acquisition_type === 'CASH' || o.acquisition_type === 'LOAN') {
    return `${ACQUISITION_LABELS[o.acquisition_type]} • ${fmt(o.purchase_price)} €`
  }
  return `${o.acquisition_type} • ${fmt(o.lease_monthly_rent)} €/mois sur ${o.lease_duration_months} mois`
}

const currentOwnershipStep = ref(1)

const OWNERSHIP_STEPS = [
  { step: 1, title: 'Mode & Début', description: 'Type de contrat et date' },
  { step: 2, title: 'Modalités Financières', description: 'Coûts et mensualités' },
  { step: 3, title: 'Conditions & Fin', description: 'Kilométrage, options et clôture' },
]

function validateOwnershipStep(step: number): boolean {
  const f = ownershipForm.value
  if (step === 1) {
    if (!f.acquisition_type) {
      showAlert("Veuillez sélectionner un mode d'acquisition", 'Champ requis', 'warning')
      return false
    }
    if (!f.start_date) {
      showAlert('Veuillez renseigner la date de début', 'Champ requis', 'warning')
      return false
    }
    return true
  }
  if (step === 2) {
    if (isPurchase.value) {
      if (f.purchase_price === null || f.purchase_price === undefined || Number(f.purchase_price) <= 0) {
        showAlert("Veuillez renseigner le prix d'achat TTC", 'Champ requis', 'warning')
        return false
      }
    }
    if (f.acquisition_type === 'LOAN') {
      if (!f.loan_amount || Number(f.loan_amount) <= 0) {
        showAlert('Veuillez renseigner le montant emprunté', 'Champ requis', 'warning')
        return false
      }
      if (f.loan_duration_months === null || Number(f.loan_duration_months) <= 0) {
        showAlert('Veuillez renseigner la durée du crédit en mois', 'Champ requis', 'warning')
        return false
      }
    }
    if (isLease.value) {
      if (f.lease_monthly_rent === null || f.lease_monthly_rent === undefined || Number(f.lease_monthly_rent) < 0) {
        showAlert('Veuillez renseigner le loyer mensuel', 'Champ requis', 'warning')
        return false
      }
      if (!f.lease_duration_months || Number(f.lease_duration_months) <= 0) {
        showAlert('Veuillez renseigner la durée de la location en mois', 'Champ requis', 'warning')
        return false
      }
    }
    return true
  }
  return true
}

function nextOwnershipStep() {
  if (validateOwnershipStep(currentOwnershipStep.value)) {
    if (currentOwnershipStep.value < 3) {
      currentOwnershipStep.value++
    }
  }
}

function prevOwnershipStep() {
  if (currentOwnershipStep.value > 1) {
    currentOwnershipStep.value--
  }
}

function openOwnershipModal(v: any) {
  ownershipVehicle.value = v
  currentOwnershipStep.value = 1
  const o = ownerships.value[v.id]
  ownershipForm.value = o
    ? {
        ...emptyOwnership(),
        ...o,
        start_date: toDateInput(o.start_date),
        option_exercised_date: toDateInput(o.option_exercised_date),
        end_date: toDateInput(o.end_date),
      }
    : { ...emptyOwnership(), start_odometer: v.current_odometer ? Math.round(v.current_odometer) : null }
  showOwnershipModal.value = true
}

async function handleSaveOwnership() {
  if (!ownershipVehicle.value) return
  if (!validateOwnershipStep(1) || !validateOwnershipStep(2)) return
  const f: any = { ...ownershipForm.value }
  for (const key of Object.keys(f)) {
    if (typeof f[key] !== 'boolean') f[key] = nullIfEmpty(f[key])
  }
  try {
    ownerships.value[ownershipVehicle.value.id] = await api.saveOwnership(ownershipVehicle.value.id, f)
    showOwnershipModal.value = false
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteOwnership() {
  if (!ownershipVehicle.value) return
  const ok = await showConfirm({
    title: "Supprimer le contrat d'acquisition",
    message: "Supprimer le contrat d'acquisition de ce véhicule ?",
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteOwnership(ownershipVehicle.value.id)
    ownerships.value[ownershipVehicle.value.id] = null
    showOwnershipModal.value = false
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// Pre-TeslaMate Energy state
const preTeslaMateForm = ref<{ kwh_100km: number | null; eur_per_kwh: number | null }>({
  kwh_100km: null,
  eur_per_kwh: null,
})
const savingPreTeslaMate = ref(false)
const checkpointsVehicleTco = ref<any | null>(null)

const preTeslaMatePreview = computed(() => {
  const kwh100 = Number(preTeslaMateForm.value.kwh_100km)
  const rate = Number(preTeslaMateForm.value.eur_per_kwh)
  if (!kwh100 || !rate || kwh100 <= 0 || rate <= 0) return null
  const distance = checkpointsVehicleTco.value?.pre_teslamate_distance_km ?? (checkpointsVehicleTco.value?.completeness?.untracked_distance_km || 0)
  if (distance <= 0) return null
  const kwh = (distance * kwh100) / 100
  const cost = kwh * rate
  return { distance, kwh, cost }
})

async function openCheckpointsModal(v: any) {
  checkpointsVehicle.value = v
  showCheckpointsModal.value = true
  preTeslaMateForm.value = {
    kwh_100km: v.pre_teslamate_kwh_100km ?? null,
    eur_per_kwh: v.pre_teslamate_eur_per_kwh ?? null,
  }
  resetCheckpointForm()
  await Promise.all([
    loadCheckpoints(v.id),
    api.getTCO(v.id).then(t => { checkpointsVehicleTco.value = t }).catch(() => { checkpointsVehicleTco.value = null }),
  ])
}

async function handleSavePreTeslaMateEnergy() {
  if (!checkpointsVehicle.value) return
  const kwh100 = preTeslaMateForm.value.kwh_100km != null ? Number(preTeslaMateForm.value.kwh_100km) : null
  const rate = preTeslaMateForm.value.eur_per_kwh != null ? Number(preTeslaMateForm.value.eur_per_kwh) : null
  if (kwh100 !== null && (kwh100 <= 0 || kwh100 > 100)) {
    showAlert('Consommation moyenne invalide (doit être comprise entre 1 et 100 kWh/100km)', 'Champ invalide', 'warning')
    return
  }
  if (rate !== null && (rate <= 0 || rate > 10)) {
    showAlert('Tarif électricité invalide (doit être compris entre 0.01 et 10 €/kWh)', 'Champ invalide', 'warning')
    return
  }
  savingPreTeslaMate.value = true
  try {
    const res = await api.updatePreTeslaMateEnergy(checkpointsVehicle.value.id, {
      pre_teslamate_kwh_100km: kwh100,
      pre_teslamate_eur_per_kwh: rate,
    })
    checkpointsVehicle.value.pre_teslamate_kwh_100km = res.pre_teslamate_kwh_100km
    checkpointsVehicle.value.pre_teslamate_eur_per_kwh = res.pre_teslamate_eur_per_kwh
    await vehicleStore.fetchVehicles()
    checkpointsVehicleTco.value = await api.getTCO(checkpointsVehicle.value.id).catch(() => null)
    vehicleStore.lastSyncTimestamp = Date.now()
    showAlert('Coûts de recharge avant TeslaMate enregistrés avec succès !', 'Succès', 'info')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    savingPreTeslaMate.value = false
  }
}

async function handleClearPreTeslaMateEnergy() {
  if (!checkpointsVehicle.value) return
  preTeslaMateForm.value = { kwh_100km: null, eur_per_kwh: null }
  await handleSavePreTeslaMateEnergy()
}

async function loadCheckpoints(vehicleId: string) {
  loadingCheckpoints.value = true
  try {
    checkpoints.value = await api.getOdometerCheckpoints(vehicleId)
  } catch (err: any) {
    showAlert(`Erreur de chargement des relevés : ${err.message}`, 'Erreur', 'danger')
  } finally {
    loadingCheckpoints.value = false
  }
}

function resetCheckpointForm() {
  editingCheckpointId.value = null
  checkpointForm.value = {
    date: new Date().toISOString().substring(0, 10),
    odometer: checkpointsVehicle.value?.current_odometer ? String(Math.round(checkpointsVehicle.value.current_odometer)) : '',
    notes: '',
  }
}

function startEditCheckpoint(cp: any) {
  editingCheckpointId.value = cp.id
  checkpointForm.value = {
    date: new Date(cp.date).toISOString().substring(0, 10),
    odometer: String(Math.round(cp.odometer)),
    notes: cp.notes || '',
  }
}

async function handleSaveCheckpoint() {
  if (!checkpointsVehicle.value) return
  const odo = Number(checkpointForm.value.odometer)
  if (Number.isNaN(odo) || odo < 0) {
    showAlert("Veuillez saisir un kilométrage d'odomètre valide", 'Champ requis', 'warning')
    return
  }
  try {
    const payload = {
      date: checkpointForm.value.date,
      odometer: odo,
      notes: checkpointForm.value.notes.trim() || undefined,
    }
    if (editingCheckpointId.value) {
      await api.updateOdometerCheckpoint(checkpointsVehicle.value.id, editingCheckpointId.value, payload)
    } else {
      await api.createOdometerCheckpoint(checkpointsVehicle.value.id, payload)
    }
    resetCheckpointForm()
    await loadCheckpoints(checkpointsVehicle.value.id)
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteCheckpoint(cp: any) {
  if (!checkpointsVehicle.value) return
  const ok = await showConfirm({
    title: 'Supprimer le relevé kilométrique',
    message: `Supprimer le relevé de ${Math.round(cp.odometer).toLocaleString('fr-FR')} km du ${new Date(cp.date).toLocaleDateString('fr-FR')} ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteOdometerCheckpoint(checkpointsVehicle.value.id, cp.id)
    await loadCheckpoints(checkpointsVehicle.value.id)
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openCreateModal() {
  isEditing.value = false
  editingId.value = null
  modalTestResult.value = null
  form.value = {
    name: 'Tesla Model 3',
    powertrain: 'EV',
    vin: '',
    current_odometer: 0,
    teslamate_car_id: 1,
    teslamate_api_url: '',
    teslamate_auth_type: 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: '',
    teslamate_basic_pass: '',
    pre_teslamate_kwh_100km: null,
    pre_teslamate_eur_per_kwh: null,
  }
  showModal.value = true
}

function openEditModal(v: any) {
  isEditing.value = true
  editingId.value = v.id
  modalTestResult.value = null
  form.value = {
    name: v.name,
    powertrain: v.powertrain || 'EV',
    vin: v.vin || '',
    current_odometer: v.current_odometer ? Math.round(v.current_odometer) : 0,
    teslamate_car_id: v.teslamate_car_id || 1,
    teslamate_api_url: v.teslamate_api_url || '',
    teslamate_auth_type: v.teslamate_auth_type || 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: v.teslamate_basic_user || '',
    teslamate_basic_pass: '',
    pre_teslamate_kwh_100km: v.pre_teslamate_kwh_100km ?? null,
    pre_teslamate_eur_per_kwh: v.pre_teslamate_eur_per_kwh ?? null,
  }
  showModal.value = true
}

async function handleSave() {
  try {
    if (isEditing.value && editingId.value) {
      await api.updateVehicle(editingId.value, form.value)
    } else {
      await api.createVehicle(form.value)
    }
    showModal.value = false
    await vehicleStore.fetchVehicles()
    await loadOwnerships()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDelete(id: string) {
  const ok = await showConfirm({
    title: 'Supprimer le véhicule',
    message: 'Supprimer ce véhicule et tout son historique ? Cette action est irréversible.',
    confirmText: 'Supprimer définitivement',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteVehicle(id)
    await vehicleStore.fetchVehicles()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function testModalConnection() {
  if (!form.value.teslamate_api_url) {
    modalTestResult.value = { success: false, error: "Veuillez d'abord renseigner l'URL de l'API TeslaMate" }
    return
  }
  modalTestLoading.value = true
  modalTestResult.value = null
  try {
    const res = await api.testTeslaMateRaw(form.value)
    modalTestResult.value = { success: true, status: res.status }
  } catch (err: any) {
    modalTestResult.value = { success: false, error: err.message }
  } finally {
    modalTestLoading.value = false
  }
}

async function testCardConnection(id: string) {
  cardTestResults.value[id] = { loading: true }
  try {
    const res = await api.testTeslaMate(id)
    cardTestResults.value[id] = { loading: false, success: true, status: res.status }
  } catch (err: any) {
    cardTestResults.value[id] = { loading: false, success: false, error: err.message }
  }
}

function clearCardTestResult(id: string) {
  delete cardTestResults.value[id]
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Gestion des Véhicules</h2>
        <p class="text-sm text-slate-400">Configurez vos véhicules et la synchronisation avec TeslaMateApi</p>
      </div>

      <button
        @click="openCreateModal"
        class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
      >
        <Plus class="w-3.5 h-3.5" />
        Ajouter un véhicule
      </button>
    </div>

    <!-- Vehicles Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div
        v-for="v in vehicleStore.vehicles"
        :key="v.id"
        class="bg-slate-900 border border-slate-800 p-5 rounded-2xl relative transition-all"
        :class="{ 'border-rose-500/40 shadow-lg shadow-rose-500/5': vehicleStore.activeVehicleId === v.id }"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-3">
            <div class="p-3 bg-slate-800 rounded-xl text-rose-400">
              <Car class="w-6 h-6" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-base font-bold text-white">{{ v.name }}</h3>
                <span
                  v-if="v.role"
                  class="px-2 py-0.5 text-[10px] font-semibold rounded-full uppercase tracking-wider"
                  :class="{
                    'bg-amber-500/10 text-amber-400 border border-amber-500/20': v.role === 'OWNER',
                    'bg-sky-500/10 text-sky-400 border border-sky-500/20': v.role === 'EDITOR',
                    'bg-slate-700/50 text-slate-400 border border-slate-600/30': v.role === 'VIEWER',
                  }"
                >
                  {{ v.role === 'OWNER' ? 'Propriétaire' : v.role === 'EDITOR' ? 'Co-conducteur' : 'Lecteur' }}
                </span>
              </div>
              <p v-if="v.vin" class="text-xs text-slate-400 font-mono">{{ v.vin }}</p>
            </div>
          </div>

          <div v-if="v.role === 'OWNER'" class="flex items-center gap-1">
            <button
              @click="openEditModal(v)"
              class="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
              title="Modifier"
            >
              <Edit2 class="w-4 h-4" />
            </button>
            <button
              @click="handleDelete(v.id)"
              class="p-2 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
              title="Supprimer"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Telemetry & Stats -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mt-4 pt-4 border-t border-slate-800 text-xs">
          <div>
            <span class="text-slate-400">Odomètre actuel</span>
            <p class="text-sm font-bold text-slate-200 flex items-center gap-1.5 mt-0.5">
              <Gauge class="w-3.5 h-3.5 text-rose-400" />
              {{ Math.round(v.current_odometer).toLocaleString('fr-FR') }} km
            </p>
          </div>

          <div v-if="v.role === 'OWNER'">
            <span class="text-slate-400 block mb-1">Acquisition & Contrat</span>
            <button
              type="button"
              @click="openOwnershipModal(v)"
              class="group text-left inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl text-xs font-semibold transition-all border"
              :class="
                ownerships[v.id]
                  ? 'bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-300 border-indigo-500/30'
                  : 'bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border-amber-500/30'
              "
              :title="ownerships[v.id] ? 'Cliquer pour modifier les termes du contrat' : 'Cliquer pour configurer l\'achat ou la location (LOA/LLD)'"
            >
              <FileText v-if="ownerships[v.id]" class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
              <Plus v-else class="w-3.5 h-3.5 text-amber-400 shrink-0" />
              <span>{{ ownerships[v.id] ? ownershipSummary(ownerships[v.id]) : 'Renseigner le contrat' }}</span>
              <Pencil v-if="ownerships[v.id]" class="w-3 h-3 text-indigo-400 opacity-60 group-hover:opacity-100 shrink-0 ml-0.5" />
            </button>
          </div>
          <div v-else>
            <span class="text-slate-400">Accès véhicule</span>
            <p class="text-xs font-semibold text-slate-300 mt-1">
              {{ v.role === 'EDITOR' ? 'Éditeur (Co-conducteur)' : 'Lecteur seul' }}
            </p>
          </div>

          <div v-if="v.powertrain === 'ICE'">
            <span class="text-slate-400">Motorisation</span>
            <p class="text-sm font-semibold mt-0.5 text-amber-300">Thermique · pleins saisis à la main</p>
          </div>
          <div v-if="v.powertrain !== 'ICE'">
            <span class="text-slate-400">Connexion TeslaMate</span>
            <p
              class="text-sm font-semibold mt-0.5"
              :class="
                cardTestResults[v.id]?.success
                  ? 'text-emerald-400'
                  : cardTestResults[v.id]?.error
                  ? 'text-rose-400'
                  : v.teslamate_api_url
                  ? 'text-emerald-400/80'
                  : 'text-slate-500'
              "
            >
              {{
                v.role === 'OWNER'
                  ? (cardTestResults[v.id]?.success
                    ? 'En ligne'
                    : cardTestResults[v.id]?.error
                    ? 'Erreur de connexion'
                    : v.teslamate_api_url
                    ? 'Configurée'
                    : 'Non configurée')
                  : 'Gérée par l\'administrateur'
              }}
            </p>
          </div>

          <div v-if="v.powertrain !== 'ICE'">
            <span class="text-slate-400">Recharge avant TM</span>
            <p v-if="v.pre_teslamate_kwh_100km && v.pre_teslamate_eur_per_kwh" class="text-xs font-semibold text-sky-400 flex items-center gap-1 mt-1">
              <Zap class="w-3.5 h-3.5 text-sky-400" />
              {{ v.pre_teslamate_kwh_100km }} kWh/100km • {{ v.pre_teslamate_eur_per_kwh }} €/kWh
            </p>
            <p v-else class="text-xs text-slate-500 mt-1">Non configurée</p>
          </div>
        </div>

        <!-- Actions -->
        <div class="mt-4 pt-3 flex items-center justify-between gap-2 flex-wrap">
          <button
            @click="openMembersModal(v)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
            title="Gérer les accès et co-conducteurs"
          >
            <Users class="w-3.5 h-3.5 text-violet-400" />
            <span>Partage & Accès</span>
          </button>
          <button
            v-if="v.role === 'OWNER'"
            @click="openOwnershipModal(v)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
          >
            <FileText class="w-3.5 h-3.5 text-indigo-400" />
            <span>Acquisition & financement</span>
          </button>
          <button
            @click="openCheckpointsModal(v)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
            title="Relevés manuels au compteur pour lissage kilométrique"
          >
            <Gauge class="w-3.5 h-3.5 text-cyan-400" />
            <span>Relevés kilométriques</span>
          </button>
          <button
            v-if="v.role === 'OWNER' && v.teslamate_api_url"
            @click="testCardConnection(v.id)"
            :disabled="cardTestResults[v.id]?.loading"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
          >
            <RefreshCw v-if="cardTestResults[v.id]?.loading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
            <Link2 v-else class="w-3.5 h-3.5" />
            <span>{{ cardTestResults[v.id]?.loading ? 'Test...' : 'Tester l\'API' }}</span>
          </button>

          <button
            v-if="vehicleStore.activeVehicleId !== v.id"
            @click="vehicleStore.setActiveVehicle(v.id)"
            class="ml-auto px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-xs text-rose-400 font-semibold rounded-lg"
          >
            Sélectionner
          </button>
          <span v-else class="ml-auto text-xs font-semibold text-rose-400 px-3 py-1.5 bg-rose-500/10 rounded-lg">
            Véhicule actif
          </span>
        </div>

        <!-- Test feedback per card -->
        <div
          v-if="cardTestResults[v.id] && !cardTestResults[v.id].loading"
          class="mt-3 p-3 rounded-xl text-xs flex items-start justify-between gap-2"
          :class="cardTestResults[v.id].success ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
        >
          <div class="flex items-start gap-2">
            <CheckCircle2 v-if="cardTestResults[v.id].success" class="w-4 h-4 shrink-0 text-emerald-400 mt-0.5" />
            <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
            <div>
              <span v-if="cardTestResults[v.id].success">
                Connexion réussie ! Statut : {{ cardTestResults[v.id].status?.state || 'En ligne' }} ({{ Math.round(cardTestResults[v.id].status?.odometer || 0).toLocaleString('fr-FR') }} km)
              </span>
              <span v-else>{{ cardTestResults[v.id].error }}</span>
            </div>
          </div>
          <button @click="clearCardTestResult(v.id)" class="text-slate-400 hover:text-white p-0.5">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Modal : Add/Edit Vehicle -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white">{{ isEditing ? 'Modifier le véhicule' : 'Ajouter un véhicule' }}</h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="vehicle-modal-form" @submit.prevent="handleSave" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-3.5">
          <div>
            <label for="vehicle-name" class="block text-xs font-semibold text-slate-300 mb-1">Nom du véhicule</label>
            <input id="vehicle-name" v-model="form.name" required placeholder="ex: Tesla Model Y LR" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <label for="vehicle-powertrain" class="block text-xs font-semibold text-slate-300 mb-1">Motorisation</label>
            <select id="vehicle-powertrain" v-model="form.powertrain" :disabled="isEditing" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50">
              <option value="EV">Électrique (suivi TeslaMate possible)</option>
              <option value="ICE">Thermique (saisie manuelle des pleins)</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1">VIN (optionnel)</label>
              <input id="vehicle-vin" v-model="form.vin" placeholder="5YJ3E1EB..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-current-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ form.powertrain === 'ICE' ? 'Kilométrage actuel (km)' : 'Odomètre initial (km)' }}</label>
              <input
                id="vehicle-current-odometer"
                v-model.number="form.current_odometer"
                type="number"
                step="1"
                :disabled="!!form.teslamate_api_url"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
          </div>

          <!-- Pre-TeslaMate energy section -->
          <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
            <h4 class="text-xs font-bold text-sky-400 uppercase tracking-wider">Recharge avant TeslaMate (Optionnel)</h4>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-pre-kwh" class="block text-xs font-semibold text-slate-300 mb-1">Conso (kWh/100km)</label>
                <input id="vehicle-pre-kwh" v-model.number="form.pre_teslamate_kwh_100km" type="number" step="0.1" min="1" max="100" placeholder="ex: 16.5" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="vehicle-pre-rate" class="block text-xs font-semibold text-slate-300 mb-1">Tarif (€/kWh)</label>
                <input id="vehicle-pre-rate" v-model.number="form.pre_teslamate_eur_per_kwh" type="number" step="0.0001" min="0.01" max="5" placeholder="ex: 0.22" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
          </div>

          <!-- TeslaMate API Section -->
          <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
            <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">Connexion TeslaMateApi (Optionnelle)</h4>

            <div>
              <label for="vehicle-teslamate-api-url" class="block text-xs font-semibold text-slate-300 mb-1">URL de base teslamateapi</label>
              <input id="vehicle-teslamate-api-url" v-model="form.teslamate_api_url" placeholder="ex: http://192.168.1.50:8080 ou http://host.docker.internal:8080" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-teslamate-car-id" class="block text-xs font-semibold text-slate-300 mb-1">ID Voiture dans TeslaMate</label>
                <input id="vehicle-teslamate-car-id" v-model.number="form.teslamate_car_id" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="vehicle-teslamate-auth-type" class="block text-xs font-semibold text-slate-300 mb-1">Mode d'authentification</label>
                <select id="vehicle-teslamate-auth-type" v-model="form.teslamate_auth_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                  <option value="NONE">Aucun (LAN)</option>
                  <option value="BEARER">Token Bearer (API_TOKEN)</option>
                  <option value="BASIC">HTTP Basic Auth</option>
                </select>
              </div>
            </div>

            <div v-if="form.teslamate_auth_type === 'BEARER'">
              <label for="vehicle-teslamate-api-key" class="block text-xs font-semibold text-slate-300 mb-1">Clé / Token API TeslaMate</label>
              <input id="vehicle-teslamate-api-key" v-model="form.teslamate_api_key" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>

            <div v-if="form.teslamate_auth_type === 'BASIC'" class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-teslamate-basic-user" class="block text-xs font-semibold text-slate-300 mb-1">Utilisateur Basic Auth</label>
                <input id="vehicle-teslamate-basic-user" v-model="form.teslamate_basic_user" placeholder="admin" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="vehicle-teslamate-basic-pass" class="block text-xs font-semibold text-slate-300 mb-1">Mot de passe Basic Auth</label>
                <input id="vehicle-teslamate-basic-pass" v-model="form.teslamate_basic_pass" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>

            <!-- Test Connection inside modal -->
            <div v-if="form.teslamate_api_url" class="pt-2">
              <button
                type="button"
                @click="testModalConnection"
                :disabled="modalTestLoading"
                class="w-full py-2 px-3 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center justify-center gap-2 border border-slate-700 disabled:opacity-50 transition-colors"
              >
                <RefreshCw v-if="modalTestLoading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
                <Link2 v-else class="w-3.5 h-3.5 text-rose-400" />
                <span>{{ modalTestLoading ? 'Test de connexion en cours...' : 'Tester la connexion TeslaMate' }}</span>
              </button>

              <div
                v-if="modalTestResult"
                class="mt-2.5 p-3 rounded-xl text-xs flex items-start gap-2"
                :class="modalTestResult.success ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
              >
                <CheckCircle2 v-if="modalTestResult.success" class="w-4 h-4 shrink-0 text-emerald-400 mt-0.5" />
                <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
                <div class="flex-1">
                  <div v-if="modalTestResult.success">
                    <strong class="font-semibold">Connexion réussie !</strong>
                    <p class="text-[11px] text-emerald-200/80 mt-0.5">
                      Statut : {{ modalTestResult.status?.state || 'En ligne' }} • Odomètre : {{ Math.round(modalTestResult.status?.odometer || 0).toLocaleString('fr-FR') }} km
                    </p>
                  </div>
                  <div v-else>
                    <strong class="font-semibold">Échec de la connexion :</strong>
                    <p class="text-[11px] text-rose-200/90 mt-0.5">{{ modalTestResult.error }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="vehicle-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Ownership contract (3-step Wizard) -->
    <div
      v-if="showOwnershipModal && ownershipVehicle"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showOwnershipModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <!-- Modal Header -->
        <div class="px-5 py-4 border-b border-slate-800/80 shrink-0 bg-slate-900/95">
          <div class="flex items-center justify-between">
            <h3 class="text-base font-bold text-white flex items-center gap-2 truncate pr-2">
              <FileText class="w-5 h-5 text-indigo-400 shrink-0" />
              <span class="truncate">Acquisition & financement — {{ ownershipVehicle.name }}</span>
            </h3>
            <button @click="showOwnershipModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0">
              <X class="w-5 h-5" />
            </button>
          </div>

          <!-- Wizard Stepper Indicator -->
          <div class="grid grid-cols-3 gap-2 mt-4 pt-3 border-t border-slate-800/60">
            <button
              v-for="s in OWNERSHIP_STEPS"
              :key="s.step"
              type="button"
              @click="s.step < currentOwnershipStep ? currentOwnershipStep = s.step : null"
              :disabled="s.step > currentOwnershipStep"
              class="flex items-center gap-2 p-1.5 rounded-xl text-left transition-colors"
              :class="[
                currentOwnershipStep === s.step
                  ? 'bg-indigo-500/15 border border-indigo-500/40 text-indigo-300'
                  : currentOwnershipStep > s.step
                  ? 'bg-slate-800/60 text-slate-300 hover:bg-slate-800 cursor-pointer'
                  : 'bg-slate-900/40 text-slate-500 opacity-60 cursor-not-allowed'
              ]"
            >
              <div
                class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold shrink-0 transition-colors"
                :class="[
                  currentOwnershipStep === s.step
                    ? 'bg-indigo-500 text-white shadow-sm shadow-indigo-500/40'
                    : currentOwnershipStep > s.step
                    ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                    : 'bg-slate-800 text-slate-500 border border-slate-700'
                ]"
              >
                <Check v-if="currentOwnershipStep > s.step" class="w-3.5 h-3.5" />
                <span v-else>{{ s.step }}</span>
              </div>
              <div class="min-w-0 hidden sm:block">
                <div class="text-xs font-semibold truncate">{{ s.title }}</div>
                <div class="text-[10px] text-slate-400 truncate">{{ s.description }}</div>
              </div>
            </button>
          </div>
        </div>

        <!-- Wizard Content -->
        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <!-- STEP 1: Acquisition Type & Dates -->
          <div v-show="currentOwnershipStep === 1" class="space-y-4">
            <div>
              <span class="block text-xs font-semibold text-slate-300 mb-2">Mode d'acquisition</span>
              <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
                <button
                  type="button"
                  @click="ownershipForm.acquisition_type = 'CASH'"
                  class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                  :class="[
                    ownershipForm.acquisition_type === 'CASH'
                      ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                      : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                  ]"
                >
                  <div class="flex items-center justify-between mb-2">
                    <Wallet class="w-5 h-5 text-emerald-400" />
                    <span v-if="ownershipForm.acquisition_type === 'CASH'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                  </div>
                  <div>
                    <div class="text-xs font-bold text-white">Comptant</div>
                    <div class="text-[10px] text-slate-400">Achat direct</div>
                  </div>
                </button>

                <button
                  type="button"
                  @click="ownershipForm.acquisition_type = 'LOAN'"
                  class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                  :class="[
                    ownershipForm.acquisition_type === 'LOAN'
                      ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                      : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                  ]"
                >
                  <div class="flex items-center justify-between mb-2">
                    <CreditCard class="w-5 h-5 text-indigo-400" />
                    <span v-if="ownershipForm.acquisition_type === 'LOAN'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                  </div>
                  <div>
                    <div class="text-xs font-bold text-white">Crédit</div>
                    <div class="text-[10px] text-slate-400">Prêt bancaire</div>
                  </div>
                </button>

                <button
                  type="button"
                  @click="ownershipForm.acquisition_type = 'LOA'"
                  class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                  :class="[
                    ownershipForm.acquisition_type === 'LOA'
                      ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                      : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                  ]"
                >
                  <div class="flex items-center justify-between mb-2">
                    <KeyRound class="w-5 h-5 text-amber-400" />
                    <span v-if="ownershipForm.acquisition_type === 'LOA'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                  </div>
                  <div>
                    <div class="text-xs font-bold text-white">LOA</div>
                    <div class="text-[10px] text-slate-400">Option d'achat</div>
                  </div>
                </button>

                <button
                  type="button"
                  @click="ownershipForm.acquisition_type = 'LLD'"
                  class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                  :class="[
                    ownershipForm.acquisition_type === 'LLD'
                      ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                      : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                  ]"
                >
                  <div class="flex items-center justify-between mb-2">
                    <RefreshCw class="w-5 h-5 text-sky-400" />
                    <span v-if="ownershipForm.acquisition_type === 'LLD'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                  </div>
                  <div>
                    <div class="text-xs font-bold text-white">LLD</div>
                    <div class="text-[10px] text-slate-400">Longue durée</div>
                  </div>
                </button>
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
              <div>
                <label for="own-start-date" class="block text-xs font-semibold text-slate-300 mb-1">
                  {{ isLease ? 'Début du contrat de location' : "Date d'achat du véhicule" }} <span class="text-rose-400">*</span>
                </label>
                <AppDatePicker id="own-start-date" v-model="ownershipForm.start_date" required size="sm" />
              </div>
              <div>
                <label for="own-start-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre au début (km)</label>
                <input id="own-start-odometer" v-model.number="ownershipForm.start_odometer" type="number" min="0" placeholder="ex: 0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>

          <!-- STEP 2: Financial Terms -->
          <div v-show="currentOwnershipStep === 2" class="space-y-4">
            <!-- Purchase terms -->
            <div v-if="isPurchase" class="space-y-3">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                <Wallet class="w-4 h-4" />
                <span>Modalités d'achat</span>
              </h4>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label for="own-purchase-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix d'achat TTC (€) <span class="text-rose-400">*</span></label>
                  <input id="own-purchase-price" v-model.number="ownershipForm.purchase_price" type="number" step="0.01" min="0" required placeholder="ex: 42000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-purchase-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais annexes (carte grise, mise en route) (€)</label>
                  <input id="own-purchase-fees" v-model.number="ownershipForm.purchase_fees" type="number" step="0.01" min="0" placeholder="ex: 350" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-incentives" class="block text-xs font-semibold text-slate-300 mb-1">Bonus et aides (€)</label>
                  <input id="own-incentives" v-model.number="ownershipForm.incentives" type="number" step="0.01" min="0" placeholder="ex: 4000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>
            </div>

            <!-- Loan specific terms -->
            <div v-if="ownershipForm.acquisition_type === 'LOAN'" class="space-y-3 pt-3 border-t border-slate-800">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                <CreditCard class="w-4 h-4" />
                <span>Modalités de l'emprunt</span>
              </h4>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label for="own-loan-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant emprunté (€) <span class="text-rose-400">*</span></label>
                  <input id="own-loan-amount" v-model.number="ownershipForm.loan_amount" type="number" step="0.01" min="0" required placeholder="ex: 30000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-loan-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux annuel (%) <span class="text-rose-400">*</span></label>
                  <input id="own-loan-rate" v-model.number="ownershipForm.loan_rate_pct" type="number" step="0.001" min="0" max="30" required placeholder="ex: 3.8" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-loan-duration" class="block text-xs font-semibold text-slate-300 mb-1">Durée (mois) <span class="text-rose-400">*</span></label>
                  <input id="own-loan-duration" v-model.number="ownershipForm.loan_duration_months" type="number" min="1" max="360" required placeholder="ex: 60" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-loan-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de dossier (€)</label>
                  <input id="own-loan-fees" v-model.number="ownershipForm.loan_fees" type="number" step="0.01" min="0" placeholder="ex: 200" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-loan-insurance" class="block text-xs font-semibold text-slate-300 mb-1">Assurance emprunteur (€/mois)</label>
                  <input id="own-loan-insurance" v-model.number="ownershipForm.loan_insurance_monthly" type="number" step="0.01" min="0" placeholder="ex: 15" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>

              <!-- Loan Live Preview Card -->
              <div v-if="loanPreview" class="p-3 bg-slate-800/70 border border-indigo-500/20 rounded-xl space-y-1 text-xs">
                <div class="flex items-center justify-between text-white font-semibold">
                  <span>Mensualité estimée :</span>
                  <span class="text-indigo-300 font-bold text-sm">{{ loanPreview.payment.toFixed(2) }} € / mois</span>
                </div>
                <div class="flex items-center justify-between text-slate-400 text-[11px]">
                  <span>Total des intérêts bancaires :</span>
                  <span>{{ loanPreview.totalInterest.toFixed(2) }} €</span>
                </div>
                <div class="flex items-center justify-between text-slate-400 text-[11px]">
                  <span>Coût total du crédit (intérêts + frais + assurance) :</span>
                  <span class="text-slate-200 font-medium">{{ loanPreview.totalCost.toFixed(2) }} €</span>
                </div>
              </div>
            </div>

            <!-- Lease specific terms -->
            <div v-if="isLease" class="space-y-3">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                <RefreshCw class="w-4 h-4" />
                <span>Modalités de location ({{ ownershipForm.acquisition_type }})</span>
              </h4>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label for="own-lease-down" class="block text-xs font-semibold text-slate-300 mb-1">Apport / 1er loyer majoré (€)</label>
                  <input id="own-lease-down" v-model.number="ownershipForm.lease_down_payment" type="number" step="0.01" min="0" placeholder="ex: 3000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-rent" class="block text-xs font-semibold text-slate-300 mb-1">Loyer mensuel (€) <span class="text-rose-400">*</span></label>
                  <input id="own-lease-rent" v-model.number="ownershipForm.lease_monthly_rent" type="number" step="0.01" min="0" required placeholder="ex: 450" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-duration" class="block text-xs font-semibold text-slate-300 mb-1">Durée (mois) <span class="text-rose-400">*</span></label>
                  <input id="own-lease-duration" v-model.number="ownershipForm.lease_duration_months" type="number" min="1" max="360" required placeholder="ex: 36" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de dossier (€)</label>
                  <input id="own-lease-fees" v-model.number="ownershipForm.lease_fees" type="number" step="0.01" min="0" placeholder="ex: 150" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-deposit" class="block text-xs font-semibold text-slate-300 mb-1">Dépôt de garantie remboursable (€)</label>
                  <input id="own-lease-deposit" v-model.number="ownershipForm.lease_deposit" type="number" step="0.01" min="0" placeholder="ex: 500" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>

              <!-- Lease Live Preview Card -->
              <div v-if="leasePreview" class="p-3 bg-slate-800/70 border border-indigo-500/20 rounded-xl space-y-1 text-xs">
                <div class="flex items-center justify-between text-white font-semibold">
                  <span>Coût total des loyers engagés :</span>
                  <span class="text-indigo-300 font-bold text-sm">{{ leasePreview.total.toFixed(2) }} €</span>
                </div>
                <div class="flex items-center justify-between text-slate-400 text-[11px]">
                  <span>Moyenne lissée sur la durée :</span>
                  <span>{{ leasePreview.perMonth.toFixed(2) }} € / mois</span>
                </div>
                <div v-if="leasePreview.totalKm" class="flex items-center justify-between text-slate-400 text-[11px]">
                  <span>Kilométrage total inclus sur le bail :</span>
                  <span class="text-slate-200 font-medium">{{ Math.round(leasePreview.totalKm).toLocaleString('fr-FR') }} km</span>
                </div>
              </div>
            </div>
          </div>

          <!-- STEP 3: Usage, Buyout, Holding & End of Ownership -->
          <div v-show="currentOwnershipStep === 3" class="space-y-4">
            <!-- Lease Conditions & Buyout -->
            <div v-if="isLease" class="space-y-3">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Forfait kilométrique & Inclusions</h4>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label for="own-lease-allowance" class="block text-xs font-semibold text-slate-300 mb-1">Forfait kilométrique (km/an)</label>
                  <input id="own-lease-allowance" v-model.number="ownershipForm.lease_km_allowance_per_year" type="number" min="0" placeholder="ex: 15000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-excess" class="block text-xs font-semibold text-slate-300 mb-1">Prix du km supplémentaire (€/km)</label>
                  <input id="own-lease-excess" v-model.number="ownershipForm.lease_excess_km_price" type="number" step="0.001" min="0" max="5" placeholder="ex: 0.15" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-lease-end-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de restitution estimés (€)</label>
                  <input id="own-lease-end-fees" v-model.number="ownershipForm.lease_end_fees_estimate" type="number" step="0.01" min="0" placeholder="ex: 400" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>

              <!-- Included Services -->
              <div class="p-3 bg-slate-800/50 rounded-xl border border-slate-800 space-y-2">
                <div class="text-xs font-semibold text-slate-300">Services inclus dans le contrat</div>
                <div class="flex flex-wrap gap-x-6 gap-y-2">
                  <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                    <input id="own-incl-maintenance" v-model="ownershipForm.lease_includes_maintenance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                    <span>Entretien inclus</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                    <input id="own-incl-insurance" v-model="ownershipForm.lease_includes_insurance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                    <span>Assurance incluse</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                    <input id="own-incl-tires" v-model="ownershipForm.lease_includes_tires" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                    <span>Pneus inclus</span>
                  </label>
                </div>
              </div>

              <!-- LOA Buyout Option -->
              <div v-if="ownershipForm.acquisition_type === 'LOA'" class="pt-2 border-t border-slate-800/80">
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label for="own-lease-option" class="block text-xs font-semibold text-slate-300 mb-1">Option d'achat résiduelle TTC (€)</label>
                    <input id="own-lease-option" v-model.number="ownershipForm.lease_purchase_option_price" type="number" step="0.01" min="0" placeholder="ex: 18000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                  </div>
                  <div>
                    <label for="own-option-date" class="block text-xs font-semibold text-slate-300 mb-1">Option levée le (vide si non levée)</label>
                    <AppDatePicker id="own-option-date" v-model="ownershipForm.option_exercised_date" size="sm" :clearable="true" />
                  </div>
                </div>
              </div>
            </div>

            <!-- Depreciation for owned vehicles -->
            <div v-if="isOwnedPhase" class="space-y-3 pt-3 border-t border-slate-800">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Décote & Détention prévisionnelle</h4>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label for="own-resale" class="block text-xs font-semibold text-slate-300 mb-1">Revente prévisionnelle estimée (€)</label>
                  <input id="own-resale" v-model.number="ownershipForm.expected_resale_value" type="number" step="0.01" min="0" placeholder="ex: 22000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-holding" class="block text-xs font-semibold text-slate-300 mb-1">
                    {{ ownershipForm.acquisition_type === 'LOA' && ownershipForm.option_exercised_date ? 'Durée de détention après rachat (mois)' : 'Durée de détention totale prévue (mois)' }}
                  </label>
                  <input id="own-holding" v-model.number="ownershipForm.expected_holding_months" type="number" min="1" max="360" placeholder="ex: 48" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>
            </div>

            <!-- Clôture / End of Contract -->
            <div class="space-y-3 pt-3 border-t border-slate-800">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Clôture effective (si terminée)</h4>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label for="own-end-date" class="block text-xs font-semibold text-slate-300 mb-1">
                    {{ isLease && !ownershipForm.option_exercised_date ? 'Véhicule restitué le' : 'Véhicule vendu le' }}
                  </label>
                  <AppDatePicker id="own-end-date" v-model="ownershipForm.end_date" size="sm" :clearable="true" />
                </div>
                <div v-if="isOwnedPhase">
                  <label for="own-sale-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix de revente effectif (€)</label>
                  <input id="own-sale-price" v-model.number="ownershipForm.sale_price" type="number" step="0.01" min="0" placeholder="ex: 21500" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Modal Footer: Navigation Controls -->
        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-between items-center gap-2 shrink-0 bg-slate-900/95">
          <div>
            <button
              v-if="ownerships[ownershipVehicle.id]"
              type="button"
              @click="handleDeleteOwnership"
              class="px-4 py-2 bg-slate-800 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl transition-colors"
            >
              Supprimer le contrat
            </button>
          </div>

          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="showOwnershipModal = false"
              class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
            >
              Annuler
            </button>

            <button
              v-if="currentOwnershipStep > 1"
              type="button"
              @click="prevOwnershipStep"
              class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
            >
              <ChevronLeft class="w-4 h-4" />
              <span>Précédent</span>
            </button>

            <button
              v-if="currentOwnershipStep < 3"
              type="button"
              @click="nextOwnershipStep"
              class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
            >
              <span>Suivant</span>
              <ChevronRight class="w-4 h-4" />
            </button>

            <button
              v-else
              type="button"
              @click="handleSaveOwnership"
              class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
            >
              <Check class="w-4 h-4" />
              <span>Enregistrer le contrat</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Odometer Checkpoints & Smoothing Modal -->
    <div
      v-if="showCheckpointsModal && checkpointsVehicle"
      class="fixed inset-0 z-[60] flex items-center justify-center p-3 sm:p-4 bg-black/75 backdrop-blur-sm overflow-y-auto"
      @click.self="showCheckpointsModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <!-- Header -->
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-3">
            <div class="p-2.5 bg-cyan-500/10 text-cyan-400 rounded-xl">
              <Gauge class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-base font-bold text-white">Relevés Kilométriques & Lissage</h3>
              <p class="text-xs text-slate-400">{{ checkpointsVehicle.name }}</p>
            </div>
          </div>
          <button @click="showCheckpointsModal = false" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Body -->
        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-6">
          <!-- Add / Edit Form -->
        <form @submit.prevent="handleSaveCheckpoint" class="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-4">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-cyan-400 uppercase tracking-wider">
              {{ editingCheckpointId ? 'Modifier le relevé' : 'Nouveau relevé kilométrique' }}
            </span>
            <button
              v-if="editingCheckpointId"
              type="button"
              @click="resetCheckpointForm"
              class="text-xs text-slate-400 hover:text-white"
            >
              Annuler modification
            </button>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="checkpoint-date" class="block text-xs font-semibold text-slate-300 mb-1">Date du relevé</label>
              <AppDatePicker
                id="checkpoint-date"
                v-model="checkpointForm.date"
                required
                size="sm"
              />
            </div>
            <div>
              <label for="checkpoint-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Kilométrage (km)</label>
              <input
                id="checkpoint-odometer"
                v-model="checkpointForm.odometer"
                type="number"
                step="1"
                min="0"
                max="2000000"
                required
                placeholder="ex: 45000"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-cyan-500"
              />
            </div>
            <div>
              <label for="checkpoint-notes" class="block text-xs font-semibold text-slate-300 mb-1">Motif / Événement</label>
              <input
                id="checkpoint-notes"
                v-model="checkpointForm.notes"
                type="text"
                placeholder="ex: Contrôle technique"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-cyan-500"
              />
            </div>
          </div>

          <div class="flex justify-end pt-1">
            <button
              type="submit"
              class="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-lg shadow-cyan-600/20"
            >
              <span>{{ editingCheckpointId ? 'Mettre à jour' : 'Ajouter le relevé' }}</span>
            </button>
          </div>
        </form>

        <!-- Pre-TeslaMate Charging & Energy Section -->
        <div class="bg-gradient-to-br from-sky-950/40 to-slate-950/60 border border-sky-500/20 rounded-xl p-4 space-y-4">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2.5">
              <div class="p-2 bg-sky-500/10 text-sky-400 rounded-lg">
                <Zap class="w-4 h-4" />
              </div>
              <div>
                <h4 class="text-xs font-bold text-sky-400 uppercase tracking-wider">
                  Coûts de recharge avant TeslaMate
                </h4>
                <p class="text-[11px] text-slate-400">
                  Complétez automatiquement l'énergie et le coût des kilomètres non suivis
                </p>
              </div>
            </div>
            <span
              v-if="checkpointsVehicle?.pre_teslamate_kwh_100km && checkpointsVehicle?.pre_teslamate_eur_per_kwh"
              class="text-[10px] px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold flex items-center gap-1"
            >
              <CheckCircle2 class="w-3 h-3" /> Actif
            </span>
            <span v-else class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">
              Non configuré
            </span>
          </div>

          <form @submit.prevent="handleSavePreTeslaMateEnergy" class="space-y-3">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="pre-tm-kwh" class="block text-xs font-semibold text-slate-300 mb-1">
                  Consommation moyenne (kWh/100km)
                </label>
                <input
                  id="pre-tm-kwh"
                  v-model.number="preTeslaMateForm.kwh_100km"
                  type="number"
                  step="0.1"
                  min="1"
                  max="100"
                  placeholder="ex: 16.5"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-sky-500"
                />
              </div>
              <div>
                <label for="pre-tm-rate" class="block text-xs font-semibold text-slate-300 mb-1">
                  Tarif de l'électricité (€/kWh)
                </label>
                <input
                  id="pre-tm-rate"
                  v-model.number="preTeslaMateForm.eur_per_kwh"
                  type="number"
                  step="0.0001"
                  min="0.01"
                  max="5"
                  placeholder="ex: 0.22"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-sky-500"
                />
              </div>
            </div>

            <!-- Live calculation summary -->
            <div
              v-if="preTeslaMatePreview"
              class="bg-slate-900/80 border border-slate-800 rounded-xl p-3 text-xs space-y-1"
            >
              <div class="text-slate-300 font-semibold flex items-center justify-between">
                <span>Estimation sur {{ Math.round(preTeslaMatePreview.distance).toLocaleString('fr-FR') }} km lissés :</span>
                <span class="text-sky-400 font-bold font-mono">
                  ≈ {{ preTeslaMatePreview.cost.toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
                </span>
              </div>
              <p class="text-slate-400 text-[11px]">
                Volume estimé :
                <strong class="text-slate-200 font-mono">{{ Math.round(preTeslaMatePreview.kwh).toLocaleString('fr-FR') }} kWh</strong>
                ({{ (preTeslaMatePreview.cost / (preTeslaMatePreview.distance || 1)).toFixed(3) }} €/km)
                distribués au prorata dans chaque mois lissé.
              </p>
            </div>

            <div class="flex items-center justify-between pt-1">
              <button
                v-if="checkpointsVehicle?.pre_teslamate_kwh_100km || checkpointsVehicle?.pre_teslamate_eur_per_kwh"
                type="button"
                @click="handleClearPreTeslaMateEnergy"
                class="text-xs text-slate-400 hover:text-rose-400 transition-colors"
              >
                Réinitialiser (désactiver)
              </button>
              <span v-else></span>

              <button
                type="submit"
                :disabled="savingPreTeslaMate"
                class="px-4 py-2 bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-lg shadow-sky-600/20"
              >
                <Zap class="w-3.5 h-3.5" />
                <span>{{ savingPreTeslaMate ? 'Enregistrement...' : 'Enregistrer la recharge avant TM' }}</span>
              </button>
            </div>
          </form>
        </div>

        <!-- Existing Checkpoints List -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white uppercase tracking-wider">Historique des relevés enregistrés</h4>
            <span class="text-xs text-slate-400">{{ checkpoints.length }} relevé(s)</span>
          </div>

          <div v-if="loadingCheckpoints" class="py-8 text-center text-xs text-slate-400">
            Chargement des relevés...
          </div>

          <div v-else-if="checkpoints.length === 0" class="py-8 text-center bg-slate-950/40 rounded-xl border border-slate-800 text-xs text-slate-500">
            Aucun relevé manuel pour l'instant. Utilisez le formulaire ci-dessus pour en créer un.
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="cp in checkpoints"
              :key="cp.id"
              class="bg-slate-950/70 border border-slate-800/80 rounded-xl p-3.5 flex items-center justify-between gap-3"
            >
              <div class="flex items-center gap-3 min-w-0">
                <div class="p-2 bg-slate-800/80 text-cyan-400 rounded-lg shrink-0">
                  <Gauge class="w-4 h-4" />
                </div>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-bold text-white font-mono">
                      {{ Math.round(cp.odometer).toLocaleString('fr-FR') }} km
                    </span>
                    <span class="text-xs text-slate-400">
                      le {{ new Date(cp.date).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' }) }}
                    </span>
                  </div>
                  <p v-if="cp.notes" class="text-xs text-slate-400 truncate mt-0.5">
                    {{ cp.notes }}
                  </p>
                </div>
              </div>

              <div class="flex items-center gap-1.5 shrink-0">
                <button
                  type="button"
                  @click="startEditCheckpoint(cp)"
                  class="p-1.5 text-slate-400 hover:text-cyan-400 hover:bg-slate-800 rounded-lg transition-colors"
                  title="Modifier"
                >
                  <Edit2 class="w-4 h-4" />
                </button>
                <button
                  type="button"
                  @click="handleDeleteCheckpoint(cp)"
                  class="p-1.5 text-slate-400 hover:text-rose-400 hover:bg-slate-800 rounded-lg transition-colors"
                  title="Supprimer"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        </div>

        <!-- Footer -->
        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showCheckpointsModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>

    <!-- Members Modal (Partage & Accès) -->
    <div
      v-if="showMembersModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
        <!-- Header -->
        <div class="px-5 py-4 border-b border-slate-800 flex items-center justify-between shrink-0">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-violet-500/10 text-violet-400 rounded-xl">
              <Users class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-base font-bold text-white">Partage & Accès</h3>
              <p class="text-xs text-slate-400">{{ membersVehicle?.name }}</p>
            </div>
          </div>
          <button
            @click="showMembersModal = false"
            class="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Body -->
        <div class="p-5 overflow-y-auto space-y-6 text-xs">
          <!-- Add Member Section (Owners only) -->
          <div
            v-if="membersVehicle?.role === 'OWNER'"
            class="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3"
          >
            <div class="flex items-center gap-2">
              <UserPlus class="w-4 h-4 text-violet-400" />
              <h4 class="text-xs font-bold text-white uppercase tracking-wider">Ajouter un membre</h4>
            </div>
            <p class="text-[11px] text-slate-400">
              Invitez un co-conducteur ou un membre de votre foyer. L'utilisateur doit déjà posséder un compte sur l'application.
            </p>

            <form @submit.prevent="handleAddMember" class="space-y-3">
              <div class="grid grid-cols-1 sm:grid-cols-12 gap-2">
                <div class="sm:col-span-7">
                  <label for="new-member-email" class="sr-only">Email du membre</label>
                  <input
                    id="new-member-email"
                    v-model="newMemberEmail"
                    type="email"
                    required
                    placeholder="email@exemple.com"
                    class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-violet-500"
                  />
                </div>
                <div class="sm:col-span-5">
                  <label for="new-member-role" class="sr-only">Rôle du membre</label>
                  <select
                    id="new-member-role"
                    v-model="newMemberRole"
                    class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white focus:outline-none focus:border-violet-500"
                  >
                    <option value="EDITOR">Co-conducteur (Éditeur)</option>
                    <option value="VIEWER">Lecteur seul</option>
                  </select>
                </div>
              </div>

              <div class="flex items-center justify-between gap-3 pt-1">
                <p class="text-[10px] text-slate-500 leading-tight">
                  <ShieldCheck class="w-3 h-3 text-emerald-400 inline mr-0.5 -mt-0.5" />
                  Vos clés API TeslaMate et données de financement restent invisibles pour les membres.
                </p>
                <button
                  type="submit"
                  :disabled="addingMember || !newMemberEmail.trim()"
                  class="px-3 py-1.5 bg-violet-600 hover:bg-violet-500 disabled:opacity-50 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shrink-0 transition-colors shadow-lg shadow-violet-600/20"
                >
                  <RefreshCw v-if="addingMember" class="w-3.5 h-3.5 animate-spin" />
                  <UserPlus v-else class="w-3.5 h-3.5" />
                  <span>Ajouter</span>
                </button>
              </div>
            </form>
          </div>

          <!-- Members List -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <h4 class="text-xs font-bold text-white uppercase tracking-wider">Membres autorisés</h4>
              <span class="text-xs text-slate-400">{{ members.length }} membre(s)</span>
            </div>

            <div v-if="loadingMembers" class="py-8 text-center text-xs text-slate-400">
              Chargement des accès...
            </div>

            <div v-else-if="members.length === 0" class="py-6 text-center text-xs text-slate-500 bg-slate-950/40 rounded-xl border border-slate-800">
              Aucun membre trouvé.
            </div>

            <div v-else class="space-y-2">
              <div
                v-for="m in members"
                :key="m.user_id"
                class="bg-slate-950/60 border border-slate-800 rounded-xl p-3 flex items-center justify-between gap-3"
              >
                <div class="flex items-center gap-3 min-w-0">
                  <div
                    class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-xs shrink-0"
                    :class="m.role === 'OWNER' ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20' : 'bg-slate-800 text-slate-300'"
                  >
                    {{ (m.user_email || '?').charAt(0).toUpperCase() }}
                  </div>
                  <div class="min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <span class="text-xs font-semibold text-white truncate">{{ m.user_email }}</span>
                      <span
                        v-if="m.user_id === authStore.user?.id"
                        class="text-[10px] px-1.5 py-0.2 bg-slate-800 text-slate-400 rounded"
                      >
                        Vous
                      </span>
                    </div>
                    <div class="flex items-center gap-1.5 mt-0.5">
                      <span
                        class="text-[10px] px-2 py-0.5 rounded-full font-semibold uppercase tracking-wider"
                        :class="{
                          'bg-amber-500/10 text-amber-400 border border-amber-500/20': m.role === 'OWNER',
                          'bg-sky-500/10 text-sky-400 border border-sky-500/20': m.role === 'EDITOR',
                          'bg-slate-800 text-slate-400 border border-slate-700': m.role === 'VIEWER',
                        }"
                      >
                        {{ m.role === 'OWNER' ? 'Propriétaire' : m.role === 'EDITOR' ? 'Co-conducteur' : 'Lecteur' }}
                      </span>
                    </div>
                  </div>
                </div>

                <!-- Member Management Actions -->
                <div class="flex items-center gap-2 shrink-0">
                  <!-- If current user is OWNER and this member is not OWNER: allow role change or removal -->
                  <template v-if="membersVehicle?.role === 'OWNER' && m.role !== 'OWNER'">
                    <label :for="'member-role-' + m.user_id" class="sr-only">Rôle du membre {{ m.user_email }}</label>
                    <select
                      :id="'member-role-' + m.user_id"
                      :value="m.role"
                      :disabled="updatingMemberId === m.user_id"
                      @change="handleUpdateMemberRole(m, ($event.target as HTMLSelectElement).value)"
                      class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
                    >
                      <option value="EDITOR">Co-conducteur</option>
                      <option value="VIEWER">Lecteur</option>
                    </select>

                    <button
                      type="button"
                      @click="handleRemoveMember(m)"
                      class="p-1.5 text-slate-400 hover:text-rose-400 hover:bg-slate-800 rounded-lg transition-colors"
                      title="Retirer l'accès"
                    >
                      <Trash2 class="w-4 h-4" />
                    </button>
                  </template>

                  <!-- If current user is non-owner and viewing themselves: allow leaving -->
                  <template v-else-if="membersVehicle?.role !== 'OWNER' && m.user_id === authStore.user?.id">
                    <button
                      type="button"
                      @click="handleRemoveMember(m)"
                      class="px-2.5 py-1 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-colors border border-rose-500/20"
                    >
                      <LogOut class="w-3.5 h-3.5" />
                      <span>Quitter</span>
                    </button>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="px-5 py-3.5 border-t border-slate-800 flex justify-end shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showMembersModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
