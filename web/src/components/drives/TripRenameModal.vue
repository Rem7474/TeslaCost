<script setup lang="ts">
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Layers, X } from 'lucide-vue-next'

// Renames a trip group and edits its notes.
const props = defineProps<{ vehicleId: string; trip: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const tripEditForm = ref({ id: '', name: '', notes: '' })

watch(open, (isOpen) => {
  if (isOpen && props.trip) tripEditForm.value = { id: props.trip.id, name: props.trip.name, notes: props.trip.notes || '' }
})

async function handleSaveTripEdit() {
  if (!props.vehicleId || !tripEditForm.value.name.trim()) return
  try {
    await api.updateTripGroup(props.vehicleId, tripEditForm.value.id, {
      name: tripEditForm.value.name,
      notes: tripEditForm.value.notes || null,
    })
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Layers class="w-5 h-5 text-indigo-400" />
          {{ $t('drives.tripRenameModal.editTheTrip') }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="trip-edit-form" @submit.prevent="handleSaveTripEdit" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <label for="trip-edit-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.tripRenameModal.name') }}</label>
          <input id="trip-edit-name" v-model="tripEditForm.name" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
        </div>
        <div>
          <label for="trip-edit-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('common.notes') }}</label>
          <input id="trip-edit-notes" v-model="tripEditForm.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="trip-edit-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
