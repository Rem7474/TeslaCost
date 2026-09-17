<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import AppDatePicker from '@/components/AppDatePicker.vue'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import {
  Disc,
  Plus,
  RefreshCw,
  Ruler,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  X,
  Gauge,
  Calendar,
  Layers,
  Sun,
  Snowflake,
  CloudSun,
  History,
  ArrowUpDown,
  Shuffle,
  Trash2,
  Edit2,
  Package,
  Wrench,
  Check,
  ChevronRight,
  Zap,
  Pencil,
  Archive,
  CheckSquare,
  Square,
  Copy,
  ClipboardPaste,
} from 'lucide-vue-next'

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const tires = ref<any[]>([])
const loading = ref(false)

// Active tab: 'chassis' (Montés) or 'storage' (Au garage)
const activeTab = ref<'chassis' | 'storage' | 'disposed'>('chassis')
const chassisSubView = ref<'cards' | 'timeline'>('cards')

// Modals
const showAddTireModal = ref(false)
const showHistoryModal = ref(false)
const showSessionModal = ref(false)
const showBatchSessionModal = ref(false)
const showDuplicateSessionModal = ref(false)
const showCopyHistoryModal = ref(false)
const showBatchDisposeModal = ref(false)
const showLogModal = ref(false)
const showPackSwapModal = ref(false)

// Copy full history across tires
const copyHistorySourceTire = ref<any | null>(null)
const copyHistoryTargetTireIds = ref<string[]>([])
const copyHistoryOptions = ref({
  copy_sessions: true,
  copy_logs: true,
  adapt_position: true,
})
const copyingHistory = ref(false)

// Batch dispose
const batchDisposeForm = ref({
  date: new Date().toISOString().substring(0, 10),
  odometer: '' as number | string,
})
const disposingBatch = ref(false)

// Selected tire for history / session
const selectedTire = ref<any | null>(null)
const selectedTireStats = ref<any | null>(null)
const tireSessions = ref<any[]>([])
const tireLogs = ref<any[]>([])

// Batch past session for garage tires (Feature A)
const batchSessionTireIds = ref<string[]>([])
const savingBatchSession = ref(false)
const batchSessionForm = ref({
  mounted_date: new Date().toISOString().substring(0, 10),
  mounted_odometer: 0,
  dismounted_date: new Date().toISOString().substring(0, 10),
  dismounted_odometer: 0,
  distance_km: 0,
  notes: '',
  position: 'STORAGE',
})

// Copy-paste / duplicate session across tires (Feature C)
const copiedSession = ref<any | null>(null)
const sessionToDuplicate = ref<any | null>(null)
const duplicateTargetTireIds = ref<string[]>([])
const duplicatingSession = ref(false)

// Form: Add Tires (Batch / Single)
const addType = ref<'SET_4' | 'SET_4_STORAGE' | 'SET_2_FRONT' | 'SET_2_REAR' | 'SET_2_STORAGE' | 'SINGLE'>('SET_4')
const dimensionPreset = ref('235/40 R19 96W')
const isCustomDimension = ref(false)
const isTotalPrice = ref(true)

const addTireForm = ref({
  brand: 'Michelin',
  model: 'Pilot Sport EV',
  dimension: '235/40 R19 96W',
  season: 'SUMMER',
  purchase_date: new Date().toISOString().split('T')[0],
  total_price: 880,
  unit_price: 220,
  initial_depth_mm: 8.0,
  min_legal_depth_mm: 1.6,
  dot_code: '',
  mounted_odometer: 0,
  accumulated_distance_km: 0,
  estimated_lifespan_km: 45000,
  current_position: 'FL',
})

// Tesla predefined tire dimensions
const teslaDimensionPresets = [
  { group: 'Tesla Model 3', label: '18" Aero — 235/45 R18 98Y', value: '235/45 R18 98Y' },
  { group: 'Tesla Model 3', label: '19" Sport — 235/40 R19 96W', value: '235/40 R19 96W' },
  { group: 'Tesla Model 3', label: '20" Performance — 245/35 R20 95Y', value: '245/35 R20 95Y' },
  { group: 'Tesla Model Y', label: '19" Gemini — 255/45 R19 104W', value: '255/45 R19 104W' },
  { group: 'Tesla Model Y', label: '20" Induction — 255/40 R20 101W', value: '255/40 R20 101W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Av — 255/35 R21 98W', value: '255/35 R21 98W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Ar — 275/35 R21 103W', value: '275/35 R21 103W' },
  { group: 'Tesla Model S', label: '19" Tempest — 255/45 R19 104Y', value: '255/45 R19 104Y' },
  { group: 'Tesla Model S', label: '21" Arachnid Av — 265/35 R21', value: '265/35 R21' },
  { group: 'Tesla Model S', label: '21" Arachnid Ar — 295/30 R21', value: '295/30 R21' },
  { group: 'Tesla Model X', label: '20" Cyberstream — 265/45 R20 / 275/45 R20', value: '265/45 R20' },
  { group: 'Autre', label: 'Dimension personnalisée...', value: 'CUSTOM' },
]

function onDimensionPresetChange() {
  if (dimensionPreset.value === 'CUSTOM') {
    isCustomDimension.value = true
    addTireForm.value.dimension = ''
  } else {
    isCustomDimension.value = false
    addTireForm.value.dimension = dimensionPreset.value
  }
}

// Form: Mount Session (Add / Edit)
const editingSessionId = ref<string | null>(null)
const sessionForm = ref({
  position: 'FL',
  mounted_date: new Date().toISOString().substring(0, 10),
  mounted_odometer: 0,
  is_dismounted: true,
  dismounted_date: new Date().toISOString().substring(0, 10),
  dismounted_odometer: 0,
  distance_km: 0,
  notes: '',
})

// Form: Tread Depth Log
const newLogForm = ref({
  depth_mm: 6.5,
  odometer: 0,
  notes: '',
  date: new Date().toISOString().substring(0, 10),
})
const editingLogId = ref<string | null>(null)

// Selection for batch edition
const selectedTireIds = ref<string[]>([])
function toggleTireSelection(id: string) {
  selectedTireIds.value = selectedTireIds.value.includes(id)
    ? selectedTireIds.value.filter((x) => x !== id)
    : [...selectedTireIds.value, id]
}
function selectMountedTires() {
  selectedTireIds.value = ['FL', 'FR', 'RL', 'RR'].map((pos) => mountedTires.value[pos]?.tire.id).filter(Boolean)
}

const currentTabTireIds = computed<string[]>(() => {
  if (activeTab.value === 'chassis') {
    return ['FL', 'FR', 'RL', 'RR'].map((pos) => mountedTires.value[pos]?.tire.id).filter(Boolean)
  }
  if (activeTab.value === 'storage') {
    return storageTires.value.map((t) => t.tire.id)
  }
  if (activeTab.value === 'disposed') {
    return disposedTires.value.map((t) => t.tire.id)
  }
  return []
})

const isCurrentTabAllSelected = computed<boolean>(() => {
  const ids = currentTabTireIds.value
  return ids.length > 0 && ids.every((id) => selectedTireIds.value.includes(id))
})

function toggleSelectAllCurrentTab() {
  const ids = currentTabTireIds.value
  if (!ids.length) return
  if (isCurrentTabAllSelected.value) {
    selectedTireIds.value = selectedTireIds.value.filter((id) => !ids.includes(id))
  } else {
    selectedTireIds.value = Array.from(new Set([...selectedTireIds.value, ...ids]))
  }
}

// Edit (single or batch): empty fields are left unchanged
const showTireEditModal = ref(false)
const tireEditIds = ref<string[]>([])
const tireEditPriceMode = ref<'UNIT' | 'TOTAL'>('UNIT')
const tireEditForm = ref(emptyTireEdit())

function emptyTireEdit() {
  return {
    brand: '',
    model: '',
    dimension: '',
    season: '',
    purchase_date: '',
    price: '' as number | string,
    dot_code: '',
    initial_depth_mm: '' as number | string,
    min_legal_depth_mm: '' as number | string,
    initial_distance_km: '' as number | string,
    estimated_lifespan_km: '' as number | string,
    mounted_date: '',
    mounted_odometer: '' as number | string,
  }
}

const editedTires = computed(() => tires.value.filter((t) => tireEditIds.value.includes(t.tire.id)))
const editIncludesMounted = computed(() => editedTires.value.some((t) => ['FL', 'FR', 'RL', 'RR'].includes(t.tire.current_position)))

function openTireEdit(ids: string[]) {
  tireEditIds.value = [...ids]
  tireEditPriceMode.value = ids.length > 1 ? 'TOTAL' : 'UNIT'
  const form = emptyTireEdit()
  if (ids.length === 1) {
    const stats = tires.value.find((t) => t.tire.id === ids[0]) || selectedTireStats.value
    const t = stats?.tire || selectedTire.value
    const activeSession = (stats?.sessions || []).find((ss: any) => !ss.dismounted_date)
    Object.assign(form, {
      brand: t.brand,
      model: t.model,
      dimension: t.dimension,
      season: t.season,
      purchase_date: t.purchase_date ? new Date(t.purchase_date).toISOString().substring(0, 10) : '',
      price: t.purchase_price,
      dot_code: t.dot_code || '',
      initial_depth_mm: t.initial_depth_mm,
      min_legal_depth_mm: t.min_legal_depth_mm,
      initial_distance_km: t.initial_distance_km,
      estimated_lifespan_km: t.estimated_lifespan_km,
      mounted_date: activeSession ? new Date(activeSession.mounted_date).toISOString().substring(0, 10) : '',
      mounted_odometer: activeSession ? activeSession.mounted_odometer : '',
    })
  }
  tireEditForm.value = form
  showTireEditModal.value = true
}

