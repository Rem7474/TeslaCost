<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api, type ExpenseDocumentHeader, type MaintenanceReminder, type VehicleWebhook } from '@/services/api'
import {
  Receipt,
  Plus,
  Wrench,
  Zap,
  Calendar,
  DollarSign,
  X,
  Repeat,
  Navigation,
  Layers,
  Users,
  CheckSquare,
  Square,
  ArrowRight,
  Pencil,
  Trash2,
  AlertTriangle,
  Paperclip,
  FileText,
  Download,
  Eye,
  UploadCloud,
  ExternalLink,
  Image as ImageIcon,
  Loader2,
  Bell,
  Clock,
  CheckCircle2,
  Radio,
  Sparkles,
  Check,
} from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const activeTab = ref<'TOLLS' | 'MAINTENANCE' | 'REMINDERS' | 'CHARGES' | 'DOCUMENTS'>('TOLLS')

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const chargesWithoutCost = ref(0)
const chargesTotal = ref(0)
const chargesPage = ref(1)
const loadingMoreCharges = ref(false)
const missingCostOnly = ref(false)
const loading = ref(false)

// Documents & Invoices
const documents = ref<ExpenseDocumentHeader[]>([])
const isUploadingDocument = ref(false)
const showUploadDocModal = ref(false)
const uploadDocDescription = ref('')
const uploadDocFile = ref<File | null>(null)

// Modals
const showAddTollModal = ref(false)
const showAddMaintModal = ref(false)
const editingTollId = ref<string | null>(null)
const editingMaintId = ref<string | null>(null)

const recentDrives = ref<any[]>([])
const associationMode = ref<'NONE' | 'SINGLE' | 'MULTI'>('NONE')
const selectedDriveId = ref('')
const selectedDriveIds = ref<string[]>([])

const CURRENCIES = ['EUR', 'CHF', 'GBP', 'USD']

const CATEGORY_LABELS: Record<string, string> = {
  MAINTENANCE: 'Entretien',
  REPAIR: 'Réparation / sinistre',
  INSURANCE: 'Assurance',
  SUBSCRIPTION: 'Abonnement',
  TAX: 'Taxe',
  FINANCING: 'Financement',
  ACCESSORY: 'Accessoire',
  OTHER: 'Autre',
}

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

