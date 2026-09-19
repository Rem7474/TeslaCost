<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import { buildFuelPayload, isQueued, toLocalDateInput, toNumber } from '@/utils/quickAdd'

const props = defineProps<{ vehicle: any }>()
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const form = reactive({
  date: toLocalDateInput(new Date()),
  amount: '',
  liters: '',
  fullTank: true,
  odometer: '',
  notes: '',
})

const saving = ref(false)
const error = ref('')
const showDetails = ref(false)
const amountInput = ref<HTMLInputElement | null>(null)

onMounted(() => amountInput.value?.focus())

const pricePerLiter = computed(() => {
  const amount = toNumber(form.amount)
  const liters = toNumber(form.liters)
  if (amount === null || liters === null || amount <= 0 || liters <= 0) return null
  return amount / liters
})
const fmtPrice = (v: number) => v.toLocaleString('fr-FR', { minimumFractionDigits: 3, maximumFractionDigits: 3 })

async function submit() {
  error.value = ''
  let payload: ReturnType<typeof buildFuelPayload>
  try {
    payload = buildFuelPayload({ ...form })
  } catch (err: any) {
    error.value = err.message
    return
  }
  saving.value = true
  try {
    const result = await api.createFuelLog(props.vehicle.id, payload)
    emit('saved', { queued: isQueued(result), message: `Plein de ${payload.amount.toLocaleString('fr-FR')} €` })
  } catch (err: any) {
    error.value = err?.message || "Impossible d'enregistrer le plein."
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QuickFormShell submit-label="Enregistrer le plein" :saving="saving" :error="error" @submit="submit">
    <div>
      <label for="qf-amount" class="quick-label">Montant (€)</label>
      <input id="qf-amount" ref="amountInput" v-model="form.amount" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
    </div>

    <div>
      <label for="qf-liters" class="quick-label">Quantité (L)</label>
      <input id="qf-liters" v-model="form.liters" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
      <p class="mt-1.5 min-h-4 text-[11px] text-slate-400" aria-live="polite">
        <template v-if="pricePerLiter !== null">Soit {{ fmtPrice(pricePerLiter) }} €/L.</template>
      </p>
    </div>

    <label class="flex min-h-12 cursor-pointer items-center justify-between gap-3 rounded-xl border border-slate-700 bg-slate-800 px-4 text-sm font-semibold text-white">
      Plein complet
      <input v-model="form.fullTank" type="checkbox" class="h-6 w-6 accent-rose-500" />
    </label>

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-between rounded-xl px-1 text-sm font-semibold text-slate-300"
      :aria-expanded="showDetails"
      aria-controls="qf-details"
      @click="showDetails = !showDetails"
    >
      Plus de détails
      <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': showDetails }" aria-hidden="true" />
    </button>

    <div v-show="showDetails" id="qf-details" class="space-y-4">
      <div>
        <label for="qf-date" class="quick-label">Date</label>
        <input id="qf-date" v-model="form.date" type="date" class="quick-input" />
      </div>
      <div>
        <label for="qf-odometer" class="quick-label">Odomètre (km)</label>
        <input id="qf-odometer" v-model="form.odometer" type="number" inputmode="numeric" min="0" class="quick-input" />
      </div>
      <div>
        <label for="qf-notes" class="quick-label">Notes</label>
        <input id="qf-notes" v-model="form.notes" maxlength="200" autocomplete="off" class="quick-input" />
      </div>
    </div>
  </QuickFormShell>
</template>