async function handleSaveTireEdit() {
  if (!vehicleStore.activeVehicle || !tireEditIds.value.length) return
  const f = tireEditForm.value
  const text = (v: string) => (v.trim() ? v.trim() : undefined)
  const num = (v: number | string) => (v === '' || v === null ? undefined : Number(v))
  const payload: any = {
    tire_ids: tireEditIds.value,
    brand: text(f.brand),
    model: text(f.model),
    dimension: text(f.dimension),
    season: f.season || undefined,
    purchase_date: f.purchase_date || undefined,
    dot_code: text(f.dot_code),
    initial_depth_mm: num(f.initial_depth_mm),
    min_legal_depth_mm: num(f.min_legal_depth_mm),
    initial_distance_km: num(f.initial_distance_km),
    estimated_lifespan_km: num(f.estimated_lifespan_km),
    mounted_date: f.mounted_date ? new Date(f.mounted_date).toISOString() : undefined,
    mounted_odometer: num(f.mounted_odometer),
  }
  if (num(f.price) !== undefined) {
    payload[tireEditPriceMode.value === 'TOTAL' ? 'total_price' : 'purchase_price'] = num(f.price)
  }
  try {
    await api.batchUpdateTires(vehicleStore.activeVehicle.id, payload)
    showTireEditModal.value = false
    selectedTireIds.value = []
    await loadTires()
    if (showHistoryModal.value && selectedTire.value) await openHistoryModal({ tire: selectedTire.value })
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function getLastDismountInfo(tireId: string) {
  const tireEntry = tires.value.find((t) => t.tire.id === tireId)
  const sessions = tireEntry?.sessions || []
  const dismountedSessions = sessions
    .filter((s: any) => s.dismounted_date)
    .sort((a: any, b: any) => new Date(b.dismounted_date).getTime() - new Date(a.dismounted_date).getTime())
  if (dismountedSessions.length > 0) {
    const s = dismountedSessions[0]
    return {
      date: new Date(s.dismounted_date).toISOString().substring(0, 10),
      odometer: s.dismounted_odometer || null,
    }
  }
  return null
}

// Dispose (worn out, damaged, sold) keeps history and cost; delete removes an erroneous entry
const showDisposeModal = ref(false)
const disposeForm = ref({ date: new Date().toISOString().substring(0, 10), odometer: 0 as number | string })

function openDisposeModal(tireToDispose?: any) {
  if (tireToDispose) {
    selectedTire.value = tireToDispose.tire || tireToDispose
  }
  const t = selectedTire.value
  const isMounted = t && ['FL', 'FR', 'RL', 'RR'].includes(t.current_position)
  const lastDismount = t ? getLastDismountInfo(t.id) : null

  disposeForm.value = {
    date: (!isMounted && lastDismount?.date) ? lastDismount.date : new Date().toISOString().substring(0, 10),
    odometer: isMounted ? Math.round(vehicleStore.activeVehicle?.current_odometer || 0) : (lastDismount?.odometer || ''),
  }
  showDisposeModal.value = true
}

async function handleDisposeTire() {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  try {
    await api.disposeTire(vehicleStore.activeVehicle.id, selectedTire.value.id, {
      date: new Date(disposeForm.value.date).toISOString(),
      odometer: disposeForm.value.odometer === '' ? null : Number(disposeForm.value.odometer),
    })
    showDisposeModal.value = false
    showHistoryModal.value = false
    selectedTireIds.value = selectedTireIds.value.filter((id) => id !== selectedTire.value.id)
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openBatchDisposeModal() {
  if (!selectedTireIds.value.length) return
  const selectedEntries = tires.value.filter((t) => selectedTireIds.value.includes(t.tire.id))
  const anyMounted = selectedEntries.some((t) => ['FL', 'FR', 'RL', 'RR'].includes(t.tire.current_position))

  let defaultDate = new Date().toISOString().substring(0, 10)
  if (!anyMounted) {
    let latestTimestamp = 0
    for (const entry of selectedEntries) {
      const info = getLastDismountInfo(entry.tire.id)
      if (info?.date) {
        const ts = new Date(info.date).getTime()
        if (ts > latestTimestamp) {
          latestTimestamp = ts
          defaultDate = info.date
        }
      }
    }
  }

  batchDisposeForm.value = {
    date: defaultDate,
    odometer: anyMounted ? Math.round(vehicleStore.activeVehicle?.current_odometer || 0) : '',
  }
  showBatchDisposeModal.value = true
}

async function handleBatchDisposeSubmit() {
  if (!vehicleStore.activeVehicle || !selectedTireIds.value.length) return
  disposingBatch.value = true
  try {
    await api.batchDisposeTires(vehicleStore.activeVehicle.id, {
      tire_ids: selectedTireIds.value,
      date: new Date(batchDisposeForm.value.date).toISOString(),
      odometer: batchDisposeForm.value.odometer !== '' ? Number(batchDisposeForm.value.odometer) : null,
    })
    showBatchDisposeModal.value = false
    showAlert(`${selectedTireIds.value.length} pneu(s) mis au rebut avec succès.`, 'Mise au rebut', 'success')
    selectedTireIds.value = []
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur lors de la mise au rebut : ${err.message}`, 'Erreur', 'danger')
  } finally {
    disposingBatch.value = false
  }
}

function updateCopyHistoryTargets() {
  if (!copyHistorySourceTire.value) return
  const otherTires = tires.value.filter((t) => t.tire.id !== copyHistorySourceTire.value.id)
  const sameFamily = otherTires.filter(
    (t) => t.tire.brand === copyHistorySourceTire.value.brand && t.tire.model === copyHistorySourceTire.value.model
  )
  copyHistoryTargetTireIds.value = sameFamily.length > 0 ? sameFamily.map((t) => t.tire.id) : otherTires.map((t) => t.tire.id)
}

function onSourceTireChange(sourceId: string) {
  const found = tires.value.find((t) => t.tire.id === sourceId)
  if (found) {
    copyHistorySourceTire.value = found.tire
    updateCopyHistoryTargets()
  }
}

function openCopyHistoryModal(sourceTire?: any) {
  const resolved = sourceTire?.tire || sourceTire
  if (resolved) {
    copyHistorySourceTire.value = resolved
  } else if (selectedTireIds.value.length === 1) {
    const s = tires.value.find((t) => t.tire.id === selectedTireIds.value[0])
    copyHistorySourceTire.value = s?.tire || tires.value[0]?.tire || null
  } else {
    copyHistorySourceTire.value = storageTires.value[0]?.tire || tires.value[0]?.tire || null
  }
  if (!copyHistorySourceTire.value) return

  updateCopyHistoryTargets()
  copyHistoryOptions.value = {
    copy_sessions: true,
    copy_logs: true,
    adapt_position: true,
  }
  showCopyHistoryModal.value = true
}

function toggleCopyHistoryTargetTire(id: string) {
  if (copyHistoryTargetTireIds.value.includes(id)) {
    copyHistoryTargetTireIds.value = copyHistoryTargetTireIds.value.filter((x) => x !== id)
  } else {
    copyHistoryTargetTireIds.value.push(id)
  }
}

async function handleCopyHistorySubmit() {
  if (!vehicleStore.activeVehicle || !copyHistorySourceTire.value || !copyHistoryTargetTireIds.value.length) return
  copyingHistory.value = true
  try {
    await api.copyTireHistory(vehicleStore.activeVehicle.id, copyHistorySourceTire.value.id, {
      target_tire_ids: copyHistoryTargetTireIds.value,
      copy_sessions: copyHistoryOptions.value.copy_sessions,
      copy_logs: copyHistoryOptions.value.copy_logs,
      adapt_position: copyHistoryOptions.value.adapt_position,
    })
    showCopyHistoryModal.value = false
    showAlert(`Historique copié avec succès vers ${copyHistoryTargetTireIds.value.length} pneu(s).`, 'Succès', 'success')
    await loadTires()
    if (showHistoryModal.value && selectedTire.value) {
      await openHistoryModal({ tire: selectedTire.value })
    }
  } catch (err: any) {
    showAlert(`Erreur lors de la copie d'historique : ${err.message}`, 'Erreur', 'danger')
  } finally {
    copyingHistory.value = false
  }
}

const wheelTimelines = computed(() => {
  const positions: Array<'FL' | 'FR' | 'RL' | 'RR'> = ['FL', 'FR', 'RL', 'RR']
  const result: Record<string, any[]> = { FL: [], FR: [], RL: [], RR: [] }

  for (const pos of positions) {
    const list: any[] = []
    for (const t of tires.value) {
      const sessions = t.sessions || []
      for (const s of sessions) {
        if (s.position === pos) {
          list.push({
            session: s,
            tire: t.tire,
            stats: t,
            isCurrent: !s.dismounted_date && t.tire.current_position === pos,
          })
        }
      }
    }
    list.sort((a, b) => new Date(b.session.mounted_date).getTime() - new Date(a.session.mounted_date).getTime())
    result[pos] = list
  }
  return result
})

const selectedDisposedCount = computed(() => {
  return tires.value.filter((t) => selectedTireIds.value.includes(t.tire.id) && t.tire.current_position === 'DISPOSED').length
})
const canBatchDispose = computed(() => {
  return selectedTireIds.value.length > 0 && selectedTireIds.value.length > selectedDisposedCount.value
})

async function handleDeleteTire(t: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer définitivement le pneu',
    message: `Supprimer définitivement le pneu ${t.brand} ${t.model} (${t.dimension}) avec son historique et son coût ? Pour un pneu usé, crevé ou vendu, préférez « Mettre au rebut » qui conserve son coût dans le TCO.`,
    confirmText: 'Supprimer définitivement',
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTire(vehicleStore.activeVehicle.id, t.id)
    showHistoryModal.value = false
    selectedTireIds.value = selectedTireIds.value.filter((id) => id !== t.id)
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteLog(l: any) {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  const ok = await showConfirm({
    title: 'Supprimer le relevé de gomme',
    message: `Supprimer le relevé de ${l.depth_mm} mm du ${formatDate(l.date)} ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, l.id)
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// Form: Seasonal Swap Pack
const packSwapForm = ref({
  odometer: 0,
  tires: {
    FL: '',
    FR: '',
    RL: '',
    RR: '',
  },
})

// Mounted tires mapped by position
const mountedTires = computed(() => {
  const map: Record<string, any> = { FL: null, FR: null, RL: null, RR: null }
  tires.value.forEach((t) => {
    const pos = t.tire.current_position
    if (pos in map) {
      map[pos] = t
    }
  })
  return map
})

const storageTires = computed(() => {
  return tires.value.filter((t) => t.tire.current_position === 'STORAGE')
})

const disposedTires = computed(() => tires.value.filter((t) => t.tire.current_position === 'DISPOSED'))

async function loadTires() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    tires.value = await api.getTires(vehicleStore.activeVehicle.id)
    if (vehicleStore.activeVehicle?.current_odometer) {
      const odo = Math.round(vehicleStore.activeVehicle.current_odometer)
      addTireForm.value.mounted_odometer = odo
      newLogForm.value.odometer = odo
      packSwapForm.value.odometer = odo
    }
  } catch (err) {
    console.error('Failed to load tires', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value],
  () => {
    selectedTireIds.value = []
  }
)

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => {
    loadTires()
  }
)

onMounted(() => {
  loadTires()
})

// Quick rotations
async function handleQuickRotate(mode: 'FRONT_BACK' | 'CROSS') {
  if (!vehicleStore.activeVehicle) return
  const odo = Math.round(vehicleStore.activeVehicle.current_odometer || 0)
  const label = mode === 'FRONT_BACK' ? 'Avant ⇄ Arrière (FL ⇄ RL, FR ⇄ RR)' : 'Croisée (FL ⇄ RR, FR ⇄ RL)'
  const ok = await showConfirm({
    title: 'Permutation rapide',
    message: `Confirmez-vous la permutation rapide ${label} à ${odo.toLocaleString('fr-FR')} km ?`,
    confirmText: 'Permuter',
    type: 'warning',
  })
  if (!ok) return

  try {
    await api.quickRotateTires(vehicleStore.activeVehicle.id, {
      mode,
      odometer: odo,
    })
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur lors de la permutation : ${err.message}`, 'Erreur', 'danger')
  }
}

// Open pack swap modal
function openPackSwapModal() {
  if (!vehicleStore.activeVehicle) return
  packSwapForm.value.odometer = Math.round(vehicleStore.activeVehicle.current_odometer || 0)
  // Pre-fill with first 4 storage tires if available
  const st = storageTires.value
  packSwapForm.value.tires.FL = st[0]?.tire.id || ''
  packSwapForm.value.tires.FR = st[1]?.tire.id || ''
  packSwapForm.value.tires.RL = st[2]?.tire.id || ''
  packSwapForm.value.tires.RR = st[3]?.tire.id || ''
  showPackSwapModal.value = true
}

async function handlePackSwapSubmit() {
  if (!vehicleStore.activeVehicle) return
  const selectedIDs = [
    packSwapForm.value.tires.FL,
    packSwapForm.value.tires.FR,
    packSwapForm.value.tires.RL,
    packSwapForm.value.tires.RR,
  ].filter(Boolean)

  if (selectedIDs.length !== 4) {
    showAlert('Veuillez sélectionner 4 pneus distincts du garage pour remplacer les pneus montés.', 'Sélection requise', 'warning')
    return
  }

  try {
    await api.quickRotateTires(vehicleStore.activeVehicle.id, {
      mode: 'SWAP_PACK',
      odometer: packSwapForm.value.odometer,
      swap_with_pack_tire_ids: selectedIDs,
    })
    showPackSwapModal.value = false
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// Open Add modal
function openAddModal() {
  if (vehicleStore.activeVehicle?.current_odometer) {
    addTireForm.value.mounted_odometer = Math.round(vehicleStore.activeVehicle.current_odometer)
  }
  showAddTireModal.value = true
}

async function handleCreateTires() {
  if (!vehicleStore.activeVehicle) return
  if (!addTireForm.value.brand || !addTireForm.value.model || !addTireForm.value.dimension) {
    showAlert('Veuillez renseigner la marque, le modèle et la dimension', 'Champs requis', 'warning')
    return
  }

  try {
    if (addType.value === 'SINGLE') {
      await api.createTire(vehicleStore.activeVehicle.id, {
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        purchase_price: addTireForm.value.unit_price,
        current_position: addTireForm.value.current_position,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addTireForm.value.current_position !== 'STORAGE' ? addTireForm.value.mounted_odometer : null,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    } else {
      await api.batchCreateTires(vehicleStore.activeVehicle.id, {
        type: addType.value,
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        total_price: isTotalPrice.value ? addTireForm.value.total_price : 0,
        unit_price: !isTotalPrice.value ? addTireForm.value.unit_price : 0,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addType.value.includes('STORAGE') ? null : addTireForm.value.mounted_odometer,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    }

    showAddTireModal.value = false
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

// History & Timeline Modal
async function openHistoryModal(t: any) {
  selectedTire.value = t.tire
  selectedTireStats.value = t
  try {
    const res = await api.getTireHistory(vehicleStore.activeVehicle!.id, t.tire.id)
    selectedTire.value = res.tire
    selectedTireStats.value = res.stats
    tireSessions.value = res.sessions || []
    tireLogs.value = res.logs || []
    showHistoryModal.value = true
  } catch (err: any) {
    showAlert(`Erreur de chargement : ${err.message}`, 'Erreur', 'danger')
  }
}

// Open manual session modal (add / edit)
function openAddSessionModal() {
  editingSessionId.value = null
  sessionForm.value = {
    position: 'FL',
    mounted_date: new Date().toISOString().substring(0, 10),
    mounted_odometer: 0,
    is_dismounted: true,
    dismounted_date: new Date().toISOString().substring(0, 10),
    dismounted_odometer: 0,
    distance_km: 0,
    notes: '',
  }
  showSessionModal.value = true
}

function openEditSessionModal(s: any) {
  editingSessionId.value = s.id
  sessionForm.value = {
    position: s.position,
    mounted_date: new Date(s.mounted_date).toISOString().substring(0, 10),
    mounted_odometer: s.mounted_odometer,
    is_dismounted: !!s.dismounted_date,
    dismounted_date: s.dismounted_date ? new Date(s.dismounted_date).toISOString().substring(0, 10) : new Date().toISOString().substring(0, 10),
    dismounted_odometer: s.dismounted_odometer || 0,
    distance_km: s.distance_km || 0,
    notes: s.notes || '',
  }
  showSessionModal.value = true
}

async function handleSaveSession() {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return

  try {
    const payload: any = {
      position: sessionForm.value.position,
      mounted_date: new Date(sessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(sessionForm.value.mounted_odometer),
      distance_km: Number(sessionForm.value.distance_km),
      notes: sessionForm.value.notes ? sessionForm.value.notes : null,
    }

    if (sessionForm.value.is_dismounted) {
      payload.dismounted_date = new Date(sessionForm.value.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(sessionForm.value.dismounted_odometer)
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    if (editingSessionId.value) {
      await api.updateTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, editingSessionId.value, payload)
    } else {
      await api.createTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, payload)
    }

    showSessionModal.value = false
    // Refresh history
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteSession(session: any) {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  const ok = await showConfirm({
    title: 'Supprimer la session de montage',
    message: 'Confirmez-vous la suppression de cette session de montage ?',
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, session.id)
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    showAlert(`Erreur lors de la suppression : ${err.message}`, 'Erreur', 'danger')
  }
}

// Batch session for storage tires (Feature A)
function openBatchSessionModal() {
  if (storageTires.value.length === 0) return
  const storageIds = storageTires.value.map((t) => t.tire.id)
  const selectedStorage = selectedTireIds.value.filter((id) => storageIds.includes(id))
  batchSessionTireIds.value = selectedStorage.length > 0 ? [...selectedStorage] : [...storageIds]

  const curOdo = Math.round(vehicleStore.activeVehicle?.current_odometer || 0)
  batchSessionForm.value = {
    mounted_date: new Date().toISOString().substring(0, 10),
    mounted_odometer: curOdo,
    dismounted_date: new Date().toISOString().substring(0, 10),
    dismounted_odometer: curOdo,
    distance_km: 0,
    notes: '',
    position: 'STORAGE',
  }
  showBatchSessionModal.value = true
}

function toggleBatchSessionTire(id: string) {
  if (batchSessionTireIds.value.includes(id)) {
    batchSessionTireIds.value = batchSessionTireIds.value.filter((x) => x !== id)
  } else {
    batchSessionTireIds.value.push(id)
  }
}

function selectAllBatchSessionTires() {
  batchSessionTireIds.value = storageTires.value.map((t) => t.tire.id)
}

function deselectAllBatchSessionTires() {
  batchSessionTireIds.value = []
}

function onBatchOdometerChange() {
  const mount = Number(batchSessionForm.value.mounted_odometer) || 0
  const dismount = Number(batchSessionForm.value.dismounted_odometer) || 0
  if (dismount > mount) {
    batchSessionForm.value.distance_km = dismount - mount
  }
}

function onSessionOdometerChange() {
  const mount = Number(sessionForm.value.mounted_odometer) || 0
  const dismount = Number(sessionForm.value.dismounted_odometer) || 0
  if (dismount > mount) {
    sessionForm.value.distance_km = dismount - mount
  }
}

async function handleSaveBatchSession() {
  if (!vehicleStore.activeVehicle || batchSessionTireIds.value.length === 0) return
  if (!batchSessionForm.value.mounted_date || !batchSessionForm.value.dismounted_date) {
    showAlert('Veuillez renseigner les dates de montage et de démontage.', 'Dates requises', 'warning')
    return
  }

  savingBatchSession.value = true
  try {
    const payload: any = {
      position: 'STORAGE',
      mounted_date: new Date(batchSessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(batchSessionForm.value.mounted_odometer) || 0,
      dismounted_date: new Date(batchSessionForm.value.dismounted_date).toISOString(),
      dismounted_odometer: Number(batchSessionForm.value.dismounted_odometer) || 0,
      distance_km: Number(batchSessionForm.value.distance_km) || 0,
      notes: batchSessionForm.value.notes ? batchSessionForm.value.notes : null,
    }

    if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
      payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
    }

    for (const tireId of batchSessionTireIds.value) {
      await api.createTireSession(vehicleStore.activeVehicle.id, tireId, payload)
    }

    showBatchSessionModal.value = false
    await loadTires()
    showAlert(`Session enregistrée avec succès pour ${batchSessionTireIds.value.length} pneu(s).`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors de l'enregistrement du lot : ${err.message}`, 'Erreur', 'danger')
  } finally {
    savingBatchSession.value = false
  }
}

// Copy-paste / duplicate session across tires (Feature C)
function copySession(s: any) {
  copiedSession.value = {
    position: s.position,
    mounted_date: s.mounted_date ? new Date(s.mounted_date).toISOString().substring(0, 10) : '',
    mounted_odometer: s.mounted_odometer || 0,
    is_dismounted: !!s.dismounted_date,
    dismounted_date: s.dismounted_date ? new Date(s.dismounted_date).toISOString().substring(0, 10) : '',
    dismounted_odometer: s.dismounted_odometer || 0,
    distance_km: s.distance_km || 0,
    notes: s.notes || '',
  }
}

function pasteSessionToCurrentTire() {
  if (!copiedSession.value) return
  editingSessionId.value = null
  sessionForm.value = {
    ...copiedSession.value,
    position: (selectedTire.value && selectedTire.value.current_position !== 'STORAGE' && selectedTire.value.current_position !== 'DISPOSED')
      ? selectedTire.value.current_position
      : (copiedSession.value.position || 'FL'),
  }
  showSessionModal.value = true
}

function applyCopiedSessionToForm() {
  if (!copiedSession.value) return
  sessionForm.value = {
    ...sessionForm.value,
    ...copiedSession.value,
    position: sessionForm.value.position || copiedSession.value.position,
  }
}

function openDuplicateSessionModal(s: any) {
  sessionToDuplicate.value = s
  const otherTires = tires.value.filter((t) => t.tire.id !== selectedTire.value?.id)
  const sameFamily = otherTires.filter(
    (t) => t.tire.brand === selectedTire.value?.brand && t.tire.model === selectedTire.value?.model
  )
  duplicateTargetTireIds.value = sameFamily.length > 0 ? sameFamily.map((t) => t.tire.id) : otherTires.map((t) => t.tire.id)
  showDuplicateSessionModal.value = true
}

function toggleDuplicateTargetTire(id: string) {
  if (duplicateTargetTireIds.value.includes(id)) {
    duplicateTargetTireIds.value = duplicateTargetTireIds.value.filter((x) => x !== id)
  } else {
    duplicateTargetTireIds.value.push(id)
  }
}

async function handleDuplicateSessionSubmit() {
  if (!vehicleStore.activeVehicle || !sessionToDuplicate.value || duplicateTargetTireIds.value.length === 0) return

  duplicatingSession.value = true
  try {
    const s = sessionToDuplicate.value
    const payload: any = {
      position: s.position || 'STORAGE',
      mounted_date: new Date(s.mounted_date).toISOString(),
      mounted_odometer: Number(s.mounted_odometer) || 0,
      distance_km: Number(s.distance_km) || 0,
      notes: s.notes || null,
    }
    if (s.dismounted_date) {
      payload.dismounted_date = new Date(s.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(s.dismounted_odometer) || 0
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    for (const targetId of duplicateTargetTireIds.value) {
      await api.createTireSession(vehicleStore.activeVehicle.id, targetId, payload)
    }

    showDuplicateSessionModal.value = false
    await loadTires()
    showAlert(`Session dupliquée vers ${duplicateTargetTireIds.value.length} pneu(s).`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors de la duplication : ${err.message}`, 'Erreur', 'danger')
  } finally {
    duplicatingSession.value = false
  }
}

// Open log modal (mesure de gomme)
function openLogModal(t: any, log?: any) {
  selectedTire.value = t.tire
  editingLogId.value = log ? log.id : null
  newLogForm.value = log
    ? { depth_mm: log.depth_mm, odometer: Math.round(log.odometer), notes: log.notes || '', date: new Date(log.date).toISOString().substring(0, 10) }
    : {
        depth_mm: t.current_depth_mm || 6.5,
        odometer: Math.round(vehicleStore.activeVehicle?.current_odometer || 0),
        notes: '',
        date: new Date().toISOString().substring(0, 10),
      }
  showLogModal.value = true
}

async function handleAddLog() {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  try {
    const payload = {
      depth_mm: Number(newLogForm.value.depth_mm),
      odometer: Number(newLogForm.value.odometer),
      notes: newLogForm.value.notes ? newLogForm.value.notes : null,
      date: new Date(newLogForm.value.date).toISOString(),
    }
    if (editingLogId.value) {
      await api.updateTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, editingLogId.value, payload)
    } else {
      await api.addTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, payload)
    }
    showLogModal.value = false
    await loadTires()
    if (showHistoryModal.value) {
      await openHistoryModal({ tire: selectedTire.value })
    }
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function getConditionBadge(condition: string) {
  switch (condition) {
    case 'GOOD':
      return { label: 'Bon état', class: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' }
    case 'WARNING':
      return { label: 'À surveiller', class: 'bg-amber-500/10 text-amber-400 border-amber-500/20' }
    case 'CRITICAL':
      return { label: 'Usure critique', class: 'bg-rose-500/10 text-rose-400 border-rose-500/20' }
    default:
      return { label: 'Inconnu', class: 'bg-slate-800 text-slate-400 border-slate-700' }
  }
}

function getSeasonIcon(season: string) {
  switch (season) {
    case 'WINTER':
      return { icon: Snowflake, color: 'text-sky-400', label: 'Hiver' }
    case 'ALL_SEASON':
      return { icon: CloudSun, color: 'text-amber-400', label: '4 Saisons' }
    default:
      return { icon: Sun, color: 'text-orange-400', label: 'Été' }
  }
}

function formatDate(d: string) {
  return new Date(d).toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Actions -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white flex items-center gap-2.5">
          <Disc class="w-6 h-6 text-rose-500" />
          Pneumatiques & Cycles de vie
        </h2>
        <p class="text-sm text-slate-400">
          Suivi de l'usure en mm, durée de vie estimée, permutations en 1 clic et historique complet des montages
        </p>
      </div>

      <!-- Action Buttons -->
      <div v-if="vehicleStore.canEdit" class="flex items-center gap-2 flex-wrap">
        <button
          @click="openPackSwapModal()"
          :disabled="storageTires.length === 0"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3.5 py-2.5 rounded-xl flex items-center gap-2 transition-colors disabled:opacity-40"
          title="Permuter le pack complet monté avec un pack de réserve (ex: Hiver / Été)"
        >
          <Snowflake class="w-4 h-4 text-sky-400" />
          <span class="hidden md:inline">Changer de pack</span>
        </button>

        <button
          @click="openAddModal()"
          class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all"
        >
          <Plus class="w-4 h-4" />
          Ajouter des pneus
        </button>
      </div>
    </div>

    <!-- Viewer mode banner -->
    <div
      v-if="!vehicleStore.canEdit"
      class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex items-center gap-3 text-xs text-slate-400"
    >
      <Disc class="w-4 h-4 text-slate-400 shrink-0" />
      <span>Vous consultez ce véhicule en mode <strong>Lecteur seul</strong>. Les modifications de pneumatiques, permutations et relevés sont désactivés.</span>
    </div>

    <!-- Quick Permutations Bar -->
    <div v-if="vehicleStore.canEdit" class="bg-slate-900 border border-slate-800 p-3 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
      <div class="flex items-center gap-2 text-slate-300 font-semibold">
        <RefreshCw class="w-4 h-4 text-rose-400" />
        <span>Permutations rapides du véhicule en 1 clic :</span>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button
          @click="handleQuickRotate('FRONT_BACK')"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
        >
          <ArrowUpDown class="w-3.5 h-3.5 text-blue-400" />
          Avant ⇄ Arrière
        </button>
        <button
          @click="handleQuickRotate('CROSS')"
          class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
        >
          <Shuffle class="w-3.5 h-3.5 text-indigo-400" />
          Permutation Croisée
        </button>
      </div>
    </div>

    <!-- Sticky Bulk Selection Bar -->
    <BulkSelectionBar
      v-if="vehicleStore.canEdit"
      :count="selectedTireIds.length"
      item-label="pneu"
      @clear="selectedTireIds = []"
    >
      <button
        type="button"
        @click="openTireEdit(selectedTireIds)"
        class="px-2.5 py-1 bg-rose-600 hover:bg-rose-500 text-white font-semibold rounded-lg flex items-center gap-1 transition-colors text-xs"
      >
        <Pencil class="w-3 h-3" />
        <span>Modifier par lot</span>
      </button>

      <button
        v-if="canBatchDispose"
        type="button"
        @click="openBatchDisposeModal()"
        class="px-2.5 py-1 bg-amber-600 hover:bg-amber-500 text-white font-semibold rounded-lg flex items-center gap-1 transition-colors text-xs"
        title="Mettre au rebut les pneus sélectionnés"
      >
        <Archive class="w-3 h-3" />
        <span>Mettre au rebut</span>
      </button>
    </BulkSelectionBar>

    <!-- Header row: Select all toggle & Total info -->
    <div v-if="vehicleStore.canEdit" class="flex items-center justify-between text-xs text-slate-400 px-2">
      <button
        type="button"
        @click="toggleSelectAllCurrentTab"
        class="flex items-center gap-2 hover:text-slate-200 transition-colors"
      >
        <component :is="isCurrentTabAllSelected ? CheckSquare : Square" class="w-4 h-4 text-rose-400" />
        <span>{{ isCurrentTabAllSelected ? 'Tout désélectionner' : 'Tout sélectionner' }}</span>
      </button>
      <span>{{ currentTabTireIds.length }} pneu(s) dans cette vue</span>
    </div>

    <!-- View Switcher Tabs -->
    <div class="flex items-center gap-2 border-b border-slate-800 pb-2">
      <button
        @click="activeTab = 'chassis'"
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
        :class="
          activeTab === 'chassis'
            ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40'
        "
      >
        <Disc class="w-4 h-4" />
        Pneus montés sur la Tesla (4)
      </button>

      <button
        @click="activeTab = 'storage'"
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
        :class="
          activeTab === 'storage'
            ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
            : 'text-slate-400 hover:text-white hover:bg-slate-800/40'
        "
      >
        <Package class="w-4 h-4" />
        Catalogue & Stock au garage ({{ storageTires.length }})
      </button>

      <button
        v-if="disposedTires.length"
        @click="activeTab = 'disposed'"
        class="px-4 py-2 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
        :class="activeTab === 'disposed' ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white hover:bg-slate-800/40'"
      >
        <Archive class="w-4 h-4" />
        Mis au rebut ({{ disposedTires.length }})
      </button>
    </div>

    <!-- TAB 1: CHASSIS INTERACTIF (PNEUS MONTÉS) -->
    <div v-if="activeTab === 'chassis'" class="space-y-6">
      <!-- Sub-view switcher: Cards vs Wheel Timeline -->
      <div class="flex items-center justify-between flex-wrap gap-2">
        <div class="inline-flex p-1 bg-slate-900 border border-slate-800 rounded-xl text-xs font-semibold">
          <button
            type="button"
            @click="chassisSubView = 'cards'"
            class="px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-all"
            :class="chassisSubView === 'cards' ? 'bg-rose-500/20 text-rose-300 font-bold border border-rose-500/30' : 'text-slate-400 hover:text-white'"
          >
            <Disc class="w-3.5 h-3.5" />
            <span>Vue 4 roues</span>
          </button>
          <button
            type="button"
            @click="chassisSubView = 'timeline'"
            class="px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-all"
            :class="chassisSubView === 'timeline' ? 'bg-rose-500/20 text-rose-300 font-bold border border-rose-500/30' : 'text-slate-400 hover:text-white'"
          >
            <History class="w-3.5 h-3.5" />
            <span>Timeline par roue</span>
          </button>
        </div>
      </div>

      <!-- Mode A: Cartes des 4 roues -->
      <div v-if="chassisSubView === 'cards'" class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- Wheel Card: FL (Avant Gauche) -->
        <div
          v-if="mountedTires.FL"
          @click="openHistoryModal(mountedTires.FL)"
          class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-3xl p-5 space-y-4 shadow-sm transition-all group"
          :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(mountedTires.FL.tire.id) }"
        >
          <div class="flex items-start justify-between">
            <div>
              <label :for="'chassis-select-fl-' + mountedTires.FL.tire.id" @click.stop class="flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-rose-400 hover:text-rose-300 cursor-pointer" :title="selectedTireIds.includes(mountedTires.FL.tire.id) ? 'Retirer de la sélection' : 'Sélectionner pour une action par lot'">
                <input
                  :id="'chassis-select-fl-' + mountedTires.FL.tire.id"
                  type="checkbox"
                  :checked="selectedTireIds.includes(mountedTires.FL.tire.id)"
                  @change="toggleTireSelection(mountedTires.FL.tire.id)"
                  class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                />
                <span>Avant Gauche (FL)</span>
              </label>
              <h3 class="text-base font-bold text-white group-hover:text-rose-300 transition-colors mt-0.5">{{ mountedTires.FL.tire.brand }} {{ mountedTires.FL.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.FL.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.FL.condition).class"
            >
              {{ getConditionBadge(mountedTires.FL.condition).label }}
            </span>
          </div>

          <!-- Metrics Row -->
          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.FL.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.FL.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>

          <!-- Lifespan progress bar -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.FL.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.FL.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.FL.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.FL.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Avant Gauche (FL)</span>
        </div>

        <!-- Wheel Card: FR (Avant Droit) -->
        <div
          v-if="mountedTires.FR"
          @click="openHistoryModal(mountedTires.FR)"
          class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-3xl p-5 space-y-4 shadow-sm transition-all group"
          :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(mountedTires.FR.tire.id) }"
        >
          <div class="flex items-start justify-between">
            <div>
              <label :for="'chassis-select-fr-' + mountedTires.FR.tire.id" @click.stop class="flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-rose-400 hover:text-rose-300 cursor-pointer" :title="selectedTireIds.includes(mountedTires.FR.tire.id) ? 'Retirer de la sélection' : 'Sélectionner pour une action par lot'">
                <input
                  :id="'chassis-select-fr-' + mountedTires.FR.tire.id"
                  type="checkbox"
                  :checked="selectedTireIds.includes(mountedTires.FR.tire.id)"
                  @change="toggleTireSelection(mountedTires.FR.tire.id)"
                  class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                />
                <span>Avant Droit (FR)</span>
              </label>
              <h3 class="text-base font-bold text-white group-hover:text-rose-300 transition-colors mt-0.5">{{ mountedTires.FR.tire.brand }} {{ mountedTires.FR.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.FR.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.FR.condition).class"
            >
              {{ getConditionBadge(mountedTires.FR.condition).label }}
            </span>
          </div>

          <!-- Metrics Row -->
          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.FR.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.FR.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.FR.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.FR.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.FR.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.FR.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Avant Droit (FR)</span>
        </div>

        <!-- Wheel Card: RL (Arrière Gauche) -->
        <div
          v-if="mountedTires.RL"
          @click="openHistoryModal(mountedTires.RL)"
          class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-3xl p-5 space-y-4 shadow-sm transition-all group"
          :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(mountedTires.RL.tire.id) }"
        >
          <div class="flex items-start justify-between">
            <div>
              <label :for="'chassis-select-rl-' + mountedTires.RL.tire.id" @click.stop class="flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-rose-400 hover:text-rose-300 cursor-pointer" :title="selectedTireIds.includes(mountedTires.RL.tire.id) ? 'Retirer de la sélection' : 'Sélectionner pour une action par lot'">
                <input
                  :id="'chassis-select-rl-' + mountedTires.RL.tire.id"
                  type="checkbox"
                  :checked="selectedTireIds.includes(mountedTires.RL.tire.id)"
                  @change="toggleTireSelection(mountedTires.RL.tire.id)"
                  class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                />
                <span>Arrière Gauche (RL)</span>
              </label>
              <h3 class="text-base font-bold text-white group-hover:text-rose-300 transition-colors mt-0.5">{{ mountedTires.RL.tire.brand }} {{ mountedTires.RL.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.RL.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.RL.condition).class"
            >
              {{ getConditionBadge(mountedTires.RL.condition).label }}
            </span>
          </div>

          <!-- Metrics Row -->
          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.RL.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.RL.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.RL.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.RL.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.RL.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.RL.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Arrière Gauche (RL)</span>
        </div>

        <!-- Wheel Card: RR (Arrière Droit) -->
        <div
          v-if="mountedTires.RR"
          @click="openHistoryModal(mountedTires.RR)"
          class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-3xl p-5 space-y-4 shadow-sm transition-all group"
          :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(mountedTires.RR.tire.id) }"
        >
          <div class="flex items-start justify-between">
            <div>
              <label :for="'chassis-select-rr-' + mountedTires.RR.tire.id" @click.stop class="flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-rose-400 hover:text-rose-300 cursor-pointer" :title="selectedTireIds.includes(mountedTires.RR.tire.id) ? 'Retirer de la sélection' : 'Sélectionner pour une action par lot'">
                <input
                  :id="'chassis-select-rr-' + mountedTires.RR.tire.id"
                  type="checkbox"
                  :checked="selectedTireIds.includes(mountedTires.RR.tire.id)"
                  @change="toggleTireSelection(mountedTires.RR.tire.id)"
                  class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                />
                <span>Arrière Droit (RR)</span>
              </label>
              <h3 class="text-base font-bold text-white group-hover:text-rose-300 transition-colors mt-0.5">{{ mountedTires.RR.tire.brand }} {{ mountedTires.RR.tire.model }}</h3>
              <div class="text-xs text-slate-400 font-mono">{{ mountedTires.RR.tire.dimension }}</div>
            </div>
            <span
              class="px-2.5 py-1 rounded-full text-[11px] font-semibold border"
              :class="getConditionBadge(mountedTires.RR.condition).class"
            >
              {{ getConditionBadge(mountedTires.RR.condition).label }}
            </span>
          </div>

          <!-- Metrics Row -->
          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-2xl border border-slate-800/80 text-center">
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Total parcouru</div>
              <div class="text-sm font-bold text-slate-200">{{ Math.round(mountedTires.RR.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500 uppercase">Coût / km</div>
              <div class="text-sm font-bold text-amber-400">{{ Number(mountedTires.RR.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>Usure durée de vie estimée ({{ mountedTires.RR.estimated_lifespan_km.toLocaleString('fr-FR') }} km)</span>
              <span class="font-bold text-slate-200">{{ mountedTires.RR.life_progress_pct }}%</span>
            </div>
            <div class="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="mountedTires.RR.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, mountedTires.RR.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-3xl p-8 text-center text-slate-500 flex flex-col items-center justify-center space-y-2">
          <Disc class="w-8 h-8 opacity-30" />
          <span>Aucun pneu monté à l'Arrière Droit (RR)</span>
        </div>
      </div>

      <!-- Mode B: Timeline par roue avec historique de chaque pneu -->
      <div v-else-if="chassisSubView === 'timeline'" class="space-y-6">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div
            v-for="pos in (['FL', 'FR', 'RL', 'RR'] as const)"
            :key="pos"
            class="bg-slate-900 border border-slate-800 rounded-3xl p-5 space-y-4 shadow-sm"
          >
            <!-- Wheel Header -->
            <div class="flex items-center justify-between pb-3 border-b border-slate-800">
              <div class="flex items-center gap-2.5">
                <div class="w-8 h-8 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center font-black text-xs text-rose-400">
                  {{ pos }}
                </div>
                <div>
                  <h3 class="text-sm font-bold text-white">
                    {{ pos === 'FL' ? 'Avant Gauche' : pos === 'FR' ? 'Avant Droit' : pos === 'RL' ? 'Arrière Gauche' : 'Arrière Droit' }} ({{ pos }})
                  </h3>
                  <div class="text-[11px] text-slate-400">
                    {{ wheelTimelines[pos]?.length || 0 }} cycle(s) de montage enregistré(s)
                  </div>
                </div>
              </div>
              <span v-if="mountedTires[pos]" class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                Pneu actif : {{ mountedTires[pos].tire.brand }}
              </span>
              <span v-else class="text-[10px] text-slate-500">
                Roue vide
              </span>
            </div>

            <!-- Timeline nodes -->
            <div v-if="!wheelTimelines[pos] || wheelTimelines[pos].length === 0" class="p-8 text-center text-xs text-slate-500 bg-slate-950/40 rounded-2xl">
              Aucun historique pour cet emplacement de roue.
            </div>
            <div v-else class="space-y-3 max-h-[32rem] overflow-y-auto pr-1">
              <div
                v-for="(item, idx) in wheelTimelines[pos]"
                :key="item.session.id || idx"
                class="relative pl-6 pb-2 border-l border-slate-800 last:border-l-0"
              >
                <!-- Dot -->
                <div
                  class="absolute -left-1.5 top-1 w-3 h-3 rounded-full border-2"
                  :class="item.isCurrent ? 'bg-emerald-500 border-slate-900 ring-2 ring-emerald-500/40' : item.tire.current_position === 'DISPOSED' ? 'bg-amber-500 border-slate-900' : 'bg-slate-600 border-slate-900'"
                ></div>

                <div
                  class="bg-slate-950/60 border border-slate-800/80 rounded-2xl p-3 space-y-2 hover:border-slate-700 transition-colors"
                  :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(item.tire.id) }"
                >
                  <div class="flex items-start justify-between gap-2">
                    <div>
                      <div class="flex items-center gap-1.5 text-xs font-bold text-white">
                        <component :is="getSeasonIcon(item.tire.season).icon" class="w-3.5 h-3.5" :class="getSeasonIcon(item.tire.season).color" />
                        <span>{{ item.tire.brand }} {{ item.tire.model }}</span>
                      </div>
                      <div class="text-[10px] text-slate-400 font-mono">{{ item.tire.dimension }}</div>
                    </div>
                    <span
                      class="text-[10px] font-bold px-2 py-0.5 rounded-full border shrink-0"
                      :class="item.isCurrent ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : item.tire.current_position === 'DISPOSED' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
                    >
                      {{ item.isCurrent ? '🟢 En cours' : item.tire.current_position === 'DISPOSED' ? 'Au rebut' : 'Démonté' }}
                    </span>
                  </div>

                  <div class="grid grid-cols-2 gap-2 text-[11px] text-slate-400 bg-slate-900/60 p-2 rounded-xl">
                    <div>
                      <span class="text-slate-500 block text-[10px]">Montage</span>
                      <span class="text-slate-200 font-medium">{{ formatDate(item.session.mounted_date) }}</span>
                      <span class="text-slate-400 text-[10px] block">à {{ Math.round(item.session.mounted_odometer).toLocaleString('fr-FR') }} km</span>
                    </div>
                    <div>
                      <span class="text-slate-500 block text-[10px]">Démontage</span>
                      <span class="text-slate-200 font-medium">{{ item.session.dismounted_date ? formatDate(item.session.dismounted_date) : 'Actuel' }}</span>
                      <span class="text-slate-400 text-[10px] block">
                        {{ item.session.dismounted_odometer ? `à ${Math.round(item.session.dismounted_odometer).toLocaleString('fr-FR')} km` : 'En service' }}
                      </span>
                    </div>
                  </div>

                  <div class="flex items-center justify-between text-xs pt-1">
                    <span class="font-bold text-rose-400 text-[11px]">+{{ Math.round(item.session.distance_km || 0).toLocaleString('fr-FR') }} km sur cette roue</span>
                    <button
                      type="button"
                      @click="openHistoryModal(item.stats || { tire: item.tire })"
                      class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1 font-semibold transition-colors"
                    >
                      <span>Fiche & historique</span>
                      <ChevronRight class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- TAB 2: CATALOGUE & STOCK AU GARAGE -->
    <div v-if="activeTab === 'storage'" class="space-y-4">
      <div v-if="storageTires.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center text-slate-400 space-y-3">
        <Package class="w-10 h-10 mx-auto text-slate-600" />
        <h3 class="text-base font-bold text-white">Aucun pneu stocké au garage</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          Vous pouvez enregistrer vos packs de pneus hiver ou de rechange pour suivre précisément leur kilométrage même démontés.
        </p>
      </div>

      <div v-else class="space-y-4">
        <!-- Garage batch actions bar -->
        <div class="flex items-center justify-between flex-wrap gap-2 bg-slate-900/60 border border-slate-800 p-3 rounded-2xl">
          <div class="flex items-center gap-2">
            <Package class="w-4 h-4 text-slate-400" />
            <span class="text-xs text-slate-300 font-semibold">{{ storageTires.length }} pneu(s) stocké(s) au garage</span>
          </div>
          <div class="flex items-center gap-2 flex-wrap">
            <button
              v-if="vehicleStore.canEdit"
              @click="openCopyHistoryModal()"
              class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3 py-1.5 rounded-xl flex items-center gap-1.5 transition-colors"
              title="Copier tout l'historique d'un pneu vers d'autres pneus"
            >
              <Copy class="w-3.5 h-3.5 text-indigo-400" />
              <span>Copier l'historique d'un pneu</span>
            </button>
            <button
              v-if="vehicleStore.canEdit"
              @click="openBatchSessionModal()"
              class="bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold px-3 py-1.5 rounded-xl flex items-center gap-1.5 transition-colors"
              title="Enregistrer une session passée sur un lot de pneus du garage"
            >
              <History class="w-3.5 h-3.5 text-rose-400" />
              <span>Ajouter une session passée sur un lot</span>
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="t in storageTires"
          :key="t.tire.id"
          @click="openHistoryModal(t)"
          class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-2xl p-4 space-y-3 shadow-sm transition-all group"
          :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(t.tire.id) }"
        >
          <div class="flex items-start justify-between">
            <div>
              <div class="flex items-center gap-1.5 text-xs font-semibold">
                <component :is="getSeasonIcon(t.tire.season).icon" class="w-3.5 h-3.5" :class="getSeasonIcon(t.tire.season).color" />
                <span class="text-slate-300">{{ getSeasonIcon(t.tire.season).label }}</span>
              </div>
              <h4 class="text-sm font-bold text-white group-hover:text-rose-300 transition-colors mt-1 flex items-center gap-1.5">
                <label :for="'storage-select-' + t.tire.id" @click.stop class="cursor-pointer flex items-center" title="Sélectionner pour une action par lot">
                  <input
                    :id="'storage-select-' + t.tire.id"
                    type="checkbox"
                    :checked="selectedTireIds.includes(t.tire.id)"
                    @change="toggleTireSelection(t.tire.id)"
                    class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                  />
                </label>
                {{ t.tire.brand }} {{ t.tire.model }}
              </h4>
              <div class="text-[11px] text-slate-400 font-mono">{{ t.tire.dimension }}</div>
            </div>
            <span class="bg-slate-800 text-slate-400 text-[10px] px-2 py-0.5 rounded-full border border-slate-700 font-medium">
              Au garage
            </span>
          </div>

          <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80 text-center text-xs">
            <div>
              <div class="text-[10px] text-slate-500">Total parcouru</div>
              <div class="font-bold text-white">{{ Math.round(t.total_distance_km).toLocaleString('fr-FR') }} km</div>
            </div>
            <div>
              <div class="text-[10px] text-slate-500">Usure estimée</div>
              <div class="font-bold" :class="t.life_progress_pct > 80 ? 'text-rose-400' : 'text-emerald-400'">{{ t.life_progress_pct }}%</div>
            </div>
          </div>

          <div class="space-y-1">
            <div class="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="t.life_progress_pct > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
                :style="{ width: `${Math.min(100, t.life_progress_pct)}%` }"
              ></div>
            </div>
          </div>
        </div>
      </div>
      </div>
    </div>

    <!-- MODAL: AJOUTER DES PNEUS (BATCH / SINGLE) -->
    <div
      v-if="showAddTireModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showAddTireModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Plus class="w-5 h-5 text-rose-500" />
            Ajouter des pneus
          </h3>
          <button @click="showAddTireModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

        <!-- Add Type Selection -->
        <div class="space-y-1.5">
          <span class="block text-xs font-semibold text-slate-300">Format d'enregistrement :</span>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
            <button
              type="button"
              @click="addType = 'SET_4'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_4' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Train complet monté (4 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SET_4_STORAGE'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_4_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Pack au garage (4 pneus hiver/été)
            </button>
            <button
              type="button"
              @click="addType = 'SET_2_FRONT'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_2_FRONT' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Essieu avant monté (2 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SET_2_REAR'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_2_REAR' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Essieu arrière monté (2 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SET_2_STORAGE'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SET_2_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Paire au garage (2 pneus)
            </button>
            <button
              type="button"
              @click="addType = 'SINGLE'"
              class="p-2.5 rounded-xl border text-left transition-all"
              :class="addType === 'SINGLE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
            >
              Pneu isolé (achat unique)
            </button>
          </div>
        </div>

        <!-- Position (single tire only) -->
        <div v-if="addType === 'SINGLE'" class="space-y-1">
          <label for="tire-add-tire-position" class="block text-xs font-semibold text-slate-400">Position :</label>
          <select id="tire-add-tire-position"
            v-model="addTireForm.current_position"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          >
            <option value="FL">Avant Gauche (FL)</option>
            <option value="FR">Avant Droit (FR)</option>
            <option value="RL">Arrière Gauche (RL)</option>
            <option value="RR">Arrière Droit (RR)</option>
            <option value="STORAGE">Stock au garage (non monté)</option>
          </select>
        </div>

        <!-- Dimension Dropdown -->
        <div class="space-y-1">
          <label for="tire-dimension-preset" class="block text-xs font-semibold text-slate-400">Dimension homologuée :</label>
          <select id="tire-dimension-preset"
            v-model="dimensionPreset"
            @change="onDimensionPresetChange"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
          >
            <option v-for="p in teslaDimensionPresets" :key="p.value" :value="p.value">
              {{ p.label }}
            </option>
          </select>
          <div v-if="isCustomDimension" class="pt-1.5">
            <label for="tire-add-tire-dimension" class="sr-only">Ex: 245/40 R19 98Y</label>
            <input id="tire-add-tire-dimension"
              v-model="addTireForm.dimension"
              type="text"
              placeholder="Ex: 245/40 R19 98Y"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
            />
          </div>
        </div>

        <!-- Brand, Model, Season -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label for="tire-add-tire-brand" class="block text-xs font-semibold text-slate-400 mb-1">Marque</label>
            <input id="tire-add-tire-brand"
              v-model="addTireForm.brand"
              type="text"
              placeholder="Michelin, Pirelli, Hankook..."
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label for="tire-add-tire-model" class="block text-xs font-semibold text-slate-400 mb-1">Modèle</label>
            <input id="tire-add-tire-model"
              v-model="addTireForm.model"
              type="text"
              placeholder="Pilot Sport EV, Winter Sottozero..."
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label for="tire-add-tire-season" class="block text-xs font-semibold text-slate-400 mb-1">Saison</label>
            <select id="tire-add-tire-season"
              v-model="addTireForm.season"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            >
              <option value="SUMMER">☀️ Été</option>
              <option value="WINTER">❄️ Hiver</option>
              <option value="ALL_SEASON">🌦️ 4 Saisons</option>
            </select>
          </div>
        </div>

        <!-- Pricing & Lifespan -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-950/60 p-3.5 rounded-2xl border border-slate-800">
          <div>
            <div class="flex items-center justify-between mb-1">
              <label for="tire-add-tire-total-price" class="text-xs font-semibold text-slate-400">
                {{ isTotalPrice ? 'Prix total du lot (€)' : 'Prix par pneu (€)' }}
              </label>
              <button
                type="button"
                @click="isTotalPrice = !isTotalPrice"
                class="text-[10px] text-rose-400 hover:text-rose-300 underline"
              >
                Passer en {{ isTotalPrice ? 'prix unitaire' : 'prix total' }}
              </button>
            </div>
            <input id="tire-add-tire-total-price"
              v-if="isTotalPrice"
              v-model.number="addTireForm.total_price"
              type="number"
              step="10"
              class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
            <input id="tire-add-tire-total-price"
              v-else
              v-model.number="addTireForm.unit_price"
              type="number"
              step="5"
              class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>

          <div>
            <label for="tire-add-tire-estimated-lifespan-km" class="block text-xs font-semibold text-slate-400 mb-1">Durée de vie estimée (km)</label>
            <input id="tire-add-tire-estimated-lifespan-km"
              v-model.number="addTireForm.estimated_lifespan_km"
              type="number"
              step="5000"
              class="w-full bg-slate-900 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
        </div>

        <!-- Km already driven (second-hand) -->
        <div>
          <label for="tire-add-tire-accumulated-distance-km" class="block text-xs font-semibold text-slate-400 mb-1">Km déjà parcourus (si occasion)</label>
          <input id="tire-add-tire-accumulated-distance-km"
            v-model.number="addTireForm.accumulated_distance_km"
            type="number"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>

        <!-- Date & Sculptures -->
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label for="tire-add-tire-purchase-date" class="block text-[11px] text-slate-400 mb-1">Date d'achat</label>
            <AppDatePicker
              id="tire-add-tire-purchase-date"
              v-model="addTireForm.purchase_date"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-add-tire-initial-depth-mm" class="block text-[11px] text-slate-400 mb-1">Gomme neuve (mm)</label>
            <input id="tire-add-tire-initial-depth-mm"
              v-model.number="addTireForm.initial_depth_mm"
              type="number"
              step="0.1"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
          <div>
            <label for="tire-add-tire-min-legal-depth-mm" class="block text-[11px] text-slate-400 mb-1">Témoin légal (mm)</label>
            <input id="tire-add-tire-min-legal-depth-mm"
              v-model.number="addTireForm.min_legal_depth_mm"
              type="number"
              step="0.1"
              class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
            />
          </div>
        </div>

        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-3 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showAddTireModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 rounded-xl text-xs font-semibold text-slate-300 transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleCreateTires"
            class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2 rounded-xl shadow-lg shadow-rose-600/20 transition-colors"
          >
            Créer les pneus
          </button>
        </div>
      </div>
    </div>

    <!-- TAB 3: PNEUS MIS AU REBUT -->
    <div v-if="activeTab === 'disposed'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="t in disposedTires"
        :key="t.tire.id"
        @click="openHistoryModal(t)"
        class="bg-slate-900/60 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-2xl p-4 space-y-2 transition-all group"
        :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selectedTireIds.includes(t.tire.id) }"
      >
        <div class="flex items-start justify-between">
          <div>
            <h4 class="text-sm font-bold text-slate-300 group-hover:text-rose-300 transition-colors flex items-center gap-1.5">
              <label :for="'disposed-select-' + t.tire.id" @click.stop class="cursor-pointer flex items-center" title="Sélectionner pour une action par lot">
                <input
                  :id="'disposed-select-' + t.tire.id"
                  type="checkbox"
                  :checked="selectedTireIds.includes(t.tire.id)"
                  @change="toggleTireSelection(t.tire.id)"
                  class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
                />
              </label>
              {{ t.tire.brand }} {{ t.tire.model }}
            </h4>
            <div class="text-[11px] text-slate-500 font-mono">{{ t.tire.dimension }}</div>
          </div>
          <span class="bg-slate-800 text-slate-400 text-[10px] px-2 py-0.5 rounded-full border border-slate-700 font-medium">
            Au rebut
          </span>
        </div>
        <div class="text-xs text-slate-400">
          {{ Math.round(t.total_distance_km).toLocaleString('fr-FR') }} km parcourus • Usure : {{ t.life_progress_pct }}% • {{ t.tire.purchase_price }} €
        </div>
      </div>
    </div>

    <!-- MODAL: HISTORIQUE COMPLET & TIMELINE D'UN PNEU -->
    <div
      v-if="showHistoryModal && selectedTire"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showHistoryModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <!-- Header -->
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-start justify-between shrink-0 bg-slate-900/95">
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-base font-bold text-white">{{ selectedTire.brand }} {{ selectedTire.model }}</h3>
              <span class="text-xs font-mono bg-slate-800 px-2 py-0.5 rounded text-slate-300">
                {{ selectedTire.dimension }}
              </span>
            </div>
            <div class="text-xs text-slate-400 mt-0.5">
              Acheté le {{ formatDate(selectedTire.purchase_date) }} • {{ selectedTire.purchase_price }} €
            </div>
          </div>
          <div class="flex items-center gap-1.5">
            <button @click="openTireEdit([selectedTire.id])" class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white rounded-lg transition-colors" title="Modifier le pneu">
              <Pencil class="w-4 h-4" />
            </button>
            <button
              v-if="selectedTire.current_position !== 'DISPOSED'"
              @click="openDisposeModal"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-amber-400 rounded-lg transition-colors"
              title="Mettre au rebut (usé, crevé, vendu)"
            >
              <Archive class="w-4 h-4" />
            </button>
            <button @click="handleDeleteTire(selectedTire)" class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-lg transition-colors" title="Supprimer (saisie erronée)">
              <Trash2 class="w-4 h-4" />
            </button>
            <button @click="showHistoryModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Body -->
        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-6">

        <!-- Life KPI Card -->
        <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-2xl space-y-3">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">Kilométrage total de vie :</span>
            <span class="text-base font-bold text-white">
              {{ Math.round(selectedTireStats?.total_distance_km || 0).toLocaleString('fr-FR') }} km
              <span class="text-xs text-slate-400 font-normal">/ {{ (selectedTire.estimated_lifespan_km || 45000).toLocaleString('fr-FR') }} km estimés</span>
            </span>
          </div>

          <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all"
              :class="(selectedTireStats?.life_progress_pct || 0) > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
              :style="{ width: `${Math.min(100, selectedTireStats?.life_progress_pct || 0)}%` }"
            ></div>
          </div>

          <div class="grid grid-cols-3 gap-2 text-center text-xs pt-1">
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Sculpture actuelle</div>
              <div class="font-bold text-emerald-400">{{ selectedTireStats?.current_depth_mm }} mm</div>
            </div>
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Durée de vie consommée</div>
              <div class="font-bold text-slate-200">{{ selectedTireStats?.life_progress_pct }}%</div>
            </div>
            <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500">Coût réel / km</div>
              <div class="font-bold text-amber-400">{{ Number(selectedTireStats?.cost_per_km).toFixed(4) }} €</div>
            </div>
          </div>
        </div>

        <!-- TeslaMate Driving Telemetry & Stress Analysis Card -->
        <div v-if="selectedTireStats?.driving_stress_index > 0" class="bg-gradient-to-br from-slate-950 to-slate-900 border border-slate-800 p-4 rounded-2xl space-y-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Zap class="w-4 h-4 text-amber-400" />
              <h4 class="text-xs font-bold text-white uppercase tracking-wider">Télémétrie Dynamique TeslaMate</h4>
            </div>
            <span
              class="px-2.5 py-0.5 rounded-full text-xs font-bold border"
              :class="
                selectedTireStats.driving_style === 'SPORT'
                  ? 'bg-rose-500/15 text-rose-400 border-rose-500/30'
                  : selectedTireStats.driving_style === 'ECO'
                  ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                  : 'bg-sky-500/15 text-sky-400 border-sky-500/30'
              "
            >
              {{ selectedTireStats.driving_style === 'SPORT' ? 'Contrainte Sportive' : selectedTireStats.driving_style === 'ECO' ? 'Éco-conduite' : 'Conduite Équilibrée' }} (Indice : x{{ selectedTireStats.driving_stress_index }})
            </span>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-center text-xs">
            <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500 uppercase">Pointe Accélération</div>
              <div class="font-bold text-rose-400 text-sm mt-0.5">+{{ selectedTireStats.avg_power_max_kw }} kW</div>
            </div>
            <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500 uppercase">Pointe Régénération</div>
              <div class="font-bold text-emerald-400 text-sm mt-0.5">{{ selectedTireStats.avg_power_min_kw }} kW</div>
            </div>
            <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500 uppercase">Conso moyenne</div>
              <div class="font-bold text-sky-400 text-sm mt-0.5">{{ selectedTireStats.avg_consumption_kwh_100km }} kWh</div>
            </div>
            <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
              <div class="text-[10px] text-slate-500 uppercase">Longévité ajustée</div>
              <div class="font-bold text-indigo-300 text-sm mt-0.5">~{{ (selectedTireStats.dynamic_lifespan_km || selectedTire.estimated_lifespan_km).toLocaleString('fr-FR') }} km</div>
            </div>
          </div>

          <p v-if="selectedTireStats.wear_explanation" class="text-xs text-slate-300 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/80">
            {{ selectedTireStats.wear_explanation }}
          </p>
        </div>

        <!-- Timeline: Mount/Dismount Sessions -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
              <History class="w-4 h-4 text-rose-500" />
              Historique des montages, démontages & permutations
            </h4>
            <div class="flex items-center gap-2">
              <button
                v-if="copiedSession"
                @click="pasteSessionToCurrentTire()"
                class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1 transition-colors bg-indigo-950/40 border border-indigo-800/60 px-2 py-1 rounded-lg"
                :title="'Coller la session copiée (' + (copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '') + ')'"
              >
                <ClipboardPaste class="w-3.5 h-3.5" />
                <span>Coller</span>
              </button>
              <button
                @click="openAddSessionModal()"
                class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1 transition-colors"
              >
                <Plus class="w-3.5 h-3.5" />
                Ajouter une session passée
              </button>
            </div>
          </div>

          <div v-if="tireSessions.length === 0" class="p-6 text-center bg-slate-950/40 rounded-2xl text-xs text-slate-500">
            Aucune session enregistrée pour ce pneu.
          </div>

          <div v-else class="space-y-2.5">
            <div
              v-for="s in tireSessions"
              :key="s.id"
              class="bg-slate-950/80 border border-slate-800/80 rounded-2xl p-3.5 text-xs space-y-2 hover:border-slate-700 transition-all"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span
                    class="px-2 py-0.5 rounded-md font-bold text-[10px]"
                    :class="s.dismounted_date ? 'bg-slate-800 text-slate-300' : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'"
                  >
                    {{ s.dismounted_date ? 'Session terminée' : '🟢 Montage en cours' }}
                  </span>
                  <span class="font-bold text-white">Roue : {{ s.position }}</span>
                </div>

                <div class="flex items-center gap-1.5">
                  <button
                    @click="copySession(s)"
                    class="p-1 rounded transition-colors"
                    :class="copiedSession?.mounted_date === (s.mounted_date ? new Date(s.mounted_date).toISOString().substring(0, 10) : '') && copiedSession?.mounted_odometer === s.mounted_odometer ? 'text-indigo-400 bg-indigo-950/60' : 'text-slate-400 hover:text-indigo-400'"
                    title="Copier les données de cette session"
                  >
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="openDuplicateSessionModal(s)"
                    class="p-1 text-slate-400 hover:text-sky-400 rounded transition-colors"
                    title="Dupliquer vers d'autres pneus..."
                  >
                    <Shuffle class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="openEditSessionModal(s)"
                    class="p-1 text-slate-400 hover:text-white rounded"
                    title="Modifier la session"
                  >
                    <Edit2 class="w-3.5 h-3.5" />
                  </button>
                  <button
                    @click="handleDeleteSession(s)"
                    class="p-1 text-slate-400 hover:text-rose-400 rounded"
                    title="Supprimer la session"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <!-- Session Details -->
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] text-slate-400 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/60">
                <div>
                  <div class="text-slate-500">Montage :</div>
                  <div class="text-slate-200 font-medium">{{ formatDate(s.mounted_date) }} à {{ Math.round(s.mounted_odometer).toLocaleString('fr-FR') }} km</div>
                </div>
                <div>
                  <div class="text-slate-500">Démontage :</div>
                  <div class="text-slate-200 font-medium">
                    {{ s.dismounted_date ? `${formatDate(s.dismounted_date)} à ${Math.round(s.dismounted_odometer).toLocaleString('fr-FR')} km` : 'Actuellement sur le véhicule' }}
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-[11px] pt-1">
                <span v-if="s.notes" class="text-slate-400 italic">"{{ s.notes }}"</span>
                <span v-else></span>
                <span class="font-bold text-rose-400">+{{ Math.round(s.distance_km).toLocaleString('fr-FR') }} km parcourus</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Tread Logs (Mesures de gomme) -->
        <div class="space-y-2 pt-3 border-t border-slate-800">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
              <Ruler class="w-4 h-4 text-emerald-400" />
              Relevés de profondeur de gomme ({{ tireLogs.length }})
            </h4>
            <button
              @click="openLogModal(selectedTireStats)"
              class="text-xs text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              Ajouter un relevé
            </button>
          </div>

          <div v-if="tireLogs.length > 0" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
            <div
              v-for="l in tireLogs"
              :key="l.id"
              class="bg-slate-950/60 border border-slate-800 p-2.5 rounded-xl text-xs space-y-0.5"
            >
              <div class="flex items-center justify-between">
                <span class="font-bold text-emerald-400">{{ l.depth_mm }} mm</span>
                <span class="text-[10px] text-slate-500">{{ formatDate(l.date) }}</span>
              </div>
              <div class="flex items-center justify-between text-[10px] text-slate-400">
                <span>à {{ Math.round(l.odometer).toLocaleString('fr-FR') }} km</span>
                <span class="flex items-center gap-1">
                  <button @click="openLogModal(selectedTireStats || { tire: selectedTire }, l)" class="text-slate-500 hover:text-emerald-400" title="Modifier le relevé">
                    <Pencil class="w-3 h-3" />
                  </button>
                  <button @click="handleDeleteLog(l)" class="text-slate-500 hover:text-rose-400" title="Supprimer le relevé">
                    <Trash2 class="w-3 h-3" />
                  </button>
                </span>
              </div>
            </div>
          </div>
        </div>
        </div>

        <!-- Pinned Footer -->
        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showHistoryModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: AJOUTER / MODIFIER UNE SESSION DE MONTAGE -->
    <div
      v-if="showSessionModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showSessionModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white">
            {{ editingSessionId ? 'Modifier la session de montage' : 'Ajouter une session de montage passée' }}
          </h3>
          <button @click="showSessionModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <!-- Quick paste banner if session copied -->
          <button
            v-if="copiedSession && !editingSessionId"
            type="button"
            @click="applyCopiedSessionToForm()"
            class="w-full px-3 py-2 bg-indigo-950/40 border border-indigo-800/60 rounded-xl text-indigo-300 hover:text-white text-xs flex items-center justify-center gap-2 transition-colors font-semibold"
          >
            <ClipboardPaste class="w-4 h-4 text-indigo-400" />
            <span>Coller les données de la session copiée ({{ copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '' }})</span>
          </button>

          <div>
            <label for="tire-session-position" class="block text-slate-400 mb-1 font-semibold">Position occupée</label>
            <select id="tire-session-position"
              v-model="sessionForm.position"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            >
              <option value="FL">Avant Gauche (FL)</option>
              <option value="FR">Avant Droit (FR)</option>
              <option value="RL">Arrière Gauche (RL)</option>
              <option value="RR">Arrière Droit (RR)</option>
              <option value="STORAGE">Au garage / Non spécifié</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <div>
              <label for="tire-session-mounted-date" class="block text-slate-400 mb-1 font-semibold">Date de montage</label>
              <AppDatePicker
                id="tire-session-mounted-date"
                v-model="sessionForm.mounted_date"
                size="xs"
                :clearable="true"
              />
            </div>
            <div>
              <label for="tire-session-mounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre montage (km)</label>
              <input id="tire-session-mounted-odometer"
                v-model.number="sessionForm.mounted_odometer"
                @input="onSessionOdometerChange"
                type="number"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              />
            </div>
          </div>

          <div class="pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300">
              <input type="checkbox" v-model="sessionForm.is_dismounted" class="rounded accent-rose-500" />
              <span>Cette session est terminée (pneu démonté)</span>
            </label>
          </div>

          <div v-if="sessionForm.is_dismounted" class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-xl border border-slate-800">
            <div>
              <label for="tire-session-dismounted-date" class="block text-slate-400 mb-1 font-semibold">Date démontage</label>
              <AppDatePicker
                id="tire-session-dismounted-date"
                v-model="sessionForm.dismounted_date"
                size="xs"
                :clearable="true"
              />
            </div>
            <div>
              <label for="tire-session-dismounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre démontage (km)</label>
              <input id="tire-session-dismounted-odometer"
                v-model.number="sessionForm.dismounted_odometer"
                @input="onSessionOdometerChange"
                type="number"
                class="w-full bg-slate-900 text-slate-100 rounded-lg px-2 py-1.5 border border-slate-700"
              />
            </div>
          </div>

          <div>
            <label for="tire-session-distance-km" class="block text-slate-400 mb-1 font-semibold">Distance de la session (km)</label>
            <input id="tire-session-distance-km"
              v-model.number="sessionForm.distance_km"
              type="number"
              placeholder="Auto-calculé ou forcé"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>

          <div>
            <label for="tire-session-notes" class="block text-slate-400 mb-1 font-semibold">Commentaire / Notes</label>
            <input id="tire-session-notes"
              v-model="sessionForm.notes"
              type="text"
              placeholder="Ex: Saison hivernale 2024"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showSessionModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleSaveSession"
            class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
          >
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: MESURE DE GOMME -->
    <div
      v-if="showLogModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showLogModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Ruler class="w-4 h-4 text-emerald-400" />
            {{ editingLogId ? 'Modifier le relevé' : 'Relevé de sculpture' }}
          </h3>
          <button @click="showLogModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <div>
            <label for="tire-new-log-depth-mm" class="block text-slate-400 mb-1 font-semibold">Profondeur mesurée (mm)</label>
            <input id="tire-new-log-depth-mm"
              v-model.number="newLogForm.depth_mm"
              type="number"
              step="0.1"
              min="1.0"
              max="10.0"
              class="w-full bg-slate-800 text-slate-100 font-bold rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 text-sm"
            />
          </div>
          <div>
            <label for="tire-new-log-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre actuel (km)</label>
            <input id="tire-new-log-odometer"
              v-model.number="newLogForm.odometer"
              type="number"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label for="tire-new-log-date" class="block text-slate-400 mb-1 font-semibold">Date du relevé</label>
            <AppDatePicker id="tire-new-log-date" v-model="newLogForm.date" size="sm" required />
          </div>
          <div>
            <label for="tire-new-log-notes" class="block text-slate-400 mb-1 font-semibold">Notes (optionnel)</label>
            <input id="tire-new-log-notes"
              v-model="newLogForm.notes"
              type="text"
              placeholder="Ex: Contrôle avant vacances"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showLogModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleAddLog"
            class="bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
          >
            Enregistrer le relevé
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: CHANGEMENT DE PACK COMPLET (ÉTÉ ⇄ HIVER) -->
    <div
      v-if="showPackSwapModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showPackSwapModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Snowflake class="w-4 h-4 text-sky-400" />
            Permutation saisonnière (Changement de pack complet)
          </h3>
          <button @click="showPackSwapModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <div>
            <label for="tire-pack-swap-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre de la permutation (km)</label>
            <input id="tire-pack-swap-odometer"
              v-model.number="packSwapForm.odometer"
              type="number"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
            <div>
              <label for="tire-pack-swap-tires-fl" class="block text-slate-400 mb-1 font-semibold">Avant Gauche (FL)</label>
              <select id="tire-pack-swap-tires-fl"
                v-model="packSwapForm.tires.FL"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label for="tire-pack-swap-tires-fr" class="block text-slate-400 mb-1 font-semibold">Avant Droit (FR)</label>
              <select id="tire-pack-swap-tires-fr"
                v-model="packSwapForm.tires.FR"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label for="tire-pack-swap-tires-rl" class="block text-slate-400 mb-1 font-semibold">Arrière Gauche (RL)</label>
              <select id="tire-pack-swap-tires-rl"
                v-model="packSwapForm.tires.RL"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>

            <div>
              <label for="tire-pack-swap-tires-rr" class="block text-slate-400 mb-1 font-semibold">Arrière Droit (RR)</label>
              <select id="tire-pack-swap-tires-rr"
                v-model="packSwapForm.tires.RR"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
              >
                <option value="">-- Choisir un pneu --</option>
                <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                  {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.season }})
                </option>
              </select>
            </div>
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showPackSwapModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handlePackSwapSubmit"
            class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
          >
            Confirmer la permutation
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: MODIFIER UN OU PLUSIEURS PNEUS -->
    <div
      v-if="showTireEditModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showTireEditModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Pencil class="w-4 h-4 text-rose-400" />
            {{ tireEditIds.length > 1 ? `Modifier ${tireEditIds.length} pneus` : 'Modifier le pneu' }}
          </h3>
          <button type="button" @click="showTireEditModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form id="tire-edit-modal-form" @submit.prevent="handleSaveTireEdit" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <p v-if="tireEditIds.length > 1" class="text-[11px] text-slate-400">
            {{ editedTires.map((t) => `${t.tire.brand} ${t.tire.current_position}`).join(' • ') }} — les champs laissés vides ne sont pas modifiés.
          </p>

          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="tire-edit-brand" class="block text-[11px] text-slate-400 mb-1 font-semibold">Marque</label>
              <input id="tire-edit-brand" v-model="tireEditForm.brand" type="text"  :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-model" class="block text-[11px] text-slate-400 mb-1 font-semibold">Modèle</label>
              <input id="tire-edit-model" v-model="tireEditForm.model" type="text"  :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-dimension" class="block text-[11px] text-slate-400 mb-1 font-semibold">Dimension</label>
              <input id="tire-edit-dimension" v-model="tireEditForm.dimension" type="text"  :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-season" class="block text-[11px] text-slate-400 mb-1 font-semibold">Saison</label>
              <select id="tire-edit-season" v-model="tireEditForm.season" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500">
                <option value="">{{ tireEditIds.length > 1 ? 'Inchangée' : '—' }}</option>
                <option value="SUMMER">Été</option>
                <option value="WINTER">Hiver</option>
                <option value="ALL_SEASON">4 saisons</option>
              </select>
            </div>
            <div>
              <label for="tire-edit-purchase-date" class="block text-[11px] text-slate-400 mb-1 font-semibold">Date d'achat</label>
              <AppDatePicker
                id="tire-edit-purchase-date"
                v-model="tireEditForm.purchase_date"
                :placeholder="tireEditIds.length > 1 ? 'Inchangé' : 'Sélectionner une date'"
                size="xs"
                :clearable="true"
              />
            </div>
            <div>
              <label for="tire-edit-dot" class="block text-[11px] text-slate-400 mb-1 font-semibold">Code DOT</label>
              <input id="tire-edit-dot" v-model="tireEditForm.dot_code" type="text"  :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 items-end">
            <div>
              <label for="tire-edit-price-mode" class="block text-[11px] text-slate-400 mb-1 font-semibold">Saisie du prix</label>
              <select id="tire-edit-price-mode" v-model="tireEditPriceMode" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500">
                <option value="UNIT">Prix unitaire</option>
                <option value="TOTAL" :disabled="tireEditIds.length < 2">Prix total réparti</option>
              </select>
            </div>
            <div>
              <label for="tire-edit-price" class="block text-[11px] text-slate-400 mb-1 font-semibold">Prix (€)</label>
              <input id="tire-edit-price" v-model.number="tireEditForm.price" type="number" step="0.01" min="0" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-lifespan" class="block text-[11px] text-slate-400 mb-1 font-semibold">Durée de vie estimée (km)</label>
              <input id="tire-edit-lifespan" v-model.number="tireEditForm.estimated_lifespan_km" type="number" min="1" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-initial-depth" class="block text-[11px] text-slate-400 mb-1 font-semibold">Profondeur neuve (mm)</label>
              <input id="tire-edit-initial-depth" v-model.number="tireEditForm.initial_depth_mm" type="number" step="0.1" min="0" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-min-depth" class="block text-[11px] text-slate-400 mb-1 font-semibold">Profondeur minimale (mm)</label>
              <input id="tire-edit-min-depth" v-model.number="tireEditForm.min_legal_depth_mm" type="number" step="0.1" min="0" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
            <div>
              <label for="tire-edit-initial-distance" class="block text-[11px] text-slate-400 mb-1 font-semibold">Km avant TeslaCost (occasion)</label>
              <input id="tire-edit-initial-distance" v-model.number="tireEditForm.initial_distance_km" type="number" min="0" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
            </div>
          </div>

          <div v-if="editIncludesMounted" class="space-y-2 pt-3 border-t border-slate-800">
            <h4 class="text-[11px] font-bold text-rose-400 uppercase tracking-wider">Montage en cours</h4>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="tire-edit-mounted-date" class="block text-[11px] text-slate-400 mb-1 font-semibold">Date de montage</label>
                <AppDatePicker
                  id="tire-edit-mounted-date"
                  v-model="tireEditForm.mounted_date"
                  :placeholder="tireEditIds.length > 1 ? 'Inchangé' : 'Sélectionner une date'"
                  size="xs"
                  :clearable="true"
                />
              </div>
              <div>
                <label for="tire-edit-mounted-odometer" class="block text-[11px] text-slate-400 mb-1 font-semibold">Odomètre au montage (km)</label>
                <input id="tire-edit-mounted-odometer" v-model.number="tireEditForm.mounted_odometer" type="number" min="0" :placeholder="tireEditIds.length > 1 ? 'Inchangé' : ''" class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
              </div>
            </div>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showTireEditModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="tire-edit-modal-form" class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors">
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: METTRE AU REBUT -->
    <div
      v-if="showDisposeModal && selectedTire"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showDisposeModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Archive class="w-4 h-4 text-amber-400" />
            Mettre au rebut
          </h3>
          <button type="button" @click="showDisposeModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-4 h-4" />
          </button>
        </div>

        <form id="tire-dispose-modal-form" @submit.prevent="handleDisposeTire" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <p class="text-xs text-slate-300 font-semibold">
            {{ selectedTire.brand }} {{ selectedTire.model }}
          </p>
          <div>
            <label for="tire-dispose-date" class="block text-[11px] text-slate-400 mb-1 font-semibold">Date</label>
            <AppDatePicker
              id="tire-dispose-date"
              v-model="disposeForm.date"
              required
              size="xs"
            />
          </div>
          <div v-if="['FL', 'FR', 'RL', 'RR'].includes(selectedTire.current_position)">
            <label for="tire-dispose-odometer" class="block text-[11px] text-slate-400 mb-1 font-semibold">Odomètre au démontage (km)</label>
            <input id="tire-dispose-odometer" v-model.number="disposeForm.odometer" type="number" min="0" required class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showDisposeModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="tire-dispose-modal-form" class="bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors">
            Mettre au rebut
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: AJOUTER UNE SESSION PASSÉE SUR UN LOT DE PNEUS DU GARAGE (FEATURE A) -->
    <div
      v-if="showBatchSessionModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showBatchSessionModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <History class="w-5 h-5 text-rose-400" />
            <h3 class="text-base font-bold text-white">
              Ajouter une session passée sur un lot
            </h3>
          </div>
          <button @click="showBatchSessionModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <!-- Tire selection from storage -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="font-semibold text-slate-300">
                Pneus du garage concernés ({{ batchSessionTireIds.length }}/{{ storageTires.length }})
              </span>
              <div class="flex items-center gap-2 text-[11px]">
                <button type="button" @click="selectAllBatchSessionTires()" class="text-rose-400 hover:text-rose-300 font-semibold">
                  Tout cocher
                </button>
                <span class="text-slate-600">|</span>
                <button type="button" @click="deselectAllBatchSessionTires()" class="text-slate-400 hover:text-slate-200">
                  Tout décocher
                </button>
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-40 overflow-y-auto p-1 bg-slate-950/60 rounded-xl border border-slate-800/80">
              <label
                v-for="t in storageTires"
                :key="t.tire.id"
                class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
              >
                <input
                  type="checkbox"
                  :checked="batchSessionTireIds.includes(t.tire.id)"
                  @change="toggleBatchSessionTire(t.tire.id)"
                  class="rounded accent-rose-500 w-4 h-4"
                />
                <div class="min-w-0 flex-1">
                  <div class="font-bold truncate text-white">{{ t.tire.brand }} {{ t.tire.model }}</div>
                  <div class="text-[10px] text-slate-400 truncate">{{ t.tire.dimension }}</div>
                </div>
              </label>
            </div>
          </div>

          <!-- Dates & Odometers -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="batch-session-mounted-date" class="block text-slate-400 mb-1 font-semibold">Date de montage</label>
              <AppDatePicker
                id="batch-session-mounted-date"
                v-model="batchSessionForm.mounted_date"
                size="sm"
                :clearable="true"
              />
            </div>
            <div>
              <label for="batch-session-mounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre montage (km)</label>
              <input
                id="batch-session-mounted-odometer"
                v-model.number="batchSessionForm.mounted_odometer"
                @input="onBatchOdometerChange"
                type="number"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="batch-session-dismounted-date" class="block text-slate-400 mb-1 font-semibold">Date démontage</label>
              <AppDatePicker
                id="batch-session-dismounted-date"
                v-model="batchSessionForm.dismounted_date"
                size="sm"
                :clearable="true"
              />
            </div>
            <div>
              <label for="batch-session-dismounted-odometer" class="block text-slate-400 mb-1 font-semibold">Odomètre démontage (km)</label>
              <input
                id="batch-session-dismounted-odometer"
                v-model.number="batchSessionForm.dismounted_odometer"
                @input="onBatchOdometerChange"
                type="number"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
              />
            </div>
          </div>

          <div>
            <label for="batch-session-distance-km" class="block text-slate-400 mb-1 font-semibold">Distance de la session (km)</label>
            <input
              id="batch-session-distance-km"
              v-model.number="batchSessionForm.distance_km"
              type="number"
              placeholder="Auto-calculé par les odomètres ou manuel"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
            />
          </div>

          <div>
            <label for="batch-session-notes" class="block text-slate-400 mb-1 font-semibold">Commentaire / Notes</label>
            <input
              id="batch-session-notes"
              v-model="batchSessionForm.notes"
              type="text"
              placeholder="Ex: Saison hiver 2023-2024"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
            />
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showBatchSessionModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleSaveBatchSession()"
            :disabled="savingBatchSession || batchSessionTireIds.length === 0"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/20 transition-all flex items-center gap-1.5"
          >
            <Check class="w-4 h-4" />
            <span>{{ savingBatchSession ? 'Enregistrement...' : `Appliquer à ${batchSessionTireIds.length} pneu(s)` }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: DUPLIQUER LA SESSION VERS D'AUTRES PNEUS (FEATURE C) -->
    <div
      v-if="showDuplicateSessionModal && sessionToDuplicate"
      class="fixed inset-0 z-[70] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showDuplicateSessionModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <Copy class="w-5 h-5 text-indigo-400" />
            <h3 class="text-base font-bold text-white">
              Dupliquer la session vers d'autres pneus
            </h3>
          </div>
          <button @click="showDuplicateSessionModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <!-- Session recap -->
          <div class="bg-slate-950/60 p-3 rounded-xl border border-slate-800 space-y-1">
            <div class="text-slate-400">Période : <strong class="text-white">{{ formatDate(sessionToDuplicate.mounted_date) }} → {{ sessionToDuplicate.dismounted_date ? formatDate(sessionToDuplicate.dismounted_date) : 'En cours' }}</strong></div>
            <div class="text-slate-400">Distance : <strong class="text-rose-400">+{{ Math.round(sessionToDuplicate.distance_km || 0).toLocaleString('fr-FR') }} km</strong></div>
            <div v-if="sessionToDuplicate.notes" class="text-slate-400 italic">"{{ sessionToDuplicate.notes }}"</div>
          </div>

          <!-- Target tires selection -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="font-semibold text-slate-300">Sélectionner les pneus cibles :</span>
              <div class="flex items-center gap-2 text-[11px]">
                <button
                  type="button"
                  @click="duplicateTargetTireIds = tires.filter(x => x.tire.id !== selectedTire?.id).map(x => x.tire.id)"
                  class="text-indigo-400 hover:text-indigo-300 font-semibold"
                >
                  Tout cocher
                </button>
                <span class="text-slate-600">|</span>
                <button
                  type="button"
                  @click="duplicateTargetTireIds = []"
                  class="text-slate-400 hover:text-slate-200"
                >
                  Tout décocher
                </button>
              </div>
            </div>
            <div class="space-y-1.5 max-h-56 overflow-y-auto p-1 bg-slate-950/40 rounded-xl border border-slate-800/60">
              <label
                v-for="t in tires.filter(x => x.tire.id !== selectedTire?.id)"
                :key="t.tire.id"
                class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
              >
                <input
                  type="checkbox"
                  :checked="duplicateTargetTireIds.includes(t.tire.id)"
                  @change="toggleDuplicateTargetTire(t.tire.id)"
                  class="rounded accent-indigo-500 w-4 h-4"
                />
                <div class="min-w-0 flex-1">
                  <div class="font-bold truncate text-white">{{ t.tire.brand }} {{ t.tire.model }}</div>
                  <div class="text-[10px] text-slate-400 truncate">{{ t.tire.dimension }} — {{ t.tire.current_position === 'STORAGE' ? 'Au garage' : 'Roue ' + t.tire.current_position }}</div>
                </div>
              </label>
            </div>
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showDuplicateSessionModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleDuplicateSessionSubmit()"
            :disabled="duplicatingSession || duplicateTargetTireIds.length === 0"
            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-indigo-600/20 transition-all flex items-center gap-1.5"
          >
            <Copy class="w-4 h-4" />
            <span>{{ duplicatingSession ? 'Duplication...' : `Dupliquer vers ${duplicateTargetTireIds.length} pneu(s)` }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: METTRE AU REBUT UNE SÉLECTION -->
    <div
      v-if="showBatchDisposeModal"
      class="fixed inset-0 z-[70] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showBatchDisposeModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <Archive class="w-5 h-5 text-amber-500" />
            <h3 class="text-base font-bold text-white">
              Mettre au rebut {{ selectedTireIds.length }} pneu(s)
            </h3>
          </div>
          <button @click="showBatchDisposeModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <p class="text-slate-400 leading-relaxed">
            Mettre ces pneus au rebut les archive définitivement (usure maximale, perforation, vente). Leurs sessions passées et coûts restent conservés dans l'historique et le coût au kilomètre.
          </p>

          <!-- Selected tires summary -->
          <div class="space-y-1.5 max-h-40 overflow-y-auto p-2 bg-slate-950/50 rounded-xl border border-slate-800">
            <div
              v-for="t in tires.filter(x => selectedTireIds.includes(x.tire.id))"
              :key="t.tire.id"
              class="flex items-center justify-between py-1 px-1.5 border-b border-slate-800/50 last:border-0"
            >
              <div>
                <span class="font-bold text-white">{{ t.tire.brand }} {{ t.tire.model }}</span>
                <span class="text-[10px] text-slate-400 ml-1.5">({{ t.tire.dimension }})</span>
              </div>
              <span class="text-[10px] px-2 py-0.5 rounded font-mono bg-slate-800 text-slate-300">
                {{ ['FL', 'FR', 'RL', 'RR'].includes(t.tire.current_position) ? `Roue ${t.tire.current_position}` : 'Garage' }}
              </span>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
            <div>
              <label for="batch-dispose-date" class="block text-slate-300 mb-1 font-semibold">Date de mise au rebut</label>
              <input
                id="batch-dispose-date"
                v-model="batchDisposeForm.date"
                type="date"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-amber-500 focus:outline-none"
              />
              <p class="text-[10px] text-slate-500 mt-1">Par défaut : date du dernier démontage pour les pneus au garage.</p>
            </div>
            <div>
              <label for="batch-dispose-odo" class="block text-slate-300 mb-1 font-semibold">Kilométrage véhicule</label>
              <input
                id="batch-dispose-odo"
                v-model="batchDisposeForm.odometer"
                type="number"
                placeholder="Optionnel pour pneu garage"
                class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-amber-500 focus:outline-none"
              />
              <p class="text-[10px] text-slate-500 mt-1">Odomètre final si démonté à cette date.</p>
            </div>
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showBatchDisposeModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleBatchDisposeSubmit()"
            :disabled="disposingBatch || !batchDisposeForm.date"
            class="px-4 py-2 bg-amber-600 hover:bg-amber-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-amber-600/20 transition-all flex items-center gap-1.5"
          >
            <Archive class="w-4 h-4" />
            <span>{{ disposingBatch ? 'Mise au rebut...' : `Mettre au rebut (${selectedTireIds.length})` }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL: COPIER TOUT L'HISTORIQUE D'UN PNEU -->
    <div
      v-if="showCopyHistoryModal && copyHistorySourceTire"
      class="fixed inset-0 z-[70] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showCopyHistoryModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <Copy class="w-5 h-5 text-indigo-400" />
            <h3 class="text-base font-bold text-white">
              Copier l'historique complet
            </h3>
          </div>
          <button @click="showCopyHistoryModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
          <!-- Source selection -->
          <div class="bg-slate-950/60 p-3 rounded-xl border border-slate-800 space-y-2">
            <label for="copy-history-source-select" class="text-[11px] text-indigo-400 font-semibold uppercase tracking-wider block">Pneu source à cloner</label>
            <select
              id="copy-history-source-select"
              :value="copyHistorySourceTire.id"
              @change="onSourceTireChange(($event.target as HTMLSelectElement).value)"
              class="w-full bg-slate-900 border border-slate-700 text-white rounded-xl px-3 py-2 text-xs focus:outline-none focus:border-indigo-500 cursor-pointer"
            >
              <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">
                {{ t.tire.brand }} {{ t.tire.model }} ({{ t.tire.dimension }}) — {{ t.tire.current_position === 'STORAGE' ? 'Au garage' : t.tire.current_position === 'DISPOSED' ? 'Au rebut' : 'Roue ' + t.tire.current_position }}
              </option>
            </select>
          </div>

          <!-- Options -->
          <div class="space-y-2 bg-slate-950/40 p-3 rounded-xl border border-slate-800/80">
            <div class="font-semibold text-slate-300">Données à répliquer :</div>
            <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
              <input
                type="checkbox"
                v-model="copyHistoryOptions.copy_sessions"
                class="rounded accent-indigo-500 w-4 h-4 mt-0.5"
              />
              <div>
                <span class="font-medium text-white">Sessions de montage & démontage</span>
                <span class="block text-[11px] text-slate-400">Copie les périodes, dates, odomètres et distances parcourues.</span>
              </div>
            </label>

            <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
              <input
                type="checkbox"
                v-model="copyHistoryOptions.adapt_position"
                :disabled="!copyHistoryOptions.copy_sessions"
                class="rounded accent-indigo-500 w-4 h-4 mt-0.5"
              />
              <div>
                <span class="font-medium text-white">Adapter la position de montage à chaque pneu cible</span>
                <span class="block text-[11px] text-slate-400">Si activé, chaque pneu cible utilisera sa propre position (ex: FL, FR, RL, RR) au lieu de reproduire fidèlement la position exacte de la source.</span>
              </div>
            </label>

            <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
              <input
                type="checkbox"
                v-model="copyHistoryOptions.copy_logs"
                class="rounded accent-indigo-500 w-4 h-4 mt-0.5"
              />
              <div>
                <span class="font-medium text-white">Mesures d'usure et sculptures (logs)</span>
                <span class="block text-[11px] text-slate-400">Copie les relevés en mm de sculpture effectués au fil du temps.</span>
              </div>
            </label>
          </div>

          <!-- Target tires list -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="font-semibold text-slate-300">Appliquer aux pneus cibles :</span>
              <div class="flex items-center gap-2 text-[11px]">
                <button
                  type="button"
                  @click="copyHistoryTargetTireIds = tires.filter(x => x.tire.id !== copyHistorySourceTire?.id).map(x => x.tire.id)"
                  class="text-indigo-400 hover:text-indigo-300 font-semibold"
                >
                  Tout cocher
                </button>
                <span class="text-slate-600">|</span>
                <button
                  type="button"
                  @click="copyHistoryTargetTireIds = []"
                  class="text-slate-400 hover:text-slate-200"
                >
                  Tout décocher
                </button>
              </div>
            </div>

            <div class="space-y-1.5 max-h-48 overflow-y-auto p-1 bg-slate-950/40 rounded-xl border border-slate-800/60">
              <label
                v-for="t in tires.filter(x => x.tire.id !== copyHistorySourceTire?.id)"
                :key="t.tire.id"
                class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
              >
                <input
                  type="checkbox"
                  :checked="copyHistoryTargetTireIds.includes(t.tire.id)"
                  @change="toggleCopyHistoryTargetTire(t.tire.id)"
                  class="rounded accent-indigo-500 w-4 h-4"
                />
                <div class="min-w-0 flex-1">
                  <div class="font-bold truncate text-white">{{ t.tire.brand }} {{ t.tire.model }}</div>
                  <div class="text-[10px] text-slate-400 truncate">
                    {{ t.tire.dimension }} — {{ t.tire.current_position === 'STORAGE' ? 'Au garage' : t.tire.current_position === 'DISPOSED' ? 'Au rebut' : 'Roue ' + t.tire.current_position }}
                  </div>
                </div>
              </label>
            </div>
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showCopyHistoryModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleCopyHistorySubmit()"
            :disabled="copyingHistory || copyHistoryTargetTireIds.length === 0"
            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-indigo-600/20 transition-all flex items-center gap-1.5"
          >
            <Copy class="w-4 h-4" />
            <span>{{ copyingHistory ? 'Copie en cours...' : `Copier vers ${copyHistoryTargetTireIds.length} pneu(s)` }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
