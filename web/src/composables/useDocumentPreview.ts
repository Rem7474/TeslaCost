import { onUnmounted, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'

export interface DocumentPreviewState {
  id: string
  url: string
  filename: string
  isPdf: boolean
  isImage: boolean
}

/** Opens an attached document in the in-app preview, or downloads it. vehicleId is read when the action runs. */
export function useDocumentPreview(vehicleId: () => string | undefined) {
  const { showAlert } = useConfirm()
  const previewDoc = ref<DocumentPreviewState | null>(null)
  const loadingDocId = ref<string | null>(null)

  function closeDocPreview() {
    if (previewDoc.value?.url) {
      URL.revokeObjectURL(previewDoc.value.url)
    }
    previewDoc.value = null
  }

  function handlePreviewKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && previewDoc.value) {
      closeDocPreview()
    }
  }

  watch(previewDoc, (val) => {
    if (val) {
      window.addEventListener('keydown', handlePreviewKeydown)
    } else {
      window.removeEventListener('keydown', handlePreviewKeydown)
    }
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', handlePreviewKeydown)
    closeDocPreview()
  })

  async function viewOrDownloadDocument(docId: string | null | undefined, filename?: string | null, download = false) {
    const id = vehicleId()
    if (!id || !docId) return
    loadingDocId.value = docId
    try {
      const { blob, filename: serverFilename } = await api.downloadDocumentBlob(id, docId)
      const finalName = filename || serverFilename || 'document'
      const blobUrl = URL.createObjectURL(blob)
      if (download) {
        const a = document.createElement('a')
        a.href = blobUrl
        a.download = finalName
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        setTimeout(() => URL.revokeObjectURL(blobUrl), 1000)
      } else {
        closeDocPreview()
        const lower = finalName.toLowerCase()
        const mime = (blob.type || '').toLowerCase()
        const isPdf = lower.endsWith('.pdf') || mime.includes('pdf')
        const isImage = /\.(png|jpe?g|webp|gif|svg)$/i.test(lower) || mime.startsWith('image/')
        previewDoc.value = {
          id: docId,
          url: blobUrl,
          filename: finalName,
          isPdf,
          isImage,
        }
      }
    } catch (err: any) {
      showAlert(`Erreur lors de l'accès au document : ${err.message}`, 'Erreur', 'danger')
    } finally {
      loadingDocId.value = null
    }
  }

  return { previewDoc, loadingDocId, closeDocPreview, viewOrDownloadDocument }
}
