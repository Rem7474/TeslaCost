<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useDocumentAttach } from '@/composables/useDocumentAttach'
import { Wrench, X, Paperclip, FileText, Eye } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import AppDropzone from '@/components/AppDropzone.vue'
import { CURRENCIES, currencyPayload, findCloseCandidate, formatDate, todayIso } from '@/utils/expenses'

// Adds a maintenance / fixed expense, or edits it when `editing` is set. maintenanceExpenses lets a new one close an earlier revision.
const props = defineProps<{
  vehicleId: string
  editing: any | null
  documents: ExpenseDocumentHeader[]
  maintenanceExpenses: any[]
}>()
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

const editingMaintId = computed(() => props.editing?.id ?? null)

const insuranceAnnualPremium = ref<number | ''>('')

// Annual premium paid monthly: amount = premium / 12, recurring every month
function applyMonthlyPremium() {
  const annual = Number(insuranceAnnualPremium.value)
  if (!annual || annual <= 0) {
    showAlert('Veuillez saisir la prime annuelle', 'Champ requis', 'warning')
    return
  }
  maintForm.value.amount = (Math.round((annual / 12) * 100) / 100).toFixed(2)
  maintForm.value.is_recurring = true
  maintForm.value.recurrence_interval_months = 1
  if (!maintForm.value.description) {
    maintForm.value.description = `Prime d'assurance (${annual.toFixed(2)} €/an)`
  }
}

const maintForm = ref({
  category: 'MAINTENANCE',
  amount: '',
  currency: 'EUR',
  fx_rate: '',
  date: todayIso(),
  odometer: 0,
  is_recurring: false,
  recurrence_interval_months: 12,
  recurrence_end_date: '',
  amortization_mode: 'DISTANCE',
  coverage_km: 50000,
  coverage_months: 24,
  closes_maintenance_id: null as string | null,
  description: '',
  document_id: null as string | null,
  document_filename: null as string | null,
})

const detectedOdometer = ref<number | null>(null)
const detectingOdometer = ref(false)
const shouldClosePrevious = ref(false)

const closeCandidateMaintenance = computed(() =>
  findCloseCandidate(props.maintenanceExpenses, editingMaintId.value, maintForm.value.date)
)

async function checkOdometerForDate(dateVal: string) {
  if (!props.vehicleId || !dateVal) return
  detectingOdometer.value = true
  try {
    const res = await api.getOdometerAt(props.vehicleId, dateVal)
    if (res && typeof res.odometer === 'number' && res.odometer > 0) {
      detectedOdometer.value = res.odometer
      if (!maintForm.value.odometer || maintForm.value.odometer === 0) {
        maintForm.value.odometer = Math.round(res.odometer)
      }
    } else {
      detectedOdometer.value = null
    }
  } catch {
    detectedOdometer.value = null
  } finally {
    detectingOdometer.value = false
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  const m = props.editing
  detectedOdometer.value = null
  if (!m) {
    shouldClosePrevious.value = false
    maintForm.value = {
      category: 'MAINTENANCE',
      amount: '',
      currency: 'EUR',
      fx_rate: '',
      date: todayIso(),
      odometer: 0,
      is_recurring: false,
      recurrence_interval_months: 12,
      recurrence_end_date: '',
      amortization_mode: 'DISTANCE',
      coverage_km: 50000,
      coverage_months: 24,
      closes_maintenance_id: null,
      description: '',
      document_id: null,
      document_filename: null,
    }
    checkOdometerForDate(maintForm.value.date)
  } else {
    shouldClosePrevious.value = Boolean(m.closes_maintenance_id)
    maintForm.value = {
      category: m.category || 'MAINTENANCE',
      amount: String(m.amount),
      currency: m.currency || 'EUR',
      fx_rate: m.fx_rate ? String(m.fx_rate) : '',
      date: new Date(m.date).toISOString().substring(0, 10),
      odometer: m.odometer ? Math.round(m.odometer) : 0,
      is_recurring: Boolean(m.is_recurring),
      recurrence_interval_months: m.recurrence_interval_months || 12,
      recurrence_end_date: m.recurrence_end_date ? new Date(m.recurrence_end_date).toISOString().substring(0, 10) : '',
      amortization_mode: m.amortization_mode || 'NONE',
      coverage_km: m.coverage_km ? Number(m.coverage_km) : 50000,
      coverage_months: m.coverage_months ? Number(m.coverage_months) : 24,
      closes_maintenance_id: m.closes_maintenance_id || null,
      description: m.description || '',
      document_id: m.document_id || null,
      document_filename: m.document_filename || null,
    }
  }
})

