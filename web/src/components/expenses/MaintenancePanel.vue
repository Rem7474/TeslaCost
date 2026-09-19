<script setup lang="ts">
import { useVehicleStore } from '@/stores/vehicle'
import { Repeat, Pencil, Trash2, Paperclip } from 'lucide-vue-next'
import { CATEGORY_LABELS, formatDate } from '@/utils/expenses'

defineProps<{ maintenanceExpenses: any[]; loading: boolean }>()
const emit = defineEmits<{
  edit: [expense: any]
  delete: [expense: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div>
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
        <div class="space-y-1 min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-pink-500/10 text-pink-400 border border-pink-500/20 shrink-0">
              {{ CATEGORY_LABELS[m.category] || m.category }}
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(m.date) }}</span>
            <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
              <Repeat class="w-3 h-3 text-pink-400" /> tous les {{ m.recurrence_interval_months }} mois
              <template v-if="m.recurrence_end_date">jusqu'au {{ formatDate(m.recurrence_end_date) }}</template>
            </span>
            <span v-else-if="m.amortization_mode === 'DISTANCE'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shrink-0">
              Lissé sur {{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km
            </span>
            <span v-else-if="m.amortization_mode === 'DURATION'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20 shrink-0">
              Lissé sur {{ m.coverage_months || 24 }} mois
            </span>
            <span v-else-if="m.amortization_mode === 'HYBRID'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shrink-0">
              Lissé mixte ({{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km / {{ m.coverage_months || 24 }} mois)
            </span>
            <span v-if="m.closes_maintenance_id" class="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0">
              Clôture la révision précédente
            </span>
            <button
              v-if="m.document_id"
              @click="emit('view-document', m.document_id, m.document_filename, false)"
              class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
              title="Voir le justificatif"
            >
              <Paperclip class="w-3 h-3 shrink-0" />
              <span class="truncate">{{ m.document_filename || 'Facture' }}</span>
            </button>
          </div>
          <p class="text-sm font-semibold text-slate-200 truncate">{{ m.description }}</p>
          <p v-if="m.odometer" class="text-xs text-slate-400">À {{ Math.round(m.odometer) }} km</p>
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-lg font-extrabold text-pink-400">
            {{ m.amount.toFixed(2) }} {{ m.currency }}
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('edit', m)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-pink-400 rounded-xl transition-colors border border-slate-700/60"
              title="Modifier cette dépense"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', m)"
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
</template>
