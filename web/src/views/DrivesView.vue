<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import AppDatePicker from '@/components/AppDatePicker.vue'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import { downloadCsv } from '@/utils/csv'
import {
  Navigation as NavIcon,
  Tag,
  CheckSquare,
  Square,
  Receipt,
  Layers,
  MapPin,
  Clock,
  Zap,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  X,
  Users,
  Coins,
  Shield,
  Wrench,
  Disc,
  Plus,
  ArrowRight,
  TrendingUp,
  AlertTriangle,
  Ban,
  Pencil,
  Trash2,
  Save,
  List,
  Search,
  Calendar,
  RotateCcw,
  Download,
} from 'lucide-vue-next'

const router = useRouter()
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const drives = ref<any[]>([])
const total = ref(0)
const page = ref(1)

// Page limit with persistent storage
const savedLimit = Number(localStorage.getItem('drives_limit'))
const limit = ref(savedLimit === 20 || savedLimit === 50 || savedLimit === 100 ? savedLimit : 20)
const totalPages = computed(() => Math.ceil(total.value / limit.value) || 1)
const selectedTag = ref('')
const unqualifiedOnly = ref(false)
const unqualifiedCount = ref(0)
const loading = ref(true)

// Period / Month navigation (Mix A + C)
function getCurrentYearMonth() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

const periodMode = ref<'ALL' | 'MONTH' | 'CUSTOM'>('ALL')
const selectedMonth = ref(getCurrentYearMonth())
const customFrom = ref('')
const customTo = ref('')

// Text search on start/end address
const searchQuery = ref('')
let searchDebounceTimeout: any = null

function onSearchInput() {
  if (searchDebounceTimeout) clearTimeout(searchDebounceTimeout)
  searchDebounceTimeout = setTimeout(() => {
    page.value = 1
    loadDrives()
  }, 300)
}

function clearSearch() {
  searchQuery.value = ''
  page.value = 1
  loadDrives()
}

// Month navigation helpers
const formattedSelectedMonth = computed(() => {
  if (!selectedMonth.value) return ''
  const [y, m] = selectedMonth.value.split('-').map(Number)
  const d = new Date(y, m - 1, 1)
  const str = d.toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })
  return str.charAt(0).toUpperCase() + str.slice(1)
})

const isCurrentMonth = computed(() => selectedMonth.value === getCurrentYearMonth())

function prevMonth() {
  const [y, m] = selectedMonth.value.split('-').map(Number)
  const d = new Date(y, m - 2, 1)
  selectedMonth.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  page.value = 1
  loadDrives()
}

function nextMonth() {
  const [y, m] = selectedMonth.value.split('-').map(Number)
  const d = new Date(y, m, 1)
  selectedMonth.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  page.value = 1
  loadDrives()
}

function resetToCurrentMonth() {
  selectedMonth.value = getCurrentYearMonth()
  page.value = 1
  loadDrives()
}

function setPeriodMode(mode: 'ALL' | 'MONTH' | 'CUSTOM') {
  periodMode.value = mode
  page.value = 1
  loadDrives()
}

function onMonthChange() {
  page.value = 1
  loadDrives()
}

function onCustomDateChange() {
  page.value = 1
  loadDrives()
}

// Page sizing & navigation
function setLimit(newLimit: number) {
  limit.value = newLimit
  localStorage.setItem('drives_limit', newLimit.toString())
  page.value = 1
  loadDrives()
}

function goToPage(targetPage: number) {
  const p = Math.max(1, Math.min(totalPages.value, targetPage))
  if (p !== page.value) {
    page.value = p
    loadDrives()
    const scrollContainer = document.querySelector('main')?.parentElement
    if (scrollContainer) {
      scrollContainer.scrollTo({ top: 0, behavior: 'smooth' })
    } else {
      window.scrollTo({ top: 0, behavior: 'smooth' })
    }
  }
}

const jumpInput = ref<number | ''>('')
function applyJump() {
  if (jumpInput.value !== '') {
    goToPage(Number(jumpInput.value))
    jumpInput.value = ''
  }
}

const itemRangeStart = computed(() => {
  if (total.value === 0) return 0
  return (page.value - 1) * limit.value + 1
})

const itemRangeEnd = computed(() => {
  return Math.min(page.value * limit.value, total.value)
})

const paginationPages = computed(() => {
  const totalP = totalPages.value
  const current = page.value
  if (totalP <= 7) {
    return Array.from({ length: totalP }, (_, i) => i + 1)
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5, '...', totalP]
  }
  if (current >= totalP - 3) {
    return [1, '...', totalP - 4, totalP - 3, totalP - 2, totalP - 1, totalP]
  }
  return [1, '...', current - 1, current, current + 1, '...', totalP]
})

// Summary metrics of current page drives
const pageDistance = computed(() => drives.value.reduce((acc, d) => acc + (d.distance_km || 0), 0))
const pageEnergy = computed(() => drives.value.reduce((acc, d) => acc + (d.energy_consumed_kwh || 0), 0))
const pageCost = computed(() => drives.value.reduce((acc, d) => acc + (d.costs?.total_cost || 0), 0) / 100)

function resetAllFilters() {
  searchQuery.value = ''
  selectedTag.value = ''
  unqualifiedOnly.value = false
  periodMode.value = 'ALL'
  customFrom.value = ''
  customTo.value = ''
  page.value = 1
  loadDrives()
}

// Multi-selection for trip grouping & tolls & carpooling. Drives are kept by id so that the selection
// survives pagination and filters.
const selectedDrives = ref<Record<string, any>>({})
const selectedDriveIds = computed(() => Object.keys(selectedDrives.value))
const selectedList = computed(() =>
  Object.values(selectedDrives.value).sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
)
const selectedOffPage = computed(() => selectedDriveIds.value.filter((id) => !drives.value.some((d) => d.id === id)).length)
const allPageSelected = computed(() => drives.value.length > 0 && drives.value.every((d) => selectedDrives.value[d.id]))

// Unified selection summary metrics (same as a Voyage)
const selectedSummaryMetrics = computed(() => {
  if (!selectedList.value.length) return ''
  const totalKm = selectedList.value.reduce((s, d) => s + (Number(d.distance_km) || 0), 0)
  const totalKwh = selectedList.value.reduce((s, d) => s + (Number(d.costs?.electricity_kwh) || 0), 0)
  const totalCost = selectedList.value.reduce((s, d) => s + (Number(d.costs?.total_cost) || 0), 0)
  return `${Math.round(totalKm).toLocaleString('fr-FR')} km • ${Math.round(totalKwh)} kWh • ${totalCost.toFixed(2)} €`
})

