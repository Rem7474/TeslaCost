import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { QuickKind } from '@/utils/quickAdd'

// Open state of the quick entry sheet, shared by the mobile bar, the desktop sidebar and PWA shortcuts.
export const useQuickAddStore = defineStore('quickAdd', () => {
  const isOpen = ref(false)
  // Tab asked by the caller; the sheet falls back to the first tab the active vehicle supports.
  const requestedKind = ref<QuickKind | null>(null)

  function open(kind: QuickKind | null = null) {
    requestedKind.value = kind
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
    requestedKind.value = null
  }

  return { isOpen, requestedKind, open, close }
})
