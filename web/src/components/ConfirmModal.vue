<script setup lang="ts">
import { onMounted, onUnmounted, watch, nextTick, ref } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { Trash2, AlertTriangle, Info, X } from 'lucide-vue-next'

const { isOpen, options, onConfirm, onCancel } = useConfirm()
const confirmBtnRef = ref<HTMLButtonElement | null>(null)
const cancelBtnRef = ref<HTMLButtonElement | null>(null)

function handleKeydown(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    e.preventDefault()
    onCancel()
  } else if (e.key === 'Enter' && !options.value.isAlert) {
    // Prevent accidental confirm on enter unless explicitly handled
  }
}

watch(isOpen, async (open) => {
  if (open) {
    await nextTick()
    // Focus cancel button by default for destructive actions, or confirm button for alerts
    if (options.value.isAlert) {
      confirmBtnRef.value?.focus()
    } else {
      cancelBtnRef.value?.focus()
    }
  }
})

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="isOpen"
        class="fixed inset-0 z-[9999] flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm"
        @click.self="onCancel"
      >
        <Transition
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0 scale-95 translate-y-2"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 translate-y-2"
        >
          <div
            v-if="isOpen"
            class="relative w-full max-w-md bg-slate-900 border border-slate-800 rounded-3xl p-6 shadow-2xl shadow-black/80 space-y-5 overflow-hidden"
            role="dialog"
            aria-modal="true"
          >
            <!-- Background glow accent -->
            <div
              class="absolute -top-12 -right-12 w-32 h-32 rounded-full blur-3xl pointer-events-none opacity-20"
              :class="{
                'bg-rose-500': options.type === 'danger',
                'bg-amber-500': options.type === 'warning',
                'bg-indigo-500': options.type === 'info',
              }"
            />

            <!-- Header with Icon & Close -->
            <div class="flex items-start justify-between gap-4">
              <div class="flex items-center gap-3.5">
                <div
                  class="p-3 rounded-2xl flex items-center justify-center border"
                  :class="{
                    'bg-rose-500/10 text-rose-400 border-rose-500/20': options.type === 'danger',
                    'bg-amber-500/10 text-amber-400 border-amber-500/20': options.type === 'warning',
                    'bg-indigo-500/10 text-indigo-400 border-indigo-500/20': options.type === 'info',
                  }"
                >
                  <Trash2 v-if="options.type === 'danger'" class="w-5 h-5" />
                  <AlertTriangle v-else-if="options.type === 'warning'" class="w-5 h-5" />
                  <Info v-else class="w-5 h-5" />
                </div>
                <div>
                  <h3 class="text-base font-bold text-white tracking-tight">
                    {{ options.title }}
                  </h3>
                </div>
              </div>

              <button
                type="button"
                @click="onCancel"
                class="p-1.5 rounded-xl text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
                title="Fermer"
                aria-label="Fermer"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Body Message -->
            <div class="text-sm text-slate-300 leading-relaxed pl-1">
              {{ options.message }}
            </div>

            <!-- Actions -->
            <div class="flex items-center justify-end gap-3 pt-2">
              <button
                v-if="!options.isAlert"
                ref="cancelBtnRef"
                type="button"
                @click="onCancel"
                class="px-4 py-2.5 rounded-xl text-xs font-semibold text-slate-300 bg-slate-800 hover:bg-slate-700 hover:text-white transition-all border border-slate-700/60"
              >
                {{ options.cancelText || 'Annuler' }}
              </button>
              <button
                ref="confirmBtnRef"
                type="button"
                @click="onConfirm"
                class="px-5 py-2.5 rounded-xl text-xs font-semibold text-white transition-all shadow-lg"
                :class="{
                  'bg-rose-600 hover:bg-rose-500 shadow-rose-600/30': options.type === 'danger',
                  'bg-amber-600 hover:bg-amber-500 shadow-amber-600/30': options.type === 'warning',
                  'bg-indigo-600 hover:bg-indigo-500 shadow-indigo-600/30': options.type === 'info',
                }"
              >
                {{ options.confirmText || 'Confirmer' }}
              </button>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
