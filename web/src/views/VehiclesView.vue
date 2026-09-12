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
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const showModal = ref(false)
const isEditing = ref(false)
const testLoading = ref(false)
const testResult = ref<any | null>(null)
const editingId = ref<string | null>(null)

const form = ref({
  name: 'Tesla Model 3',
  vin: '',
  current_odometer: 0,
  teslamate_car_id: 1,
  teslamate_api_url: 'http://localhost:8080',
  teslamate_auth_type: 'NONE',
  teslamate_api_key: '',
  teslamate_basic_user: '',
  teslamate_basic_pass: '',
})

onMounted(() => {
  vehicleStore.fetchVehicles()
})

function openCreateModal() {
  isEditing.value = false
  editingId.value = null
  testResult.value = null
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
  testResult.value = null
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

async function testConnection(id: string) {
  testLoading.value = true
  testResult.value = null
  try {
    const res = await api.testTeslaMate(id)
    testResult.value = { success: true, status: res.status }
  } catch (err: any) {
    testResult.value = { success: false, error: err.message }
  } finally {
    testLoading.value = false
  }
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
        <div class="grid grid-cols-2 gap-3 mt-4 pt-4 border-t border-slate-800 text-xs">
          <div>
            <span class="text-slate-400">Odomètre actuel</span>
            <p class="text-sm font-bold text-slate-200 flex items-center gap-1.5 mt-0.5">
              <Gauge class="w-3.5 h-3.5 text-rose-400" />
              {{ Math.round(v.current_odometer).toLocaleString('fr-FR') }} km
            </p>
          </div>

          <div>
            <span class="text-slate-400">Connexion TeslaMate</span>
            <p class="text-sm font-semibold mt-0.5" :class="v.teslamate_api_url ? 'text-emerald-400' : 'text-slate-500'">
              {{ v.teslamate_api_url ? 'Active' : 'Non configurée' }}
            </p>
          </div>
        </div>

        <!-- Actions -->
        <div class="mt-4 pt-3 flex items-center justify-between gap-2">
          <button
            v-if="v.teslamate_api_url"
            @click="testConnection(v.id)"
            :disabled="testLoading"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5"
          >
            <Link2 class="w-3.5 h-3.5" />
            Tester l'API
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

        <!-- Test feedback -->
        <div v-if="testResult" class="mt-3 p-3 rounded-xl text-xs" :class="testResult.success ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'">
          <span v-if="testResult.success">
            Connexion réussie ! Statut : {{ testResult.status?.state }} (Odomètre : {{ Math.round(testResult.status?.odometer) }} km)
          </span>
          <span v-else>{{ testResult.error }}</span>
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
