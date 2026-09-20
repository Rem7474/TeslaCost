<script setup lang="ts">
import { computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import { Users, Trash2, Edit2, Calendar, MapPin, Zap, Disc, Wrench, Shield, CreditCard, CheckCircle2, Receipt, Sparkles, CheckSquare, Square, Navigation, RotateCw, Download } from 'lucide-vue-next'
import { fmt, formatDate, stopNames } from '@/utils/carpool'

// The carpool trips with their legs and passengers, and the selection actions (recalculate, export)
const props = defineProps<{ trips: any[]; selectedTripIds: string[]; recalculating: boolean }>()
const emit = defineEmits<{
  clear: []
  'batch-recalculate': []
  export: []
  'toggle-all': []
  toggle: [tripId: string]
  recalculate: [trip: any]
  edit: [trip: any]
  delete: [trip: any]
}>()
const vehicleStore = useVehicleStore()

const isAllSelected = computed(() => props.trips.length > 0 && props.selectedTripIds.length === props.trips.length)
</script>

<template>
  <div class="space-y-4">
    <!-- Sticky Bulk Selection Bar -->
    <BulkSelectionBar
      v-if="vehicleStore.canEdit"
      :count="selectedTripIds.length"
      item-label="covoiturage"
      @clear="emit('clear')"
    >
      <button
        type="button"
        @click="emit('batch-recalculate')"
        :disabled="recalculating"
        class="px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-md shadow-rose-600/20 disabled:opacity-50 transition-all"
        title="Recalculer les coûts réels des covoiturages sélectionnés"
      >
        <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': recalculating }" />
        <span>Recalculer coûts réels ({{ selectedTripIds.length }})</span>
      </button>

      <button
        type="button"
        @click="emit('export')"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
        title="Exporter la sélection en CSV"
      >
        <Download class="w-3.5 h-3.5 text-slate-300" />
        <span>Exporter CSV</span>
      </button>
    </BulkSelectionBar>

    <!-- Header row: Select all checkbox & Total info -->
    <div v-if="vehicleStore.canEdit" class="flex items-center justify-between text-xs text-slate-400 px-2">
      <button
        type="button"
        @click="emit('toggle-all')"
        class="flex items-center gap-2 hover:text-slate-200 transition-colors"
      >
        <component :is="isAllSelected ? CheckSquare : Square" class="w-4 h-4 text-rose-400" />
        <span>{{ isAllSelected ? 'Tout désélectionner' : 'Tout sélectionner' }}</span>
      </button>
      <span>{{ trips.length }} covoiturage(s) au total</span>
    </div>

    <div
      v-for="trip in trips"
      :key="trip.id"
      class="bg-slate-900 border rounded-2xl p-5 transition-all shadow-sm space-y-4"
      :class="selectedTripIds.includes(trip.id) ? 'border-rose-500/50 bg-rose-500/[0.02]' : 'border-slate-800 hover:border-slate-700'"
    >
      <!-- Trip Header -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Select Checkbox -->
          <label
            v-if="vehicleStore.canEdit"
            :for="'carpool-select-' + trip.id"
            class="mt-1 shrink-0 flex items-center cursor-pointer"
            title="Sélectionner ce covoiturage"
          >
            <span class="sr-only">Sélectionner ce covoiturage</span>
            <input
              :id="'carpool-select-' + trip.id"
              type="checkbox"
              :checked="selectedTripIds.includes(trip.id)"
              @change="emit('toggle', trip.id)"
              class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
            />
          </label>

          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2.5 flex-wrap min-w-0">
              <h3 class="text-base font-bold text-white truncate max-w-sm sm:max-w-md" :title="trip.title">{{ trip.title }}</h3>
              <span class="text-xs bg-slate-800 text-slate-300 px-2.5 py-0.5 rounded-full border border-slate-700/60 font-medium shrink-0">{{ trip.distance_km }} km</span>
              <span
                class="text-xs px-2.5 py-0.5 rounded-full font-semibold border shrink-0"
                :class="trip.net_cost <= 0 ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
              >
                {{ trip.net_cost <= 0 ? 'Trajet 100% rentabilisé !' : `Amorti à ${trip.total_cost > 0 ? Math.min(100, Math.round((trip.total_revenue / trip.total_cost) * 100)) : 0}%` }}
              </span>
            </div>
            <div class="flex items-center gap-2 text-xs text-slate-400 min-w-0">
              <Calendar class="w-3.5 h-3.5 shrink-0" />
              <span class="shrink-0">{{ formatDate(trip.date) }}</span>
              <span v-if="trip.notes" class="text-slate-500 truncate">• {{ trip.notes }}</span>
            </div>
          </div>
        </div>
        <div v-if="vehicleStore.canEdit" class="flex items-center gap-1 sm:gap-2 shrink-0 self-end sm:self-auto">
          <button
            @click="emit('recalculate', trip)"
            :disabled="recalculating"
            class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
            title="Recalculer les coûts réels de ce covoiturage"
          >
            <RotateCw class="w-4 h-4" />
          </button>
          <button @click="emit('edit', trip)" class="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors" title="Modifier">
            <Edit2 class="w-4 h-4" />
          </button>
          <button @click="emit('delete', trip)" class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors" title="Supprimer">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- Legs -->
      <div class="space-y-2">
        <div class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
          <Navigation class="w-3.5 h-3.5 text-indigo-400" />
          Étapes ({{ trip.legs.length }})
        </div>
        <div class="flex flex-wrap gap-2">
          <div
            v-for="(leg, i) in trip.legs"
            :key="leg.id"
            class="bg-slate-950/60 border border-slate-800/80 rounded-xl px-2.5 py-1.5 text-[11px] text-slate-300"
          >
            <div class="font-semibold text-slate-200">{{ stopNames(trip.legs)[Number(i)] }} → {{ stopNames(trip.legs)[Number(i) + 1] }}</div>
            <div class="text-slate-400">
              {{ leg.distance_km }} km • {{ fmt(leg.total_cost) }} € •
              {{ 1 + leg.passenger_seats }} à bord • {{ fmt(leg.cost_per_person) }} €/pers.
            </div>
          </div>
        </div>
      </div>

      <!-- Passengers -->
      <div class="space-y-2">
        <div class="text-xs font-semibold text-slate-300 flex items-center justify-between">
          <span class="flex items-center gap-1.5">
            <Users class="w-3.5 h-3.5 text-blue-400" />
            Passagers ({{ trip.passengers?.length || 0 }})
          </span>
          <span class="text-emerald-400 font-bold">Total perçu : +{{ fmt(trip.total_revenue) }} €</span>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5">
          <div v-for="p in trip.passengers" :key="p.id" class="bg-slate-950/60 border border-slate-800/80 rounded-xl p-2.5 text-xs space-y-1">
            <div class="flex items-center justify-between gap-2">
              <span class="font-semibold text-slate-200 truncate">{{ p.passenger_name }}</span>
              <span class="font-bold text-emerald-400 shrink-0">+{{ fmt(p.amount_paid) }} €</span>
            </div>
            <div class="text-[11px] text-slate-400 flex items-center gap-1 truncate">
              <MapPin class="w-3 h-3 shrink-0 text-slate-500" />
              <span class="truncate">
                {{ stopNames(trip.legs)[p.board_stop_index] }} → {{ stopNames(trip.legs)[p.alight_stop_index] }}
                • {{ p.seats }} place{{ p.seats > 1 ? 's' : '' }}
              </span>
            </div>
            <div class="flex items-center justify-between text-[11px]">
              <span class="text-slate-400">Part : {{ fmt(p.cost_share) }} €</span>
              <span :class="p.balance >= 0 ? 'text-emerald-400' : 'text-amber-400'">
                {{ p.balance >= 0 ? `+${fmt(p.balance)} € au-dessus` : `${fmt(-p.balance)} € sous sa part` }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Cost Breakdown -->
      <div class="pt-3 border-t border-slate-800/60 space-y-2">
        <div class="text-xs font-semibold text-slate-400">Coûts réels du trajet :</div>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <Zap class="w-3.5 h-3.5 text-amber-400" /> Électricité : <strong>{{ fmt(trip.electricity_cost) }} €</strong>
          </div>
          <div v-if="trip.tolls_cost > 0" class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <CreditCard class="w-3.5 h-3.5 text-blue-400" /> Péages : <strong>{{ fmt(trip.tolls_cost) }} €</strong>
          </div>
          <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <Disc class="w-3.5 h-3.5 text-rose-400" /> Usure pneus : <strong>{{ fmt(trip.tires_cost) }} €</strong>
          </div>
          <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <Wrench class="w-3.5 h-3.5 text-indigo-400" /> Entretien : <strong>{{ fmt(trip.maintenance_cost) }} €</strong>
          </div>
          <div class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <Shield class="w-3.5 h-3.5 text-emerald-400" /> Assurance : <strong>{{ fmt(trip.insurance_cost) }} €</strong>
          </div>
          <div v-if="trip.other_cost > 0" class="bg-slate-800/80 border border-slate-700/60 px-2.5 py-1 rounded-lg flex items-center gap-1.5 text-slate-300">
            <Receipt class="w-3.5 h-3.5 text-slate-400" /> Divers : <strong>{{ fmt(trip.other_cost) }} €</strong>
          </div>
        </div>
      </div>

      <!-- Bottom Line -->
      <div
        class="rounded-xl p-3 flex flex-col lg:flex-row lg:items-center justify-between gap-3 text-xs"
        :class="trip.net_cost <= 0 ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-300' : 'bg-slate-950/80 border border-slate-800 text-slate-300'"
      >
        <div class="flex items-center gap-2 flex-wrap">
          <CheckCircle2 v-if="trip.net_cost <= 0" class="w-4 h-4 text-emerald-400 shrink-0" />
          <Sparkles v-else class="w-4 h-4 text-amber-400 shrink-0" />
          <span>Coût réel : <strong>{{ fmt(trip.total_cost) }} €</strong></span>
          <span>•</span>
          <span>Part des passagers : <strong>{{ fmt(trip.passengers_cost_share) }} €</strong></span>
          <span>•</span>
          <span>Part du conducteur : <strong>{{ fmt(trip.driver_cost_share) }} €</strong></span>
        </div>
        <div>
          <template v-if="trip.net_cost > 0">
            Reste à charge conducteur : <strong class="text-white text-sm">{{ fmt(trip.net_cost) }} €</strong>
            <span v-if="trip.distance_km > 0" class="text-slate-400 text-[11px] ml-1">({{ (trip.net_cost / trip.distance_km).toFixed(3) }} €/km)</span>
          </template>
          <span v-else class="font-bold text-emerald-400 text-sm">Excédent net : +{{ fmt(Math.abs(trip.net_cost)) }} €</span>
        </div>
      </div>
    </div>
  </div>
</template>
