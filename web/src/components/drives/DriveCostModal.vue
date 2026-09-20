<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Receipt, Layers, MapPin, ExternalLink, Zap, X, Users, Coins, Shield, Wrench, Disc, Plus, AlertTriangle, Pencil, Trash2, Save } from 'lucide-vue-next'
import { teslamateDriveUrl as buildTeslamateDriveUrl, tollApplyStatusLabel, uniqueById } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'

// Cost breakdown of a drive, or of a trip group (drive.is_trip_group, whose drives are tripDriveIds), with its
// expenses (edit, delete, add a toll) and the toll detection. refreshDrive reloads the drives list and returns the
// refreshed drive so the breakdown follows the server-side costs.
const props = defineProps<{
  vehicleId: string
  tripDriveIds: string[]
  startWithTollEntry: boolean
  refreshDrive: (driveId: string) => Promise<any | null>
}>()
const emit = defineEmits<{ 'toggle-tag': [drive: any, tag: string] }>()
const open = defineModel<boolean>('open', { required: true })
const selectedCostDrive = defineModel<any | null>('drive', { required: true })
const router = useRouter()
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const formatDate = formatDayTime
const teslamateDriveUrl = (d: any) => buildTeslamateDriveUrl(vehicleStore.activeVehicle, d)

// Expense edition inside the cost modal
const editingExpenseId = ref<string | null>(null)
const expenseEditForm = ref({ type: 'TOLL', amount: '' as number | string, notes: '' })

const driveExpenses = ref<any[]>([])
const loadingExpenses = ref(false)
const showAddTollInline = ref(false)
const inlineTollAmount = ref<number | ''>('')
const inlineTollType = ref('TOLL')
const inlineTollNotes = ref('')
const addingToll = ref(false)
const tollDetection = ref<any | null>(null)
const tollDetectionLoading = ref(false)
const tollDetectionError = ref('')
const tollDetectionEstimatedTotal = computed(() => {
  const priced = (tollDetection.value?.segments || []).filter((s: any) => s.estimated_price != null)
  if (!priced.length) return null
  return priced.reduce((sum: number, s: any) => sum + s.estimated_price, 0)
})

watch(open, (isOpen) => {
  const drive = selectedCostDrive.value
  if (!isOpen || !drive) return
  editingExpenseId.value = null
  if (drive.is_trip_group) {
    showAddTollInline.value = false
    loadTripExpenses()
    return
  }
  showAddTollInline.value = props.startWithTollEntry
  inlineTollAmount.value = ''
  inlineTollNotes.value = ''
  loadDriveExpenses(drive.id)
  loadTollDetection(drive)
})

// Expenses of a trip group: those of its drives, each counted once
async function loadTripExpenses() {
  if (!props.vehicleId) return
  loadingExpenses.value = true
  try {
    const expPromises = props.tripDriveIds.map((id) => api.getDriveExpensesForDrive(props.vehicleId, id).catch(() => []))
    const expResults = await Promise.all(expPromises)
    driveExpenses.value = uniqueById(expResults.flat())
  } finally {
    loadingExpenses.value = false
  }
}

async function loadTollDetection(drive: any) {
  tollDetection.value = null
  tollDetectionError.value = ''
  if (!props.vehicleId || drive.is_trip_group) return
  try {
    tollDetection.value = await api.getTollDetection(props.vehicleId, drive.id)
  } catch (err) {
    console.error('Failed to load toll detection', err)
  }
}

const existingTollExpense = computed(() => driveExpenses.value.find((e: any) => e.type === 'TOLL'))
const canApplyTollEstimate = computed(
  () =>
    vehicleStore.canEdit &&
    tollDetectionEstimatedTotal.value != null &&
    (!existingTollExpense.value || existingTollExpense.value.source === 'AUTO_TOLL') &&
    !existingTollExpense.value?.trip_group_id
)
const applyingToll = ref(false)

async function handleApplyTollEstimate() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  applyingToll.value = true
  try {
    const res = await api.applyTollEstimate(props.vehicleId, selectedCostDrive.value.id)
    if (res.status === 'created' || res.status === 'updated') {
      await refreshCostModal()
    } else {
      showAlert(tollApplyStatusLabel(res.status), 'Péage non appliqué', 'warning')
    }
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    applyingToll.value = false
  }
}

