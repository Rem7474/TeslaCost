<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
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
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const showModal = ref(false)
const isEditing = ref(false)
const modalTestLoading = ref(false)
const modalTestResult = ref<{ success: boolean; status?: any; error?: string } | null>(null)
const cardTestResults = ref<Record<string, { loading: boolean; success?: boolean; status?: any; error?: string }>>({})
const editingId = ref<string | null>(null)

const form = ref({
  name: 'Tesla Model 3',
  vin: '',
  current_odometer: 0,
  teslamate_car_id: 1,
  teslamate_api_url: '',
  teslamate_auth_type: 'NONE',
  teslamate_api_key: '',
  teslamate_basic_user: '',
  teslamate_basic_pass: '',
})

// Ownership contracts (purchase, loan, LOA, LLD) by vehicle id
const ownerships = ref<Record<string, any | null>>({})
const showOwnershipModal = ref(false)
const ownershipVehicle = ref<any | null>(null)
const ownershipForm = ref(emptyOwnership())

const ACQUISITION_LABELS: Record<string, string> = {
  CASH: 'Achat comptant',
  LOAN: 'Achat à crédit',
  LOA: 'LOA (location avec option d\'achat)',
  LLD: 'LLD (location longue durée)',
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

function openOwnershipModal(v: any) {
  ownershipVehicle.value = v
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
  const f: any = { ...ownershipForm.value }
  for (const key of Object.keys(f)) {
    if (typeof f[key] !== 'boolean') f[key] = nullIfEmpty(f[key])
  }
  try {
    ownerships.value[ownershipVehicle.value.id] = await api.saveOwnership(ownershipVehicle.value.id, f)
    showOwnershipModal.value = false
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

async function handleDeleteOwnership() {
  if (!ownershipVehicle.value) return
  if (!confirm('Supprimer le contrat d\'acquisition de ce véhicule ?')) return
  try {
    await api.deleteOwnership(ownershipVehicle.value.id)
    ownerships.value[ownershipVehicle.value.id] = null
    showOwnershipModal.value = false
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
  }
}

function openCreateModal() {
  isEditing.value = false
  editingId.value = null
  modalTestResult.value = null
  form.value = {
    name: 'Tesla Model 3',
    vin: '',
    current_odometer: 0,
    teslamate_car_id: 1,
    teslamate_api_url: '',
    teslamate_auth_type: 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: '',
    teslamate_basic_pass: '',
  }
  showModal.value = true
}

function openEditModal(v: any) {
  isEditing.value = true
  editingId.value = v.id
  modalTestResult.value = null
  form.value = {
    name: v.name,
    vin: v.vin || '',
    current_odometer: v.current_odometer,
    teslamate_car_id: v.teslamate_car_id || 1,
    teslamate_api_url: v.teslamate_api_url || '',
    teslamate_auth_type: v.teslamate_auth_type || 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: v.teslamate_basic_user || '',
    teslamate_basic_pass: '',
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
    alert(`Erreur : ${err.message}`)
  }
}

async function handleDelete(id: string) {
  if (!confirm('Supprimer ce véhicule et tout son historique ?')) return
  try {
    await api.deleteVehicle(id)
    await vehicleStore.fetchVehicles()
  } catch (err: any) {
    alert(`Erreur : ${err.message}`)
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
              <h3 class="text-base font-bold text-white">{{ v.name }}</h3>
              <p v-if="v.vin" class="text-xs text-slate-400 font-mono">{{ v.vin }}</p>
            </div>
          </div>

          <div class="flex items-center gap-1">
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
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-4 pt-4 border-t border-slate-800 text-xs">
          <div>
            <span class="text-slate-400">Odomètre actuel</span>
            <p class="text-sm font-bold text-slate-200 flex items-center gap-1.5 mt-0.5">
              <Gauge class="w-3.5 h-3.5 text-rose-400" />
              {{ Math.round(v.current_odometer).toLocaleString('fr-FR') }} km
            </p>
          </div>

          <div>
            <span class="text-slate-400">Acquisition</span>
            <p v-if="ownerships[v.id]" class="text-xs font-semibold text-indigo-300 mt-1">
              {{ ownershipSummary(ownerships[v.id]) }}
              <span v-if="ownerships[v.id].end_date" class="block text-[11px] font-normal text-slate-400">
                Fin de détention le {{ new Date(ownerships[v.id].end_date).toLocaleDateString('fr-FR') }}
              </span>
            </p>
            <p v-else class="text-xs text-amber-400/90 mt-1">Non renseignée</p>
          </div>

          <div>
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
                cardTestResults[v.id]?.success
                  ? 'En ligne'
                  : cardTestResults[v.id]?.error
                  ? 'Erreur de connexion'
                  : v.teslamate_api_url
                  ? 'Configurée'
                  : 'Non configurée'
              }}
            </p>
          </div>
        </div>

        <!-- Actions -->
        <div class="mt-4 pt-3 flex items-center justify-between gap-2 flex-wrap">
          <button
            @click="openOwnershipModal(v)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
          >
            <FileText class="w-3.5 h-3.5 text-indigo-400" />
            <span>Acquisition & financement</span>
          </button>
          <button
            v-if="v.teslamate_api_url"
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
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 space-y-4 shadow-2xl my-8">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white">{{ isEditing ? 'Modifier le véhicule' : 'Ajouter un véhicule' }}</h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleSave" class="space-y-3">
          <div>
            <label for="vehicle-name" class="block text-xs font-semibold text-slate-300 mb-1">Nom du véhicule</label>
            <input id="vehicle-name" v-model="form.name" required placeholder="ex: Tesla Model Y LR" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1">VIN (optionnel)</label>
              <input id="vehicle-vin" v-model="form.vin" placeholder="5YJ3E1EB..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-current-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre initial (km)</label>
              <input id="vehicle-current-odometer" v-model.number="form.current_odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>

          <!-- TeslaMate API Section -->
          <div class="pt-2 border-t border-slate-800 space-y-3">
            <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">Connexion TeslaMateApi (Optionnelle)</h4>

            <div>
              <label for="vehicle-teslamate-api-url" class="block text-xs font-semibold text-slate-300 mb-1">URL de base teslamateapi</label>
              <input id="vehicle-teslamate-api-url" v-model="form.teslamate_api_url" placeholder="http://192.168.1.50:8080 ou https://tm.mondomaine.com" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <p class="text-[11px] text-slate-400 mt-1">
                💡 Si TeslaCost s'exécute dans Docker, utilisez <code class="text-rose-300">http://host.docker.internal:PORT</code> ou l'IP locale (ex: <code class="text-rose-300">192.168.x.x</code>) au lieu de <code class="text-slate-500">localhost</code>.
              </p>
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

          <p class="text-[11px] text-slate-400 pt-2 border-t border-slate-800">
            L'acquisition (achat, crédit, LOA, LLD) se configure depuis la carte du véhicule, et l'assurance se saisit dans
            Dépenses → Entretien & coûts fixes (catégorie « Assurance », dépense récurrente).
          </p>

          <div class="flex justify-end gap-2 pt-3">
            <button type="button" @click="showModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
              Annuler
            </button>
            <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              Enregistrer
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Ownership contract -->
    <div
      v-if="showOwnershipModal && ownershipVehicle"
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 overflow-y-auto"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full p-6 space-y-4 shadow-2xl my-8">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <FileText class="w-5 h-5 text-indigo-400" />
            Acquisition & financement — {{ ownershipVehicle.name }}
          </h3>
          <button @click="showOwnershipModal = false" class="text-slate-400 hover:text-white">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleSaveOwnership" class="space-y-4">
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="own-type" class="block text-xs font-semibold text-slate-300 mb-1">Mode d'acquisition</label>
              <select id="own-type" v-model="ownershipForm.acquisition_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500">
                <option v-for="(label, key) in ACQUISITION_LABELS" :key="key" :value="key">{{ label }}</option>
              </select>
            </div>
              <div>
                <label for="own-start-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ isLease ? 'Début du contrat' : 'Date d\'achat' }}</label>
                <input id="own-start-date" v-model="ownershipForm.start_date" type="date" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-start-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre au début (km)</label>
                <input id="own-start-odometer" v-model.number="ownershipForm.start_odometer" type="number" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
          </div>

          <!-- Purchase -->
          <div v-if="isPurchase" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Achat</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-purchase-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix d'achat TTC (€)</label>
                <input id="own-purchase-price" v-model.number="ownershipForm.purchase_price" type="number" step="0.01" min="0" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-purchase-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais (carte grise, malus, mise à la route) (€)</label>
                <input id="own-purchase-fees" v-model.number="ownershipForm.purchase_fees" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-incentives" class="block text-xs font-semibold text-slate-300 mb-1">Aides et remises (€)</label>
                <input id="own-incentives" v-model.number="ownershipForm.incentives" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>

          <!-- Loan -->
          <div v-if="ownershipForm.acquisition_type === 'LOAN'" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Crédit</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-loan-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant emprunté (€)</label>
                <input id="own-loan-amount" v-model.number="ownershipForm.loan_amount" type="number" step="0.01" min="0" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux annuel (%)</label>
                <input id="own-loan-rate" v-model.number="ownershipForm.loan_rate_pct" type="number" step="0.001" min="0" max="30" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-duration" class="block text-xs font-semibold text-slate-300 mb-1">Durée (mois)</label>
                <input id="own-loan-duration" v-model.number="ownershipForm.loan_duration_months" type="number" min="1" max="360" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de dossier (€)</label>
                <input id="own-loan-fees" v-model.number="ownershipForm.loan_fees" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-insurance" class="block text-xs font-semibold text-slate-300 mb-1">Assurance emprunteur (€/mois)</label>
                <input id="own-loan-insurance" v-model.number="ownershipForm.loan_insurance_monthly" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
            <p v-if="loanPreview" class="text-[11px] text-slate-400">
              Mensualité {{ loanPreview.payment.toFixed(2) }} € • intérêts totaux {{ loanPreview.totalInterest.toFixed(2) }} €
              • coût total du crédit {{ loanPreview.totalCost.toFixed(2) }} € (intérêts, assurance et frais comptés mois par mois dans le TCO ;
              le capital est déjà compté dans la décote).
            </p>
          </div>

          <!-- Lease -->
          <div v-if="isLease" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Contrat de location</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-lease-down" class="block text-xs font-semibold text-slate-300 mb-1">Apport / 1er loyer majoré (€)</label>
                <input id="own-lease-down" v-model.number="ownershipForm.lease_down_payment" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-rent" class="block text-xs font-semibold text-slate-300 mb-1">Loyer mensuel (€)</label>
                <input id="own-lease-rent" v-model.number="ownershipForm.lease_monthly_rent" type="number" step="0.01" min="0" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-duration" class="block text-xs font-semibold text-slate-300 mb-1">Durée (mois)</label>
                <input id="own-lease-duration" v-model.number="ownershipForm.lease_duration_months" type="number" min="1" max="360" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de dossier (€)</label>
                <input id="own-lease-fees" v-model.number="ownershipForm.lease_fees" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-deposit" class="block text-xs font-semibold text-slate-300 mb-1">Dépôt de garantie (€, remboursable)</label>
                <input id="own-lease-deposit" v-model.number="ownershipForm.lease_deposit" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-end-fees" class="block text-xs font-semibold text-slate-300 mb-1">Frais de restitution estimés (€)</label>
                <input id="own-lease-end-fees" v-model.number="ownershipForm.lease_end_fees_estimate" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-allowance" class="block text-xs font-semibold text-slate-300 mb-1">Forfait kilométrique (km/an)</label>
                <input id="own-lease-allowance" v-model.number="ownershipForm.lease_km_allowance_per_year" type="number" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-excess" class="block text-xs font-semibold text-slate-300 mb-1">Prix du km supplémentaire (€/km)</label>
                <input id="own-lease-excess" v-model.number="ownershipForm.lease_excess_km_price" type="number" step="0.001" min="0" max="5" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div v-if="ownershipForm.acquisition_type === 'LOA'">
                <label for="own-lease-option" class="block text-xs font-semibold text-slate-300 mb-1">Prix de l'option d'achat (€)</label>
                <input id="own-lease-option" v-model.number="ownershipForm.lease_purchase_option_price" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
            <div class="flex flex-wrap gap-x-5 gap-y-2">
              <div class="flex items-center gap-2">
                <input id="own-incl-maintenance" v-model="ownershipForm.lease_includes_maintenance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500" />
                <label for="own-incl-maintenance" class="text-xs text-slate-300">Entretien inclus</label>
              </div>
              <div class="flex items-center gap-2">
                <input id="own-incl-insurance" v-model="ownershipForm.lease_includes_insurance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500" />
                <label for="own-incl-insurance" class="text-xs text-slate-300">Assurance incluse</label>
              </div>
              <div class="flex items-center gap-2">
                <input id="own-incl-tires" v-model="ownershipForm.lease_includes_tires" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500" />
                <label for="own-incl-tires" class="text-xs text-slate-300">Pneus inclus</label>
              </div>
            </div>
            <p v-if="leasePreview" class="text-[11px] text-slate-400">
              Coût total du contrat {{ leasePreview.total.toFixed(2) }} € (≈ {{ leasePreview.perMonth.toFixed(2) }} €/mois tout compris)
              <template v-if="leasePreview.totalKm"> • {{ Math.round(leasePreview.totalKm).toLocaleString('fr-FR') }} km inclus</template>.
              L'apport et les frais sont étalés sur la durée dans le coût complet ; le dépassement kilométrique est estimé au fil du contrat.
            </p>
            <div v-if="ownershipForm.acquisition_type === 'LOA'" class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-option-date" class="block text-xs font-semibold text-slate-300 mb-1">Option levée le (vide si non levée)</label>
                <input id="own-option-date" v-model="ownershipForm.option_exercised_date" type="date"  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>

          <!-- Owned phase: depreciation -->
          <div v-if="isOwnedPhase" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Décote</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-resale" class="block text-xs font-semibold text-slate-300 mb-1">Revente estimée (€)</label>
                <input id="own-resale" v-model.number="ownershipForm.expected_resale_value" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-holding" class="block text-xs font-semibold text-slate-300 mb-1">Durée de détention prévue (mois)</label>
                <input id="own-holding" v-model.number="ownershipForm.expected_holding_months" type="number" min="1" max="360" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
            <p class="text-[11px] text-slate-400">
              Décote linéaire : (prix + frais − aides − revente estimée) répartie sur la durée de détention.
              <template v-if="ownershipForm.acquisition_type === 'LOA'">Après la levée d'option, la base est le prix de l'option.</template>
            </p>
          </div>

          <!-- End of ownership -->
          <div class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Fin de détention</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-end-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ isLease && !ownershipForm.option_exercised_date ? 'Restitué le' : 'Vendu le' }}</label>
                <input id="own-end-date" v-model="ownershipForm.end_date" type="date"  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div v-if="isOwnedPhase">
                <label for="own-sale-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix de revente (€)</label>
                <input id="own-sale-price" v-model.number="ownershipForm.sale_price" type="number" step="0.01" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
            <p class="text-[11px] text-slate-400">
              Les dépenses récurrentes (assurance, abonnements…) s'arrêtent à cette date et la décote est figée sur le prix de revente réel.
            </p>
          </div>

          <div class="flex justify-between gap-2 pt-3 border-t border-slate-800">
            <button
              v-if="ownerships[ownershipVehicle.id]"
              type="button"
              @click="handleDeleteOwnership"
              class="px-4 py-2 bg-slate-800 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl"
            >
              Supprimer le contrat
            </button>
            <div class="flex gap-2 ml-auto">
              <button type="button" @click="showOwnershipModal = false" class="px-4 py-2 bg-slate-800 text-slate-300 text-xs font-semibold rounded-xl">
                Annuler
              </button>
              <button type="submit" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
                Enregistrer
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
