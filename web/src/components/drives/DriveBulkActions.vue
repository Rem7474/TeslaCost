<script setup lang="ts">
import { useVehicleStore } from '@/stores/vehicle'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import { Receipt, Layers, Users, Plus, Download } from 'lucide-vue-next'

// Actions on the drives selected in the list; the page runs them.
defineProps<{ selectedDriveIds: string[]; selectedOffPage: number; selectedSummaryMetrics: string; bulkApplyingToll: boolean }>()
const emit = defineEmits<{
  clear: []
  carpool: []
  group: []
  'bulk-toll': []
  tag: [tag: 'Pro' | 'Perso']
  export: []
  'add-to-trip': []
}>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <BulkSelectionBar
    v-if="vehicleStore.canEdit"
    :count="selectedDriveIds.length"
    noun="drive"
    :off-screen-count="selectedOffPage"
    :metrics-summary="selectedSummaryMetrics"
    @clear="emit('clear')"
  >
    <!-- Direct Carpool button for single or multi-drives -->
    <button
      type="button"
      @click="emit('carpool')"
      class="px-3 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-1.5 shadow-lg shadow-rose-600/25 transition-all"
    >
      <Users class="w-3.5 h-3.5" />
      <span>{{ selectedDriveIds.length > 1 ? `Covoiturer (${selectedDriveIds.length})` : 'Covoiturer' }}</span>
    </button>

    <!-- Fusion Voyage Group -->
    <button
      type="button"
      @click="emit('group')"
      class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>Fusionner & Péage</span>
    </button>

    <!-- Auto toll -->
    <button
      v-if="vehicleStore.canEdit"
      type="button"
      @click="emit('bulk-toll')"
      :disabled="bulkApplyingToll"
      class="px-3 py-1.5 bg-cyan-700 hover:bg-cyan-600 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors disabled:opacity-50"
      title="Détecte les péages et enregistre le tarif estimé (sans toucher aux péages manuels)"
    >
      <Receipt class="w-3.5 h-3.5" />
      <span>{{ bulkApplyingToll ? 'Application...' : `Péage auto (${selectedDriveIds.length})` }}</span>
    </button>

    <!-- Batch Tag actions -->
    <button
      type="button"
      @click="emit('tag', 'Pro')"
      class="px-2.5 py-1.5 bg-blue-500/20 hover:bg-blue-500/30 text-blue-300 border border-blue-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
      title="Marquer la sélection comme trajets Pro"
    >
      <span>Pro</span>
    </button>

    <button
      type="button"
      @click="emit('tag', 'Perso')"
      class="px-2.5 py-1.5 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 border border-emerald-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
      title="Marquer la sélection comme trajets Perso"
    >
      <span>Perso</span>
    </button>

    <button
      type="button"
      @click="emit('export')"
      class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
      title="Exporter la sélection en CSV"
    >
      <Download class="w-3.5 h-3.5 text-slate-300" />
      <span>Export</span>
    </button>

    <button
      type="button"
      @click="emit('add-to-trip')"
      class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
    >
      <Plus class="w-3.5 h-3.5" />
      <span>Ajouter voyage</span>
    </button>
  </BulkSelectionBar>
</template>
