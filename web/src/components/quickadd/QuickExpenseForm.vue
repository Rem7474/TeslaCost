<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import QuickPhotoField from './QuickPhotoField.vue'
import { buildExpensePayload, isQueued, toLocalDateTimeInput, type ExpenseType } from '@/utils/quickAdd'

const props = defineProps<{ vehicle: any }>()
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const types: { value: ExpenseType; label: string }[] = [
  { value: 'TOLL', label: 'Péage' },
  { value: 'PARKING', label: 'Parking' },
  { value: 'FERRY', label: 'Ferry' },
  { value: 'OTHER', label: 'Autre' },
]

const form = reactive({
  type: 'TOLL' as ExpenseType,
  date: toLocalDateTimeInput(new Date()),
  amount: '',
  notes: '',
  documentId: null as string | null,
  documentFilename: null as string | null,
})

const saving = ref(false)
const error = ref('')
const showDetails = ref(false)
const amountInput = ref<HTMLInputElement | null>(null)

onMounted(() => amountInput.value?.focus())

async function submit() {
  error.value = ''
  let payload: ReturnType<typeof buildExpensePayload>
  try {
    payload = buildExpensePayload({ ...form })
  } catch (err: any) {
    error.value = err.message
    return
  }
  saving.value = true
  try {
    const result = await api.createDriveExpense(props.vehicle.id, payload)
    const label = types.find((t) => t.value === payload.type)?.label ?? 'Dépense'
    emit('saved', { queued: isQueued(result), message: `${label} de ${payload.amount.toLocaleString('fr-FR')} €` })
  } catch (err: any) {
    error.value = err?.message || "Impossible d'enregistrer la dépense."
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QuickFormShell submit-label="Enregistrer la dépense" :saving="saving" :error="error" @submit="submit">
    <div role="radiogroup" aria-label="Type de dépense" class="grid grid-cols-4 gap-2">
      <button
        v-for="t in types"
        :key="t.value"
        type="button"
        role="radio"
        :aria-checked="form.type === t.value"
        class="quick-chip"
        :class="form.type === t.value ? 'border-rose-500/50 bg-rose-500/15 text-rose-200' : 'border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700'"
        @click="form.type = t.value"
      >
        {{ t.label }}
      </button>
    </div>

    <div>
      <label for="qe-amount" class="quick-label">Montant (€)</label>
      <input id="qe-amount" ref="amountInput" v-model="form.amount" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
    </div>

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-between rounded-xl px-1 text-sm font-semibold text-slate-300"
      :aria-expanded="showDetails"
      aria-controls="qe-details"
      @click="showDetails = !showDetails"
    >
      Plus de détails
      <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': showDetails }" aria-hidden="true" />
    </button>

    <div v-show="showDetails" id="qe-details" class="space-y-4">
      <div>
        <label for="qe-date" class="quick-label">Date et heure</label>
        <input id="qe-date" v-model="form.date" type="datetime-local" class="quick-input" />
      </div>
      <div>
        <label for="qe-notes" class="quick-label">Notes</label>
        <input id="qe-notes" v-model="form.notes" autocomplete="off" class="quick-input" />
      </div>
    </div>

    <QuickPhotoField
      v-model:document-id="form.documentId"
      v-model:filename="form.documentFilename"
      :vehicle-id="vehicle.id"
    />
  </QuickFormShell>
</template>
