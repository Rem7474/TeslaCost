<script setup lang="ts">
import { computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useOfflineStore } from '@/stores/offline'
import { RefreshCw, Car, Gauge, Plus, AlertCircle, AlertTriangle, X, CheckCircle2, WifiOff, CloudUpload } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { APP_VERSION } from '@/version'

const vehicleStore = useVehicleStore()
const offlineStore = useOfflineStore()
const router = useRouter()

const syncSummary = computed(() => {
  const res = vehicleStore.syncResult
  if (!res) return ''
  const drives = res.drives_added !== undefined ? res.drives_added : res.drives_synced
  const charges = res.charges_added !== undefined ? res.charges_added : res.charges_synced
  const parts: string[] = []
  if (drives > 0) parts.push(`+${drives} trajet${drives > 1 ? 's' : ''}`)
  if (charges > 0) parts.push(`+${charges} charge${charges > 1 ? 's' : ''}`)
  if (parts.length > 0) {
    return parts.join(', ')
  }
  return 'À jour (aucun nouveau trajet)'
})

function onVehicleChange(event: Event) {
  const target = event.target as HTMLSelectElement
  if (target.value === 'new') {
    router.push('/vehicles')
  } else {
    vehicleStore.setActiveVehicle(target.value)
  }
}
</script>