function clearSelection() {
  selectedDrives.value = {}
}

// Batch tagging
async function handleBatchTag(tag: 'Pro' | 'Perso' | null) {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  const ids = [...selectedDriveIds.value]
  try {
    for (const id of ids) {
      const d = selectedDrives.value[id] || drives.value.find((x) => x.id === id)
      let currentTags = [...(d?.tags || [])]
      if (tag === 'Pro') {
        currentTags = currentTags.filter((t) => t !== 'Perso')
        if (!currentTags.includes('Pro')) currentTags.push('Pro')
      } else if (tag === 'Perso') {
        currentTags = currentTags.filter((t) => t !== 'Pro')
        if (!currentTags.includes('Perso')) currentTags.push('Perso')
      } else {
        // Clear Pro & Perso
        currentTags = currentTags.filter((t) => t !== 'Pro' && t !== 'Perso')
      }
      await api.updateDriveTags(vehicleStore.activeVehicle.id, id, currentTags)
      if (d) d.tags = currentTags
      const listed = drives.value.find((x) => x.id === id)
      if (listed) listed.tags = currentTags
    }
    showAlert(`Tags mis à jour pour ${ids.length} trajet(s).`, 'Succès', 'success')
  } catch (err: any) {
    showAlert(`Erreur lors du taggage par lot : ${err.message}`, 'Erreur', 'danger')
  }
}

// Export selected drives to CSV
function exportSelectedDrives() {
  if (!selectedList.value.length) return
  const headers = ['ID', 'Date', 'Depart', 'Arrivee', 'Distance_km', 'Duree_min', 'Conso_kWh_100km', 'Energie_kWh', 'Cout_Total_EUR', 'Cout_km_EUR', 'Tags']
  const rows = selectedList.value.map((d) => [
    d.id,
    d.start_time ? new Date(d.start_time).toISOString().slice(0, 16) : '',
    `"${(d.start_address || '').replace(/"/g, '""')}"`,
    `"${(d.end_address || '').replace(/"/g, '""')}"`,
    d.distance_km || 0,
    d.duration_min || 0,
    d.consumption_kwh_100km || 0,
    d.costs?.electricity_kwh || 0,
    (d.costs?.total_cost || 0).toFixed(2),
    (d.costs?.cost_per_km || 0).toFixed(3),
    `"${(d.tags || []).join(', ')}"`,
  ])
  downloadCsv(`trajets_export_${new Date().toISOString().slice(0, 10)}.csv`, headers, rows)
}

// View mode: drives list or trip groups ("voyages")
const viewMode = ref<'DRIVES' | 'TRIPS'>('DRIVES')
const tripGroups = ref<any[]>([])
const loadingTrips = ref(false)
const expandedTripId = ref<string | null>(null)
const tripDrives = ref<any[]>([])
const showTripEditModal = ref(false)
const tripEditForm = ref({ id: '', name: '', notes: '' })
const showAddToTripModal = ref(false)
const addToTripId = ref('')

// Expense edition inside the cost modal
const editingExpenseId = ref<string | null>(null)
const expenseEditForm = ref({ type: 'TOLL', amount: '' as number | string, notes: '' })
const showGroupModal = ref(false)
const groupName = ref('')
const tollAmount = ref<number | ''>('')
const expenseType = ref('TOLL')

// Cost breakdown modal state
const selectedCostDrive = ref<any | null>(null)
const showCostModal = ref(false)
const driveExpenses = ref<any[]>([])
const loadingExpenses = ref(false)
const showAddTollInline = ref(false)
const inlineTollAmount = ref<number | ''>('')
const inlineTollType = ref('TOLL')
const inlineTollNotes = ref('')
const addingToll = ref(false)

