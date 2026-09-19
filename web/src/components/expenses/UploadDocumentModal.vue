<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { X, UploadCloud } from 'lucide-vue-next'
import AppDropzone from '@/components/AppDropzone.vue'

// Adds a receipt that is not attached to any expense yet
const props = defineProps<{ vehicleId: string }>()
const emit = defineEmits<{ 'document-added': [doc: ExpenseDocumentHeader] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const isUploadingDocument = ref(false)
const uploadDocDescription = ref('')
const uploadDocFile = ref<File | null>(null)

watch(open, (isOpen) => {
  if (!isOpen) return
  uploadDocDescription.value = ''
  uploadDocFile.value = null
})

async function handleUploadStandaloneDocument() {
  if (!props.vehicleId || !uploadDocFile.value) {
    showAlert('Veuillez sélectionner un fichier', 'Champ requis', 'warning')
    return
  }
  isUploadingDocument.value = true
  try {
    const doc = await api.uploadDocument(props.vehicleId, uploadDocFile.value, uploadDocDescription.value)
    emit('document-added', doc)
    open.value = false
    showAlert(`Fichier « ${doc.filename} » ajouté avec succès`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors du téléversement : ${err.message}`, 'Erreur', 'danger')
  } finally {
    isUploadingDocument.value = false
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
          <UploadCloud class="w-5 h-5 text-indigo-400" />
          Ajouter un Justificatif ou une Facture
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="standalone-doc-form" @submit.prevent="handleUploadStandaloneDocument" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <span class="block text-xs font-semibold text-slate-300 mb-1.5">Fichier justificatif</span>
          <AppDropzone
            v-model="uploadDocFile"
            :disabled="isUploadingDocument"
            label="Glissez votre facture ou cliquez pour parcourir"
            helperText="Formats acceptés : PDF, PNG, JPG, WEBP (max. 15 Mo)"
          />
        </div>

        <div>
          <label for="standalone-doc-desc" class="block text-xs font-semibold text-slate-300 mb-1">Description / Réf. facture (optionnel)</label>
          <input
            id="standalone-doc-desc"
            v-model="uploadDocDescription"
            placeholder="ex: Facture révision Tesla Chambourcy, péages août 2026..."
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
          />
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          Annuler
        </button>
        <button
          type="submit"
          form="standalone-doc-form"
          :disabled="isUploadingDocument"
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition-colors disabled:opacity-50 flex items-center gap-2 font-medium"
        >
          <UploadCloud class="w-4 h-4" />
          <span>{{ isUploadingDocument ? 'Téléversement...' : 'Téléverser' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
