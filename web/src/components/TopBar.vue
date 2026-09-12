<script setup lang="ts">
import { useVehicleStore } from '@/stores/vehicle'
import { RefreshCw, Car, Gauge, Plus, AlertCircle, AlertTriangle, X, CheckCircle2 } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

const vehicleStore = useVehicleStore()
const router = useRouter()

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
          <select
            :value="vehicleStore.activeVehicle?.id"
            @change="onVehicleChange"
            class="bg-slate-800 text-slate-100 text-sm font-semibold rounded-lg px-3 py-1.5 border border-slate-700 focus:outline-none focus:border-rose-500 transition-colors"
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
          <span>+{{ vehicleStore.syncResult.drives_synced }} trajets, +{{ vehicleStore.syncResult.charges_synced }} charges</span>
          <button @click="vehicleStore.clearSyncStatus" class="ml-1 text-slate-400 hover:text-slate-200">
            <X class="w-3 h-3" />
          </button>
        </div>
      </div>
    </header>

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
          <strong>Synchronisation partielle :</strong> +{{ vehicleStore.syncResult.drives_synced }} trajets, +{{ vehicleStore.syncResult.charges_synced }} charges synchronisés.
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