async function loadDrives() {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    let fromStr: string | undefined
    let toStr: string | undefined

    if (periodMode.value === 'MONTH' && selectedMonth.value) {
      const [y, m] = selectedMonth.value.split('-').map(Number)
      const lastDay = new Date(y, m, 0).getDate()
      fromStr = `${y}-${String(m).padStart(2, '0')}-01`
      toStr = `${y}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    } else if (periodMode.value === 'CUSTOM') {
      if (customFrom.value) fromStr = customFrom.value
      if (customTo.value) toStr = customTo.value
    }

    const res = await api.getDrives(vehicleStore.activeVehicle.id, {
      tag: selectedTag.value,
      page: page.value,
      limit: limit.value,
      unqualified: unqualifiedOnly.value,
      from: fromStr,
      to: toStr,
      q: searchQuery.value.trim() || undefined,
    })
    drives.value = res.drives
    total.value = res.total
    unqualifiedCount.value = res.unqualified_count || 0
  } catch (err) {
    console.error('Failed to load drives', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicle?.id, selectedTag.value, unqualifiedOnly.value, vehicleStore.lastSyncTimestamp],
  () => {
    page.value = 1
    loadDrives()
    if (viewMode.value === 'TRIPS') loadTripGroups()
  }
)

watch(
  () => vehicleStore.activeVehicle?.id,
  () => clearSelection()
)

onMounted(() => {
  loadDrives()
})

// Highway-like drive with no toll attached and not reviewed yet (same rule as the backend queue)
function needsTollQualification(d: any) {
  const isHighway =
    (d.distance_km >= 40 && (d.speed_avg || 0) >= 70) ||
    (d.distance_km >= 20 && (d.speed_max || 0) > 125)
  return !d.toll_reviewed_at && isHighway && !(d.costs?.tolls_cost > 0)
}

async function markNoToll(d: any) {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.setDriveTollReview(vehicleStore.activeVehicle.id, d.id, true)
    d.toll_reviewed_at = new Date().toISOString()
    unqualifiedCount.value = Math.max(0, unqualifiedCount.value - 1)
    if (unqualifiedOnly.value) {
      drives.value = drives.value.filter((x) => x.id !== d.id)
      total.value = Math.max(0, total.value - 1)
    }
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openTollEntry(d: any) {
  openCostModal(d)
  showAddTollInline.value = true
}

function toggleSelectDrive(d: any) {
  const next = { ...selectedDrives.value }
  if (next[d.id]) {
    delete next[d.id]
  } else {
    next[d.id] = d
  }
  selectedDrives.value = next
}

// Selects or unselects the drives of the current page, keeping selections made on other pages
function selectAll() {
  const next = { ...selectedDrives.value }
  const select = !allPageSelected.value
  for (const d of drives.value) {
    if (select) next[d.id] = d
    else delete next[d.id]
  }
  selectedDrives.value = next
}

// ----- Trip groups ("voyages") -----
async function loadTripGroups() {
  if (!vehicleStore.activeVehicle) return
  loadingTrips.value = true
  try {
    tripGroups.value = await api.getTripGroups(vehicleStore.activeVehicle.id)
  } catch (err) {
    console.error('Failed to load trip groups', err)
  } finally {
    loadingTrips.value = false
  }
}

function switchView(mode: 'DRIVES' | 'TRIPS') {
  viewMode.value = mode
  if (mode === 'TRIPS') loadTripGroups()
}

async function toggleTripDetails(tg: any) {
  if (expandedTripId.value === tg.id) {
    expandedTripId.value = null
    return
  }
  expandedTripId.value = tg.id
  tripDrives.value = []
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle!.id, { tripGroupId: tg.id, limit: 200 })
    tripDrives.value = [...res.drives].sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function openTripEdit(tg: any) {
  tripEditForm.value = { id: tg.id, name: tg.name, notes: tg.notes || '' }
  showTripEditModal.value = true
}

async function handleSaveTripEdit() {
  if (!vehicleStore.activeVehicle || !tripEditForm.value.name.trim()) return
  try {
    await api.updateTripGroup(vehicleStore.activeVehicle.id, tripEditForm.value.id, {
      name: tripEditForm.value.name,
      notes: tripEditForm.value.notes || null,
    })
    showTripEditModal.value = false
    await loadTripGroups()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function removeDriveFromTrip(tg: any, driveId: string) {
  if (!vehicleStore.activeVehicle) return
  const remaining = (tg.drive_ids || []).filter((id: string) => id !== driveId)
  if (!remaining.length) {
    showAlert('Un voyage doit garder au moins un trajet : supprimez le voyage à la place.', 'Action impossible', 'warning')
    return
  }
  try {
    await api.updateTripGroup(vehicleStore.activeVehicle.id, tg.id, { name: tg.name, notes: tg.notes, drive_ids: remaining })
    await loadTripGroups()
    const updated = tripGroups.value.find((g) => g.id === tg.id)
    expandedTripId.value = null
    if (updated) await toggleTripDetails(updated)
    loadDrives()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleDeleteTrip(tg: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: 'Supprimer le voyage',
    message: `Supprimer le voyage « ${tg.name} » ? Les trajets ne sont pas supprimés.`,
    confirmText: 'Supprimer le voyage',
    type: 'danger',
  })
  if (!ok) return

  let deleteExpenses = false
  if (tg.expense_count > 0) {
    deleteExpenses = await showConfirm({
      title: 'Frais associés au voyage',
      message: `Ce voyage porte ${tg.expense_count} frais (${Number(tg.expenses_total).toFixed(2)} €). Souhaitez-vous également supprimer ces frais ?`,
      confirmText: 'Supprimer aussi les frais',
      cancelText: 'Conserver les frais sans lien',
      type: 'warning',
    })
  }
  try {
    await api.deleteTripGroup(vehicleStore.activeVehicle.id, tg.id, deleteExpenses)
    await loadTripGroups()
    loadDrives()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function openAddToTrip() {
  await loadTripGroups()
  addToTripId.value = tripGroups.value[0]?.id || ''
  showAddToTripModal.value = true
}

async function handleAddToTrip() {
  const tg = tripGroups.value.find((g) => g.id === addToTripId.value)
  if (!vehicleStore.activeVehicle || !tg) return
  const driveIds = Array.from(new Set([...(tg.drive_ids || []), ...selectedDriveIds.value]))
  try {
    await api.updateTripGroup(vehicleStore.activeVehicle.id, tg.id, { name: tg.name, notes: tg.notes, drive_ids: driveIds })
    showAddToTripModal.value = false
    clearSelection()
    await loadTripGroups()
    loadDrives()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function formatTripDates(tg: any) {
  if (!tg.start_time) return 'Aucun trajet'
  const start = new Date(tg.start_time).toLocaleDateString('fr-FR', { day: '2-digit', month: 'short', year: 'numeric' })
  const end = tg.end_time ? new Date(tg.end_time).toLocaleDateString('fr-FR', { day: '2-digit', month: 'short', year: 'numeric' }) : start
  return start === end ? start : `${start} → ${end}`
}

async function toggleDriveTag(drive: any, tagToToggle: string) {
  if (!vehicleStore.activeVehicle) return
  let currentTags = [...(drive.tags || [])]
  const idx = currentTags.indexOf(tagToToggle)

  if (idx > -1) {
    currentTags.splice(idx, 1)
  } else {
    if (tagToToggle === 'Pro') {
      currentTags = currentTags.filter((t) => t !== 'Perso')
    } else if (tagToToggle === 'Perso') {
      currentTags = currentTags.filter((t) => t !== 'Pro')
    }
    currentTags.push(tagToToggle)
  }

  try {
    await api.updateDriveTags(vehicleStore.activeVehicle.id, drive.id, currentTags)
    drive.tags = currentTags
  } catch (err: any) {
    showAlert(`Erreur de mise à jour du tag : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleCreateGroupAndExpense() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  if (!groupName.value) {
    showAlert('Veuillez donner un nom au groupe de trajets (ex: Voyage Paris-Lyon)', 'Champ requis', 'warning')
    return
  }

  try {
    if (tollAmount.value && Number(tollAmount.value) > 0) {
      // Group and expense are created atomically by the backend; the expense is dated at the trip start
      const firstStart = selectedList.value.length ? new Date(selectedList.value[0].start_time).getTime() : undefined
      await api.createDriveExpense(vehicleStore.activeVehicle.id, {
        drive_ids: selectedDriveIds.value,
        type: expenseType.value,
        amount: Number(tollAmount.value),
        currency: 'EUR',
        date: new Date(firstStart || Date.now()).toISOString(),
        notes: groupName.value,
      })
    } else {
      await api.createTripGroup(vehicleStore.activeVehicle.id, {
        name: groupName.value,
        drive_ids: selectedDriveIds.value,
      })
    }

    showAlert('Groupe de trajets créé avec succès !', 'Succès', 'success')
    showGroupModal.value = false
    clearSelection()
    groupName.value = ''
    tollAmount.value = ''
    loadDrives()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleCarpoolSelectedDrives() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  // Each selected drive becomes a leg of the carpool, in chronological order
  const ids = selectedList.value.map((d: any) => d.id)
  clearSelection()
  router.push({ path: '/carpools', query: { new_drive_ids: ids.join(',') } })
}

async function openTripCostModal(tg: any) {
  if (!vehicleStore.activeVehicle) return
  loadingExpenses.value = true
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { tripGroupId: tg.id, limit: 200 })
    const tgDrives = [...res.drives].sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
    
    // Aggregated drive costs
    const totalKm = tgDrives.reduce((s, d) => s + (Number(d.distance_km) || 0), 0)
    const totalDuration = tgDrives.reduce((s, d) => s + (Number(d.duration_min) || 0), 0)
    const totalKwh = tgDrives.reduce((s, d) => s + (Number(d.costs?.electricity_kwh) || 0), 0)
    const elecCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.electricity_cost) || 0), 0)
    const tiresCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.tires_cost) || 0), 0)
    const maintCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.maintenance_cost) || 0), 0)
    const insurCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.insurance_cost) || 0), 0)
    const tollsCost = Number(tg.expenses_total || 0)
    const totalCost = elecCost + tiresCost + maintCost + insurCost + tollsCost
    const costPerKm = totalKm > 0 ? totalCost / totalKm : 0

    const firstDrive = tgDrives[0]
    const lastDrive = tgDrives[tgDrives.length - 1]

    selectedCostDrive.value = {
      id: tg.id,
      is_trip_group: true,
      start_time: tg.start_time || firstDrive?.start_time || tg.created_at,
      start_address: firstDrive ? (firstDrive.start_address || 'Départ').split(',')[0] : 'Départ',
      end_address: lastDrive ? (lastDrive.end_address || 'Arrivée').split(',')[0] : 'Arrivée',
      distance_km: Math.round(totalKm),
      duration_min: totalDuration,
      tags: [],
      trip_group_name: tg.name,
      drives_count: tgDrives.length,
      costs: {
        electricity_kwh: Math.round(totalKwh),
        electricity_rate: totalKwh > 0 ? elecCost / totalKwh : 0.22,
        electricity_cost: elecCost,
        tires_rate: totalKm > 0 ? tiresCost / totalKm : 0.02,
        tires_cost: tiresCost,
        maintenance_rate: totalKm > 0 ? maintCost / totalKm : 0.015,
        maintenance_cost: maintCost,
        insurance_rate: totalKm > 0 ? insurCost / totalKm : 0,
        insurance_cost: insurCost,
        tolls_cost: tollsCost,
        total_cost: totalCost,
        cost_per_km: costPerKm,
        has_estimates: tgDrives.some((d) => d.costs?.has_estimates),
      }
    }

    // Load group expenses from its drives
    const expPromises = tgDrives.map((d: any) => api.getDriveExpensesForDrive(vehicleStore.activeVehicle!.id, d.id).catch(() => []))
    const expResults = await Promise.all(expPromises)
    const allExp = expResults.flat()
    const uniqueExpMap = new Map()
    for (const e of allExp) {
      if (!uniqueExpMap.has(e.id)) uniqueExpMap.set(e.id, e)
    }
    driveExpenses.value = Array.from(uniqueExpMap.values())
    showCostModal.value = true
    showAddTollInline.value = false
    editingExpenseId.value = null
  } catch (err: any) {
    showAlert(`Erreur lors du chargement des détails : ${err.message}`, 'Erreur', 'danger')
  } finally {
    loadingExpenses.value = false
  }
}