watch(() => maintForm.value.date, (newDate) => {
  if (open.value && newDate) {
    checkOdometerForDate(newDate)
  }
})

async function handleCreateMaint() {
  if (!props.vehicleId) return
  try {
    let closesId: string | null = null
    if (shouldClosePrevious.value && closeCandidateMaintenance.value) {
      closesId = closeCandidateMaintenance.value.id
    }

    const payload = {
      ...maintForm.value,
      ...currencyPayload(maintForm.value),
      amount: Number(maintForm.value.amount),
      odometer: maintForm.value.odometer ? Number(maintForm.value.odometer) : null,
      coverage_km:
        maintForm.value.amortization_mode === 'DISTANCE' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_km || 50000)
          : null,
      coverage_months:
        maintForm.value.amortization_mode === 'DURATION' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_months || 24)
          : null,
      closes_maintenance_id: closesId,
      date: new Date(maintForm.value.date).toISOString(),
      recurrence_end_date:
        maintForm.value.is_recurring && maintForm.value.recurrence_end_date
          ? new Date(maintForm.value.recurrence_end_date).toISOString()
          : null,
      document_id: maintForm.value.document_id || null,
    }
    if (editingMaintId.value) {
      await api.updateMaintenance(props.vehicleId, editingMaintId.value, payload)
    } else {
      await api.createMaintenance(props.vehicleId, payload)
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
          <Wrench class="w-5 h-5 text-pink-400" />
          {{ editingMaintId ? 'Modifier Entretien / Dépense Fixe' : 'Ajouter Entretien / Dépense Fixe' }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="maint-modal-form" @submit.prevent="handleCreateMaint" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <label for="expense-maint-category" class="block text-xs font-semibold text-slate-300 mb-1">Catégorie</label>
          <select id="expense-maint-category" v-model="maintForm.category" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
            <option value="MAINTENANCE">Entretien / Révision</option>
            <option value="REPAIR">Réparation / sinistre (franchise)</option>
            <option value="INSURANCE">Assurance (prime)</option>
            <option value="SUBSCRIPTION">Abonnement (Connectivité...)</option>
            <option value="TAX">Taxe / Carte grise</option>
            <option value="FINANCING">Autre financement (hors contrat du véhicule)</option>
            <option value="ACCESSORY">Accessoire</option>
            <option value="OTHER">Autre</option>
          </select>
        </div>
        <!-- Insurance: annual premium paid monthly -->
        <div v-if="maintForm.category === 'INSURANCE'" class="bg-indigo-500/10 border border-indigo-500/20 rounded-xl p-3 space-y-2">
          <div class="flex items-end gap-2">
            <div class="flex-1">
              <label for="expense-insurance-annual" class="block text-xs font-semibold text-indigo-200 mb-1">Prime annuelle (€)</label>
              <input
                id="expense-insurance-annual"
                v-model="insuranceAnnualPremium"
                type="number"
                step="0.01"
                min="0"
                placeholder="ex: 850"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <button type="button" @click="applyMonthlyPremium" class="px-3 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl">
              Mensualiser
            </button>
          </div>
          <p class="text-[11px] text-indigo-200/80">
            Crée une dépense récurrente mensuelle de prime / 12, à compter de la date de début de la couverture.
            L'assurance est un coût fixe dans le temps : elle n'est répartie au kilomètre que pour le coût d'un trajet, sur les kilomètres réellement parcourus.
          </p>
        </div>
        <p v-else-if="maintForm.category === 'FINANCING'" class="text-[11px] text-amber-300/90">
          Les loyers de LOA/LLD, l'apport et les intérêts d'un crédit sont générés automatiquement depuis « Acquisition & financement » du véhicule : ne les saisissez pas ici.
        </p>

        <div>
          <label for="expense-maint-description" class="block text-xs font-semibold text-slate-300 mb-1">Description</label>
          <input id="expense-maint-description" v-model="maintForm.description" required placeholder="ex: Remplacement filtre habitacle" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="expense-maint-date" class="block text-xs font-semibold text-slate-300 mb-1">Date</label>
            <AppDatePicker id="expense-maint-date" v-model="maintForm.date" required size="sm" />
          </div>
          <div>
            <label for="maint-form-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant</label>
            <div class="flex gap-1.5">
              <input id="maint-form-amount" v-model="maintForm.amount" type="number" step="0.01" min="0.01" required class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <label for="maint-form-currency" class="sr-only">Devise</label>
              <select id="maint-form-currency" v-model="maintForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
        </div>
        <div v-if="maintForm.currency !== 'EUR'">
          <label for="maint-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ maintForm.currency }} = ? €)</label>
          <input id="maint-form-fx-rate" v-model="maintForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
        </div>

        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="expense-maint-odometer" class="block text-xs font-semibold text-slate-300">Odomètre (km)</label>
            <span v-if="detectingOdometer" class="text-[11px] text-slate-400">Détection TeslaMate...</span>
          </div>
          <input id="expense-maint-odometer" v-model.number="maintForm.odometer" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          <div v-if="detectedOdometer !== null && detectedOdometer > 0" class="flex items-center justify-between text-[11px] text-emerald-400 mt-1">
            <span>✓ Détecté via TeslaMate : {{ Math.round(detectedOdometer) }} km</span>
            <button
              type="button"
              v-if="maintForm.odometer !== Math.round(detectedOdometer)"
              @click="maintForm.odometer = Math.round(detectedOdometer)"
              class="underline hover:text-emerald-300 transition-colors ml-2"
            >
              Appliquer
            </button>
          </div>
        </div>

        <!-- Lissage du coût pour dépenses non-récurrentes -->
        <div v-if="!maintForm.is_recurring" class="space-y-2.5 bg-slate-800/40 p-3.5 rounded-xl border border-slate-700/60">
          <span class="block text-xs font-semibold text-slate-200">
            Lissage du coût de revient au km
          </span>

          <div class="grid grid-cols-4 gap-1.5 pt-1">
            <button
              type="button"
              @click="maintForm.amortization_mode = 'NONE'"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'NONE' ? 'bg-pink-500/20 text-pink-300 border-pink-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Immédiat
            </button>
            <button
              type="button"
              @click="maintForm.amortization_mode = 'DISTANCE'"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'DISTANCE' ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Au km
            </button>
            <button
              type="button"
              @click="maintForm.amortization_mode = 'DURATION'"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'DURATION' ? 'bg-purple-500/20 text-purple-300 border-purple-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              À la durée
            </button>
            <button
              type="button"
              @click="maintForm.amortization_mode = 'HYBRID'"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'HYBRID' ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              Mixte
            </button>
          </div>

          <!-- Distance parameter -->
          <div v-if="maintForm.amortization_mode === 'DISTANCE' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
            <label for="maint-coverage-km" class="block text-xs text-slate-300 mb-1">Kilométrage couvert (km)</label>
            <input
              id="maint-coverage-km"
              v-model.number="maintForm.coverage_km"
              type="number"
              min="1000"
              step="1000"
              placeholder="50000"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
            />
          </div>

          <!-- Duration parameter -->
          <div v-if="maintForm.amortization_mode === 'DURATION' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
            <label for="maint-coverage-months" class="block text-xs text-slate-300 mb-1">Durée couverte (mois)</label>
            <input
              id="maint-coverage-months"
              v-model.number="maintForm.coverage_months"
              type="number"
              min="1"
              max="120"
              placeholder="24"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
            />
          </div>

          <!-- Clôture de la maintenance précédente -->
          <div v-if="closeCandidateMaintenance && maintForm.amortization_mode !== 'NONE'" class="pt-2 border-t border-slate-700/60">
            <div class="flex items-start gap-2">
              <input
                id="close-candidate"
                v-model="shouldClosePrevious"
                type="checkbox"
                class="mt-0.5 rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500"
              />
              <label for="close-candidate" class="text-xs text-slate-300 leading-snug cursor-pointer">
                Clôturer la révision précédente en cours
                <span class="block text-[11px] text-amber-400 font-normal">
                  {{ closeCandidateMaintenance.description }} ({{ formatDate(closeCandidateMaintenance.date) }} — {{ Number(closeCandidateMaintenance.amount).toFixed(2) }} €)
                </span>
              </label>
            </div>
          </div>
        </div>

        <div class="space-y-2 pt-1">
          <div class="flex items-center gap-2">
            <input v-model="maintForm.is_recurring" type="checkbox" id="rec" class="rounded border-slate-700 bg-slate-800 text-rose-600 focus:ring-rose-500" />
            <label for="rec" class="text-xs text-slate-300 font-medium">Dépense récurrente</label>
          </div>
          <div v-if="maintForm.is_recurring" class="pt-1 grid grid-cols-2 gap-3">
            <div>
              <label for="maint-form-recurrence-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">Intervalle (mois)</label>
              <input id="maint-form-recurrence-interval-months" v-model.number="maintForm.recurrence_interval_months" type="number" min="1" max="120" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="maint-form-recurrence-end-date" class="block text-xs font-semibold text-slate-300 mb-1">Fin (optionnelle)</label>
              <AppDatePicker id="maint-form-recurrence-end-date" v-model="maintForm.recurrence_end_date" size="sm" :clearable="true" />
            </div>
          </div>
        </div>

        <!-- Justificatif / Facture -->
        <div class="space-y-2 bg-slate-800/40 p-3 rounded-xl border border-slate-700/60">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Paperclip class="w-3.5 h-3.5 text-indigo-400" />
              Justificatif / Facture
            </span>
            <span v-if="maintForm.document_id" class="text-[11px] text-emerald-400 font-medium">Lié</span>
          </div>

          <div v-if="maintForm.document_id" class="flex items-center justify-between p-2.5 bg-slate-900 border border-indigo-500/30 rounded-xl">
            <div class="flex items-center gap-2 min-w-0">
              <FileText class="w-4 h-4 text-indigo-400 shrink-0" />
              <span class="text-xs text-white truncate font-medium">{{ maintForm.document_filename || 'Facture liée' }}</span>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                @click="emit('view-document', maintForm.document_id, maintForm.document_filename, false)"
                class="p-1 text-slate-400 hover:text-indigo-400 rounded-lg hover:bg-slate-800"
                title="Voir le document"
              >
                <Eye class="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                @click="maintForm.document_id = null; maintForm.document_filename = null"
                class="p-1 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
                title="Détacher le justificatif"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-else class="space-y-2.5">
            <div v-if="documents.length > 0">
              <label for="maint-existing-doc" class="block text-[11px] text-slate-400 mb-1">Rattacher une facture existante</label>
              <select
                id="maint-existing-doc"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-slate-300"
                @change="(e: any) => onSelectExistingDoc(e.target.value, maintForm)"
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
                helperText="PDF ou image (téléversé et rattaché automatiquement à cette dépense)"
                @update:model-value="(f) => onDropzoneDirectUpload(f, maintForm)"
              />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          Annuler
        </button>
        <button type="submit" form="maint-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ editingMaintId ? 'Mettre à jour' : 'Enregistrer' }}
        </button>
      </div>
    </div>
  </div>
</template>
