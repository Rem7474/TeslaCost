<script setup lang="ts">
import { ref, onMounted } from 'vue'
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
  Shield,
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
  annual_insurance_cost: null as number | null,
  annual_expected_mileage: 15000 as number | null,
  ...emptyAcquisition(),
})

function emptyAcquisition() {
  return {
    acquisition_type: '' as string,
    purchase_price: null as number | null,
    purchase_date: '' as string,
    purchase_odometer: null as number | null,
    purchase_incentives: null as number | null,
    expected_resale_value: null as number | null,
    expected_holding_months: null as number | null,
  }
}

// Empty numeric inputs are sent as null, never as ""
function nullIfEmpty(v: any) {
  return v === '' || v === undefined ? null : v
}

onMounted(() => {
  vehicleStore.fetchVehicles()
})

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
    annual_insurance_cost: null,
    annual_expected_mileage: 15000,
    ...emptyAcquisition(),
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
    annual_insurance_cost: v.annual_insurance_cost || null,
    annual_expected_mileage: v.annual_expected_mileage || 15000,
    acquisition_type: v.acquisition_type || '',
    purchase_price: v.purchase_price ?? null,
    purchase_date: v.purchase_date ? new Date(v.purchase_date).toISOString().substring(0, 10) : '',
    purchase_odometer: v.purchase_odometer ?? null,
    purchase_incentives: v.purchase_incentives ?? null,
    expected_resale_value: v.expected_resale_value ?? null,
    expected_holding_months: v.expected_holding_months ?? null,
  }
  showModal.value = true
}