const maintForm = ref({
  category: 'MAINTENANCE',
  amount: '',
  currency: 'EUR',
  fx_rate: '',
  date: new Date().toISOString().substring(0, 10),
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

const closeCandidateMaintenance = computed(() => {
  if (!maintenanceExpenses.value || !maintenanceExpenses.value.length) return null
  const currentId = editingMaintId.value
  const formDate = maintForm.value.date
  return maintenanceExpenses.value.find(
    (m) =>
      m.id !== currentId &&
      m.amortization_mode &&
      m.amortization_mode !== 'NONE' &&
      new Date(m.date).toISOString().substring(0, 10) <= formDate
  ) || null
})

async function checkOdometerForDate(dateVal: string) {
  if (!vehicleStore.activeVehicle || !dateVal) return
  detectingOdometer.value = true
  try {
    const res = await api.getOdometerAt(vehicleStore.activeVehicle.id, dateVal)
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

watch(() => maintForm.value.date, (newDate) => {
  if (showAddMaintModal.value && newDate) {
    checkOdometerForDate(newDate)
  }
})

// Charges: manual entry and cost completion
const showChargeModal = ref(false)
const editingCharge = ref<any | null>(null)
const chargeForm = ref({
  date: toLocalDateTimeInput(new Date()),
  kwh_added: '',
  cost: '',
  currency: 'EUR',
  fx_rate: '',
  address: '',
  odometer: '',
  notes: '',
})

// datetime-local inputs expect local time, not UTC
function toLocalDateTimeInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function currencyPayload(form: { currency: string; fx_rate: string }) {
  if (form.currency === 'EUR') return { currency: 'EUR', fx_rate: null }
  return { currency: form.currency, fx_rate: form.fx_rate ? Number(form.fx_rate) : null }
}

async function ensureDocumentsLoaded() {
  if (!vehicleStore.activeVehicle) return
  try {
    documents.value = await api.getDocuments(vehicleStore.activeVehicle.id)
  } catch (err) {
    console.error('Failed to load documents', err)
  }
}

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    if (activeTab.value === 'TOLLS') {
      driveExpenses.value = await api.getDriveExpenses(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'MAINTENANCE') {
      maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'REMINDERS') {
      await loadReminders()
    } else if (activeTab.value === 'CHARGES') {
      chargesPage.value = 1
      const res = await api.getCharges(vehicleStore.activeVehicle.id, { missingCost: missingCostOnly.value })
      charges.value = res.charges
      chargesTotal.value = res.total || 0
      chargesWithoutCost.value = res.charges_without_cost || 0
    } else if (activeTab.value === 'DOCUMENTS') {
      documents.value = await api.getDocuments(vehicleStore.activeVehicle.id)
    }
    ensureDocumentsLoaded()
    if (activeTab.value !== 'REMINDERS') {
      loadReminders()
    }
  } catch (err) {
    console.error('Failed to load expenses', err)
  } finally {
    loading.value = false
  }
}

async function loadRecentDrives() {
  if (!vehicleStore.activeVehicle) return
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { limit: 200 })
    recentDrives.value = res.drives || []
  } catch (err) {
    console.error('Failed to load recent drives', err)
  }
}

function openAddTollModal() {
  editingTollId.value = null
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
  showAddTollModal.value = true
  loadRecentDrives()
  ensureDocumentsLoaded()
}

function openEditTollModal(e: any) {
  editingTollId.value = e.id
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
  showAddTollModal.value = true
  loadRecentDrives()
  ensureDocumentsLoaded()
}

async function handleDeleteToll(e: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer la dépense',
    message: `Supprimer ce péage / parking de ${Number(e.amount).toFixed(2)} € ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(vehicleStore.activeVehicle.id, e.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
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

watch(
  () => route.query.tab,
  (tab) => {
    if (tab && ['TOLLS', 'MAINTENANCE', 'REMINDERS', 'CHARGES', 'DOCUMENTS'].includes(String(tab))) {
      activeTab.value = tab as any
    }
  }
)

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value, vehicleStore.lastSyncTimestamp],
  () => {
    loadData()
  }
)

onMounted(() => {
  if (route.query.tab && ['TOLLS', 'MAINTENANCE', 'REMINDERS', 'CHARGES', 'DOCUMENTS'].includes(String(route.query.tab))) {
    activeTab.value = route.query.tab as any
  }
  loadData()
  loadReminders()
})

async function handleCreateToll() {
  if (!vehicleStore.activeVehicle) return
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
      await api.updateDriveExpense(vehicleStore.activeVehicle.id, editingTollId.value, payload)
    } else {
      await api.createDriveExpense(vehicleStore.activeVehicle.id, payload)
    }
    showAddTollModal.value = false
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openAddMaintModal() {
  editingMaintId.value = null
  detectedOdometer.value = null
  shouldClosePrevious.value = false
  maintForm.value = {
    category: 'MAINTENANCE',
    amount: '',
    currency: 'EUR',
    fx_rate: '',
    date: new Date().toISOString().substring(0, 10),
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
  showAddMaintModal.value = true
  checkOdometerForDate(maintForm.value.date)
  ensureDocumentsLoaded()
}

function openEditMaintModal(m: any) {
  editingMaintId.value = m.id
  detectedOdometer.value = null
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
  showAddMaintModal.value = true
  ensureDocumentsLoaded()
}

async function handleDeleteMaint(m: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer la dépense',
    message: `Supprimer la dépense "${m.description}" de ${Number(m.amount).toFixed(2)} € ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteMaintenance(vehicleStore.activeVehicle.id, m.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleCreateMaint() {
  if (!vehicleStore.activeVehicle) return
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
      await api.updateMaintenance(vehicleStore.activeVehicle.id, editingMaintId.value, payload)
    } else {
      await api.createMaintenance(vehicleStore.activeVehicle.id, payload)
    }
    showAddMaintModal.value = false
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// Next page of charges, so that older charges can also be completed or corrected
async function loadMoreCharges() {
  if (!vehicleStore.activeVehicle) return
  loadingMoreCharges.value = true
  try {
    const res = await api.getCharges(vehicleStore.activeVehicle.id, { page: chargesPage.value + 1, missingCost: missingCostOnly.value })
    chargesPage.value += 1
    charges.value = [...charges.value, ...res.charges]
    chargesTotal.value = res.total || 0
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    loadingMoreCharges.value = false
  }
}

// Drives selected for a toll that are older than the loaded recent drives
const selectedDrivesNotListed = computed(
  () => selectedDriveIds.value.filter((id) => !recentDrives.value.some((d) => d.id === id)).length
)

function toggleMissingCostFilter() {
  missingCostOnly.value = !missingCostOnly.value
  loadData()
}

function openAddChargeModal() {
  editingCharge.value = null
  chargeForm.value = {
    date: toLocalDateTimeInput(new Date()),
    kwh_added: '',
    cost: '',
    currency: 'EUR',
    fx_rate: '',
    address: '',
    odometer: vehicleStore.activeVehicle?.current_odometer ? String(Math.round(vehicleStore.activeVehicle.current_odometer)) : '',
    notes: '',
    document_id: null,
    document_filename: null,
  }
  showChargeModal.value = true
  ensureDocumentsLoaded()
}

function openEditChargeModal(c: any) {
  editingCharge.value = c
  chargeForm.value = {
    date: toLocalDateTimeInput(new Date(c.date)),
    kwh_added: String(c.kwh_added),
    cost: c.cost !== null && c.cost !== undefined ? String(c.cost) : '',
    currency: c.currency || 'EUR',
    fx_rate: c.fx_rate ? String(c.fx_rate) : '',
    address: c.address || '',
    odometer: c.odometer ? String(Math.round(c.odometer)) : '',
    notes: c.notes || '',
    document_id: c.document_id || null,
    document_filename: c.document_filename || null,
  }
  showChargeModal.value = true
  ensureDocumentsLoaded()
}

async function handleSaveCharge() {
  if (!vehicleStore.activeVehicle) return
  const payload = {
    date: new Date(chargeForm.value.date).toISOString(),
    kwh_added: Number(chargeForm.value.kwh_added),
    cost: Number(chargeForm.value.cost),
    ...currencyPayload(chargeForm.value),
    address: chargeForm.value.address || null,
    odometer: chargeForm.value.odometer ? Number(chargeForm.value.odometer) : null,
    notes: chargeForm.value.notes || null,
    document_id: chargeForm.value.document_id || null,
  }
  try {
    if (editingCharge.value) {
      await api.updateCharge(vehicleStore.activeVehicle.id, editingCharge.value.id, payload)
    } else {
      await api.createCharge(vehicleStore.activeVehicle.id, payload)
    }
    showChargeModal.value = false
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function onSelectExistingDoc(docId: string, form: any) {
  if (!docId) {
    form.document_id = null
    form.document_filename = null
    return
  }
  const found = documents.value.find((d) => d.id === docId)
  if (found) {
    form.document_id = found.id
    form.document_filename = found.filename
  }
}

async function onFileInputChange(event: Event, form: any) {
  const input = event.target as HTMLInputElement
  if (!input.files || !input.files.length) return
  const file = input.files[0]
  if (!vehicleStore.activeVehicle) return
  isUploadingDocument.value = true
  try {
    const doc = await api.uploadDocument(vehicleStore.activeVehicle.id, file)
    documents.value.unshift(doc)
    form.document_id = doc.id
    form.document_filename = doc.filename
    showAlert(`Fichier « ${doc.filename} » téléversé et rattaché`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors du téléversement : ${err.message}`, 'Erreur', 'danger')
  } finally {
    isUploadingDocument.value = false
    input.value = ''
  }
}

interface DocumentPreviewState {
  url: string
  filename: string
  isPdf: boolean
  isImage: boolean
}
const previewDoc = ref<DocumentPreviewState | null>(null)
const loadingDocId = ref<string | null>(null)

function closeDocPreview() {
  if (previewDoc.value?.url) {
    URL.revokeObjectURL(previewDoc.value.url)
  }
  previewDoc.value = null
}

function downloadFromPreview() {
  if (!previewDoc.value) return
  const a = document.createElement('a')
  a.href = previewDoc.value.url
  a.download = previewDoc.value.filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function openInNewTab() {
  if (!previewDoc.value) return
  window.open(previewDoc.value.url, '_blank')
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

async function viewOrDownloadDocument(docId: string, filename?: string, download = false) {
  if (!vehicleStore.activeVehicle || !docId) return
  loadingDocId.value = docId
  try {
    const { blob, filename: serverFilename } = await api.downloadDocumentBlob(vehicleStore.activeVehicle.id, docId)
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

async function handleDeleteDocument(doc: ExpenseDocumentHeader) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer le justificatif',
    message: `Supprimer le justificatif « ${doc.filename} » ? Les dépenses associées seront conservées mais ne pointeront plus vers ce document.`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDocument(vehicleStore.activeVehicle.id, doc.id)
    documents.value = documents.value.filter((d) => d.id !== doc.id)
    showAlert('Justificatif supprimé', 'Succès', 'success')
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openUploadDocumentModal() {
  uploadDocDescription.value = ''
  uploadDocFile.value = null
  showUploadDocModal.value = true
}

function onUploadDocFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    uploadDocFile.value = input.files[0]
  }
}

async function handleUploadStandaloneDocument() {
  if (!vehicleStore.activeVehicle || !uploadDocFile.value) {
    showAlert('Veuillez sélectionner un fichier', 'Champ requis', 'warning')
    return
  }
  isUploadingDocument.value = true
  try {
    const doc = await api.uploadDocument(vehicleStore.activeVehicle.id, uploadDocFile.value, uploadDocDescription.value)
    documents.value.unshift(doc)
    showUploadDocModal.value = false
    showAlert(`Fichier « ${doc.filename} » ajouté avec succès`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors du téléversement : ${err.message}`, 'Erreur', 'danger')
  } finally {
    isUploadingDocument.value = false
  }
}

function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 o'
  const k = 1024
  const sizes = ['o', 'Ko', 'Mo', 'Go']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

async function handleDeleteCharge(c: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer la recharge',
    message: `Supprimer cette recharge manuelle de ${c.kwh_added} kWh ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteCharge(vehicleStore.activeVehicle.id, c.id)
    await loadData()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

function formatDriveTime(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
// --- RAPPELS D'ENTRETIEN & WEBHOOKS ---
const reminders = ref<MaintenanceReminder[]>([])
const loadingReminders = ref(false)
const showReminderModal = ref(false)
const editingReminderId = ref<string | null>(null)
const reminderForm = ref({
  title: '',
  category: 'MAINTENANCE',
  interval_km: '' as number | '',
  interval_months: '' as number | '',
  last_service_odometer: '' as number | '',
  last_service_date: new Date().toISOString().substring(0, 10),
  lead_km: 1000,
  lead_days: 15,
  webhook_enabled: true,
})

const showCompleteReminderModal = ref(false)
const completingReminder = ref<MaintenanceReminder | null>(null)
const completeForm = ref({
  service_date: new Date().toISOString().substring(0, 10),
  service_odometer: '' as number | '',
  log_expense: false,
  expense_amount: '',
  expense_description: '',
})

const showWebhookModal = ref(false)
const vehicleWebhook = ref<VehicleWebhook | null>(null)
const webhookForm = ref({
  url: '',
  type: 'DISCORD',
  enabled: true,
})
const isTestingWebhook = ref(false)
const isSavingWebhook = ref(false)
const webhookTestResult = ref<{ success: boolean; message: string } | null>(null)

interface ReminderPreset {
  title: string
  category: string
  interval_km: number | ''
  interval_months: number | ''
  lead_km: number
  lead_days: number
}

const REMINDER_PRESETS: ReminderPreset[] = [
  {
    title: 'Permutation des pneus',
    category: 'TIRES',
    interval_km: 10000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
  {
    title: 'Filtre d\'habitacle',
    category: 'MAINTENANCE',
    interval_km: 40000,
    interval_months: 24,
    lead_km: 2000,
    lead_days: 30,
  },
  {
    title: 'Contrôle liquide de frein',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: 'Contrôle technique',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: 'Balais d\'essuie-glace',
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 12,
    lead_km: 0,
    lead_days: 15,
  },
  {
    title: 'Nettoyage & graissage des étriers',
    category: 'MAINTENANCE',
    interval_km: 20000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
]

function applyReminderPreset(preset: ReminderPreset) {
  reminderForm.value.title = preset.title
  reminderForm.value.category = preset.category
  reminderForm.value.interval_km = preset.interval_km
  reminderForm.value.interval_months = preset.interval_months
  reminderForm.value.lead_km = preset.lead_km
  reminderForm.value.lead_days = preset.lead_days
}

const overdueReminders = computed(() => reminders.value.filter((r) => r.status === 'OVERDUE'))
const dueSoonReminders = computed(() => reminders.value.filter((r) => r.status === 'DUE_SOON'))
const urgentRemindersCount = computed(() => overdueReminders.value.length + dueSoonReminders.value.length)

async function loadReminders() {
  if (!vehicleStore.activeVehicle) return
  loadingReminders.value = true
  try {
    reminders.value = await api.getReminders(vehicleStore.activeVehicle.id)
  } catch (err: any) {
    console.error('Failed to load reminders', err)
  } finally {
    loadingReminders.value = false
  }
}

function openAddReminderModal() {
  editingReminderId.value = null
  const currentOdo = vehicleStore.activeVehicle?.current_odometer
    ? Math.round(vehicleStore.activeVehicle.current_odometer)
    : ''
  reminderForm.value = {
    title: '',
    category: 'MAINTENANCE',
    interval_km: 10000,
    interval_months: 12,
    last_service_odometer: currentOdo,
    last_service_date: new Date().toISOString().substring(0, 10),
    lead_km: 1000,
    lead_days: 15,
    webhook_enabled: true,
  }
  showReminderModal.value = true
}

function openEditReminderModal(r: MaintenanceReminder) {
  editingReminderId.value = r.id
  reminderForm.value = {
    title: r.title,
    category: r.category,
    interval_km: r.interval_km ?? '',
    interval_months: r.interval_months ?? '',
    last_service_odometer: r.last_service_odometer !== null && r.last_service_odometer !== undefined ? Math.round(r.last_service_odometer) : '',
    last_service_date: r.last_service_date ? r.last_service_date.substring(0, 10) : new Date().toISOString().substring(0, 10),
    lead_km: r.lead_km,
    lead_days: r.lead_days,
    webhook_enabled: r.webhook_enabled,
  }
  showReminderModal.value = true
}

async function handleSaveReminder() {
  if (!vehicleStore.activeVehicle) return
  if (!reminderForm.value.title.trim()) {
    showAlert('Le titre du rappel est requis', 'Champ requis', 'warning')
    return
  }
  if (!reminderForm.value.interval_km && !reminderForm.value.interval_months) {
    showAlert('Spécifiez au moins un intervalle en km ou en mois', 'Champ requis', 'warning')
    return
  }
  try {
    const payload = {
      title: reminderForm.value.title.trim(),
      category: reminderForm.value.category,
      interval_km: reminderForm.value.interval_km !== '' ? Number(reminderForm.value.interval_km) : null,
      interval_months: reminderForm.value.interval_months !== '' ? Number(reminderForm.value.interval_months) : null,
      last_service_odometer: reminderForm.value.last_service_odometer !== '' ? Number(reminderForm.value.last_service_odometer) : null,
      last_service_date: reminderForm.value.last_service_date ? new Date(reminderForm.value.last_service_date).toISOString() : null,
      lead_km: Number(reminderForm.value.lead_km || 0),
      lead_days: Number(reminderForm.value.lead_days || 0),
      webhook_enabled: reminderForm.value.webhook_enabled,
    }
    if (editingReminderId.value) {
      await api.updateReminder(vehicleStore.activeVehicle.id, editingReminderId.value, payload)
      showAlert('Rappel mis à jour', 'Succès', 'success')
    } else {
      await api.createReminder(vehicleStore.activeVehicle.id, payload)
      showAlert('Rappel créé avec succès', 'Succès', 'success')
    }
    showReminderModal.value = false
    await loadReminders()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteReminder(r: MaintenanceReminder) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer le rappel',
    message: `Êtes-vous sûr de vouloir supprimer le rappel d'entretien « ${r.title} » ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteReminder(vehicleStore.activeVehicle.id, r.id)
    showAlert('Rappel supprimé', 'Succès', 'success')
    await loadReminders()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openCompleteReminder(r: MaintenanceReminder) {
  completingReminder.value = r
  const currentOdo = vehicleStore.activeVehicle?.current_odometer
    ? Math.round(vehicleStore.activeVehicle.current_odometer)
    : ''
  completeForm.value = {
    service_date: new Date().toISOString().substring(0, 10),
    service_odometer: currentOdo,
    log_expense: false,
    expense_amount: '',
    expense_description: `Entretien effectué : ${r.title}`,
  }
  showCompleteReminderModal.value = true
}

async function handleCompleteReminder() {
  if (!vehicleStore.activeVehicle || !completingReminder.value) return
  try {
    const payload = {
      service_date: new Date(completeForm.value.service_date).toISOString(),
      service_odometer: completeForm.value.service_odometer !== '' ? Number(completeForm.value.service_odometer) : null,
    }
    await api.completeReminder(vehicleStore.activeVehicle.id, completingReminder.value.id, payload)

    if (completeForm.value.log_expense && Number(completeForm.value.expense_amount) > 0) {
      await api.createMaintenance(vehicleStore.activeVehicle.id, {
        category: completingReminder.value.category === 'TIRES' ? 'TIRES' : 'MAINTENANCE',
        amount: Number(completeForm.value.expense_amount),
        currency: 'EUR',
        fx_rate: null,
        date: new Date(completeForm.value.service_date).toISOString(),
        description: completeForm.value.expense_description || completingReminder.value.title,
        odometer: completeForm.value.service_odometer ? Number(completeForm.value.service_odometer) : null,
        is_recurring: false,
      })
    }

    showAlert('Entretien marqué comme fait et intervalle réinitialisé !', 'Succès', 'success')
    showCompleteReminderModal.value = false
    await loadReminders()
    if (completeForm.value.log_expense) {
      maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
    }
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function openWebhookModal() {
  if (!vehicleStore.activeVehicle) return
  webhookTestResult.value = null
  try {
    const wh = await api.getVehicleWebhook(vehicleStore.activeVehicle.id)
    vehicleWebhook.value = wh
    if (wh) {
      webhookForm.value = {
        url: wh.url,
        type: wh.type,
        enabled: wh.enabled,
      }
    } else {
      webhookForm.value = {
        url: '',
        type: 'DISCORD',
        enabled: true,
      }
    }
  } catch (err: any) {
    console.error('Failed to load webhook', err)
  }
  showWebhookModal.value = true
}

async function handleSaveWebhook() {
  if (!vehicleStore.activeVehicle) return
  if (!webhookForm.value.url.trim()) {
    showAlert('Veuillez saisir une URL de webhook valide', 'Champ requis', 'warning')
    return
  }
  isSavingWebhook.value = true
  try {
    const saved = await api.saveVehicleWebhook(vehicleStore.activeVehicle.id, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
      enabled: webhookForm.value.enabled,
    })
    vehicleWebhook.value = saved
    showAlert('Configuration du webhook enregistrée avec succès', 'Succès', 'success')
    showWebhookModal.value = false
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    isSavingWebhook.value = false
  }
}

async function handleTestWebhook() {
  if (!vehicleStore.activeVehicle) return
  if (!webhookForm.value.url.trim()) {
    showAlert('Saisissez d\'abord une URL de webhook', 'Champ requis', 'warning')
    return
  }
  isTestingWebhook.value = true
  webhookTestResult.value = null
  try {
    const res = await api.testVehicleWebhook(vehicleStore.activeVehicle.id, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
    })
    webhookTestResult.value = res
  } catch (err: any) {
    webhookTestResult.value = { success: false, message: err.message || 'Erreur inattendue' }
  } finally {
    isTestingWebhook.value = false
  }
}

async function handleDeleteWebhook() {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer le webhook',
    message: 'Êtes-vous sûr de vouloir supprimer l\'intégration webhook pour ce véhicule ?',
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteVehicleWebhook(vehicleStore.activeVehicle.id)
    vehicleWebhook.value = null
    webhookForm.value = { url: '', type: 'DISCORD', enabled: true }
    showAlert('Webhook supprimé', 'Succès', 'success')
    showWebhookModal.value = false
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Dépenses & Entretien</h2>
        <p class="text-sm text-slate-400">Péages, parkings, entretien récurrent, assurance et recharges</p>
      </div>

      <div class="flex items-center gap-2">
        <button
          v-if="activeTab === 'TOLLS'"
          @click="openAddTollModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Péage / Parking
        </button>
        <button
          v-if="activeTab === 'MAINTENANCE'"
          @click="openAddMaintModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Entretien / Fixe
        </button>
        <button
          v-if="activeTab === 'REMINDERS'"
          @click="openWebhookModal"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-2 border border-slate-700 transition-colors"
          title="Configurer le webhook pour recevoir des notifications en temps réel"
        >
          <Radio class="w-3.5 h-3.5 text-violet-400" />
          <span class="hidden sm:inline">Webhook Homelab</span>
          <span class="sm:hidden">Webhook</span>
        </button>
        <button
          v-if="activeTab === 'REMINDERS'"
          @click="openAddReminderModal"
          class="px-3.5 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-violet-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Nouveau rappel
        </button>
        <button
          v-if="activeTab === 'CHARGES'"
          @click="openAddChargeModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Recharge hors TeslaMate
        </button>
        <button
          v-if="activeTab === 'DOCUMENTS'"
          @click="openUploadDocumentModal"
          class="px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-indigo-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          Ajouter un justificatif
        </button>
      </div>
    </div>

    <!-- Sub-tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2 overflow-x-auto">
      <button
        @click="activeTab = 'TOLLS'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors shrink-0"
        :class="activeTab === 'TOLLS' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Receipt class="w-4 h-4" />
        Péages & Parkings
      </button>
      <button
        @click="activeTab = 'MAINTENANCE'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors shrink-0"
        :class="activeTab === 'MAINTENANCE' ? 'bg-pink-500/20 text-pink-400 border border-pink-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Wrench class="w-4 h-4" />
        Entretien & Coûts Fixes
      </button>
      <button
        @click="activeTab = 'REMINDERS'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors shrink-0 relative"
        :class="activeTab === 'REMINDERS' ? 'bg-violet-500/20 text-violet-400 border border-violet-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Bell class="w-4 h-4" />
        Rappels d'Entretien
        <span
          v-if="urgentRemindersCount > 0"
          class="px-1.5 py-0.2 text-[10px] font-bold rounded-full"
          :class="overdueReminders.length > 0 ? 'bg-rose-500 text-white' : 'bg-amber-500 text-slate-950'"
        >
          {{ urgentRemindersCount }}
        </span>
      </button>
      <button
        @click="activeTab = 'CHARGES'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors shrink-0"
        :class="activeTab === 'CHARGES' ? 'bg-sky-500/20 text-sky-400 border border-sky-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Zap class="w-4 h-4" />
        Recharges Électriques
      </button>
      <button
        @click="activeTab = 'DOCUMENTS'"
        class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition-colors shrink-0"
        :class="activeTab === 'DOCUMENTS' ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30' : 'text-slate-400 hover:text-white'"
      >
        <Paperclip class="w-4 h-4" />
        Justificatifs & Factures
      </button>
    </div>

    <!-- Content: Tolls -->
    <div v-if="activeTab === 'TOLLS'">
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!driveExpenses.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucun péage ou parking enregistré.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="e in driveExpenses"
          :key="e.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        >
          <div class="space-y-1.5 min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0">
                {{ e.type }}
              </span>
              <span class="text-xs text-slate-400 shrink-0">{{ formatDate(e.date) }}</span>
              <span v-if="e.drive_title" class="text-xs px-2.5 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1 truncate max-w-xs">
                <Navigation class="w-3 h-3 shrink-0" /> <span class="truncate">{{ e.drive_title }}</span>
              </span>
              <span v-else-if="e.trip_group_name" class="text-xs px-2.5 py-0.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 truncate max-w-xs">
                <Layers class="w-3 h-3 shrink-0" /> <span class="truncate">{{ e.trip_group_name }}</span>
              </span>
              <button
                v-if="e.document_id"
                @click="viewOrDownloadDocument(e.document_id, e.document_filename, false)"
                class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
                title="Voir le justificatif"
              >
                <Paperclip class="w-3 h-3 shrink-0" />
                <span class="truncate">{{ e.document_filename || 'Facture' }}</span>
              </button>
            </div>
            <p v-if="e.notes" class="text-sm text-slate-300">{{ e.notes }}</p>
          </div>
          <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
            <div class="text-lg font-extrabold text-amber-400">
              {{ e.amount.toFixed(2) }} {{ e.currency }}
            </div>
            <div class="flex items-center gap-1.5">
              <button
                v-if="e.drive_id || e.trip_group_id"
                @click="router.push({ path: '/carpools', query: e.drive_id ? { new_drive_id: e.drive_id } : { new_trip_group_id: e.trip_group_id } })"
                class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
                title="Créer un covoiturage pour ce trajet"
              >
                <Users class="w-3.5 h-3.5 text-cyan-400" />
                <span>Covoiturer</span>
              </button>
              <button
                @click="openEditTollModal(e)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-amber-400 rounded-xl transition-colors border border-slate-700/60"
                title="Modifier ce péage"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                @click="handleDeleteToll(e)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer ce péage"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Maintenance -->
    <div v-if="activeTab === 'MAINTENANCE'">
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!maintenanceExpenses.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucune dépense d'entretien ou fixe enregistrée.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="m in maintenanceExpenses"
          :key="m.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        >
          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-pink-500/10 text-pink-400 border border-pink-500/20 shrink-0">
                {{ CATEGORY_LABELS[m.category] || m.category }}
              </span>
              <span class="text-xs text-slate-400 shrink-0">{{ formatDate(m.date) }}</span>
              <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
                <Repeat class="w-3 h-3 text-pink-400" /> tous les {{ m.recurrence_interval_months }} mois
                <template v-if="m.recurrence_end_date">jusqu'au {{ formatDate(m.recurrence_end_date) }}</template>
              </span>
              <span v-else-if="m.amortization_mode === 'DISTANCE'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shrink-0">
                Lissé sur {{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km
              </span>
              <span v-else-if="m.amortization_mode === 'DURATION'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20 shrink-0">
                Lissé sur {{ m.coverage_months || 24 }} mois
              </span>
              <span v-else-if="m.amortization_mode === 'HYBRID'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shrink-0">
                Lissé mixte ({{ m.coverage_km ? Math.round(m.coverage_km).toLocaleString('fr-FR') : '50 000' }} km / {{ m.coverage_months || 24 }} mois)
              </span>
              <span v-if="m.closes_maintenance_id" class="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0">
                Clôture la révision précédente
              </span>
              <button
                v-if="m.document_id"
                @click="viewOrDownloadDocument(m.document_id, m.document_filename, false)"
                class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
                title="Voir le justificatif"
              >
                <Paperclip class="w-3 h-3 shrink-0" />
                <span class="truncate">{{ m.document_filename || 'Facture' }}</span>
              </button>
            </div>
            <p class="text-sm font-semibold text-slate-200 truncate">{{ m.description }}</p>
            <p v-if="m.odometer" class="text-xs text-slate-400">À {{ Math.round(m.odometer) }} km</p>
          </div>
          <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
            <div class="text-lg font-extrabold text-pink-400">
              {{ m.amount.toFixed(2) }} {{ m.currency }}
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="openEditMaintModal(m)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-pink-400 rounded-xl transition-colors border border-slate-700/60"
                title="Modifier cette dépense"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                @click="handleDeleteMaint(m)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer cette dépense"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Reminders -->
    <div v-if="activeTab === 'REMINDERS'" class="space-y-4">
      <!-- Webhook homelab info banner -->
      <div class="p-4 bg-slate-900 border border-slate-800 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-violet-500/10 border border-violet-500/20 text-violet-400 shrink-0">
            <Radio class="w-5 h-5" />
          </div>
          <div>
            <div class="font-bold text-white flex items-center gap-2">
              <span>Webhook Homelab</span>
              <span
                class="px-2 py-0.5 text-[10px] rounded-full font-bold border"
                :class="vehicleWebhook?.enabled ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
              >
                {{ vehicleWebhook?.enabled ? `Actif (${vehicleWebhook.type})` : 'Non configuré' }}
              </span>
            </div>
            <p class="text-slate-400 text-[11px] mt-0.5">
              {{ vehicleWebhook?.enabled ? 'Les alertes sont automatiquement envoyées dès qu\'une synchronisation TeslaMate franchit le seuil.' : 'Recevez automatiquement des alertes sur Discord, Telegram ou Gotify dès qu\'une échéance approche.' }}
            </p>
          </div>
        </div>
        <button
          @click="openWebhookModal"
          class="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 shrink-0 transition-colors self-start sm:self-auto"
        >
          {{ vehicleWebhook ? 'Modifier le webhook' : 'Configurer un webhook' }}
        </button>
      </div>

      <!-- Quick summary stats -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
          <span class="text-xs text-slate-400">Total rappels</span>
          <span class="text-xl font-bold text-white mt-1">{{ reminders.length }}</span>
        </div>
        <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
          <span class="text-xs text-emerald-400 flex items-center gap-1.5">
            <CheckCircle2 class="w-3.5 h-3.5" />
            À jour
          </span>
          <span class="text-xl font-bold text-emerald-400 mt-1">{{ okReminders.length }}</span>
        </div>
        <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
          <span class="text-xs text-amber-400 flex items-center gap-1.5">
            <Clock class="w-3.5 h-3.5" />
            À prévoir
          </span>
          <span class="text-xl font-bold text-amber-400 mt-1">{{ dueSoonReminders.length }}</span>
        </div>
        <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
          <span class="text-xs text-rose-400 flex items-center gap-1.5">
            <AlertTriangle class="w-3.5 h-3.5" />
            En retard
          </span>
          <span class="text-xl font-bold text-rose-400 mt-1">{{ overdueReminders.length }}</span>
        </div>
      </div>

      <div v-if="loadingReminders" class="text-center py-12 text-slate-400">Chargement des rappels...</div>

      <!-- Empty state -->
      <div v-else-if="!reminders.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-4">
        <div class="p-3 bg-violet-500/10 border border-violet-500/20 text-violet-400 w-12 h-12 rounded-2xl mx-auto flex items-center justify-center">
          <Bell class="w-6 h-6" />
        </div>
        <div>
          <h3 class="text-base font-bold text-white">Aucun rappel d'entretien configuré</h3>
          <p class="text-xs text-slate-400 mt-1 max-w-md mx-auto">
            Suivez l'usure de vos pneumatiques, le remplacement du filtre habitacle, les révisions ou le contrôle technique avec des rappels par kilométrage et calendrier.
          </p>
        </div>

        <div class="pt-2">
          <p class="text-xs font-semibold text-slate-300 mb-3">Ajouter un rappel type en 1 clic :</p>
          <div class="flex flex-wrap justify-center gap-2 max-w-lg mx-auto">
            <button
              v-for="preset in REMINDER_PRESETS"
              :key="preset.title"
              type="button"
              @click="openAddReminderModal(); applyReminderPreset(preset)"
              class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded-xl border border-slate-700 transition-colors flex items-center gap-1.5"
            >
              <Sparkles class="w-3 h-3 text-violet-400" />
              {{ preset.title }}
            </button>
          </div>
        </div>

        <div class="pt-2">
          <button
            @click="openAddReminderModal"
            class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl inline-flex items-center gap-2 shadow-lg shadow-violet-600/20"
          >
            <Plus class="w-4 h-4" />
            Créer un rappel personnalisé
          </button>
        </div>
      </div>

      <!-- Reminders List -->
      <div v-else class="space-y-3">
        <div
          v-for="r in reminders"
          :key="r.id"
          class="bg-slate-900 border p-4 rounded-2xl flex flex-col justify-between gap-3 transition-colors"
          :class="r.status === 'OVERDUE' ? 'border-rose-500/40 bg-rose-500/5' : r.status === 'DUE_SOON' ? 'border-amber-500/40 bg-amber-500/5' : 'border-slate-800'"
        >
          <!-- Card top -->
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <div class="flex items-center gap-2.5 flex-wrap">
              <span
                class="text-xs px-2.5 py-0.5 rounded-full font-bold border flex items-center gap-1"
                :class="r.status === 'OVERDUE' ? 'bg-rose-500/20 text-rose-300 border-rose-500/30' : r.status === 'DUE_SOON' ? 'bg-amber-500/20 text-amber-300 border-amber-500/30' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'"
              >
                <AlertTriangle v-if="r.status === 'OVERDUE'" class="w-3 h-3" />
                <Clock v-else-if="r.status === 'DUE_SOON'" class="w-3 h-3" />
                <CheckCircle2 v-else class="w-3 h-3" />
                {{ r.status === 'OVERDUE' ? 'En retard' : r.status === 'DUE_SOON' ? 'À prévoir bientôt' : 'À jour' }}
              </span>

              <span class="text-xs px-2 py-0.5 rounded-lg bg-slate-800 text-slate-300 border border-slate-700">
                {{ r.category === 'TIRES' ? 'Pneumatiques' : 'Entretien' }}
              </span>

              <h4 class="text-sm font-bold text-white">{{ r.title }}</h4>

              <span
                v-if="r.webhook_enabled"
                class="text-[10px] px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-400 border border-violet-500/20 flex items-center gap-1"
                title="Notification webhook activée pour ce rappel"
              >
                <Radio class="w-2.5 h-2.5" />
                Webhook
              </span>
            </div>

            <!-- Due Badges / Urgency pill -->
            <div class="flex items-center gap-2 text-xs font-semibold">
              <span
                v-if="r.remaining_km !== null && r.remaining_km !== undefined"
                class="px-2 py-0.5 rounded-lg"
                :class="r.remaining_km <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_km <= r.lead_km ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
              >
                {{ r.remaining_km <= 0 ? `Dépassé de ${Math.abs(Math.round(r.remaining_km)).toLocaleString('fr-FR')} km` : `Reste ${Math.round(r.remaining_km).toLocaleString('fr-FR')} km` }}
              </span>
              <span
                v-if="r.remaining_days !== null && r.remaining_days !== undefined"
                class="px-2 py-0.5 rounded-lg"
                :class="r.remaining_days <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_days <= r.lead_days ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
              >
                {{ r.remaining_days <= 0 ? `Dépassé de ${Math.abs(r.remaining_days)} j` : `Reste ${r.remaining_days} j` }}
              </span>
            </div>
          </div>

          <!-- Card details grid -->
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5 pt-1 text-xs text-slate-300">
            <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
              <span class="text-[11px] text-slate-400 block mb-0.5">Échéance kilométrique</span>
              <span v-if="r.interval_km" class="font-medium text-white">
                Tous les {{ r.interval_km.toLocaleString('fr-FR') }} km
                <span v-if="r.due_odometer" class="text-slate-400 block text-[11px]">
                  Échéance : {{ Math.round(r.due_odometer).toLocaleString('fr-FR') }} km
                </span>
              </span>
              <span v-else class="text-slate-400 italic">Non applicable</span>
            </div>

            <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
              <span class="text-[11px] text-slate-400 block mb-0.5">Échéance calendaire</span>
              <span v-if="r.interval_months" class="font-medium text-white">
                Tous les {{ r.interval_months }} mois
                <span v-if="r.due_date" class="text-slate-400 block text-[11px]">
                  Échéance : {{ formatDate(r.due_date) }}
                </span>
              </span>
              <span v-else class="text-slate-400 italic">Non applicable</span>
            </div>

            <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
              <span class="text-[11px] text-slate-400 block mb-0.5">Dernière réalisation</span>
              <span class="font-medium text-white">
                {{ r.last_service_date ? formatDate(r.last_service_date) : 'Non renseigné' }}
                <span v-if="r.last_service_odometer" class="text-slate-400 block text-[11px]">
                  à {{ Math.round(r.last_service_odometer).toLocaleString('fr-FR') }} km
                </span>
              </span>
            </div>
          </div>

          <!-- Card footer -->
          <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
            <div class="text-[11px] text-slate-400">
              <span v-if="r.last_notified_at">
                Dernière alerte webhook : {{ formatDate(r.last_notified_at) }}
              </span>
              <span v-else>
                Alerte anticipée : {{ r.lead_km.toLocaleString('fr-FR') }} km / {{ r.lead_days }} j avant
              </span>
            </div>

            <div class="flex items-center gap-1.5">
              <button
                @click="openCompleteReminder(r)"
                class="px-2.5 py-1.5 bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 text-xs font-semibold rounded-xl border border-emerald-500/30 flex items-center gap-1.5 transition-colors"
                title="Marquer cet entretien comme effectué et mettre à jour le rappel"
              >
                <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
                <span>Marquer fait</span>
              </button>
              <button
                @click="openEditReminderModal(r)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-violet-400 rounded-xl transition-colors border border-slate-700/60"
                title="Modifier ce rappel"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                @click="handleDeleteReminder(r)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer ce rappel"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Charges -->
    <div v-if="activeTab === 'CHARGES'" class="space-y-3">
      <!-- Pre-TeslaMate Charges Banner if configured -->
      <div
        v-if="vehicleStore.activeVehicle?.pre_teslamate_kwh_100km && vehicleStore.activeVehicle?.pre_teslamate_eur_per_kwh"
        class="p-3 bg-sky-500/10 border border-sky-500/20 rounded-2xl flex items-center justify-between text-xs text-sky-300"
      >
        <div class="flex items-center gap-2">
          <Zap class="w-4 h-4 shrink-0 text-sky-400" />
          <span>
            Estimation avant TeslaMate active :
            <strong>{{ vehicleStore.activeVehicle.pre_teslamate_kwh_100km }} kWh/100km</strong> à
            <strong>{{ Number(vehicleStore.activeVehicle.pre_teslamate_eur_per_kwh).toFixed(4) }} €/kWh</strong>
            (intégrée automatiquement dans le TCO).
          </span>
        </div>
        <button
          @click="router.push('/vehicles')"
          class="shrink-0 font-medium underline hover:text-sky-200 transition-colors ml-2"
        >
          Modifier
        </button>
      </div>

      <button
        v-if="chargesWithoutCost > 0 || missingCostOnly"
        @click="toggleMissingCostFilter"
        class="w-full p-3 rounded-2xl text-left text-xs font-semibold flex items-center gap-2 border transition-colors"
        :class="missingCostOnly ? 'bg-amber-500/20 border-amber-500/40 text-amber-300' : 'bg-amber-500/10 border-amber-500/20 text-amber-400 hover:bg-amber-500/15'"
      >
        <AlertTriangle class="w-4 h-4 shrink-0" />
        <span v-if="missingCostOnly">Affichage des recharges sans coût uniquement — cliquer pour tout afficher</span>
        <span v-else>{{ chargesWithoutCost }} recharge(s) sans coût : le TCO est sous-estimé. Cliquer pour les compléter.</span>
      </button>
      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!charges.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucune recharge enregistrée. Synchronisez votre véhicule avec TeslaMate ou ajoutez une recharge manuelle.
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="c in charges"
          :key="c.id"
          class="bg-slate-900 border p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
          :class="c.cost === null ? 'border-amber-500/40' : 'border-slate-800'"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-sky-500/10 text-sky-400 border border-sky-500/20 shrink-0">
                +{{ c.kwh_added }} kWh
              </span>
              <span class="text-xs text-slate-400 shrink-0">{{ formatDate(c.date) }}</span>
              <span v-if="c.is_manual" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">Manuelle</span>
              <span v-else-if="c.cost_source === 'MANUAL'" class="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">Coût corrigé</span>
              <button
                v-if="c.document_id"
                @click="viewOrDownloadDocument(c.document_id, c.document_filename, false)"
                class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
                title="Voir le justificatif"
              >
                <Paperclip class="w-3 h-3 shrink-0" />
                <span class="truncate">{{ c.document_filename || 'Facture' }}</span>
              </button>
            </div>
            <p class="text-sm text-slate-300 mt-1 truncate" :title="c.address">{{ c.address || 'Lieu de recharge inconnu' }}</p>
          </div>
          <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
            <div class="text-left sm:text-right">
              <template v-if="c.cost !== null">
                <span class="text-lg font-extrabold text-sky-400">{{ c.cost.toFixed(2) }} {{ c.currency }}</span>
                <p v-if="c.kwh_added > 0" class="text-[11px] text-slate-400">
                  {{ (c.cost / c.kwh_added).toFixed(3) }} {{ c.currency }}/kWh
                </p>
              </template>
              <span v-else class="text-xs font-bold text-amber-400 flex items-center gap-1">
                <AlertTriangle class="w-3.5 h-3.5" /> Coût manquant
              </span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="openEditChargeModal(c)"
                class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-sky-400 rounded-xl transition-colors border border-slate-700/60"
                :title="c.is_manual ? 'Modifier cette recharge' : 'Renseigner / corriger le coût'"
              >
                <Pencil class="w-3.5 h-3.5" />
              </button>
              <button
                v-if="c.is_manual"
                @click="handleDeleteCharge(c)"
                class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
                title="Supprimer cette recharge"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
        <div class="flex items-center justify-between text-xs text-slate-400 px-1">
          <span>{{ charges.length }} recharge(s) affichée(s) sur {{ chargesTotal }}</span>
          <button
            v-if="charges.length < chargesTotal"
            @click="loadMoreCharges"
            :disabled="loadingMoreCharges"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 disabled:opacity-50"
          >
            {{ loadingMoreCharges ? 'Chargement...' : 'Charger plus' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Content: Documents & Invoices -->
    <div v-if="activeTab === 'DOCUMENTS'" class="space-y-4">
      <!-- Info banner -->
      <div class="p-4 bg-indigo-500/10 border border-indigo-500/20 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs text-indigo-300">
        <div class="flex items-center gap-2.5">
          <Paperclip class="w-4 h-4 text-indigo-400 shrink-0" />
          <span>
            Les justificatifs (factures, tickets, rapports d'atelier) sont stockés directement dans la base de données. Plusieurs dépenses peuvent être rattachées au même fichier.
          </span>
        </div>
        <span class="font-semibold shrink-0">
          {{ documents.length }} document(s)
        </span>
      </div>

      <div v-if="loading" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!documents.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-3">
        <p>Aucun justificatif ou facture téléversé pour ce véhicule.</p>
        <button
          @click="openUploadDocumentModal"
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl inline-flex items-center gap-2 shadow-lg shadow-indigo-600/20"
        >
          <UploadCloud class="w-4 h-4" />
          Téléverser un premier document
        </button>
      </div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div
          v-for="d in documents"
          :key="d.id"
          class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col justify-between gap-3 hover:border-slate-700 transition-colors"
        >
          <div class="space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="flex items-center gap-2.5 min-w-0">
                <div class="p-2 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 shrink-0">
                  <FileText class="w-5 h-5" />
                </div>
                <div class="min-w-0">
                  <h4 class="text-sm font-semibold text-white truncate" :title="d.filename">
                    {{ d.filename }}
                  </h4>
                  <p class="text-xs text-slate-400">
                    {{ formatDate(d.created_at) }} • {{ formatFileSize(d.file_size) }}
                  </p>
                </div>
              </div>
              <span
                class="text-[10px] px-2 py-0.5 rounded-full font-bold shrink-0 border"
                :class="d.linked_expenses_count > 0 ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
              >
                {{ d.linked_expenses_count > 0 ? `${d.linked_expenses_count} dépense(s) liée(s)` : 'Non associé' }}
              </span>
            </div>

            <p v-if="d.description" class="text-xs text-slate-300 italic pl-1">
              « {{ d.description }} »
            </p>
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
            <div class="flex items-center gap-1.5">
              <button
                @click="viewOrDownloadDocument(d.id, d.filename, false)"
                :disabled="loadingDocId === d.id"
                class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors disabled:opacity-50"
                title="Consulter le fichier"
              >
                <Loader2 v-if="loadingDocId === d.id" class="w-3.5 h-3.5 text-indigo-400 animate-spin" />
                <Eye v-else class="w-3.5 h-3.5 text-indigo-400" />
                <span>Ouvrir</span>
              </button>
              <button
                @click="viewOrDownloadDocument(d.id, d.filename, true)"
                class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors"
                title="Télécharger le fichier"
              >
                <Download class="w-3.5 h-3.5 text-indigo-400" />
                <span>Télécharger</span>
              </button>
            </div>

            <button
              @click="handleDeleteDocument(d)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              title="Supprimer ce document"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Add Toll/Parking -->
    <div
      v-if="showAddTollModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showAddTollModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Receipt class="w-5 h-5 text-amber-400" />
            {{ editingTollId ? 'Modifier le Péage / Parking' : 'Ajouter un Péage / Parking' }}
          </h3>
          <button @click="showAddTollModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
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
          <div class="space-y-2 bg-slate-800/50 p-3.5 rounded-xl border border-slate-700/60">
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
                  {{ formatDriveTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }} ({{ d.distance_km.toFixed(1) }} km)
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
                    <span>{{ formatDriveTime(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}</span>
                  </div>
                  <span class="font-mono text-[11px] text-slate-400">{{ d.distance_km.toFixed(0) }} km</span>
                </div>
              </div>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label for="expense-toll-date" class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
              <input id="expense-toll-date" v-model="tollForm.date" type="datetime-local" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
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
                  @click="viewOrDownloadDocument(tollForm.document_id, tollForm.document_filename, false)"
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

            <div v-else class="space-y-2">
              <div class="flex flex-col sm:flex-row gap-2">
                <div class="flex-1" v-if="documents.length > 0">
                  <label for="toll-existing-doc" class="sr-only">Choisir une facture existante</label>
                  <select
                    id="toll-existing-doc"
                    class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-slate-300"
                    @change="(e: any) => onSelectExistingDoc(e.target.value, tollForm)"
                  >
                    <option value="">-- Associer une facture existante --</option>
                    <option v-for="d in documents" :key="d.id" :value="d.id">
                      {{ d.filename }} ({{ formatDate(d.created_at) }})
                    </option>
                  </select>
                </div>
                <label class="cursor-pointer px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-medium flex items-center justify-center gap-1.5 transition-colors">
                  <UploadCloud class="w-3.5 h-3.5" />
                  <span>{{ isUploadingDocument ? 'Téléversement...' : 'Nouveau fichier' }}</span>
                  <input
                    type="file"
                    accept=".pdf,image/png,image/jpeg,image/webp"
                    class="hidden"
                    :disabled="isUploadingDocument"
                    @change="(e: any) => onFileInputChange(e, tollForm)"
                  />
                </label>
              </div>
              <p class="text-[10px] text-slate-400">
                PDF ou image. Plusieurs péages peuvent être rattachés à la même facture mensuelle.
              </p>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showAddTollModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="toll-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
            {{ editingTollId ? 'Mettre à jour' : 'Enregistrer' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Add / Edit Maintenance/Fixed -->
    <div
      v-if="showAddMaintModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showAddMaintModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Wrench class="w-5 h-5 text-pink-400" />
            {{ editingMaintId ? 'Modifier Entretien / Dépense Fixe' : 'Ajouter Entretien / Dépense Fixe' }}
          </h3>
          <button @click="showAddMaintModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
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
              <input id="expense-maint-date" v-model="maintForm.date" type="date" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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
                <input id="maint-form-recurrence-end-date" v-model="maintForm.recurrence_end_date" type="date" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
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
                  @click="viewOrDownloadDocument(maintForm.document_id, maintForm.document_filename, false)"
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

            <div v-else class="space-y-2">
              <div class="flex flex-col sm:flex-row gap-2">
                <div class="flex-1" v-if="documents.length > 0">
                  <label for="maint-existing-doc" class="sr-only">Choisir une facture existante</label>
                  <select
                    id="maint-existing-doc"
                    class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-slate-300"
                    @change="(e: any) => onSelectExistingDoc(e.target.value, maintForm)"
                  >
                    <option value="">-- Associer une facture existante --</option>
                    <option v-for="d in documents" :key="d.id" :value="d.id">
                      {{ d.filename }} ({{ formatDate(d.created_at) }})
                    </option>
                  </select>
                </div>
                <label class="cursor-pointer px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-medium flex items-center justify-center gap-1.5 transition-colors">
                  <UploadCloud class="w-3.5 h-3.5" />
                  <span>{{ isUploadingDocument ? 'Téléversement...' : 'Nouveau fichier' }}</span>
                  <input
                    type="file"
                    accept=".pdf,image/png,image/jpeg,image/webp"
                    class="hidden"
                    :disabled="isUploadingDocument"
                    @change="(e: any) => onFileInputChange(e, maintForm)"
                  />
                </label>
              </div>
              <p class="text-[10px] text-slate-400">
                PDF ou image (facture atelier, justificatif d'assurance, etc.).
              </p>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showAddMaintModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="maint-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
            {{ editingMaintId ? 'Mettre à jour' : 'Enregistrer' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Manual charge / cost completion -->
    <div
      v-if="showChargeModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showChargeModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="min-w-0 pr-2">
            <h3 class="text-base font-bold text-white flex items-center gap-2 truncate">
              <Zap class="w-5 h-5 text-sky-400 shrink-0" />
              {{ !editingCharge ? 'Recharge hors TeslaMate' : editingCharge.is_manual ? 'Modifier la recharge' : 'Coût de la recharge' }}
            </h3>
            <p v-if="editingCharge && !editingCharge.is_manual" class="text-[11px] text-slate-400 mt-1">
              Recharge TeslaMate du {{ formatDate(editingCharge.date) }} (+{{ editingCharge.kwh_added }} kWh). Le coût saisi ici ne sera pas écrasé par les synchronisations.
            </p>
          </div>
          <button @click="showChargeModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="charge-modal-form" @submit.prevent="handleSaveCharge" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <template v-if="!editingCharge || editingCharge.is_manual">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="charge-form-date" class="block text-xs font-semibold text-slate-300 mb-1">Date & Heure</label>
                <input id="charge-form-date" v-model="chargeForm.date" type="datetime-local" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white" />
              </div>
              <div>
                <label for="charge-form-kwh-added" class="block text-xs font-semibold text-slate-300 mb-1">Énergie ajoutée (kWh)</label>
                <input id="charge-form-kwh-added" v-model="chargeForm.kwh_added" type="number" step="0.001" min="0.001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="charge-form-address" class="block text-xs font-semibold text-slate-300 mb-1">Lieu (optionnel)</label>
                <input id="charge-form-address" v-model="chargeForm.address" placeholder="Borne, domicile..." class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
              <div>
                <label for="charge-form-odometer" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre (optionnel)</label>
                <input id="charge-form-odometer" v-model="chargeForm.odometer" type="number" min="0" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              </div>
            </div>
          </template>

          <div>
            <label for="charge-form-cost" class="block text-xs font-semibold text-slate-300 mb-1">Coût</label>
            <div class="flex gap-1.5">
              <input id="charge-form-cost" v-model="chargeForm.cost" type="number" step="0.01" min="0" required placeholder="0.00 si gratuite" class="w-full min-w-0 bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
              <label for="charge-form-currency" class="sr-only">Devise</label>
              <select id="charge-form-currency" v-model="chargeForm.currency" class="bg-slate-800 border border-slate-700 rounded-xl px-2 py-2 text-xs text-white">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
          <div v-if="chargeForm.currency !== 'EUR'">
            <label for="charge-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">Taux de conversion (1 {{ chargeForm.currency }} = ? €)</label>
            <input id="charge-form-fx-rate" v-model="chargeForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label for="charge-form-notes" class="block text-xs font-semibold text-slate-300 mb-1">Notes (optionnel)</label>
            <input id="charge-form-notes" v-model="chargeForm.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <!-- Justificatif / Facture -->
          <div class="space-y-2 bg-slate-800/40 p-3 rounded-xl border border-slate-700/60">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
                <Paperclip class="w-3.5 h-3.5 text-indigo-400" />
                Justificatif / Facture
              </span>
              <span v-if="chargeForm.document_id" class="text-[11px] text-emerald-400 font-medium">Lié</span>
            </div>

            <div v-if="chargeForm.document_id" class="flex items-center justify-between p-2.5 bg-slate-900 border border-indigo-500/30 rounded-xl">
              <div class="flex items-center gap-2 min-w-0">
                <FileText class="w-4 h-4 text-indigo-400 shrink-0" />
                <span class="text-xs text-white truncate font-medium">{{ chargeForm.document_filename || 'Facture liée' }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  @click="viewOrDownloadDocument(chargeForm.document_id, chargeForm.document_filename, false)"
                  class="p-1 text-slate-400 hover:text-indigo-400 rounded-lg hover:bg-slate-800"
                  title="Voir le document"
                >
                  <Eye class="w-3.5 h-3.5" />
                </button>
                <button
                  type="button"
                  @click="chargeForm.document_id = null; chargeForm.document_filename = null"
                  class="p-1 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
                  title="Détacher le justificatif"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div v-else class="space-y-2">
              <div class="flex flex-col sm:flex-row gap-2">
                <div class="flex-1" v-if="documents.length > 0">
                  <label for="charge-existing-doc" class="sr-only">Choisir une facture existante</label>
                  <select
                    id="charge-existing-doc"
                    class="w-full bg-slate-800 border border-slate-700 rounded-xl px-2.5 py-1.5 text-xs text-slate-300"
                    @change="(e: any) => onSelectExistingDoc(e.target.value, chargeForm)"
                  >
                    <option value="">-- Associer une facture existante --</option>
                    <option v-for="d in documents" :key="d.id" :value="d.id">
                      {{ d.filename }} ({{ formatDate(d.created_at) }})
                    </option>
                  </select>
                </div>
                <label class="cursor-pointer px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-medium flex items-center justify-center gap-1.5 transition-colors">
                  <UploadCloud class="w-3.5 h-3.5" />
                  <span>{{ isUploadingDocument ? 'Téléversement...' : 'Nouveau fichier' }}</span>
                  <input
                    type="file"
                    accept=".pdf,image/png,image/jpeg,image/webp"
                    class="hidden"
                    :disabled="isUploadingDocument"
                    @change="(e: any) => onFileInputChange(e, chargeForm)"
                  />
                </label>
              </div>
              <p class="text-[10px] text-slate-400">
                PDF ou image (reçu Superchargeur, borne publique, etc.).
              </p>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showChargeModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="charge-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Upload Standalone Document -->
    <div
      v-if="showUploadDocModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showUploadDocModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <UploadCloud class="w-5 h-5 text-indigo-400" />
            Ajouter un Justificatif ou une Facture
          </h3>
          <button @click="showUploadDocModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="standalone-doc-form" @submit.prevent="handleUploadStandaloneDocument" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <div>
            <label for="standalone-doc-file" class="block text-xs font-semibold text-slate-300 mb-1">Fichier (PDF, PNG, JPEG, WEBP, max 15 Mo)</label>
            <input
              id="standalone-doc-file"
              type="file"
              accept=".pdf,image/png,image/jpeg,image/webp"
              required
              @change="onUploadDocFileSelect"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white file:mr-3 file:py-1 file:px-2.5 file:rounded-lg file:border-0 file:text-xs file:font-semibold file:bg-indigo-600 file:text-white hover:file:bg-indigo-500 cursor-pointer"
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
          <button type="button" @click="showUploadDocModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
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

    <!-- Modal: Add / Edit Maintenance Reminder -->
    <div
      v-if="showReminderModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showReminderModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Bell class="w-5 h-5 text-violet-400" />
            {{ editingReminderId ? 'Modifier le rappel d\'entretien' : 'Nouveau rappel d\'entretien' }}
          </h3>
          <button @click="showReminderModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="reminder-modal-form" @submit.prevent="handleSaveReminder" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <!-- Preset chips (only when adding new) -->
          <div v-if="!editingReminderId" class="space-y-1.5">
            <span class="block text-xs font-semibold text-slate-300">Modèles rapides</span>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="preset in REMINDER_PRESETS"
                :key="preset.title"
                type="button"
                @click="applyReminderPreset(preset)"
                class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-medium rounded-lg border border-slate-700 transition-colors flex items-center gap-1"
              >
                <Sparkles class="w-3 h-3 text-violet-400" />
                {{ preset.title }}
              </button>
            </div>
          </div>

          <!-- Title & Category -->
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div class="sm:col-span-2">
              <label for="reminder-form-title" class="block text-xs font-semibold text-slate-300 mb-1">Titre de l'entretien</label>
              <input
                id="reminder-form-title"
                v-model="reminderForm.title"
                type="text"
                required
                placeholder="ex. Permutation des pneus"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <div>
              <label for="reminder-form-category" class="block text-xs font-semibold text-slate-300 mb-1">Catégorie</label>
              <select
                id="reminder-form-category"
                v-model="reminderForm.category"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              >
                <option value="MAINTENANCE">Entretien</option>
                <option value="TIRES">Pneumatiques</option>
              </select>
            </div>
          </div>

          <!-- Periodicities -->
          <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
            <span class="block text-xs font-semibold text-slate-200">Périodicité (au moins l'un des deux)</span>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="reminder-form-interval-km" class="block text-xs font-semibold text-slate-300 mb-1">Intervalle en km</label>
                <input
                  id="reminder-form-interval-km"
                  v-model="reminderForm.interval_km"
                  type="number"
                  min="500"
                  step="500"
                  placeholder="ex. 10000 (vide = ignoré)"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
              <div>
                <label for="reminder-form-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">Intervalle en mois</label>
                <input
                  id="reminder-form-interval-months"
                  v-model="reminderForm.interval_months"
                  type="number"
                  min="1"
                  max="120"
                  placeholder="ex. 12 (vide = ignoré)"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
            </div>
          </div>

          <!-- Last service date & odometer -->
          <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
            <span class="block text-xs font-semibold text-slate-200">Point de départ (dernier entretien)</span>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="reminder-form-last-odo" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre dernier entretien (km)</label>
                <input
                  id="reminder-form-last-odo"
                  v-model="reminderForm.last_service_odometer"
                  type="number"
                  min="0"
                  placeholder="ex. 45000"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
              <div>
                <label for="reminder-form-last-date" class="block text-xs font-semibold text-slate-300 mb-1">Date dernier entretien</label>
                <input
                  id="reminder-form-last-date"
                  v-model="reminderForm.last_service_date"
                  type="date"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
            </div>
          </div>

          <!-- Alert lead thresholds -->
          <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
            <span class="block text-xs font-semibold text-slate-200">Seuil d'anticipation de l'alerte</span>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label for="reminder-form-lead-km" class="block text-xs font-semibold text-slate-300 mb-1">Alerter avant (km)</label>
                <input
                  id="reminder-form-lead-km"
                  v-model.number="reminderForm.lead_km"
                  type="number"
                  min="0"
                  step="100"
                  placeholder="1000"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
              <div>
                <label for="reminder-form-lead-days" class="block text-xs font-semibold text-slate-300 mb-1">Alerter avant (jours)</label>
                <input
                  id="reminder-form-lead-days"
                  v-model.number="reminderForm.lead_days"
                  type="number"
                  min="0"
                  placeholder="15"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
            </div>
          </div>

          <!-- Webhook notification toggle -->
          <div class="flex items-center gap-2 pt-1">
            <input
              id="reminder-form-webhook-toggle"
              v-model="reminderForm.webhook_enabled"
              type="checkbox"
              class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
            />
            <label for="reminder-form-webhook-toggle" class="text-xs text-slate-300 cursor-pointer">
              Envoyer une notification webhook automatique (Discord / Telegram / Gotify) lors du franchissement du seuil
            </label>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showReminderModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="reminder-modal-form" class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl transition-colors">
            {{ editingReminderId ? 'Mettre à jour' : 'Créer le rappel' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Mark Reminder Complete -->
    <div
      v-if="showCompleteReminderModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showCompleteReminderModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <CheckCircle2 class="w-5 h-5 text-emerald-400" />
            Valider la réalisation : {{ completingReminder?.title }}
          </h3>
          <button @click="showCompleteReminderModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="complete-reminder-form" @submit.prevent="handleCompleteReminder" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <p class="text-xs text-slate-400">
            Valider cette intervention réinitialise le compteur d'intervalle et repart sur le nouvel odomètre et la date renseignés ci-dessous.
          </p>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="complete-form-date" class="block text-xs font-semibold text-slate-300 mb-1">Date d'intervention</label>
              <input
                id="complete-form-date"
                v-model="completeForm.service_date"
                type="date"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <div>
              <label for="complete-form-odo" class="block text-xs font-semibold text-slate-300 mb-1">Odomètre de l'intervention (km)</label>
              <input
                id="complete-form-odo"
                v-model="completeForm.service_odometer"
                type="number"
                min="0"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
          </div>

          <!-- Option to log an expense -->
          <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
            <div class="flex items-center gap-2">
              <input
                id="complete-form-log-expense"
                v-model="completeForm.log_expense"
                type="checkbox"
                class="rounded border-slate-700 bg-slate-800 text-emerald-600 focus:ring-emerald-500"
              />
              <label for="complete-form-log-expense" class="text-xs font-semibold text-slate-200 cursor-pointer">
                Enregistrer simultanément une dépense d'entretien financière
              </label>
            </div>

            <div v-if="completeForm.log_expense" class="space-y-3 pt-1">
              <div>
                <label for="complete-form-expense-amount" class="block text-xs font-semibold text-slate-300 mb-1">Coût de la facture (€)</label>
                <input
                  id="complete-form-expense-amount"
                  v-model="completeForm.expense_amount"
                  type="number"
                  step="0.01"
                  min="0"
                  placeholder="0.00 si gratuit"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
              <div>
                <label for="complete-form-expense-desc" class="block text-xs font-semibold text-slate-300 mb-1">Libellé de la dépense</label>
                <input
                  id="complete-form-expense-desc"
                  v-model="completeForm.expense_description"
                  placeholder="ex. Révision atelier / Permutation pneus"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
                />
              </div>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showCompleteReminderModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="complete-reminder-form" class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-xl transition-colors">
            Confirmer l'entretien
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Homelab Webhook Configuration -->
    <div
      v-if="showWebhookModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showWebhookModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Radio class="w-5 h-5 text-violet-400" />
            Notifications Webhook Homelab
          </h3>
          <button @click="showWebhookModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="webhook-modal-form" @submit.prevent="handleSaveWebhook" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <p class="text-xs text-slate-400">
            Configurez un webhook sortant vers votre serveur domotique ou de messagerie (Discord, Telegram, Gotify). Dès que TeslaMate remonte un kilométrage franchissant le seuil d'alerte, une notification est automatiquement expédiée.
          </p>

          <div>
            <label for="webhook-form-type" class="block text-xs font-semibold text-slate-300 mb-1">Plateforme / Connecteur</label>
            <select
              id="webhook-form-type"
              v-model="webhookForm.type"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
            >
              <option value="DISCORD">Discord (Webhook Embed)</option>
              <option value="TELEGRAM">Telegram Bot (API sendMessage)</option>
              <option value="GOTIFY">Gotify (Push notification)</option>
              <option value="GENERIC">Générique (JSON standard)</option>
            </select>
          </div>

          <div>
            <label for="webhook-form-url" class="block text-xs font-semibold text-slate-300 mb-1">URL cible du Webhook</label>
            <input
              id="webhook-form-url"
              v-model="webhookForm.url"
              type="url"
              required
              placeholder="https://discord.com/api/webhooks/... ou https://api.telegram.org/bot<token>/sendMessage?chat_id=<id>"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
            />
            <p class="text-[11px] text-slate-400 mt-1">
              Pour Telegram, l'URL doit contenir le token du bot et le chat_id en paramètre.
            </p>
          </div>

          <div class="flex items-center gap-2 pt-1">
            <input
              id="webhook-form-enabled"
              v-model="webhookForm.enabled"
              type="checkbox"
              class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
            />
            <label for="webhook-form-enabled" class="text-xs text-slate-300 cursor-pointer">
              Activer les notifications automatiques en tâche de fond
            </label>
          </div>

          <!-- Test result banner -->
          <div
            v-if="webhookTestResult"
            class="p-3 rounded-xl border text-xs flex items-center gap-2"
            :class="webhookTestResult.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300' : 'bg-rose-500/10 border-rose-500/30 text-rose-300'"
          >
            <CheckCircle2 v-if="webhookTestResult.success" class="w-4 h-4 shrink-0 text-emerald-400" />
            <AlertTriangle v-else class="w-4 h-4 shrink-0 text-rose-400" />
            <span>{{ webhookTestResult.message }}</span>
          </div>

          <!-- Test button -->
          <div class="pt-2">
            <button
              type="button"
              @click="handleTestWebhook"
              :disabled="isTestingWebhook || !webhookForm.url"
              class="w-full px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
            >
              <Loader2 v-if="isTestingWebhook" class="w-4 h-4 animate-spin text-violet-400" />
              <Radio v-else class="w-4 h-4 text-violet-400" />
              <span>{{ isTestingWebhook ? 'Envoi du test en cours...' : 'Envoyer un message de test maintenant' }}</span>
            </button>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div>
            <button
              v-if="vehicleWebhook"
              type="button"
              @click="handleDeleteWebhook"
              class="px-3 py-1.5 bg-rose-900/20 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl border border-rose-800/40 transition-colors"
            >
              Supprimer le webhook
            </button>
          </div>
          <div class="flex items-center gap-2">
            <button type="button" @click="showWebhookModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
              Annuler
            </button>
            <button
              type="submit"
              form="webhook-modal-form"
              :disabled="isSavingWebhook"
              class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl transition-colors disabled:opacity-50 flex items-center gap-1.5"
            >
              <Loader2 v-if="isSavingWebhook" class="w-3.5 h-3.5 animate-spin" />
              <span>Enregistrer</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Document In-App Preview -->
    <div
      v-if="previewDoc"
      class="fixed inset-0 z-[70] bg-black/80 backdrop-blur-sm flex items-center justify-center p-2 sm:p-4 overflow-hidden"
      @click.self="closeDocPreview"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-5xl h-[92vh] sm:h-[90vh] flex flex-col shadow-2xl overflow-hidden">
        <!-- Header -->
        <div class="px-4 sm:px-6 py-3.5 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95 gap-3">
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="w-8 h-8 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center shrink-0">
              <FileText v-if="previewDoc.isPdf" class="w-4 h-4 text-indigo-400" />
              <ImageIcon v-else-if="previewDoc.isImage" class="w-4 h-4 text-emerald-400" />
              <Paperclip v-else class="w-4 h-4 text-slate-400" />
            </div>
            <div class="min-w-0">
              <h3 class="text-sm font-bold text-white truncate" :title="previewDoc.filename">
                {{ previewDoc.filename }}
              </h3>
              <p class="text-[11px] text-slate-400 flex items-center gap-1.5">
                <span v-if="previewDoc.isPdf" class="text-indigo-400 font-semibold">Document PDF</span>
                <span v-else-if="previewDoc.isImage" class="text-emerald-400 font-semibold">Image</span>
                <span v-else class="text-slate-400 font-semibold">Fichier</span>
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <!-- Download Button -->
            <button
              @click="downloadFromPreview"
              class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors"
              title="Télécharger le fichier"
            >
              <Download class="w-3.5 h-3.5 text-indigo-400" />
              <span class="hidden sm:inline">Télécharger</span>
            </button>

            <!-- Open in New Tab Button -->
            <button
              @click="openInNewTab"
              class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors"
              title="Ouvrir dans un nouvel onglet"
            >
              <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
              <span class="hidden md:inline">Nouvel onglet</span>
            </button>

            <!-- Close Button -->
            <button
              @click="closeDocPreview"
              class="text-slate-400 hover:text-white p-1.5 rounded-xl hover:bg-slate-800 transition-colors ml-1"
              title="Fermer"
            >
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Preview Body -->
        <div class="flex-1 bg-slate-950/70 overflow-hidden flex items-center justify-center min-h-0 relative">
          <!-- PDF preview -->
          <iframe
            v-if="previewDoc.isPdf"
            :src="previewDoc.url"
            class="w-full h-full border-0 bg-white"
            :title="previewDoc.filename"
          />

          <!-- Image preview -->
          <div
            v-else-if="previewDoc.isImage"
            class="w-full h-full p-4 flex items-center justify-center overflow-auto"
          >
            <img
              :src="previewDoc.url"
              :alt="previewDoc.filename"
              class="max-w-full max-h-full object-contain rounded-lg shadow-lg"
            />
          </div>

          <!-- Unsupported preview fallback -->
          <div v-else class="p-8 text-center space-y-3">
            <FileText class="w-12 h-12 text-slate-500 mx-auto" />
            <p class="text-sm text-slate-300">Ce format de fichier ne supporte pas l'aperçu direct dans le navigateur.</p>
            <button
              @click="downloadFromPreview"
              class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold inline-flex items-center gap-2 transition-colors"
            >
              <Download class="w-4 h-4" />
              Télécharger pour consulter
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
