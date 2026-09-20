<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useDocumentAttach } from '@/composables/useDocumentAttach'
import { useVehicleStore } from '@/stores/vehicle'
import { Receipt, X, CheckSquare, Square, Paperclip, FileText, Eye } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import AppDropzone from '@/components/AppDropzone.vue'
import { CURRENCIES, countUnlistedDrives, currencyPayload, formatDate, toLocalDateTimeInput } from '@/utils/expenses'
import { formatDayTime } from '@/utils/dates'

// Adds a toll / parking expense, or edits it when `editing` is set. Its form is seeded when the modal opens.
const props = defineProps<{ vehicleId: string; editing: any | null; documents: ExpenseDocumentHeader[] }>()
const emit = defineEmits<{
  saved: []
  'document-added': [doc: ExpenseDocumentHeader]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()
const { isUploadingDocument, onSelectExistingDoc, onDropzoneDirectUpload } = useDocumentAttach(
  () => props.vehicleId,
  () => props.documents,
  (doc) => emit('document-added', doc)
)

const vehicleStore = useVehicleStore()

const editingTollId = computed(() => props.editing?.id ?? null)

const recentDrives = ref<any[]>([])
const associationMode = ref<'NONE' | 'SINGLE' | 'MULTI'>('NONE')
const selectedDriveId = ref('')
const selectedDriveIds = ref<string[]>([])

const tollForm = ref({
  type: 'TOLL',
  amount: '',
  currency: 'EUR',
  fx_rate: '',
  date: toLocalDateTimeInput(new Date()),
  notes: '',
  document_id: null as string | null,
  document_filename: null as string | null,
})

// Drives selected for a toll that are older than the loaded recent drives
const selectedDrivesNotListed = computed(() => countUnlistedDrives(selectedDriveIds.value, recentDrives.value))

watch(open, (isOpen) => {
  if (!isOpen) return
  const e = props.editing
  if (!e) {
    tollForm.value = {
      type: 'TOLL',
      amount: '',
      currency: 'EUR',
      fx_rate: '',
      date: toLocalDateTimeInput(new Date()),
      notes: '',
      document_id: null,
      document_filename: null,
    }
    associationMode.value = 'NONE'
    selectedDriveId.value = ''
    selectedDriveIds.value = []
  } else {
    tollForm.value = {
      type: e.type || 'TOLL',
      amount: String(e.amount),
      currency: e.currency || 'EUR',
      fx_rate: e.fx_rate ? String(e.fx_rate) : '',
      date: toLocalDateTimeInput(new Date(e.date)),
      notes: e.notes || '',
      document_id: e.document_id || null,
      document_filename: e.document_filename || null,
    }
    if (e.trip_group_id) {
      // Keep the trip group link: its drives are preselected in multi-step mode
      associationMode.value = 'MULTI'
      selectedDriveId.value = ''
      selectedDriveIds.value = [...(e.trip_group_drive_ids || [])]
    } else if (e.drive_id) {
      associationMode.value = 'SINGLE'
      selectedDriveId.value = e.drive_id
      selectedDriveIds.value = []
    } else {
      associationMode.value = 'NONE'
      selectedDriveId.value = ''
      selectedDriveIds.value = []
    }
  }
  loadRecentDrives()
})

async function loadRecentDrives() {
  if (!props.vehicleId) return
  try {
    const res = await api.getDrives(props.vehicleId, { limit: 200 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

function onSingleDriveChange() {
  const d = recentDrives.value.find((dr) => dr.id === selectedDriveId.value)
  if (d) {
    tollForm.value.date = new Date(d.start_time).toISOString().substring(0, 16)
    if (!tollForm.value.notes) {
      const from = (d.start_address || 'Départ').split(',')[0]
      const to = (d.end_address || 'Arrivée').split(',')[0]
      tollForm.value.notes = `Péage ${from} → ${to}`
    }
  }
}

function toggleMultiDrive(id: string) {
  const idx = selectedDriveIds.value.indexOf(id)
  if (idx > -1) {
    selectedDriveIds.value.splice(idx, 1)
  } else {
    selectedDriveIds.value.push(id)
  }

  const selected = recentDrives.value
    .filter((d) => selectedDriveIds.value.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())

  if (selected.length > 0) {
    tollForm.value.date = new Date(selected[0].start_time).toISOString().substring(0, 16)
    if (!tollForm.value.notes || tollForm.value.notes.startsWith('Péage ')) {
      const first = (selected[0].start_address || 'Départ').split(',')[0]
      const last = (selected[selected.length - 1].end_address || 'Arrivée').split(',')[0]
      tollForm.value.notes = `Péage ${first} → ${last} (${selected.length} étapes)`
    }
  }
}

async function handleCreateToll() {
  if (!props.vehicleId) return
  try {
    const payload: any = {
      type: tollForm.value.type,
      amount: Number(tollForm.value.amount),
      ...currencyPayload(tollForm.value),
      date: new Date(tollForm.value.date).toISOString(),
      notes: tollForm.value.notes,
      document_id: tollForm.value.document_id || null,
    }

    if (associationMode.value === 'SINGLE' && selectedDriveId.value) {
      payload.drive_id = selectedDriveId.value
    } else if (associationMode.value === 'MULTI' && selectedDriveIds.value.length === 1) {
      payload.drive_id = selectedDriveIds.value[0]
    } else if (associationMode.value === 'MULTI' && selectedDriveIds.value.length > 1) {
      payload.drive_ids = selectedDriveIds.value
    }

    if (editingTollId.value) {
      await api.updateDriveExpense(props.vehicleId, editingTollId.value, payload)
    } else {
      await api.createDriveExpense(props.vehicleId, payload)
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
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Receipt class="w-5 h-5 text-amber-400" />
          {{ editingTollId ? 'Modifier le Péage / Parking' : 'Ajouter un Péage / Parking' }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="toll-modal-form" @submit.prevent="handleCreateToll" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="expense-toll-type" class="block text-xs font-semibold text-slate-300 mb-1">Type</label>
            <select id="expense-toll-type" v-model="tollForm.type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
              <option value="TOLL">Péage</option>
              <option value="PARKING">Parking</option>
              <option value="FERRY">Ferry</option>
              <option value="OTHER">Autre</option>
            </select>
          </div>
          <div>
            <label for="toll-form-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
            <div class="flex gap-1.5">
              <input id="toll-form-amount" v-model="tollForm.amount" type="number" step="0.01" min="0.01" required placeholder="0.00" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <label for="toll-form-currency" class="sr-only">Devise</label>
              <select id="toll-form-currency" v-model="tollForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
        </div>
        <div v-if="tollForm.currency !== 'EUR'">
          <label for="toll-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ tollForm.currency }} = ? €)</label>
          <input id="toll-form-fx-rate" v-model="tollForm.fx_rate" type="number" step="0.000001" min="0.000001" required placeholder="ex: 1.05" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
        </div>

        <!-- Association à un/des trajets TeslaMate -->
        <div v-if="vehicleStore.hasTeslaMate || associationMode !== 'NONE'" class="space-y-2 bg-slate-800/50 p-3.5 rounded-xl border border-slate-700/60">
          <span class="block text-xs font-semibold text-slate-200">
            Associer à un trajet TeslaMate
          </span>

          <div class="grid grid-cols-3 gap-1.5 pt-1">
            <button
              type="button"
              @click="associationMode = 'NONE'"
              class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="associationMode === 'NONE' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Sans trajet
            </button>
            <button
              type="button"
              @click="associationMode = 'SINGLE'"
              class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="associationMode === 'SINGLE' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Trajet unique
            </button>
            <button
              type="button"
              @click="associationMode = 'MULTI'"
              class="py-1.5 px-2 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="associationMode === 'MULTI' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Multi-étapes
            </button>
          </div>

          <!-- Single drive selection -->
          <div v-if="associationMode === 'SINGLE'" class="pt-2 space-y-1.5">
            <label for="expense-selected-drive-id" class="block text-xs text-slate-400">Choisir le trajet :</label>
            <select id="expense-selected-drive-id"
              v-model="selectedDriveId"
              @change="onSingleDriveChange"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
            >
              <option value="">-- Sélectionner un trajet récent --</option>
              <option v-for="d in recentDrives" :key="d.id" :value="d.id">
                {{ formatDayTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }} ({{ d.distance_km.toFixed(1) }} km)
              </option>
            </select>
          </div>

          <!-- Multi drives selection -->
          <div v-if="associationMode === 'MULTI'" class="pt-2 space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-xs text-slate-400">Cocher les étapes composant le voyage :</span>
              <span class="text-[11px] text-amber-400 font-semibold">
                {{ selectedDriveIds.length }} étape(s)<template v-if="selectedDrivesNotListed"> dont {{ selectedDrivesNotListed }} plus ancienne(s) que les 200 derniers trajets</template>
              </span>
            </div>
            <div class="max-h-40 overflow-y-auto space-y-1.5 pr-1">
              <div
                v-for="d in recentDrives"
                :key="d.id"
                @click="toggleMultiDrive(d.id)"
                class="flex items-center justify-between p-2 rounded-lg cursor-pointer text-xs border transition-colors"
                :class="selectedDriveIds.includes(d.id) ? 'bg-amber-500/10 border-amber-500/40 text-amber-200' : 'bg-slate-800/80 border-slate-700 text-slate-300 hover:bg-slate-800'"
              >
                <div class="flex items-center gap-2">
                  <CheckSquare v-if="selectedDriveIds.includes(d.id)" class="w-4 h-4 text-amber-400" />
                  <Square v-else class="w-4 h-4 text-slate-500" />
                  <span>{{ formatDayTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}</span>
                </div>
                <span class="font-mono text-[11px] text-slate-400">{{ d.distance_km.toFixed(0) }} km</span>
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label for="expense-toll-date" class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
            <AppDatePicker id="expense-toll-date" v-model="tollForm.date" enable-time-picker size="xs" />
          </div>
          <div>
            <label for="expense-toll-notes" class="block text-xs font-semibold text-slate-300 mb-1">Notes / Description</label>
            <input id="expense-toll-notes" v-model="tollForm.notes" placeholder="A10 Paris-Bordeaux..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
          </div>
        </div>

        <!-- Justificatif / Facture -->
        <div class="space-y-2 bg-slate-800/40 p-3 rounded-xl border border-slate-700/60">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Paperclip class="w-3.5 h-3.5 text-indigo-400" />
              Justificatif / Facture
            </span>
            <span v-if="tollForm.document_id" class="text-[11px] text-emerald-400 font-medium">Lié</span>
          </div>

          <div v-if="tollForm.document_id" class="flex items-center justify-between p-2.5 bg-slate-900 border border-indigo-500/30 rounded-xl">
            <div class="flex items-center gap-2 min-w-0">
              <FileText class="w-4 h-4 text-indigo-400 shrink-0" />
              <span class="text-xs text-white truncate font-medium">{{ tollForm.document_filename || 'Facture liée' }}</span>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                @click="emit('view-document', tollForm.document_id, tollForm.document_filename, false)"
                class="p-1 text-slate-400 hover:text-indigo-400 rounded-lg hover:bg-slate-800"
                title="Voir le document"
              >
                <Eye class="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                @click="tollForm.document_id = null; tollForm.document_filename = null"
                class="p-1 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
                title="Détacher le justificatif"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-else class="space-y-2.5">
            <div v-if="documents.length > 0">
              <label for="toll-existing-doc" class="block text-[11px] text-slate-400 mb-1">Rattacher une facture existante</label>
              <select
                id="toll-existing-doc"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-slate-300"
                @change="(e: any) => onSelectExistingDoc(e.target.value, tollForm)"
              >
                <option value="">-- Sélectionner un justificatif existant --</option>
                <option v-for="d in documents" :key="d.id" :value="d.id">
                  {{ d.filename }} ({{ formatDate(d.created_at) }})
                </option>
              </select>
            </div>

            <div>
              <span class="block text-[11px] text-slate-400 mb-1">Ou déposer une nouvelle facture :</span>
              <AppDropzone
                :model-value="null"
                :disabled="isUploadingDocument"
                label="Déposez la facture ici ou cliquez pour parcourir"
                helperText="PDF ou image (téléversé et lié automatiquement au péage)"
                @update:model-value="(f) => onDropzoneDirectUpload(f, tollForm)"
              />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          Annuler
        </button>
        <button type="submit" form="toll-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ editingTollId ? 'Mettre à jour' : 'Enregistrer' }}
        </button>
      </div>
    </div>
  </div>
</template>
