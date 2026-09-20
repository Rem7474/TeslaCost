<script setup lang="ts">
import { ref } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { Car, Plus, Trash2, Edit2, RefreshCw, CheckCircle2, AlertCircle, X, Gauge, Link2, FileText, Zap, Users, Pencil } from 'lucide-vue-next'
import { ownershipSummary } from '@/utils/vehicles'

// One vehicle of the list, with its contract summary and a TeslaMate connection test of its own
const props = defineProps<{ v: any; ownership: any | null }>()
const emit = defineEmits<{
  edit: [vehicle: any]
  delete: [vehicleId: string]
  members: [vehicle: any]
  ownership: [vehicle: any]
}>()
const vehicleStore = useVehicleStore()

const cardTest = ref<{ loading: boolean; success?: boolean; status?: any; error?: string } | undefined>(undefined)

async function testCardConnection() {
  cardTest.value = { loading: true }
  try {
    const res = await api.testTeslaMate(props.v.id)
    cardTest.value = { loading: false, success: true, status: res.status }
  } catch (err: any) {
    cardTest.value = { loading: false, success: false, error: err.message }
  }
}

function clearCardTestResult() {
  cardTest.value = undefined
}
</script>

<template>
  <div
    class="bg-slate-900 border border-slate-800 p-5 rounded-2xl relative transition-all"
    :class="{ 'border-rose-500/40 shadow-lg shadow-rose-500/5': vehicleStore.activeVehicleId === v.id }"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="flex min-w-0 items-center gap-3">
        <div class="shrink-0 p-3 bg-slate-800 rounded-xl text-rose-400">
          <Car class="w-6 h-6" />
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
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
          <p v-if="v.vin" class="break-all text-xs text-slate-400 font-mono">{{ v.vin }}</p>
        </div>
      </div>

      <div v-if="v.role === 'OWNER'" class="flex shrink-0 items-center gap-1">
        <button
          @click="emit('edit', v)"
          class="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
          title="Modifier"
        >
          <Edit2 class="w-4 h-4" />
        </button>
        <button
          @click="emit('delete', v.id)"
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
          @click="emit('ownership', v)"
          class="group text-left inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl text-xs font-semibold transition-all border"
          :class="
            ownership
              ? 'bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-300 border-indigo-500/30'
              : 'bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border-amber-500/30'
          "
          :title="ownership ? 'Cliquer pour modifier les termes du contrat' : 'Cliquer pour configurer l\'achat ou la location (LOA/LLD)'"
        >
          <FileText v-if="ownership" class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
          <Plus v-else class="w-3.5 h-3.5 text-amber-400 shrink-0" />
          <span>{{ ownership ? ownershipSummary(ownership) : 'Renseigner le contrat' }}</span>
          <Pencil v-if="ownership" class="w-3 h-3 text-indigo-400 opacity-60 group-hover:opacity-100 shrink-0 ml-0.5" />
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
            cardTest?.success
              ? 'text-emerald-400'
              : cardTest?.error
              ? 'text-rose-400'
              : v.teslamate_api_url
              ? 'text-emerald-400/80'
              : 'text-slate-500'
          "
        >
          {{
            v.role === 'OWNER'
              ? (cardTest?.success
                ? 'En ligne'
                : cardTest?.error
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
        @click="emit('members', v)"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
        title="Gérer les accès et co-conducteurs"
      >
        <Users class="w-3.5 h-3.5 text-violet-400" />
        <span>Partage & Accès</span>
      </button>
      <button
        v-if="v.role === 'OWNER'"
        @click="emit('ownership', v)"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
      >
        <FileText class="w-3.5 h-3.5 text-indigo-400" />
        <span>Acquisition & financement</span>
      </button>
      <router-link
        to="/manual"
        @click="vehicleStore.setActiveVehicle(v.id)"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
        title="Relevés kilométriques et saisies manuelles de ce véhicule"
      >
        <Gauge class="w-3.5 h-3.5 text-cyan-400" />
        <span>Suivi manuel</span>
      </router-link>
      <button
        v-if="v.role === 'OWNER' && v.teslamate_api_url"
        @click="testCardConnection()"
        :disabled="cardTest?.loading"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
      >
        <RefreshCw v-if="cardTest?.loading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
        <Link2 v-else class="w-3.5 h-3.5" />
        <span>{{ cardTest?.loading ? 'Test...' : 'Tester l\'API' }}</span>
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
      v-if="cardTest && !cardTest.loading"
      class="mt-3 p-3 rounded-xl text-xs flex items-start justify-between gap-2"
      :class="cardTest.success ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
    >
      <div class="flex items-start gap-2">
        <CheckCircle2 v-if="cardTest.success" class="w-4 h-4 shrink-0 text-emerald-400 mt-0.5" />
        <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
        <div>
          <span v-if="cardTest.success">
            Connexion réussie ! Statut : {{ cardTest.status?.state || 'En ligne' }} ({{ Math.round(cardTest.status?.odometer || 0).toLocaleString('fr-FR') }} km)
          </span>
          <span v-else>{{ cardTest.error }}</span>
        </div>
      </div>
      <button @click="clearCardTestResult()" class="text-slate-400 hover:text-white p-0.5">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
