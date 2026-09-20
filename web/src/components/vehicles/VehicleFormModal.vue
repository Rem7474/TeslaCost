<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { RefreshCw, CheckCircle2, AlertCircle, X, Link2 } from 'lucide-vue-next'
import { emptyVehicleForm, vehicleFormFrom } from '@/utils/vehicles'

// Adds a vehicle, or edits \`editing\`. The TeslaMate connection can be tested with the values typed so far.
const props = defineProps<{ editing: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const isEditing = computed(() => !!props.editing)
const editingId = computed(() => props.editing?.id ?? null)
const modalTestLoading = ref(false)
const modalTestResult = ref<{ success: boolean; status?: any; error?: string } | null>(null)
const form = ref(emptyVehicleForm())

watch(open, (isOpen) => {
  if (!isOpen) return
  modalTestResult.value = null
  form.value = props.editing ? vehicleFormFrom(props.editing) : emptyVehicleForm()
})

async function handleSave() {
  try {
    if (isEditing.value && editingId.value) {
      await api.updateVehicle(editingId.value, form.value)
    } else {
      await api.createVehicle(form.value)
    }
    open.value = false
    emit('saved')
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
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white">{{ isEditing ? 'Modifier le véhicule' : 'Ajouter un véhicule' }}</h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
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

        <!-- Energy estimate section -->
        <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
          <h4 class="text-xs font-bold text-sky-400 uppercase tracking-wider">Énergie estimée (Optionnel)</h4>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-pre-kwh" class="block text-xs font-semibold text-slate-300 mb-1">Conso (kWh/100km)</label>
              <input id="vehicle-pre-kwh" v-model.number="form.estimated_kwh_100km" type="number" step="0.1" min="1" max="100" placeholder="ex: 16.5" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-pre-rate" class="block text-xs font-semibold text-slate-300 mb-1">Tarif (€/kWh)</label>
              <input id="vehicle-pre-rate" v-model.number="form.estimated_price_per_kwh" type="number" step="0.0001" min="0.01" max="5" placeholder="ex: 0.22" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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

          <div>
            <label for="vehicle-teslamate-grafana-url" class="block text-xs font-semibold text-slate-300 mb-1">URL Grafana TeslaMate (optionnelle)</label>
            <input id="vehicle-teslamate-grafana-url" v-model="form.teslamate_grafana_url" type="url" placeholder="ex: http://192.168.1.50:3000" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            <p class="text-[11px] text-slate-500 mt-1">Ajoute un lien « Ouvrir dans TeslaMate » dans le détail de chaque trajet (dashboard Drive Details).</p>
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
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          Annuler
        </button>
        <button type="submit" form="vehicle-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          Enregistrer
        </button>
      </div>
    </div>
  </div>
</template>