async function handleDetectTolls() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  tollDetectionLoading.value = true
  tollDetectionError.value = ''
  try {
    tollDetection.value = await api.detectTolls(props.vehicleId, selectedCostDrive.value.id)
  } catch (err: any) {
    tollDetectionError.value = err.message || 'Échec de la détection'
  } finally {
    tollDetectionLoading.value = false
  }
}

async function loadDriveExpenses(driveId: string) {
  if (!props.vehicleId) return
  loadingExpenses.value = true
  try {
    driveExpenses.value = await api.getDriveExpensesForDrive(props.vehicleId, driveId)
  } catch (err) {
    console.error('Failed to load drive expenses', err)
    driveExpenses.value = []
  } finally {
    loadingExpenses.value = false
  }
}

async function handleAddTollToDrive() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  if (!inlineTollAmount.value || Number(inlineTollAmount.value) <= 0) {
    showAlert('Veuillez entrer un montant valide', 'Montant invalide', 'warning')
    return
  }

  addingToll.value = true
  try {
    const amountNum = Number(inlineTollAmount.value)
    await api.createDriveExpense(props.vehicleId, {
      drive_id: selectedCostDrive.value.id,
      type: inlineTollType.value,
      amount: amountNum,
      currency: 'EUR',
      date: selectedCostDrive.value.start_time,
      notes: inlineTollNotes.value || 'Ajouté depuis la vue trajet',
    })

    await refreshCostModal()
    showAddTollInline.value = false
    inlineTollAmount.value = ''
    inlineTollNotes.value = ''
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    addingToll.value = false
  }
}

// Reloads the drive costs (computed server-side, with trip group allocation) and its expenses
async function refreshCostModal() {
  if (!selectedCostDrive.value) return
  const driveId = selectedCostDrive.value.id
  const [, refreshed] = await Promise.all([loadDriveExpenses(driveId), props.refreshDrive(driveId)])
  if (refreshed) selectedCostDrive.value = refreshed
}

function startEditExpense(exp: any) {
  editingExpenseId.value = exp.id
  expenseEditForm.value = { type: exp.type, amount: exp.amount, notes: exp.notes || '' }
}

