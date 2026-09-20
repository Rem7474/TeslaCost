<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { MapPin, Clock, Users, Coins, Plus, Ban } from 'lucide-vue-next'
import { needsTollQualification } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'

// One drive of the list: click opens its cost breakdown.
defineProps<{ d: any; selected: boolean }>()
const emit = defineEmits<{ open: [drive: any]; toggle: [drive: any]; 'toll-entry': [drive: any]; 'no-toll': [drive: any] }>()
const router = useRouter()
const vehicleStore = useVehicleStore()
const formatDate = formatDayTime
</script>

<template>
  <div
    @click="emit('open', d)"
    class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 cursor-pointer group"
    :class="{ 'border-rose-500/40 bg-slate-800/40 shadow-lg shadow-rose-950/20': selected }"
  >
    <div class="flex items-start gap-3 min-w-0 flex-1">
      <!-- Selection checkbox -->
      <label
        v-if="vehicleStore.canEdit"
        :for="'drive-select-' + d.id"
        @click.stop
        class="mt-1 shrink-0 flex items-center cursor-pointer"
        title="Sélectionner ce trajet"
      >
        <span class="sr-only">Sélectionner ce trajet</span>
        <input
          :id="'drive-select-' + d.id"
          type="checkbox"
          :checked="selected"
          @change="emit('toggle', d)"
          class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
        />
      </label>

      <!-- Drive Details -->
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap mb-1.5">
          <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatDate(d.start_time) }}</span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
            {{ d.distance_km }} km
          </span>
          <span v-if="d.duration_min" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
            <Clock class="w-3 h-3" /> {{ d.duration_min }} min
          </span>
          <span v-if="d.consumption_kwh_100km" class="text-xs text-sky-400 font-mono shrink-0">
            {{ d.consumption_kwh_100km }} kWh/100km
          </span>
          <!-- Clean tag pills -->
          <span
            v-if="d.tags?.includes('Pro')"
            class="text-[10px] px-2 py-0.5 rounded-full font-bold bg-blue-500/20 text-blue-400 border border-blue-500/40 shrink-0"
          >
            Pro
          </span>
          <span
            v-if="d.tags?.includes('Perso')"
            class="text-[10px] px-2 py-0.5 rounded-full font-bold bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 shrink-0"
          >
            Perso
          </span>
        </div>

        <!-- Route Address -->
        <div class="text-sm text-slate-300 flex items-center gap-1.5 flex-wrap min-w-0">
          <MapPin class="w-3.5 h-3.5 text-rose-400 shrink-0" />
          <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.start_address">{{ d.start_address || 'Départ inconnu' }}</span>
          <span class="text-slate-500 shrink-0">→</span>
          <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.end_address">{{ d.end_address || 'Arrivée inconnue' }}</span>
        </div>
      </div>
    </div>

    <!-- Right Side: Cost Badge & Actions -->
    <div class="flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0" @click.stop>
      <!-- Toll qualification: 2 taps -->
      <div v-if="vehicleStore.canEdit && needsTollQualification(d)" class="flex items-center gap-1">
        <button
          @click="emit('toll-entry', d)"
          class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-amber-500/40 bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 flex items-center gap-1"
          title="Renseigner le péage de ce trajet"
        >
          <Plus class="w-3.5 h-3.5" /> Péage
        </button>
        <button
          @click="emit('no-toll', d)"
          class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-400 hover:text-white flex items-center gap-1"
          title="Confirmer que ce trajet n'a pas de péage"
        >
          <Ban class="w-3.5 h-3.5" /> Sans péage
        </button>
      </div>

      <!-- Real Cost Badge -->
      <div
        class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
        title="Coût de revient réel calculé pour ce trajet"
      >
        <div class="p-1 rounded-lg bg-emerald-500/10 text-emerald-400">
          <Coins class="w-3.5 h-3.5" />
        </div>
        <div>
          <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
            <span>{{ d.costs?.has_estimates ? '~' : '' }}{{ (d.costs?.total_cost || 0).toFixed(2) }} €</span>
            <span class="text-[10px] font-normal text-emerald-400 font-mono">
              {{ (d.costs?.cost_per_km || 0).toFixed(3) }} €/km
            </span>
          </div>
        </div>
      </div>

      <!-- Quick Carpool Button -->
      <button
        v-if="vehicleStore.canEdit"
        @click="router.push({ path: '/carpools', query: { new_drive_id: d.id } })"
        class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
        title="Créer un covoiturage depuis ce trajet"
      >
        <Users class="w-3.5 h-3.5 text-rose-500" />
        <span class="hidden md:inline">Covoiturer</span>
      </button>
    </div>
  </div>
</template>
