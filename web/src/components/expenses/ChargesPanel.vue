<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { Zap, Pencil, Trash2, AlertTriangle, Paperclip } from 'lucide-vue-next'
import { formatDate } from '@/utils/expenses'

defineProps<{
  charges: any[]
  chargesTotal: number
  chargesWithoutCost: number
  loading: boolean
  loadingMoreCharges: boolean
  missingCostOnly: boolean
}>()
const emit = defineEmits<{
  'toggle-missing-cost': []
  'load-more': []
  edit: [charge: any]
  delete: [charge: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div class="space-y-3">
    <!-- Pre-TeslaMate Charges Banner if configured -->
    <div
      v-if="vehicleStore.activeVehicle?.pre_teslamate_kwh_100km && vehicleStore.activeVehicle?.pre_teslamate_eur_per_kwh"
      class="p-3 bg-sky-500/10 border border-sky-500/20 rounded-2xl flex items-center justify-between text-xs text-sky-300"
    >
      <div class="flex items-center gap-2">
        <Zap class="w-4 h-4 shrink-0 text-sky-400" />
        <span>
          Estimation avant TeslaMate active :
          <strong>{{ vehicleStore.activeVehicle.pre_teslamate_kwh_100km }} kWh/100km</strong> à
          <strong>{{ Number(vehicleStore.activeVehicle.pre_teslamate_eur_per_kwh).toFixed(4) }} €/kWh</strong>
          (intégrée automatiquement dans le TCO).
        </span>
      </div>
      <button
        @click="router.push('/vehicles')"
        class="shrink-0 font-medium underline hover:text-sky-200 transition-colors ml-2"
      >
        Modifier
      </button>
    </div>

    <button
      v-if="chargesWithoutCost > 0 || missingCostOnly"
      @click="emit('toggle-missing-cost')"
      class="w-full p-3 rounded-2xl text-left text-xs font-semibold flex items-center gap-2 border transition-colors"
      :class="missingCostOnly ? 'bg-amber-500/20 border-amber-500/40 text-amber-300' : 'bg-amber-500/10 border-amber-500/20 text-amber-400 hover:bg-amber-500/15'"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span v-if="missingCostOnly">Affichage des recharges sans coût uniquement — cliquer pour tout afficher</span>
      <span v-else>{{ chargesWithoutCost }} recharge(s) sans coût : le TCO est sous-estimé. Cliquer pour les compléter.</span>
    </button>
    <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
    <div v-else-if="!charges.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
      {{ vehicleStore.hasTeslaMate ? 'Aucune recharge enregistrée. Synchronisez votre véhicule avec TeslaMate ou ajoutez une recharge manuelle.' : 'Aucune recharge enregistrée. Ajoutez une recharge pour suivre le coût de l\'énergie.' }}
    </div>
    <div v-else class="space-y-3">
      <div
        v-for="c in charges"
        :key="c.id"
        class="bg-slate-900 border p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        :class="c.cost === null ? 'border-amber-500/40' : 'border-slate-800'"
      >
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-sky-500/10 text-sky-400 border border-sky-500/20 shrink-0">
              +{{ c.kwh_added }} kWh
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(c.date) }}</span>
            <span v-if="c.is_manual" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">Manuelle</span>
            <span v-else-if="c.cost_source === 'MANUAL'" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">Coût corrigé</span>
            <button
              v-if="c.document_id"
              @click="emit('view-document', c.document_id, c.document_filename, false)"
              class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
              title="Voir le justificatif"
            >
              <Paperclip class="w-3 h-3 shrink-0" />
              <span class="truncate">{{ c.document_filename || 'Facture' }}</span>
            </button>
          </div>
          <p class="text-sm text-slate-300 mt-1 truncate" :title="c.address">{{ c.address || 'Lieu de recharge inconnu' }}</p>
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-left sm:text-right">
            <template v-if="c.cost !== null">
              <span class="text-lg font-extrabold text-sky-400">{{ c.cost.toFixed(2) }} {{ c.currency }}</span>
              <p v-if="c.kwh_added > 0" class="text-[11px] text-slate-400">
                {{ (c.cost / c.kwh_added).toFixed(3) }} {{ c.currency }}/kWh
              </p>
            </template>
            <span v-else class="text-xs font-bold text-amber-400 flex items-center gap-1">
              <AlertTriangle class="w-3.5 h-3.5" /> Coût manquant
            </span>
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('edit', c)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-sky-400 rounded-xl transition-colors border border-slate-700/60"
              :title="c.is_manual ? 'Modifier cette recharge' : 'Renseigner / corriger le coût'"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              v-if="c.is_manual"
              @click="emit('delete', c)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              title="Supprimer cette recharge"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
      <div class="flex items-center justify-between text-xs text-slate-400 px-1">
        <span>{{ charges.length }} recharge(s) affichée(s) sur {{ chargesTotal }}</span>
        <button
          v-if="charges.length < chargesTotal"
          @click="emit('load-more')"
          :disabled="loadingMoreCharges"
          class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 disabled:opacity-50"
        >
          {{ loadingMoreCharges ? 'Chargement...' : 'Charger plus' }}
        </button>
      </div>
    </div>
  </div>
</template>
