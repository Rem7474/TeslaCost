import { ref } from 'vue'

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  type?: 'danger' | 'warning' | 'info' | 'success'
  isAlert?: boolean
}

const isOpen = ref(false)
const options = ref<ConfirmOptions>({
  title: 'Confirmation',
  message: '',
  confirmText: 'Confirmer',
  cancelText: 'Annuler',
  type: 'danger',
  isAlert: false,
})

let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirm() {
  function showConfirm(opts: ConfirmOptions | string): Promise<boolean> {
    if (typeof opts === 'string') {
      options.value = {
        title: 'Confirmation',
        message: opts,
        confirmText: 'Confirmer',
        cancelText: 'Annuler',
        type: 'danger',
        isAlert: false,
      }
    } else {
      options.value = {
        title: opts.title || (opts.type === 'danger' ? 'Suppression' : 'Confirmation'),
        message: opts.message,
        confirmText: opts.confirmText || (opts.type === 'danger' ? 'Supprimer' : 'Confirmer'),
        cancelText: opts.cancelText || 'Annuler',
        type: opts.type || 'danger',
        isAlert: false,
      }
    }
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function showAlert(message: string, title?: string, type: 'info' | 'warning' | 'danger' | 'success' = 'info'): Promise<void> {
    options.value = {
      title: title || (type === 'danger' ? 'Erreur' : 'Information'),
      message,
      confirmText: 'Compris',
      cancelText: '',
      type: type || 'info',
      isAlert: true,
    }
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = () => resolve()
    })
  }

  function onConfirm() {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(true)
      resolvePromise = null
    }
  }

  function onCancel() {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(false)
      resolvePromise = null
    }
  }

  return {
    isOpen,
    options,
    showConfirm,
    showAlert,
    onConfirm,
    onCancel,
  }
}
