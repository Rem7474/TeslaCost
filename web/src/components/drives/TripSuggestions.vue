<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { Sparkles, Zap, Timer } from 'lucide-vue-next'
import { formatTripDates } from '@/utils/drives'
import { useVehicleStore } from '@/stores/vehicle'

// Chains of ungrouped drives that look like one trip (short stops, or a charge in between), to turn into a trip in one click.
defineProps<{ suggestions: any[]; creatingKey: string | null }>()
const emit = defineEmits<{ create: [suggestion: any] }>()
const vehicleStore = useVehicleStore()

const key = (s: any) => s.drive_ids[0]
const route = (s: any) => [s.start_address, s.end_address].filter(Boolean).join(' → ')
</script>

<template>
  <div v-if="suggestions.length" class="space-y-2">
    <div class="flex items-start gap-2 text-xs text-slate-400 px-1">
      <Sparkles class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
      <div>
        <span class="font-bold text-white">{{ $t('drives.tripSuggestions.title', { count: suggestions.length }) }}</span>
        <span class="block">{{ $t('drives.tripSuggestions.help') }}</span>
      </div>
    </div>

    <div
      v-for="s in suggestions"
      :key="key(s)"
      class="bg-slate-900/60 border border-dashed border-amber-500/30 p-4 rounded-2xl flex flex-col lg:flex-row lg:items-center justify-between gap-3"
    >
      <div class="min-w-0">
        <div class="flex items-center gap-2 flex-wrap mb-1">
          <span class="text-xs font-semibold text-slate-400">{{ formatTripDates({ start_time: s.start_time, end_time: s.end_time }) }}</span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60">
            {{ Math.round(s.distance_km).toLocaleString(intlLocale()) }} km
          </span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/30">
            {{ $t('drives.tripGroupsPanel.legS', { length: s.drive_ids.length }) }}
          </span>
          <span
            class="text-xs px-2.5 py-0.5 rounded-full font-semibold border flex items-center gap-1"
            :class="s.reason === 'CHARGE' ? 'bg-sky-500/10 text-sky-300 border-sky-500/30' : 'bg-amber-500/10 text-amber-300 border-amber-500/30'"
          >
            <component :is="s.reason === 'CHARGE' ? Zap : Timer" class="w-3 h-3" />
            {{ s.reason === 'CHARGE' ? $t('drives.tripSuggestions.reasonCharge') : $t('drives.tripSuggestions.reasonPause') }}
          </span>
        </div>
        <div class="text-sm font-bold text-white truncate">{{ route(s) || $t('drives.tripSuggestions.unnamed') }}</div>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        type="button"
        :disabled="creatingKey === key(s)"
        @click="emit('create', s)"
        class="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition-colors disabled:opacity-50 self-start lg:self-auto shrink-0"
      >
        {{ creatingKey === key(s) ? '...' : $t('drives.tripSuggestions.create') }}
      </button>
    </div>
  </div>
</template>
