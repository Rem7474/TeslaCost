<script setup lang="ts">
import { ref, watch } from 'vue'
import { Archive, X } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { getLastDismountInfo, isMountedPosition, todayIso } from '@/utils/tires'

const props = defineProps<{ vehicleId: string; selectedTireIds: string[]; tires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const batchDisposeForm = ref({
  date: todayIso(),
  odometer: '' as number | string,
})
const disposingBatch = ref(false)

watch(open, (isOpen) => {
  if (!isOpen) return
  const selectedEntries = props.tires.filter((t) => props.selectedTireIds.includes(t.tire.id))
  const anyMounted = selectedEntries.some((t) => isMountedPosition(t.tire.current_position))

  let defaultDate = todayIso()
  if (!anyMounted) {
    let latestTimestamp = 0
    for (const entry of selectedEntries) {
      const info = getLastDismountInfo(props.tires, entry.tire.id)
      if (info?.date) {
        const ts = new Date(info.date).getTime()
        if (ts > latestTimestamp) {
          latestTimestamp = ts
          defaultDate = info.date
        }
      }
    }
  }

  batchDisposeForm.value = {
    date: defaultDate,
    odometer: anyMounted ? Math.round(props.currentOdometer || 0) : '',
  }
})

async function handleBatchDisposeSubmit() {
  if (!props.vehicleId || !props.selectedTireIds.length) return
  const count = props.selectedTireIds.length
  disposingBatch.value = true
  try {
    await api.batchDisposeTires(props.vehicleId, {
      tire_ids: props.selectedTireIds,
      date: new Date(batchDisposeForm.value.date).toISOString(),
      odometer: batchDisposeForm.value.odometer !== '' ? Number(batchDisposeForm.value.odometer) : null,
    })
    open.value = false
    showAlert(`${count} pneu(s) mis au rebut avec succès.`, 'Mise au rebut', 'success')
    emit('saved')
  } catch (err: any) {
    showAlert(`Erreur lors de la mise au rebut : ${err.message}`, 'Erreur', 'danger')
  } finally {
    disposingBatch.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[70] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2">
          <Archive class="w-5 h-5 text-amber-500" />
          <h3 class="text-base font-bold text-white">
            Mettre au rebut {{ selectedTireIds.length }} pneu(s)
          </h3>
        </div>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <p class="text-slate-400 leading-relaxed">
          Mettre ces pneus au rebut les archive définitivement (usure maximale, perforation, vente). Leurs sessions passées et coûts restent conservés dans l'historique et le coût au kilomètre.
        </p>

        <!-- Selected tires summary -->
        <div class="space-y-1.5 max-h-40 overflow-y-auto p-2 bg-slate-950/50 rounded-xl border border-slate-800">
          <div
            v-for="t in tires.filter(x => selectedTireIds.includes(x.tire.id))"
            :key="t.tire.id"
            class="flex items-center justify-between py-1 px-1.5 border-b border-slate-800/50 last:border-0"
          >
            <div>
              <span class="font-bold text-white">{{ t.tire.brand }} {{ t.tire.model }}</span>
              <span class="text-[10px] text-slate-400 ml-1.5">({{ t.tire.dimension }})</span>
            </div>
            <span class="text-[10px] px-2 py-0.5 rounded font-mono bg-slate-800 text-slate-300">
              {{ ['FL', 'FR', 'RL', 'RR'].includes(t.tire.current_position) ? `Roue ${t.tire.current_position}` : 'Garage' }}
            </span>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
          <div>
            <label for="batch-dispose-date" class="block text-slate-300 mb-1 font-semibold">Date de mise au rebut</label>
            <input
              id="batch-dispose-date"
              v-model="batchDisposeForm.date"
              type="date"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-amber-500 focus:outline-none"
            />
            <p class="text-[10px] text-slate-500 mt-1">Par défaut : date du dernier démontage pour les pneus au garage.</p>
          </div>
          <div>
            <label for="batch-dispose-odo" class="block text-slate-300 mb-1 font-semibold">Kilométrage véhicule</label>
            <input
              id="batch-dispose-odo"
              v-model="batchDisposeForm.odometer"
              type="number"
              placeholder="Optionnel pour pneu garage"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-amber-500 focus:outline-none"
            />
            <p class="text-[10px] text-slate-500 mt-1">Odomètre final si démonté à cette date.</p>
          </div>
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
        >
          Annuler
        </button>
        <button
          type="button"
          @click="handleBatchDisposeSubmit()"
          :disabled="disposingBatch || !batchDisposeForm.date"
          class="px-4 py-2 bg-amber-600 hover:bg-amber-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-amber-600/20 transition-all flex items-center gap-1.5"
        >
          <Archive class="w-4 h-4" />
          <span>{{ disposingBatch ? 'Mise au rebut...' : `Mettre au rebut (${selectedTireIds.length})` }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