async function handleSaveExpenseEdit(exp: any) {
  if (!props.vehicleId) return
  const amount = Number(expenseEditForm.value.amount)
  if (!amount || amount <= 0) {
    showAlert('Veuillez entrer un montant valide', 'Montant invalide', 'warning')
    return
  }
  try {
    await api.updateDriveExpense(props.vehicleId, exp.id, {
      type: expenseEditForm.value.type,
      amount,
      currency: exp.currency,
      fx_rate: exp.fx_rate ?? null,
      date: exp.date,
      notes: expenseEditForm.value.notes || null,
      // Keep the current link: single drive or trip group
      drive_id: exp.drive_id ?? null,
      trip_group_id: exp.trip_group_id ?? null,
    })
    editingExpenseId.value = null
    await refreshCostModal()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteExpense(exp: any) {
  if (!props.vehicleId) return
  const scope = exp.trip_group_id ? ` (frais du voyage « ${exp.trip_group_name} », tous les trajets du voyage sont concernés)` : ''
  const ok = await showConfirm({
    title: 'Supprimer le frais',
    message: `Supprimer ce frais de ${Number(exp.amount).toFixed(2)} ${exp.currency}${scope} ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(props.vehicleId, exp.id)
    await refreshCostModal()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}
</script>

<template>
  <div
    v-if="open && selectedCostDrive"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2.5 min-w-0 pr-2">
          <div class="p-2 rounded-xl shrink-0" :class="selectedCostDrive.is_trip_group ? 'bg-indigo-500/10 text-indigo-400' : 'bg-emerald-500/10 text-emerald-400'">
            <component :is="selectedCostDrive.is_trip_group ? Layers : Coins" class="w-5 h-5" />
          </div>
          <div class="min-w-0 truncate">
            <h3 class="text-base font-bold text-white truncate">
              {{ selectedCostDrive.is_trip_group ? `Voyage : ${selectedCostDrive.trip_group_name}` : 'Coût Réel du Trajet' }}
            </h3>
            <p class="text-xs text-slate-400">{{ formatDate(selectedCostDrive.start_time) }}</p>
          </div>
        </div>
        <a
          v-if="teslamateDriveUrl(selectedCostDrive)"
          :href="teslamateDriveUrl(selectedCostDrive)!"
          target="_blank"
          rel="noopener noreferrer"
          class="ml-auto mr-1 px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold rounded-lg flex items-center gap-1.5 shrink-0"
          title="Ouvrir ce trajet dans le dashboard TeslaMate"
        >
          <ExternalLink class="w-3.5 h-3.5 text-sky-400" />
          <span class="hidden sm:inline">TeslaMate</span>
        </a>
        <button @click="open = false" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">

      <!-- Trip Summary Route -->
      <div class="bg-slate-800/60 border border-slate-700/60 p-3.5 rounded-2xl space-y-2">
        <div class="text-sm font-semibold text-white flex items-center gap-2">
          <MapPin class="w-4 h-4 text-rose-400 shrink-0" />
          <span class="truncate">{{ selectedCostDrive.start_address || 'Départ' }}</span>
          <span class="text-slate-500">→</span>
          <span class="truncate">{{ selectedCostDrive.end_address || 'Arrivée' }}</span>
        </div>
        <div class="flex items-center gap-3 text-xs text-slate-300 flex-wrap">
          <span class="font-bold text-rose-400">{{ selectedCostDrive.distance_km }} km</span>
          <span v-if="selectedCostDrive.duration_min" class="text-slate-400">• {{ selectedCostDrive.duration_min }} min</span>
          <span v-if="selectedCostDrive.drives_count" class="text-indigo-400 font-semibold">• {{ selectedCostDrive.drives_count }} étapes</span>
          <span v-if="selectedCostDrive.speed_avg" class="text-slate-400">• {{ Math.round(selectedCostDrive.speed_avg) }} km/h moy</span>
          <span v-if="selectedCostDrive.costs?.electricity_kwh" class="text-sky-400 font-mono">• {{ selectedCostDrive.costs.electricity_kwh }} kWh</span>
        </div>

        <!-- Tag qualification (only for individual drives) -->
        <div v-if="!selectedCostDrive.is_trip_group && vehicleStore.canEdit" class="pt-2 border-t border-slate-700/60 flex items-center justify-between gap-2">
          <span class="text-xs text-slate-400">Classification :</span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Pro')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Pro')
                  ? 'bg-blue-500/20 text-blue-400 border-blue-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              Pro
            </button>
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Perso')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Perso')
                  ? 'bg-emerald-500/20 text-emerald-400 border-emerald-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              Perso
            </button>
          </div>
        </div>
      </div>

      <p v-if="selectedCostDrive.costs?.has_estimates" class="text-[11px] text-amber-400/90 bg-amber-500/10 border border-amber-500/20 rounded-xl px-3 py-2 flex items-start gap-2">
        <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        <span>Certains postes utilisent une valeur par défaut faute d'historique (marqués « estimation ») : complétez recharges, pneus, entretien et assurance pour un coût réel.</span>
      </p>

      <!-- Cost Breakdown List -->
      <div class="space-y-2.5">
        <!-- 1. Électricité -->
        <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-sky-500/10 text-sky-400 rounded-lg">
              <Zap class="w-4 h-4" />
            </div>
            <div>
              <div class="text-xs font-semibold text-white flex items-center gap-1.5">
                Énergie Électrique
                <span v-if="selectedCostDrive.costs?.energy_source === 'DEFAULT' || selectedCostDrive.costs?.electricity_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 font-medium">estimation</span>
              </div>
              <div class="text-[11px] text-slate-400 font-mono">
                {{ selectedCostDrive.costs?.electricity_kwh || 0 }} kWh × {{ (selectedCostDrive.costs?.electricity_rate || 0.22).toFixed(3) }} €/kWh
              </div>
            </div>
          </div>
          <div class="text-sm font-bold text-sky-400 font-mono">
            {{ (selectedCostDrive.costs?.electricity_cost || 0).toFixed(2) }} €
          </div>
        </div>

        <!-- 2. Pneus -->
        <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
              <Disc class="w-4 h-4" />
            </div>
            <div>
              <div class="text-xs font-semibold text-white flex items-center gap-1.5">
                Usure des Pneumatiques
                <span v-if="selectedCostDrive.costs?.tires_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 font-medium">estimation</span>
                <span v-else-if="selectedCostDrive.costs?.tires_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-medium">inclus dans la location</span>
              </div>
              <div class="text-[11px] text-slate-400 font-mono">
                {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.tires_rate || 0.02).toFixed(3) }} €/km
              </div>
            </div>
          </div>
          <div class="text-sm font-bold text-emerald-400 font-mono">
            {{ (selectedCostDrive.costs?.tires_cost || 0).toFixed(2) }} €
          </div>
        </div>

        <!-- 3. Entretien -->
        <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-pink-500/10 text-pink-400 rounded-lg">
              <Wrench class="w-4 h-4" />
            </div>
            <div>
              <div class="text-xs font-semibold text-white flex items-center gap-1.5">
                Provision Entretien
                <span v-if="selectedCostDrive.costs?.maintenance_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 font-medium">estimation</span>
                <span v-else-if="selectedCostDrive.costs?.maintenance_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-medium">inclus dans la location</span>
              </div>
              <div class="text-[11px] text-slate-400 font-mono">
                {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.maintenance_rate || 0.015).toFixed(3) }} €/km
              </div>
            </div>
          </div>
          <div class="text-sm font-bold text-pink-400 font-mono">
            {{ (selectedCostDrive.costs?.maintenance_cost || 0).toFixed(2) }} €
          </div>
        </div>

        <!-- 4. Assurance -->
        <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-purple-500/10 text-purple-400 rounded-lg">
              <Shield class="w-4 h-4" />
            </div>
            <div>
              <div class="flex items-center gap-1.5">
                <span class="text-xs font-semibold text-white">Quote-part assurance (coût fixe)</span>
                <span
                  v-if="selectedCostDrive.costs?.insurance_source === 'RECORDED_EXPENSES'"
                  class="text-[9px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 font-medium"
                  title="Primes payées sur les 12 derniers mois divisées par les kilomètres parcourus sur la même période"
                >
                  Primes réelles
                </span>
                <span
                  v-else-if="selectedCostDrive.costs?.insurance_source === 'INCLUDED_IN_LEASE'"
                  class="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-medium"
                >
                  Incluse dans la location
                </span>
                <span
                  v-else-if="selectedCostDrive.costs?.insurance_source === 'INSUFFICIENT_DISTANCE'"
                  class="text-[9px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 font-medium"
                  title="Moins de 500 km parcourus depuis la première prime : quote-part non calculée"
                >
                  Pas assez de km
                </span>
                <span
                  v-else
                  class="text-[9px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 font-medium"
                  title="Aucune prime d'assurance enregistrée dans les dépenses"
                >
                  Non renseignée
                </span>
              </div>
              <div class="text-[11px] text-slate-400 font-mono">
                {{ selectedCostDrive.distance_km }} km × {{ (selectedCostDrive.costs?.insurance_rate || 0).toFixed(3) }} €/km
              </div>
            </div>
          </div>
          <div class="text-sm font-bold text-purple-400 font-mono">
            {{ (selectedCostDrive.costs?.insurance_cost || 0).toFixed(2) }} €
          </div>
        </div>

        <!-- 5. Péages & Frais de route -->
        <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl space-y-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-amber-500/10 text-amber-400 rounded-lg">
                <Receipt class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Péages & Frais de route</div>
                <div class="text-[11px] text-slate-400">
                  {{ driveExpenses.length }} frais assigné(s)
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-sm font-bold text-amber-400 font-mono">
                {{ (selectedCostDrive.costs?.tolls_cost || 0).toFixed(2) }} €
              </span>
              <button
                @click="showAddTollInline = !showAddTollInline"
                class="p-1 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg text-xs"
                title="Ajouter un péage ou parking à ce trajet"
              >
                <Plus class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- List of attached expenses -->
          <div v-if="driveExpenses.length" class="space-y-1 pt-1 border-t border-slate-700/50">
            <div v-for="exp in driveExpenses" :key="exp.id" class="text-[11px] text-slate-300 pl-9">
              <div v-if="editingExpenseId !== exp.id" class="flex items-center justify-between gap-2">
                <span>
                  {{ exp.type === 'TOLL' ? 'Péage' : exp.type }}
                  <span v-if="exp.source === 'AUTO_TOLL'" class="text-[9px] px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-400 font-medium" title="Calculé automatiquement à partir du tracé GPS">auto</span>
                  <span v-if="exp.notes" class="text-slate-500">({{ exp.notes }})</span>
                  <span v-if="exp.trip_group_id" class="text-indigo-400"> • part du voyage sur {{ exp.amount.toFixed(2) }} {{ exp.currency }}</span>
                </span>
                <span class="flex items-center gap-1.5">
                  <span class="font-mono text-amber-400">{{ (exp.allocated_amount ?? exp.amount).toFixed(2) }} €</span>
                  <button @click="startEditExpense(exp)" class="p-0.5 text-slate-500 hover:text-amber-400" title="Modifier ce frais">
                    <Pencil class="w-3 h-3" />
                  </button>
                  <button @click="handleDeleteExpense(exp)" class="p-0.5 text-slate-500 hover:text-rose-400" title="Supprimer ce frais">
                    <Trash2 class="w-3 h-3" />
                  </button>
                </span>
              </div>
              <div v-else class="grid grid-cols-12 gap-1.5 items-center py-1">
                <label :for="`drive-expense-type-${exp.id}`" class="sr-only">Type de frais</label>
                <select :id="`drive-expense-type-${exp.id}`" v-model="expenseEditForm.type" class="col-span-3 bg-slate-800 border border-slate-700 rounded-lg px-1.5 py-1 text-[11px] text-white">
                  <option value="TOLL">Péage</option>
                  <option value="PARKING">Parking</option>
                  <option value="FERRY">Ferry</option>
                  <option value="OTHER">Autre</option>
                </select>
                <label :for="`drive-expense-amount-${exp.id}`" class="sr-only">Montant total</label>
                <input :id="`drive-expense-amount-${exp.id}`" v-model="expenseEditForm.amount" type="number" step="0.01" min="0.01" class="col-span-3 bg-slate-800 border border-slate-700 rounded-lg px-1.5 py-1 text-[11px] text-white" />
                <label :for="`drive-expense-notes-${exp.id}`" class="sr-only">Notes</label>
                <input :id="`drive-expense-notes-${exp.id}`" v-model="expenseEditForm.notes" placeholder="Notes" class="col-span-4 bg-slate-800 border border-slate-700 rounded-lg px-1.5 py-1 text-[11px] text-white" />
                <button @click="handleSaveExpenseEdit(exp)" class="col-span-1 p-1 text-emerald-400 hover:text-emerald-300" title="Enregistrer">
                  <Save class="w-3.5 h-3.5" />
                </button>
                <button @click="editingExpenseId = null" class="col-span-1 p-1 text-slate-500 hover:text-white" title="Annuler">
                  <X class="w-3.5 h-3.5" />
                </button>
                <p v-if="exp.trip_group_id" class="col-span-12 text-[10px] text-indigo-300/80">Montant total du voyage, réparti entre ses trajets au prorata des km.</p>
              </div>
            </div>
          </div>

          <!-- Inline add toll form -->
          <div v-if="showAddTollInline" class="p-3 bg-slate-900 border border-slate-700 rounded-xl space-y-2 mt-2">
            <div class="text-xs font-bold text-white">Ajouter un péage / parking</div>
            <div class="grid grid-cols-2 gap-2">
              <label for="drive-inline-toll-type" class="sr-only">Type de frais</label>
              <select id="drive-inline-toll-type"
                v-model="inlineTollType"
                class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
              >
                <option value="TOLL">Péage</option>
                <option value="PARKING">Parking</option>
                <option value="FERRY">Ferry</option>
              </select>
              <label for="drive-inline-toll-amount" class="sr-only">Montant (€)</label>
              <input id="drive-inline-toll-amount"
                v-model="inlineTollAmount"
                type="number"
                step="0.01"
                placeholder="Montant (€)"
                class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
              />
            </div>
            <label for="drive-inline-toll-notes" class="sr-only">Notes (ex: Péage A6 Beaune)</label>
            <input id="drive-inline-toll-notes"
              v-model="inlineTollNotes"
              type="text"
              placeholder="Notes (ex: Péage A6 Beaune)"
              class="w-full bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white"
            />
            <div class="flex justify-end gap-2">
              <button
                @click="showAddTollInline = false"
                class="px-2.5 py-1 text-xs text-slate-400 hover:text-white"
              >
                Annuler
              </button>
              <button
                @click="handleAddTollToDrive"
                :disabled="addingToll"
                class="px-3 py-1 bg-amber-600 hover:bg-amber-500 text-white font-semibold text-xs rounded-lg disabled:opacity-50"
              >
                {{ addingToll ? 'Enregistrement...' : 'Valider' }}
              </button>
            </div>
          </div>
        </div>

        <!-- 6. Détection péage autoroute (GPS, informatif — disponible sur tout trajet, même si l'heuristique de vitesse ne l'a pas repéré) -->
        <div
          v-if="!selectedCostDrive.is_trip_group"
          class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl space-y-2"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-3">
              <div class="p-2 bg-cyan-500/10 text-cyan-400 rounded-lg">
                <MapPin class="w-4 h-4" />
              </div>
              <div>
                <div class="text-xs font-semibold text-white">Détection péage autoroute</div>
                <div class="text-[11px] text-slate-400">Basée sur le tracé GPS TeslaMate</div>
              </div>
            </div>
            <button
              v-if="selectedCostDrive.teslamate_drive_id"
              @click="handleDetectTolls"
              :disabled="tollDetectionLoading"
              class="px-2.5 py-1 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold rounded-lg disabled:opacity-50 shrink-0"
            >
              {{ tollDetectionLoading ? 'Détection...' : tollDetection ? 'Re-détecter' : 'Détecter les péages' }}
            </button>
            <span v-else class="text-[11px] text-slate-500 shrink-0">Trajet manuel, pas de tracé GPS</span>
          </div>

          <p v-if="tollDetectionError" class="text-[11px] text-rose-400">{{ tollDetectionError }}</p>

          <div v-if="tollDetection?.segments?.length" class="space-y-1 pt-1 border-t border-slate-700/50">
            <div v-for="(seg, idx) in tollDetection.segments" :key="idx" class="text-[11px] text-slate-300 pl-9 flex items-center justify-between gap-2">
              <span v-if="seg.type === 'close' && seg.exit">
                {{ seg.operator ? `${seg.operator} : ` : '' }}{{ seg.entry }} → {{ seg.exit }}
              </span>
              <span v-else-if="seg.type === 'close'">Entrée détectée : {{ seg.entry }} (sortie non identifiée)</span>
              <span v-else>Barrière : {{ seg.entry }}</span>
              <span v-if="seg.estimated_price != null" class="text-amber-400 font-mono shrink-0">{{ seg.estimated_price.toFixed(2) }} €</span>
            </div>
            <div v-if="tollDetectionEstimatedTotal != null" class="flex items-center justify-between gap-2 pl-9 pt-1 border-t border-slate-700/50 text-[11px]">
              <span class="text-slate-400">Estimation totale <span class="text-slate-500">(classe 1, véhicule léger)</span></span>
              <span class="flex items-center gap-2 shrink-0">
                <span class="text-amber-400 font-mono font-semibold">{{ tollDetectionEstimatedTotal.toFixed(2) }} €</span>
                <button
                  v-if="canApplyTollEstimate"
                  @click="handleApplyTollEstimate"
                  :disabled="applyingToll"
                  class="px-2 py-0.5 bg-cyan-600 hover:bg-cyan-500 text-white text-[11px] font-semibold rounded-lg disabled:opacity-50"
                >
                  {{ applyingToll ? '...' : existingTollExpense ? 'Mettre à jour' : 'Appliquer' }}
                </button>
              </span>
            </div>
          </div>
          <p v-else-if="tollDetection" class="text-[11px] text-slate-500 pl-9">Aucun péage détecté sur ce trajet.</p>
        </div>
      </div>

      <!-- Grand Total Card -->
      <div class="bg-gradient-to-r from-slate-800 to-slate-800/80 border border-emerald-500/30 p-4 rounded-2xl flex items-center justify-between shadow-lg">
        <div>
          <span class="text-xs font-semibold text-emerald-400 uppercase tracking-wider">Coût de Revient Total</span>
          <div class="text-2xl font-black text-white">
            {{ (selectedCostDrive.costs?.total_cost || 0).toFixed(2) }} €
          </div>
        </div>
        <div class="text-right">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût au kilomètre</span>
          <div class="text-lg font-extrabold text-emerald-400 font-mono">
            {{ (selectedCostDrive.costs?.cost_per_km || 0).toFixed(3) }} €<span class="text-xs font-normal text-slate-400">/km</span>
          </div>
        </div>
      </div>

      </div>

      <!-- Footer Actions -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <button
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          Fermer
        </button>
        <button
          @click="open = false; router.push({ path: '/carpools', query: selectedCostDrive.is_trip_group ? { new_trip_group_id: selectedCostDrive.id } : { new_drive_id: selectedCostDrive.id } })"
          class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
        >
          <Users class="w-4 h-4" />
          <span>Partager en covoiturage</span>
        </button>
      </div>
    </div>
  </div>
</template>
