<script setup lang="ts">
import { ref, watch } from 'vue'
import { ClipboardPaste, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { emptySessionForm, formatDate, type SessionForm } from '@/utils/tires'

// Adds or edits (editingSessionId set) a mount session of selectedTire; initialForm seeds the fields when the modal opens
const props = defineProps<{
  vehicleId: string
  selectedTire: any | null
  copiedSession: SessionForm | null
  editingSessionId: string | null
  initialForm: SessionForm
}>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const sessionForm = ref<SessionForm>(emptySessionForm())

watch(open, (isOpen) => {
  if (isOpen) sessionForm.value = { ...props.initialForm }
})

function applyCopiedSessionToForm() {
  if (!props.copiedSession) return
  sessionForm.value = {
    ...sessionForm.value,
    ...props.copiedSession,
    position: sessionForm.value.position || props.copiedSession.position,
  }
}

function onSessionOdometerChange() {
  const mount = Number(sessionForm.value.mounted_odometer) || 0
  const dismount = Number(sessionForm.value.dismounted_odometer) || 0
  if (dismount > mount) {
    sessionForm.value.distance_km = dismount - mount
  }
}

async function handleSaveSession() {
  if (!props.vehicleId || !props.selectedTire) return

  try {
    const payload: any = {
      position: sessionForm.value.position,
      mounted_date: new Date(sessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(sessionForm.value.mounted_odometer),
      distance_km: Number(sessionForm.value.distance_km),
      notes: sessionForm.value.notes ? sessionForm.value.notes : null,
    }

    if (sessionForm.value.is_dismounted) {
      payload.dismounted_date = new Date(sessionForm.value.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(sessionForm.value.dismounted_odometer)
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    if (props.editingSessionId) {
      await api.updateTireSession(props.vehicleId, props.selectedTire.id, props.editingSessionId, payload)
    } else {
      await api.createTireSession(props.vehicleId, props.selectedTire.id, payload)
    }

    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
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
        <h3 class="text-base font-bold text-white">
          {{ editingSessionId ? 'Modifier la session de montage' : 'Ajouter une session de montage passée' }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <!-- Quick paste banner if session copied -->
        <button
          v-if="copiedSession && !editingSessionId"
          type="button"
          @click="applyCopiedSessionToForm()"
          class="w-full px-3 py-2 bg-indigo-950/40 border border-indigo-800/60 rounded-xl text-indigo-300 hover:text-white text-xs flex items-center justify-center gap-2 transition-colors font-semibold"
        >
          <ClipboardPaste class="w-4 h-4 text-indigo-400" />
          <span>Coller les données de la session copiée ({{ copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '' }})</span>
        </button>

        <div>
          <label for="tire-session-position" class="block text-slate-400 mb-1 font-semibold">Position occupée</label>
          <select id="tire-session-position"
            v-model="sessionForm.position"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
          >
            <option value="FL">Avant Gauche (FL)</option>
            <option value="FR">Avant Droit (FR)</option>
            <option value="RL">Arrière Gauche (RL)</option>
            <option value="RR">Arrière Droit (RR)</option>
            <option value="STORAGE">Au garage / Non spécifié</option>
          </select>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <div>
            <label for="tire-session-mounted-date" class="block text-slate-400 mb-1 font-semibold">Date de montage</label>
            <AppDatePicker
              id="tire-session-mounted-date"
              v-model="sessionForm.mounted_date"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-session-mounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre montage (km)</label>
            <input id="tire-session-mounted-odometer"
              v-model.number="sessionForm.mounted_odometer"
              @input="onSessionOdometerChange"
              type="number"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
        </div>

        <div class="pt-1">
          <label class="flex items-center gap-2 cursor-pointer text-slate-300">
            <input type="checkbox" v-model="sessionForm.is_dismounted" class="rounded accent-rose-500" />
            <span>Cette session est terminée (pneu démonté)</span>
          </label>
        </div>

        <div v-if="sessionForm.is_dismounted" class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-xl border border-slate-800">
          <div>
            <label for="tire-session-dismounted-date" class="block text-slate-400 mb-1 font-semibold">Date démontage</label>
            <AppDatePicker
              id="tire-session-dismounted-date"
              v-model="sessionForm.dismounted_date"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-session-dismounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre démontage (km)</label>
            <input id="tire-session-dismounted-odometer"
              v-model.number="sessionForm.dismounted_odometer"
              @input="onSessionOdometerChange"
              type="number"
              class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
            />
          </div>
        </div>

        <div>
          <label for="tire-session-distance-km" class="block text-slate-400 mb-1 font-semibold">Distance de la session (km)</label>
          <input id="tire-session-distance-km"
            v-model.number="sessionForm.distance_km"
            type="number"
            placeholder="Auto-calculé ou forcé"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
          />
        </div>

        <div>
          <label for="tire-session-notes" class="block text-slate-400 mb-1 font-semibold">Commentaire / Notes</label>
          <input id="tire-session-notes"
            v-model="sessionForm.notes"
            type="text"
            placeholder="Ex: Saison hivernale 2024"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
          />
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          Annuler
        </button>
        <button
          type="button"
          @click="handleSaveSession"
          class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
        >
          Enregistrer
        </button>
      </div>
    </div>
  </div>
</template>
