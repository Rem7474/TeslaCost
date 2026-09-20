<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import DrivesToolbar from '@/components/drives/DrivesToolbar.vue'
import DriveBulkActions from '@/components/drives/DriveBulkActions.vue'
import DriveCard from '@/components/drives/DriveCard.vue'
import DrivesPagination from '@/components/drives/DrivesPagination.vue'
import TripGroupsPanel from '@/components/drives/TripGroupsPanel.vue'
import DriveCostModal from '@/components/drives/DriveCostModal.vue'
import DriveGroupModal from '@/components/drives/DriveGroupModal.vue'
import TripRenameModal from '@/components/drives/TripRenameModal.vue'
import AddToTripModal from '@/components/drives/AddToTripModal.vue'
import { downloadCsv } from '@/utils/csv'
import { CheckSquare, Square, Receipt, Layers, AlertTriangle, List, RotateCcw } from 'lucide-vue-next'
import {
  DRIVE_CSV_HEADERS,
  applyBatchTag,
  buildTripCostDrive,
  currentYearMonth,
  driveCsvRows,
  monthRange,
  selectionSummary,
  toggleTag,
} from '@/utils/drives'

// The page owns the drives list, the filters, the selection and which modal is open; the toolbar, the cards,
// the pagination and the modals are components that report back to it.
const router = useRouter()
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
const drives = ref<any[]>([])
const total = ref(0)
const page = ref(1)

// Page limit with persistent storage
const savedLimit = Number(localStorage.getItem('drives_limit'))
const limit = ref(savedLimit === 20 || savedLimit === 50 || savedLimit === 100 ? savedLimit : 20)
const totalPages = computed(() => Math.ceil(total.value / limit.value) || 1)
const selectedTag = ref('')
const unqualifiedOnly = ref(false)
const hasTollOnly = ref(false)
const tollSource = ref('')
const unqualifiedCount = ref(0)
const loading = ref(true)

// Period / month and address filters (edited in the toolbar)
const periodMode = ref<'ALL' | 'MONTH' | 'CUSTOM'>('ALL')
const selectedMonth = ref(currentYearMonth())
const customFrom = ref('')
const customTo = ref('')
const searchQuery = ref('')