async function openCostModal(drive: any) {
  selectedCostDrive.value = drive
  showCostModal.value = true
  showAddTollInline.value = false
  editingExpenseId.value = null
  inlineTollAmount.value = ''
  inlineTollNotes.value = ''
  loadDriveExpenses(drive.id)
}

async function loadDriveExpenses(driveId: string) {
  if (!vehicleStore.activeVehicle) return
  loadingExpenses.value = true
  try {
    driveExpenses.value = await api.getDriveExpensesForDrive(vehicleStore.activeVehicle.id, driveId)
  } catch (err) {
    console.error('Failed to load drive expenses', err)
    driveExpenses.value = []
  } finally {
    loadingExpenses.value = false
  }
}

async function handleAddTollToDrive() {
  if (!vehicleStore.activeVehicle || !selectedCostDrive.value) return
  if (!inlineTollAmount.value || Number(inlineTollAmount.value) <= 0) {
    showAlert('Veuillez entrer un montant valide', 'Montant invalide', 'warning')
    return
  }

  addingToll.value = true
  try {
    const amountNum = Number(inlineTollAmount.value)
    await api.createDriveExpense(vehicleStore.activeVehicle.id, {
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
  await Promise.all([loadDriveExpenses(driveId), loadDrives()])
  const refreshed = drives.value.find((d) => d.id === driveId)
  if (refreshed) selectedCostDrive.value = refreshed
}

function startEditExpense(exp: any) {
  editingExpenseId.value = exp.id
  expenseEditForm.value = { type: exp.type, amount: exp.amount, notes: exp.notes || '' }
}

async function handleSaveExpenseEdit(exp: any) {
  if (!vehicleStore.activeVehicle) return
  const amount = Number(expenseEditForm.value.amount)
  if (!amount || amount <= 0) {
    showAlert('Veuillez entrer un montant valide', 'Montant invalide', 'warning')
    return
  }
  try {
    await api.updateDriveExpense(vehicleStore.activeVehicle.id, exp.id, {
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
  if (!vehicleStore.activeVehicle) return
  const scope = exp.trip_group_id ? ` (frais du voyage « ${exp.trip_group_name} », tous les trajets du voyage sont concernés)` : ''
  const ok = await showConfirm({
    title: 'Supprimer le frais',
    message: `Supprimer ce frais de ${Number(exp.amount).toFixed(2)} ${exp.currency}${scope} ?`,
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(vehicleStore.activeVehicle.id, exp.id)
    await refreshCostModal()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('fr-FR', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Filter Tabs -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">Trajets & Voyages</h2>
        <p class="text-sm text-slate-400">
          {{ total }} trajets • Énergie et péages réels, usure et charges fixes réparties au kilomètre
        </p>
        <div class="flex items-center gap-1 mt-2 bg-slate-900 border border-slate-800 p-1 rounded-xl w-fit">
          <button
            @click="switchView('DRIVES')"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
            :class="viewMode === 'DRIVES' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
          >
            <List class="w-3.5 h-3.5" /> Trajets
          </button>
          <button
            @click="switchView('TRIPS')"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
            :class="viewMode === 'TRIPS' ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30' : 'text-slate-400 hover:text-white'"
          >
            <Layers class="w-3.5 h-3.5" /> Voyages
          </button>
        </div>
      </div>

      <!-- Tag Filters -->
      <div v-if="viewMode === 'DRIVES'" class="flex items-center gap-2 bg-slate-900 border border-slate-800 p-1 rounded-xl self-start sm:self-auto flex-wrap">
        <button
          v-if="unqualifiedCount > 0 || unqualifiedOnly"
          @click="unqualifiedOnly = !unqualifiedOnly"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1.5"
          :class="unqualifiedOnly ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' : 'text-amber-400/80 hover:text-amber-300'"
          title="Trajets de type autoroutier sans péage renseigné"
        >
          <AlertTriangle class="w-3.5 h-3.5" />
          À qualifier ({{ unqualifiedCount }})
        </button>
        <button
          @click="selectedTag = ''"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === '' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
        >
          Tous
        </button>
        <button
          @click="selectedTag = 'Pro'"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === 'Pro' ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30' : 'text-slate-400 hover:text-white'"
        >
          Pro
        </button>
        <button
          @click="selectedTag = 'Perso'"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
          :class="selectedTag === 'Perso' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'text-slate-400 hover:text-white'"
        >
          Perso
        </button>
      </div>
    </div>

    <!-- Filters & Navigation Toolbar (Drives Mode) -->
    <div v-if="viewMode === 'DRIVES'" class="space-y-3">
      <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-3 bg-slate-900 border border-slate-800 p-3 rounded-2xl">
        <!-- Period Mode & Navigation (Mix A + C) -->
        <div class="flex flex-wrap items-center gap-2">
          <!-- Period mode selector -->
          <div class="flex items-center bg-slate-950 p-1 rounded-xl border border-slate-800/80">
            <button
              @click="setPeriodMode('ALL')"
              class="px-2.5 py-1 rounded-lg text-xs font-semibold transition-colors"
              :class="periodMode === 'ALL' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
            >
              Tous
            </button>
            <button
              @click="setPeriodMode('MONTH')"
              class="px-2.5 py-1 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1"
              :class="periodMode === 'MONTH' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
            >
              <Calendar class="w-3.5 h-3.5" />
              Par mois
            </button>
            <button
              @click="setPeriodMode('CUSTOM')"
              class="px-2.5 py-1 rounded-lg text-xs font-semibold transition-colors"
              :class="periodMode === 'CUSTOM' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
            >
              Période
            </button>
          </div>

          <!-- Month Selector with Prev/Next buttons (when periodMode === 'MONTH') -->
          <div v-if="periodMode === 'MONTH'" class="flex items-center gap-1.5 bg-slate-950 px-2 py-1 rounded-xl border border-slate-800/80">
            <button
              @click="prevMonth"
              class="p-1 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
              title="Mois précédent"
            >
              <ChevronLeft class="w-4 h-4" />
            </button>
            <label for="drives-month-select" class="sr-only">Sélectionner le mois</label>
            <div class="w-40 sm:w-44">
              <AppDatePicker
                id="drives-month-select"
                v-model="selectedMonth"
                month-picker
                size="xs"
                :clearable="false"
                @change="onMonthChange"
              />
            </div>
            <button
              @click="nextMonth"
              class="p-1 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
              title="Mois suivant"
            >
              <ChevronRight class="w-4 h-4" />
            </button>
            <button
              v-if="!isCurrentMonth"
              @click="resetToCurrentMonth"
              class="text-[11px] text-rose-400 hover:text-rose-300 font-medium ml-1 px-1.5 py-0.5 bg-rose-500/10 rounded-md border border-rose-500/20"
              title="Revenir au mois en cours"
            >
              Ce mois
            </button>
          </div>

          <!-- Custom Date Range (when periodMode === 'CUSTOM') -->
          <div v-if="periodMode === 'CUSTOM'" class="flex items-center gap-2 bg-slate-950 px-2.5 py-1 rounded-xl border border-slate-800/80 text-xs">
            <label for="drives-filter-from" class="text-slate-400 shrink-0">Du</label>
            <div class="w-36">
              <AppDatePicker
                id="drives-filter-from"
                v-model="customFrom"
                size="xs"
                @change="onCustomDateChange"
              />
            </div>
            <label for="drives-filter-to" class="text-slate-400 shrink-0">Au</label>
            <div class="w-36">
              <AppDatePicker
                id="drives-filter-to"
                v-model="customTo"
                size="xs"
                @change="onCustomDateChange"
              />
            </div>
          </div>
        </div>

        <!-- Search Bar -->
        <div class="relative min-w-[240px] max-w-sm flex-1">
          <label for="drives-search-input" class="sr-only">Rechercher une ville ou une adresse</label>
          <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            id="drives-search-input"
            type="text"
            v-model="searchQuery"
            @input="onSearchInput"
            placeholder="Rechercher une ville, adresse..."
            class="w-full bg-slate-950 border border-slate-800 rounded-xl pl-9 pr-8 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
          />
          <button
            v-if="searchQuery"
            @click="clearSearch"
            class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-white p-0.5"
            title="Effacer la recherche"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <!-- Quick Metrics Summary for Current Selection -->
      <div v-if="total > 0 && !loading" class="flex flex-wrap items-center gap-3 text-xs text-slate-400 px-1">
        <span class="font-medium text-slate-300">
          <strong class="text-white">{{ total }}</strong> trajet(s) trouvé(s)
          <span v-if="periodMode === 'MONTH'">en <span class="text-rose-400 font-semibold">{{ formattedSelectedMonth }}</span></span>
        </span>
        <span class="text-slate-600">•</span>
        <span>Distance page : <strong class="text-white">{{ Math.round(pageDistance).toLocaleString('fr-FR') }} km</strong></span>
        <span class="text-slate-600">•</span>
        <span>Énergie page : <strong class="text-white">{{ Math.round(pageEnergy).toLocaleString('fr-FR') }} kWh</strong></span>
        <span class="text-slate-600">•</span>
        <span>Coût page : <strong class="text-white">{{ pageCost.toFixed(2) }} €</strong></span>
      </div>
    </div>

    <!-- Sticky Bulk Selection Bar -->
    <BulkSelectionBar
      v-if="vehicleStore.canEdit"
      :count="selectedDriveIds.length"
      item-label="trajet"
      :off-screen-count="selectedOffPage"
      :metrics-summary="selectedSummaryMetrics"
      @clear="clearSelection"
    >
      <!-- Direct Carpool button for single or multi-drives -->
      <button
        type="button"
        @click="handleCarpoolSelectedDrives"
        class="px-3 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-1.5 shadow-lg shadow-rose-600/25 transition-all"
      >
        <Users class="w-3.5 h-3.5" />
        <span>{{ selectedDriveIds.length > 1 ? `Covoiturer (${selectedDriveIds.length})` : 'Covoiturer' }}</span>
      </button>

      <!-- Fusion Voyage Group -->
      <button
        type="button"
        @click="showGroupModal = true"
        class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
      >
        <Layers class="w-3.5 h-3.5" />
        <span>Fusionner & Péage</span>
      </button>

      <!-- Batch Tag actions -->
      <button
        type="button"
        @click="handleBatchTag('Pro')"
        class="px-2.5 py-1.5 bg-blue-500/20 hover:bg-blue-500/30 text-blue-300 border border-blue-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
        title="Marquer la sélection comme trajets Pro"
      >
        <span>Pro</span>
      </button>

      <button
        type="button"
        @click="handleBatchTag('Perso')"
        class="px-2.5 py-1.5 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 border border-emerald-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
        title="Marquer la sélection comme trajets Perso"
      >
        <span>Perso</span>
      </button>

      <button
        type="button"
        @click="exportSelectedDrives"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
        title="Exporter la sélection en CSV"
      >
        <Download class="w-3.5 h-3.5 text-slate-300" />
        <span>Export</span>
      </button>

      <button
        type="button"
        @click="openAddToTrip"
        class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
      >
        <Plus class="w-3.5 h-3.5" />
        <span>Ajouter voyage</span>
      </button>
    </BulkSelectionBar>

    <template v-if="viewMode === 'DRIVES'">
    <!-- SKELETON LOADING STATE -->
    <div v-if="loading" class="space-y-3 animate-pulse">
      <div v-for="i in 5" :key="i" class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between gap-4">
        <div class="flex items-start gap-3 w-2/3">
          <div class="w-5 h-5 bg-slate-800 rounded mt-1"></div>
          <div class="space-y-2.5 w-full">
            <div class="flex items-center gap-2">
              <div class="h-3 w-24 bg-slate-800 rounded"></div>
              <div class="h-4 w-16 bg-slate-800 rounded-full"></div>
              <div class="h-3 w-16 bg-slate-800 rounded"></div>
            </div>
            <div class="h-4 w-4/5 bg-slate-800/80 rounded"></div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <div class="h-8 w-24 bg-slate-800 rounded-xl"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
        </div>
      </div>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="!drives.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-3">
      <p>Aucun trajet trouvé pour cette sélection ou période.</p>
      <button
        v-if="searchQuery || periodMode !== 'ALL' || selectedTag || unqualifiedOnly"
        @click="resetAllFilters"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 transition-colors"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        Réinitialiser les filtres
      </button>
    </div>

    <!-- REAL DRIVES LIST -->
    <div v-else class="space-y-3">
      <!-- Select all toggle & Total info -->
      <div class="flex items-center justify-between text-xs text-slate-400 px-2">
        <button v-if="vehicleStore.canEdit" @click="selectAll" class="flex items-center gap-2 hover:text-slate-200 transition-colors">
          <component :is="allPageSelected ? CheckSquare : Square" class="w-4 h-4 text-rose-400" />
          <span>{{ allPageSelected ? 'Désélectionner la page' : 'Sélectionner la page' }}</span>
        </button>
        <span v-else></span>
        <span>Page {{ page }} sur {{ totalPages }}</span>
      </div>

      <!-- Drive Card -->
      <div
        v-for="d in drives"
        :key="d.id"
        @click="openCostModal(d)"
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 cursor-pointer group"
        :class="{ 'border-rose-500/40 bg-slate-800/40 shadow-lg shadow-rose-950/20': selectedDriveIds.includes(d.id) }"
      >
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Selection checkbox -->
          <label
            v-if="vehicleStore.canEdit"
            :for="'drive-select-' + d.id"
            @click.stop
            class="mt-1 shrink-0 flex items-center cursor-pointer"
            title="Sélectionner ce trajet"
          >
            <span class="sr-only">Sélectionner ce trajet</span>
            <input
              :id="'drive-select-' + d.id"
              type="checkbox"
              :checked="selectedDriveIds.includes(d.id)"
              @change="toggleSelectDrive(d)"
              class="w-5 h-5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-950 border-slate-700 cursor-pointer"
            />
          </label>

          <!-- Drive Details -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap mb-1.5">
              <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatDate(d.start_time) }}</span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
                {{ d.distance_km }} km
              </span>
              <span v-if="d.duration_min" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
                <Clock class="w-3 h-3" /> {{ d.duration_min }} min
              </span>
              <span v-if="d.consumption_kwh_100km" class="text-xs text-sky-400 font-mono shrink-0">
                {{ d.consumption_kwh_100km }} kWh/100km
              </span>
              <!-- Clean tag pills -->
              <span
                v-if="d.tags?.includes('Pro')"
                class="text-[10px] px-2 py-0.5 rounded-full font-bold bg-blue-500/20 text-blue-400 border border-blue-500/40 shrink-0"
              >
                Pro
              </span>
              <span
                v-if="d.tags?.includes('Perso')"
                class="text-[10px] px-2 py-0.5 rounded-full font-bold bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 shrink-0"
              >
                Perso
              </span>
            </div>

            <!-- Route Address -->
            <div class="text-sm text-slate-300 flex items-center gap-1.5 flex-wrap min-w-0">
              <MapPin class="w-3.5 h-3.5 text-rose-400 shrink-0" />
              <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.start_address">{{ d.start_address || 'Départ inconnu' }}</span>
              <span class="text-slate-500 shrink-0">→</span>
              <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.end_address">{{ d.end_address || 'Arrivée inconnue' }}</span>
            </div>
          </div>
        </div>

        <!-- Right Side: Cost Badge & Actions -->
        <div class="flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0" @click.stop>
          <!-- Toll qualification: 2 taps -->
          <div v-if="vehicleStore.canEdit && needsTollQualification(d)" class="flex items-center gap-1">
            <button
              @click="openTollEntry(d)"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-amber-500/40 bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 flex items-center gap-1"
              title="Renseigner le péage de ce trajet"
            >
              <Plus class="w-3.5 h-3.5" /> Péage
            </button>
            <button
              @click="markNoToll(d)"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-400 hover:text-white flex items-center gap-1"
              title="Confirmer que ce trajet n'a pas de péage"
            >
              <Ban class="w-3.5 h-3.5" /> Sans péage
            </button>
          </div>

          <!-- Real Cost Badge -->
          <div
            class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
            title="Coût de revient réel calculé pour ce trajet"
          >
            <div class="p-1 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Coins class="w-3.5 h-3.5" />
            </div>
            <div>
              <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
                <span>{{ d.costs?.has_estimates ? '~' : '' }}{{ (d.costs?.total_cost || 0).toFixed(2) }} €</span>
                <span class="text-[10px] font-normal text-emerald-400 font-mono">
                  {{ (d.costs?.cost_per_km || 0).toFixed(3) }} €/km
                </span>
              </div>
            </div>
          </div>

          <!-- Quick Carpool Button -->
          <button
            v-if="vehicleStore.canEdit"
            @click="router.push({ path: '/carpools', query: { new_drive_id: d.id } })"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
            title="Créer un covoiturage depuis ce trajet"
          >
            <Users class="w-3.5 h-3.5 text-rose-500" />
            <span class="hidden md:inline">Covoiturer</span>
          </button>
        </div>
      </div>

      <!-- Modern Pagination Controls -->
      <div class="flex flex-col md:flex-row items-center justify-between gap-4 pt-4 border-t border-slate-800/80">
        <!-- Left: Range display & Page size buttons -->
        <div class="flex flex-wrap items-center gap-3 text-xs text-slate-400">
          <span>
            Affichage <strong class="text-white">{{ itemRangeStart }}</strong>–<strong class="text-white">{{ itemRangeEnd }}</strong> sur <strong class="text-white">{{ total }}</strong> trajets
          </span>
          <div class="flex items-center gap-1.5 border-l border-slate-800 pl-3">
            <span class="text-slate-500">Par page :</span>
            <button
              v-for="s in [20, 50, 100]"
              :key="s"
              @click="setLimit(s)"
              class="px-2 py-0.5 rounded-lg text-xs font-semibold transition-colors"
              :class="limit === s ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white bg-slate-800/60'"
            >
              {{ s }}
            </button>
          </div>
        </div>

        <!-- Center / Right: Numbered buttons + quick jump -->
        <div class="flex items-center gap-1.5 flex-wrap justify-center">
          <!-- First page -->
          <button
            @click="goToPage(1)"
            :disabled="page <= 1"
            title="Première page"
            class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronsLeft class="w-4 h-4" />
          </button>
          <!-- Prev page -->
          <button
            @click="goToPage(page - 1)"
            :disabled="page <= 1"
            title="Page précédente"
            class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>

          <!-- Numbered pages with ellipses -->
          <template v-for="(p, idx) in paginationPages" :key="idx">
            <span v-if="p === '...'" class="px-1 text-xs text-slate-500 font-bold">...</span>
            <button
              v-else
              @click="goToPage(p as number)"
              class="min-w-[32px] h-8 px-2 rounded-xl text-xs font-semibold transition-all"
              :class="page === p ? 'bg-rose-600 text-white font-bold shadow-md shadow-rose-600/30' : 'bg-slate-900 border border-slate-800 text-slate-300 hover:text-white hover:border-slate-700'"
            >
              {{ p }}
            </button>
          </template>

          <!-- Next page -->
          <button
            @click="goToPage(page + 1)"
            :disabled="page >= totalPages"
            title="Page suivante"
            class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronRight class="w-4 h-4" />
          </button>
          <!-- Last page -->
          <button
            @click="goToPage(totalPages)"
            :disabled="page >= totalPages"
            title="Dernière page"
            class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronsRight class="w-4 h-4" />
          </button>

          <!-- Direct jump input -->
          <div v-if="totalPages > 1" class="flex items-center gap-1 ml-2 border-l border-slate-800 pl-2">
            <label for="drives-jump-page" class="text-xs text-slate-500">Page</label>
            <input
              id="drives-jump-page"
              type="number"
              min="1"
              :max="totalPages"
              v-model="jumpInput"
              @keydown.enter="applyJump"
              placeholder="N°"
              class="w-12 px-1.5 py-1 text-xs bg-slate-950 border border-slate-800 rounded-lg text-white text-center focus:border-rose-500 outline-none"
            />
            <button
              @click="applyJump"
              :disabled="!jumpInput"
              class="px-2 py-1 text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg disabled:opacity-30 disabled:cursor-not-allowed transition-colors font-medium"
            >
              Aller
            </button>
          </div>
        </div>
      </div>
    </div>
    </template>

    <!-- TRIP GROUPS ("VOYAGES") -->
    <div v-else class="space-y-3">
      <div v-if="loadingTrips" class="text-center py-12 text-slate-400">Chargement...</div>
      <div v-else-if="!tripGroups.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
        Aucun voyage. Sélectionnez plusieurs trajets puis « Fusionner & Péage » pour en créer un.
      </div>
      <template v-else>
      <div
        v-for="tg in tripGroups"
        :key="tg.id"
        @click="openTripCostModal(tg)"
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 cursor-pointer group"
      >
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Icon indicator -->
          <div class="mt-1 p-2 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0">
            <Layers class="w-4 h-4" />
          </div>

          <!-- Voyage Details (matching Drive details structure) -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap mb-1.5">
              <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatTripDates(tg) }}</span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
                {{ Math.round(tg.distance_km).toLocaleString('fr-FR') }} km
              </span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/30 shrink-0">
                {{ tg.drive_ids.length }} étape(s)
              </span>
              <span v-if="tg.carpool_count" class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-rose-500/10 text-rose-300 border border-rose-500/30 shrink-0">
                {{ tg.carpool_count }} covoit
              </span>
            </div>

            <!-- Voyage Title & Notes (matching Route address line) -->
            <div class="text-sm text-slate-200 flex items-center gap-2 flex-wrap min-w-0">
              <span class="font-bold text-white truncate max-w-sm sm:max-w-md" :title="tg.name">{{ tg.name }}</span>
              <span v-if="tg.notes" class="text-xs text-slate-500 truncate max-w-xs">• {{ tg.notes }}</span>
            </div>
          </div>
        </div>

        <!-- Right Side: Cost Badge & Actions (matching Drive right-side) -->
        <div class="flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0" @click.stop>
          <!-- Real Cost Badge -->
          <div
            class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
            title="Coût consolidé du voyage (cliquez sur la ligne pour le détail)"
          >
            <div class="p-1 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Coins class="w-3.5 h-3.5" />
            </div>
            <div>
              <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
                <span>{{ Number(tg.expenses_total || 0) > 0 ? `${Number(tg.expenses_total).toFixed(2)} € frais` : 'Détail coûts' }}</span>
                <span v-if="tg.distance_km > 0 && tg.expenses_total" class="text-[10px] font-normal text-emerald-400 font-mono">
                  {{ (Number(tg.expenses_total) / tg.distance_km).toFixed(3) }} €/km
                </span>
              </div>
            </div>
          </div>

          <!-- Details toggle button -->
          <button
            @click="toggleTripDetails(tg)"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white transition-colors"
          >
            {{ expandedTripId === tg.id ? 'Masquer' : 'Étapes' }}
          </button>

          <!-- Quick Carpool Button -->
          <button
            v-if="vehicleStore.canEdit"
            @click="router.push({ path: '/carpools', query: { new_trip_group_id: tg.id } })"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
            title="Covoiturer ce voyage"
          >
            <Users class="w-3.5 h-3.5 text-rose-500" />
            <span class="hidden md:inline">Covoiturer</span>
          </button>

          <!-- Edit & Delete -->
          <template v-if="vehicleStore.canEdit">
            <button
              @click="openTripEdit(tg)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-indigo-400 rounded-lg border border-slate-700/60 transition-colors"
              title="Renommer le voyage"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="handleDeleteTrip(tg)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-lg border border-slate-700/60 transition-colors"
              title="Supprimer le voyage"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </template>
        </div>
      </div>
      <!-- Expanded drives drawer -->
      <div v-if="expandedTripId === tg.id" class="space-y-1.5 bg-slate-950/40 border border-slate-800/80 rounded-2xl p-3 -mt-1 ml-4 mr-4">
        <div v-for="d in tripDrives" :key="d.id" class="flex items-center justify-between gap-3 text-xs text-slate-300 bg-slate-800/40 rounded-lg px-2.5 py-1.5 min-w-0">
          <span class="truncate min-w-0 flex-1">
            {{ formatDate(d.start_time) }} : {{ (d.start_address || 'Départ').split(',')[0] }} → {{ (d.end_address || 'Arrivée').split(',')[0] }}
            <span class="text-slate-500">({{ d.distance_km }} km)</span>
          </span>
          <button v-if="vehicleStore.canEdit" @click="removeDriveFromTrip(tg, d.id)" class="text-slate-500 hover:text-rose-400 shrink-0 p-1" title="Retirer ce trajet du voyage">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
        <p v-if="!tripDrives.length" class="text-xs text-slate-500">Chargement des trajets...</p>
      </div>
      </template>
    </div>

    <!-- MODAL : DÉCOMPOSITION COMPLÈTE DU COÛT D'UN TRAJET -->
    <div
      v-if="showCostModal && selectedCostDrive"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showCostModal = false"
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
          <button @click="showCostModal = false" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0">
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
                @click="toggleDriveTag(selectedCostDrive, 'Pro')"
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
                @click="toggleDriveTag(selectedCostDrive, 'Perso')"
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
            @click="showCostModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            Fermer
          </button>
          <button
            @click="showCostModal = false; router.push({ path: '/carpools', query: selectedCostDrive.is_trip_group ? { new_trip_group_id: selectedCostDrive.id } : { new_drive_id: selectedCostDrive.id } })"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
          >
            <Users class="w-4 h-4" />
            <span>Partager en covoiturage</span>
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL : FUSIONNER EN VOYAGE & ASSIGNER PÉAGE -->
    <div
      v-if="showGroupModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showGroupModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <div class="flex items-center gap-2">
            <Layers class="w-5 h-5 text-rose-400" />
            <h3 class="text-base font-bold text-white">Créer un Voyage / Fusion</h3>
            <span class="px-2 py-0.5 bg-rose-500/10 text-rose-300 text-xs font-semibold rounded-lg border border-rose-500/20">
              {{ selectedDriveIds.length }} trajets
            </span>
          </div>
          <button @click="showGroupModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">

          <div>
            <label for="drive-group-name" class="block text-xs font-semibold text-slate-300 mb-1">Nom du voyage / groupe</label>
            <input id="drive-group-name"
              v-model="groupName"
              type="text"
              placeholder="ex: Vacances Bretagne - Aller"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="drive-expense-type" class="block text-xs font-semibold text-slate-300 mb-1">Type de frais</label>
              <select id="drive-expense-type"
                v-model="expenseType"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              >
                <option value="TOLL">Péage</option>
                <option value="PARKING">Parking</option>
                <option value="FERRY">Ferry</option>
              </select>
            </div>
            <div>
              <label for="drive-toll-amount" class="block text-xs font-semibold text-slate-300 mb-1">Montant (€)</label>
              <input id="drive-toll-amount"
                v-model="tollAmount"
                type="number"
                step="0.01"
                placeholder="0.00"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              />
            </div>
          </div>
        </div>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
          <button
            type="button"
            @click="showGroupModal = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            Annuler
          </button>
          <button
            type="button"
            @click="handleCreateGroupAndExpense"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/20 transition-colors"
          >
            Enregistrer le groupe
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL : RENOMMER UN VOYAGE -->
    <div
      v-if="showTripEditModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showTripEditModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2">
            <Layers class="w-5 h-5 text-indigo-400" />
            Modifier le voyage
          </h3>
          <button @click="showTripEditModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="trip-edit-form" @submit.prevent="handleSaveTripEdit" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <div>
            <label for="trip-edit-name" class="block text-xs font-semibold text-slate-300 mb-1">Nom</label>
            <input id="trip-edit-name" v-model="tripEditForm.name" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
          </div>
          <div>
            <label for="trip-edit-notes" class="block text-xs font-semibold text-slate-300 mb-1">Notes</label>
            <input id="trip-edit-notes" v-model="tripEditForm.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showTripEditModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="trip-edit-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
            Enregistrer
          </button>
        </div>
      </div>
    </div>

    <!-- MODAL : AJOUTER LA SÉLECTION À UN VOYAGE -->
    <div
      v-if="showAddToTripModal"
      class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
      @click.self="showAddToTripModal = false"
    >
      <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
        <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
          <h3 class="text-base font-bold text-white flex items-center gap-2 truncate pr-2">
            <Plus class="w-5 h-5 text-indigo-400 shrink-0" />
            <span class="truncate">Ajouter {{ selectedDriveIds.length }} trajet(s) à un voyage</span>
          </h3>
          <button @click="showAddToTripModal = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form id="add-to-trip-form" @submit.prevent="handleAddToTrip" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
          <p v-if="!tripGroups.length" class="text-xs text-slate-400">Aucun voyage existant : utilisez « Fusionner & Péage » pour en créer un.</p>
          <div v-else>
            <label for="add-to-trip" class="block text-xs font-semibold text-slate-300 mb-1">Voyage</label>
            <select id="add-to-trip" v-model="addToTripId" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500">
              <option v-for="tg in tripGroups" :key="tg.id" :value="tg.id">{{ tg.name }} ({{ tg.drive_ids.length }} trajets)</option>
            </select>
          </div>
        </form>

        <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
          <button type="button" @click="showAddToTripModal = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button type="submit" form="add-to-trip-form" :disabled="!tripGroups.length" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-40 text-white text-xs font-semibold rounded-xl transition-colors">
            Ajouter
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
