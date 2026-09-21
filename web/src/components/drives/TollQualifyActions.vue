<script setup lang="ts">
import { Plus, Ban } from 'lucide-vue-next'

// The two taps that qualify a motorway-like drive or trip: enter its toll, or confirm it has none.
defineProps<{ tollTitle: string; noTollTitle: string }>()
const emit = defineEmits<{ 'toll-entry': []; 'no-toll': [] }>()
</script>

<template>
  <div class="flex items-center gap-1">
    <button
      type="button"
      @click="emit('toll-entry')"
      class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-amber-500/40 bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 flex items-center gap-1"
      :title="tollTitle"
    >
      <Plus class="w-3.5 h-3.5" /> {{ $t('drives.driveCard.toll') }}
    </button>
    <button
      type="button"
      @click="emit('no-toll')"
      class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-400 hover:text-white flex items-center gap-1"
      :title="noTollTitle"
    >
      <Ban class="w-3.5 h-3.5" /> {{ $t('drives.driveCard.noToll') }}
    </button>
  </div>
</template>