function onFiltersChange() {
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

function resetAllFilters() {
  searchQuery.value = ''
  selectedTag.value = ''
  unqualifiedOnly.value = false
  hasTollOnly.value = false
  tollSource.value = ''
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
const selectedSummaryMetrics = computed(() => selectionSummary(selectedList.value))

function clearSelection() {
  selectedDrives.value = {}
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

// Batch tagging
async function handleBatchTag(tag: 'Pro' | 'Perso' | null) {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  const ids = [...selectedDriveIds.value]
  try {
    for (const id of ids) {
      const d = selectedDrives.value[id] || drives.value.find((x) => x.id === id)
      const currentTags = applyBatchTag(d?.tags, tag)
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
  downloadCsv(`trajets_export_${new Date().toISOString().slice(0, 10)}.csv`, DRIVE_CSV_HEADERS, driveCsvRows(selectedList.value))
}

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
      const range = monthRange(selectedMonth.value)
      fromStr = range.from
      toStr = range.to
    } else if (periodMode.value === 'CUSTOM') {
      if (customFrom.value) fromStr = customFrom.value
      if (customTo.value) toStr = customTo.value
    }

    const res = await api.getDrives(vehicleStore.activeVehicle.id, {
      tag: selectedTag.value,
      page: page.value,
      limit: limit.value,
      unqualified: unqualifiedOnly.value,
      hasToll: hasTollOnly.value,
      tollSource: hasTollOnly.value ? tollSource.value : '',
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

// View mode: drives list or trip groups ("voyages")
const viewMode = ref<'DRIVES' | 'TRIPS'>('DRIVES')
const tripGroups = ref<any[]>([])
const loadingTrips = ref(false)
const expandedTripId = ref<string | null>(null)
const tripDrives = ref<any[]>([])

// Modals
const showTripEditModal = ref(false)
const tripBeingEdited = ref<any | null>(null)
const showAddToTripModal = ref(false)
const showGroupModal = ref(false)
const showCostModal = ref(false)
const selectedCostDrive = ref<any | null>(null)
const costTripDriveIds = ref<string[]>([])
const costStartWithToll = ref(false)
const bulkApplyingToll = ref(false)

watch(
  () => [vehicleStore.activeVehicle?.id, selectedTag.value, unqualifiedOnly.value, hasTollOnly.value, tollSource.value, vehicleStore.lastSyncTimestamp],
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
  openCostModal(d, true)
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
  tripBeingEdited.value = tg
  showTripEditModal.value = true
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
  showAddToTripModal.value = true
}

async function onAddedToTrip() {
  clearSelection()
  await loadTripGroups()
  loadDrives()
}

function onGroupCreated() {
  clearSelection()
  loadDrives()
}

async function toggleDriveTag(drive: any, tagToToggle: string) {
  if (!vehicleStore.activeVehicle) return
  const currentTags = toggleTag(drive.tags, tagToToggle)
  try {
    await api.updateDriveTags(vehicleStore.activeVehicle.id, drive.id, currentTags)
    drive.tags = currentTags
  } catch (err: any) {
    showAlert(`Erreur de mise à jour du tag : ${err.message}`, 'Erreur', 'danger')
  }
}

async function handleCarpoolSelectedDrives() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  // Each selected drive becomes a leg of the carpool, in chronological order
  const ids = selectedList.value.map((d: any) => d.id)
  clearSelection()
  router.push({ path: '/carpools', query: { new_drive_ids: ids.join(',') } })
}

// Cost breakdown modal
async function openTripCostModal(tg: any) {
  if (!vehicleStore.activeVehicle) return
  try {
    const res = await api.getDrives(vehicleStore.activeVehicle.id, { tripGroupId: tg.id, limit: 200 })
    const tgDrives = [...res.drives].sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
    selectedCostDrive.value = buildTripCostDrive(tg, tgDrives)
    costTripDriveIds.value = tgDrives.map((d: any) => d.id)
    showCostModal.value = true
  } catch (err: any) {
    showAlert(`Erreur lors du chargement des détails : ${err.message}`, 'Erreur', 'danger')
  }
}

function openCostModal(drive: any, startWithToll = false) {
  selectedCostDrive.value = drive
  costStartWithToll.value = startWithToll
  showCostModal.value = true
}

// Reloads the list and returns the refreshed drive, so the open cost breakdown follows the server-side costs
async function refreshCostDrive(driveId: string) {
  await loadDrives()
  return drives.value.find((d) => d.id === driveId) ?? null
}

async function handleBulkApplyToll() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  const ok = await showConfirm({
    title: 'Appliquer le péage automatique',
    message: `Détecter les péages de ${selectedDriveIds.value.length} trajet(s) et enregistrer le tarif estimé ? Les péages saisis manuellement ne sont jamais modifiés.`,
    confirmText: 'Appliquer',
    type: 'info',
  })
  if (!ok) return
  bulkApplyingToll.value = true
  try {
    const r = await api.applyTollEstimatesBulk(vehicleStore.activeVehicle.id, selectedDriveIds.value)
    const skipped = r.skipped_manual + r.skipped_trip_group + r.skipped_no_price + r.skipped_no_gps
    showAlert(
      `${r.created} créé(s), ${r.updated} mis à jour, ${skipped} ignoré(s) (manuel: ${r.skipped_manual}, voyage: ${r.skipped_trip_group}, sans tarif: ${r.skipped_no_price}, sans GPS: ${r.skipped_no_gps})${r.failed ? `, ${r.failed} en erreur` : ''}.`,
      'Péage automatique',
      r.failed ? 'warning' : 'success'
    )
    await loadDrives()
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    bulkApplyingToll.value = false
  }
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
          @click="hasTollOnly = !hasTollOnly"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1.5"
          :class="hasTollOnly ? 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30' : 'text-cyan-400/80 hover:text-cyan-300'"
          title="Trajets ayant une dépense de péage"
        >
          <Receipt class="w-3.5 h-3.5" />
          Avec péage
        </button>
        <template v-if="hasTollOnly">
          <label for="drives-toll-source" class="sr-only">Origine du péage</label>
          <select
            id="drives-toll-source"
            v-model="tollSource"
            class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs text-white focus:outline-none focus:border-cyan-500"
          >
            <option value="">Tous</option>
            <option value="AUTO_TOLL">Auto</option>
            <option value="MANUAL">Manuel</option>
          </select>
        </template>
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
    <DrivesToolbar
      v-if="viewMode === 'DRIVES'"
      v-model:period-mode="periodMode"
      v-model:selected-month="selectedMonth"
      v-model:custom-from="customFrom"
      v-model:custom-to="customTo"
      v-model:search-query="searchQuery"
      :total="total"
      :loading="loading"
      :drives="drives"
      @change="onFiltersChange"
    />

    <!-- Sticky Bulk Selection Bar -->
    <DriveBulkActions
      :selected-drive-ids="selectedDriveIds"
      :selected-off-page="selectedOffPage"
      :selected-summary-metrics="selectedSummaryMetrics"
      :bulk-applying-toll="bulkApplyingToll"
      @clear="clearSelection"
      @carpool="handleCarpoolSelectedDrives"
      @group="showGroupModal = true"
      @bulk-toll="handleBulkApplyToll"
      @tag="handleBatchTag"
      @export="exportSelectedDrives"
      @add-to-trip="openAddToTrip"
    />

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
        v-if="searchQuery || periodMode !== 'ALL' || selectedTag || unqualifiedOnly || hasTollOnly"
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

      <DriveCard
        v-for="d in drives"
        :key="d.id"
        :d="d"
        :selected="selectedDriveIds.includes(d.id)"
        @open="openCostModal"
        @toggle="toggleSelectDrive"
        @toll-entry="openTollEntry"
        @no-toll="markNoToll"
      />

      <DrivesPagination
        :page="page"
        :limit="limit"
        :total="total"
        :total-pages="totalPages"
        @go-to-page="goToPage"
        @set-limit="setLimit"
      />
    </div>
    </template>

    <!-- TRIP GROUPS ("VOYAGES") -->
    <TripGroupsPanel
      v-else
      :loading-trips="loadingTrips"
      :trip-groups="tripGroups"
      :expanded-trip-id="expandedTripId"
      :trip-drives="tripDrives"
      @open-cost="openTripCostModal"
      @toggle-details="toggleTripDetails"
      @edit="openTripEdit"
      @delete="handleDeleteTrip"
      @remove-drive="removeDriveFromTrip"
    />

    <DriveCostModal
      v-model:open="showCostModal"
      v-model:drive="selectedCostDrive"
      :vehicle-id="vehicleId"
      :trip-drive-ids="costTripDriveIds"
      :start-with-toll-entry="costStartWithToll"
      :refresh-drive="refreshCostDrive"
      @toggle-tag="toggleDriveTag"
    />

    <DriveGroupModal
      v-model:open="showGroupModal"
      :vehicle-id="vehicleId"
      :selected-drive-ids="selectedDriveIds"
      :selected-list="selectedList"
      @saved="onGroupCreated"
    />

    <TripRenameModal
      v-model:open="showTripEditModal"
      :vehicle-id="vehicleId"
      :trip="tripBeingEdited"
      @saved="loadTripGroups"
    />

    <AddToTripModal
      v-model:open="showAddToTripModal"
      :vehicle-id="vehicleId"
      :trip-groups="tripGroups"
      :selected-drive-ids="selectedDriveIds"
      @saved="onAddedToTrip"
    />
  </div>
</template>
