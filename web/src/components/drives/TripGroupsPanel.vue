<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { Layers, X, Users, Coins, Pencil, Trash2 } from 'lucide-vue-next'
import { formatDriveDate, formatTripDates } from '@/utils/drives'

// The trip groups ("voyages") list; a trip can be expanded to show its drives.
defineProps<{ loadingTrips: boolean; tripGroups: any[]; expandedTripId: string | null; tripDrives: any[] }>()
const emit = defineEmits<{
  'open-cost': [trip: any]
  'toggle-details': [trip: any]
  edit: [trip: any]
  delete: [trip: any]
  'remove-drive': [trip: any, driveId: string]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()
const formatDate = formatDriveDate
</script>

<template>
  <div class="space-y-3">
    <div v-if="loadingTrips" class="text-center py-12 text-slate-400">Chargement...</div>
    <div v-else-if="!tripGroups.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
      Aucun voyage. Sélectionnez plusieurs trajets puis « Fusionner & Péage » pour en créer un.
    </div>
    <template v-else>
    <template
      v-for="tg in tripGroups"
      :key="tg.id"
    >
      <div
        @click="emit('open-cost', tg)"
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 cursor-pointer group"
      >
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Icon indicator -->
          <div class="mt-1 p-2 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0">
            <Layers class="w-4 h-4" />
          </div>

          <!-- Voyage Details (matching Drive details structure) -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap mb-1.5">
              <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatTripDates(tg) }}</span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
                {{ Math.round(tg.distance_km).toLocaleString('fr-FR') }} km
              </span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/30 shrink-0">
                {{ tg.drive_ids?.length || 0 }} étape(s)
              </span>
              <span v-if="tg.carpool_count" class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-rose-500/10 text-rose-300 border border-rose-500/30 shrink-0">
                {{ tg.carpool_count }} covoit
              </span>
            </div>

            <!-- Voyage Title & Notes (matching Route address line) -->
            <div class="text-sm text-slate-200 flex items-center gap-2 flex-wrap min-w-0">
              <span class="font-bold text-white truncate max-w-sm sm:max-w-md" :title="tg.name">{{ tg.name }}</span>
              <span v-if="tg.notes" class="text-xs text-slate-500 truncate max-w-xs">• {{ tg.notes }}</span>
            </div>
          </div>
        </div>

        <!-- Right Side: Cost Badge & Actions (matching Drive right-side) -->
        <div class="flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0" @click.stop>
          <!-- Real Cost Badge -->
          <div
            class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
            title="Coût consolidé du voyage (cliquez sur la ligne pour le détail)"
          >
            <div class="p-1 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Coins class="w-3.5 h-3.5" />
            </div>
            <div>
              <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
                <span>{{ Number(tg.expenses_total || 0) > 0 ? `${Number(tg.expenses_total).toFixed(2)} € frais` : 'Détail coûts' }}</span>
                <span v-if="tg.distance_km > 0 && tg.expenses_total" class="text-[10px] font-normal text-emerald-400 font-mono">
                  {{ (Number(tg.expenses_total) / tg.distance_km).toFixed(3) }} €/km
                </span>
              </div>
            </div>
          </div>

          <!-- Details toggle button -->
          <button
            @click="emit('toggle-details', tg)"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white transition-colors"
          >
            {{ expandedTripId === tg.id ? 'Masquer' : 'Étapes' }}
          </button>

          <!-- Quick Carpool Button -->
          <button
            v-if="vehicleStore.canEdit"
            @click="router.push({ path: '/carpools', query: { new_trip_group_id: tg.id } })"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
            title="Covoiturer ce voyage"
          >
            <Users class="w-3.5 h-3.5 text-rose-500" />
            <span class="hidden md:inline">Covoiturer</span>
          </button>

          <!-- Edit & Delete -->
          <template v-if="vehicleStore.canEdit">
            <button
              @click="emit('edit', tg)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-indigo-400 rounded-lg border border-slate-700/60 transition-colors"
              title="Renommer le voyage"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', tg)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-lg border border-slate-700/60 transition-colors"
              title="Supprimer le voyage"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </template>
        </div>
      </div>
      <!-- Expanded drives drawer -->
      <div v-if="expandedTripId === tg.id" class="space-y-1.5 bg-slate-950/40 border border-slate-800/80 rounded-2xl p-3 -mt-1 ml-4 mr-4">
        <div v-for="d in tripDrives" :key="d.id" class="flex items-center justify-between gap-3 text-xs text-slate-300 bg-slate-800/40 rounded-lg px-2.5 py-1.5 min-w-0">
          <span class="truncate min-w-0 flex-1">
            {{ formatDate(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}
            <span class="text-slate-500">({{ d.distance_km }} km)</span>
          </span>
          <button v-if="vehicleStore.canEdit" @click="emit('remove-drive', tg, d.id)" class="text-slate-500 hover:text-rose-400 shrink-0 p-1" title="Retirer ce trajet du voyage">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
        <p v-if="!tripDrives.length" class="text-xs text-slate-500">Chargement des trajets...</p>
      </div>
    </template>
    </template>
  </div>
</template>