async function handleSave() {
  try {
    const f = form.value
    const payload = {
      ...f,
      annual_insurance_cost: nullIfEmpty(f.annual_insurance_cost),
      annual_expected_mileage: nullIfEmpty(f.annual_expected_mileage),
      acquisition_type: f.acquisition_type || null,
      purchase_price: nullIfEmpty(f.purchase_price),
      purchase_date: f.purchase_date || null,
      purchase_odometer: nullIfEmpty(f.purchase_odometer),
      purchase_incentives: nullIfEmpty(f.purchase_incentives),
      expected_resale_value: nullIfEmpty(f.expected_resale_value),
      expected_holding_months: nullIfEmpty(f.expected_holding_months),
    }
    if (isEditing.value && editingId.value) {
      await api.updateVehicle(editingId.value, payload)
    } else {
      await api.createVehicle(payload)
    }
    showModal.value = false
    await vehicleStore.fetchVehicles()
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
            <span class="text-slate-400">Assurance réelle</span>
            <p v-if="v.annual_insurance_cost" class="text-sm font-bold text-indigo-400 flex items-center gap-1 mt-0.5">
              <Shield class="w-3.5 h-3.5 text-indigo-400" />
              {{ ((Number(v.annual_insurance_cost) || 0) / (Number(v.annual_expected_mileage) || 15000)).toFixed(3) }} €/km
              <span class="text-[11px] font-normal text-slate-400">({{ Number(v.annual_insurance_cost).toFixed(0) }} €/an)</span>
            </p>
            <p v-else class="text-xs text-slate-500 mt-1 flex items-center gap-1">
              <Shield class="w-3 h-3 text-slate-600" /> Défaut (0.035 €/km)
            </p>
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
        <div class="mt-4 pt-3 flex items-center justify-between gap-2">
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
            <label class="block text-xs font-semibold text-slate-300 mb-1">Nom du véhicule</label>
            <input v-model="form.name" required placeholder="ex: Tesla Model Y LR" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">VIN (optionnel)</label>
              <input v-model="form.vin" placeholder="5YJ3E1EB..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">Odomètre initial (km)</label>
              <input v-model.number="form.current_odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>

          <!-- TeslaMate API Section -->
          <div class="pt-2 border-t border-slate-800 space-y-3">
            <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">Connexion TeslaMateApi (Optionnelle)</h4>

            <div>
              <label class="block text-xs font-semibold text-slate-300 mb-1">URL de base teslamateapi</label>
              <input v-model="form.teslamate_api_url" placeholder="http://192.168.1.50:8080 ou https://tm.mondomaine.com" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <p class="text-[11px] text-slate-400 mt-1">
                💡 Si TeslaCost s'exécute dans Docker, utilisez <code class="text-rose-300">http://host.docker.internal:PORT</code> ou l'IP locale (ex: <code class="text-rose-300">192.168.x.x</code>) au lieu de <code class="text-slate-500">localhost</code>.
              </p>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">ID Voiture dans TeslaMate</label>
                <input v-model.number="form.teslamate_car_id" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Mode d'authentification</label>
                <select v-model="form.teslamate_auth_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                  <option value="NONE">Aucun (LAN)</option>
                  <option value="BEARER">Token Bearer (API_TOKEN)</option>
                  <option value="BASIC">HTTP Basic Auth</option>
                </select>
              </div>
            </div>

            <div v-if="form.teslamate_auth_type === 'BEARER'">
              <label class="block text-xs font-semibold text-slate-300 mb-1">Clé / Token API TeslaMate</label>
              <input v-model="form.teslamate_api_key" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>

            <div v-if="form.teslamate_auth_type === 'BASIC'" class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Utilisateur Basic Auth</label>
                <input v-model="form.teslamate_basic_user" placeholder="admin" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label class="block text-xs font-semibold text-slate-300 mb-1">Mot de passe Basic Auth</label>
                <input v-model="form.teslamate_basic_pass" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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

          <!-- Real Insurance Configuration Section -->
          <div class="pt-2 border-t border-slate-800 space-y-3">
            <div class="flex items-center justify-between">
              <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                <Shield class="w-3.5 h-3.5" />
                Assurance & Frais Fixes Réels
              </h4>
              <span
                v-if="form.annual_insurance_cost && form.annual_expected_mileage"
                class="text-[11px] font-semibold text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-md border border-emerald-500/20"
              >
                {{ ((Number(form.annual_insurance_cost) || 0) / (Number(form.annual_expected_mileage) || 15000)).toFixed(3) }} €/km
              </span>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-annual-insurance-cost" class="block text-xs font-semibold text-slate-300 mb-1">Prime d'assurance (€/an)</label>
                <input id="vehicle-annual-insurance-cost"
                  v-model.number="form.annual_insurance_cost"
                  type="number"
                  step="0.01"
                  placeholder="ex: 850.00"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors"
                />
              </div>
              <div>
                <label for="vehicle-annual-expected-mileage" class="block text-xs font-semibold text-slate-300 mb-1">Kilométrage annuel prévu (km/an)</label>
                <input id="vehicle-annual-expected-mileage"
                  v-model.number="form.annual_expected_mileage"
                  type="number"
                  placeholder="ex: 15000"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors"
                />
              </div>
            </div>
            <p class="text-[11px] text-slate-400">
              💡 Cette valeur permet de calculer exactement la quote-part d'assurance réelle pour chaque trajet et covoiturage (Taux = Prime / Kilomètres).
            </p>
          </div>

          <!-- Acquisition -->
          <div class="pt-2 border-t border-slate-800 space-y-3">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">Acquisition & décote</h4>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-acquisition-type" class="block text-xs font-semibold text-slate-300 mb-1">Mode d'acquisition</label>
                <select id="vehicle-acquisition-type" v-model="form.acquisition_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors">
                  <option value="">Non renseigné</option>
                  <option value="PURCHASE">Achat (comptant ou crédit)</option>
                  <option value="LEASE">Location (LOA / LLD)</option>
                </select>
              </div>
              <div v-if="form.acquisition_type">
                <label for="vehicle-purchase-date" class="block text-xs font-semibold text-slate-300 mb-1">Date d'acquisition</label>
                <input id="vehicle-purchase-date" v-model="form.purchase_date" type="date" :required="form.acquisition_type === 'PURCHASE'" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
            </div>
            <div v-if="form.acquisition_type" class="grid grid-cols-2 gap-3">
              <div>
                <label for="vehicle-purchase-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre à l'acquisition (km)</label>
                <input id="vehicle-purchase-odometer" v-model.number="form.purchase_odometer" type="number" min="0" placeholder="ex: 0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
              <div v-if="form.acquisition_type === 'PURCHASE'">
                <label for="vehicle-purchase-price" class="block text-xs font-semibold text-slate-300 mb-1">Prix d'achat TTC (€)</label>
                <input id="vehicle-purchase-price" v-model.number="form.purchase_price" type="number" step="0.01" min="0" required placeholder="ex: 42990" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
            </div>
            <div v-if="form.acquisition_type === 'PURCHASE'" class="grid grid-cols-3 gap-3">
              <div>
                <label for="vehicle-purchase-incentives" class="block text-xs font-semibold text-slate-300 mb-1">Aides / remises (€)</label>
                <input id="vehicle-purchase-incentives" v-model.number="form.purchase_incentives" type="number" step="0.01" min="0" placeholder="Bonus écologique" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
              <div>
                <label for="vehicle-expected-resale-value" class="block text-xs font-semibold text-slate-300 mb-1">Revente estimée (€)</label>
                <input id="vehicle-expected-resale-value" v-model.number="form.expected_resale_value" type="number" step="0.01" min="0" placeholder="ex: 22000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
              <div>
                <label for="vehicle-expected-holding-months" class="block text-xs font-semibold text-slate-300 mb-1">Détention (mois)</label>
                <input id="vehicle-expected-holding-months" v-model.number="form.expected_holding_months" type="number" min="1" max="360" placeholder="ex: 60" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 transition-colors" />
              </div>
            </div>
            <p v-if="form.acquisition_type === 'PURCHASE'" class="text-[11px] text-slate-400">
              La décote (prix net des aides − revente estimée) est répartie linéairement sur la durée de détention.
              Pour un crédit, enregistrez uniquement les intérêts et l'assurance emprunteur en dépense récurrente « Financement » : le capital est déjà compté dans la décote.
            </p>
            <p v-else-if="form.acquisition_type === 'LEASE'" class="text-[11px] text-slate-400">
              Enregistrez les loyers en dépense récurrente « Financement » et le premier loyer majoré en dépense ponctuelle de la même catégorie.
            </p>
          </div>

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
  </div>
</template>