<template>
  <div>
    <header class="bg-slate-900/60 backdrop-blur-md border-b border-slate-800 sticky top-0 z-40 px-4 py-3 flex items-center justify-between">
      <!-- Vehicle Selector -->
      <div class="flex items-center gap-3">
        <div class="p-2 bg-slate-800 rounded-lg text-slate-400">
          <Car class="w-5 h-5" />
        </div>

        <div v-if="vehicleStore.vehicles.length" class="flex items-center gap-2">
          <label for="topbar-active-vehicle" class="sr-only">Véhicule actif</label>
          <select id="topbar-active-vehicle"
            :value="vehicleStore.activeVehicle?.id"
            @change="onVehicleChange"
            class="bg-slate-800 text-slate-100 text-sm font-semibold rounded-lg px-3 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500 transition-colors max-w-[130px] sm:max-w-[200px] md:max-w-xs truncate"
          >
            <option v-for="v in vehicleStore.vehicles" :key="v.id" :value="v.id">
              {{ v.name }}
            </option>
            <option value="new">+ Ajouter un véhicule</option>
          </select>

          <!-- Odometer pill -->
          <div class="hidden sm:flex items-center gap-1.5 bg-slate-800/80 px-2.5 py-1 rounded-lg text-xs text-slate-300 border border-slate-700/50">
            <Gauge class="w-3.5 h-3.5 text-rose-400" />
            <span>{{ Math.round(vehicleStore.activeVehicle?.current_odometer || 0).toLocaleString('fr-FR') }} km</span>
          </div>
        </div>

        <div v-else-if="!vehicleStore.isInitialized || vehicleStore.isLoading" class="flex items-center gap-2">
          <div class="w-4 h-4 border-2 border-rose-500 border-t-transparent rounded-full animate-spin"></div>
          <span class="text-xs text-slate-400">Chargement...</span>
        </div>

        <div v-else>
          <button
            @click="router.push('/vehicles')"
            class="text-xs bg-rose-600 hover:bg-rose-500 text-white font-medium px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
          >
            <Plus class="w-3.5 h-3.5" />
            Ajouter un véhicule
          </button>
        </div>
      </div>

      <!-- Actions (Sync) -->
      <div class="flex items-center gap-2">
        <!-- Offline queue -->
        <div
          v-if="!offlineStore.isOnline || offlineStore.pendingCount > 0"
          class="flex items-center gap-1.5 text-[11px] px-2.5 py-1 rounded-lg border"
          :class="offlineStore.isOnline ? 'text-sky-300 bg-sky-500/10 border-sky-500/20' : 'text-amber-300 bg-amber-500/10 border-amber-500/20'"
          :title="offlineStore.isOnline ? 'Envoi des saisies enregistrées hors ligne' : 'Les saisies sont conservées et seront envoyées au retour du réseau'"
        >
          <WifiOff v-if="!offlineStore.isOnline" class="w-3.5 h-3.5 shrink-0" />
          <CloudUpload v-else class="w-3.5 h-3.5 shrink-0" :class="{ 'animate-pulse': offlineStore.isFlushing }" />
          <span v-if="!offlineStore.isOnline">Hors ligne</span>
          <span v-if="offlineStore.pendingCount > 0">{{ offlineStore.pendingCount }} en attente</span>
        </div>

        <button
          v-if="vehicleStore.activeVehicle?.teslamate_api_url"
          @click="vehicleStore.syncActiveVehicle"
          :disabled="vehicleStore.isSyncing"
          class="flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-all"
          :class="
            vehicleStore.isSyncing
              ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
              : 'bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700'
          "
          title="Synchroniser avec TeslaMate"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': vehicleStore.isSyncing }" />
          <span class="hidden sm:inline">{{ vehicleStore.isSyncing ? 'Synchronisation...' : 'Synchroniser' }}</span>
        </button>

        <!-- Sync result success -->
        <div
          v-if="vehicleStore.syncResult && !vehicleStore.syncResult.warnings?.length"
          class="hidden md:flex items-center gap-1.5 text-[11px] text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2.5 py-1 rounded-lg"
        >
          <CheckCircle2 class="w-3 h-3 shrink-0" />
          <span>{{ syncSummary }}</span>
          <button @click="vehicleStore.clearSyncStatus" class="ml-1 text-slate-400 hover:text-slate-200">
            <X class="w-3 h-3" />
          </button>
        </div>

        <span class="text-[10px] font-mono px-2 py-1 rounded-lg bg-slate-800/80 text-slate-400 border border-slate-700/50 font-medium">
          {{ APP_VERSION }}
        </span>
      </div>
    </header>

    <!-- Offline confirmation -->
    <div
      v-if="offlineStore.lastQueuedLabel"
      class="bg-sky-500/15 border-b border-sky-500/30 px-4 py-2 text-xs text-sky-200 flex items-center gap-2"
    >
      <CloudUpload class="w-4 h-4 shrink-0 text-sky-400" />
      <span><strong>{{ offlineStore.lastQueuedLabel }}</strong> enregistré hors ligne : envoi automatique au retour du réseau.</span>
    </div>

    <!-- Offline replay failures -->
    <div
      v-if="offlineStore.failures.length"
      class="bg-rose-500/15 border-b border-rose-500/30 px-4 py-2.5 text-xs text-rose-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
        <span>
          <strong>Saisies hors ligne refusées :</strong>
          {{ offlineStore.failures.map((f) => `${f.label} (${f.error})`).join(' ; ') }}
        </span>
      </div>
      <button @click="offlineStore.dismissFailures" class="text-rose-400 hover:text-white p-1 rounded transition-colors">
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Error Banner for Sync Failure -->
    <div
      v-if="vehicleStore.syncError"
      class="bg-rose-500/15 border-b border-rose-500/30 px-4 py-2.5 text-xs text-rose-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
        <span><strong>Erreur de synchronisation :</strong> {{ vehicleStore.syncError }}</span>
      </div>
      <button
        @click="vehicleStore.clearSyncStatus"
        class="text-rose-400 hover:text-white p-1 rounded transition-colors"
      >
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Warning Banner for Partial Sync -->
    <div
      v-if="vehicleStore.syncResult?.warnings?.length"
      class="bg-amber-500/15 border-b border-amber-500/30 px-4 py-2.5 text-xs text-amber-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-2">
        <AlertTriangle class="w-4 h-4 shrink-0 text-amber-400" />
        <span>
          <strong>Synchronisation terminée avec remarques :</strong> {{ syncSummary }}.
          <span class="text-amber-200/80">({{ vehicleStore.syncResult.warnings.join(' ; ') }})</span>
        </span>
      </div>
      <button
        @click="vehicleStore.clearSyncStatus"
        class="text-amber-400 hover:text-white p-1 rounded transition-colors"
      >
        <X class="w-4 h-4" />
      </button>
    </div>
  </div>
</template>
