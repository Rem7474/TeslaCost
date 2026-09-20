<script setup lang="ts">
import { t } from '@/i18n'
import { APP_NAME } from '@/brand'
import { ref, onMounted, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import CarpoolSummaryGrid from '@/components/carpool/CarpoolSummaryGrid.vue'
import CarpoolTripList from '@/components/carpool/CarpoolTripList.vue'
import CarpoolTripModal from '@/components/carpool/CarpoolTripModal.vue'
import { downloadCsv } from '@/utils/csv'
import { carpoolCsvHeaders, carpoolCsvRows } from '@/utils/carpool'
import { Users, Plus } from 'lucide-vue-next'

// The page loads the trips and owns the selection and which trip the modal edits; the summary, the list and the
// form are components. Other pages open it with a drive, drives or a trip group in the query string.
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const route = useRoute()
const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')

const loading = ref(true)
const trips = ref<any[]>([])
const summary = ref<any>({
  total_trips: 0,
  total_passengers: 0,
  total_distance_km: 0,
  total_real_cost: 0,
  total_revenue: 0,
  total_net_cost: 0,
  coverage_rate_pct: 0,
  net_cost_per_km: 0,
  total_passengers_share: 0,
  total_driver_share: 0,
})

// Create / edit modal
const showModal = ref(false)
const editingTrip = ref<any | null>(null)
const createOptions = ref<{ driveIds?: string[]; tripGroupId?: string }>({})
const openToken = ref(0)

// Batch selection state
const selectedTripIds = ref<string[]>([])
const recalculating = ref(false)

const isAllSelected = computed(() => {
  return trips.value.length > 0 && selectedTripIds.value.length === trips.value.length
})

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedTripIds.value = []
  } else {
    selectedTripIds.value = trips.value.map((t) => t.id)
  }
}

function toggleTripSelection(tripId: string) {
  const idx = selectedTripIds.value.indexOf(tripId)
  if (idx > -1) {
    selectedTripIds.value.splice(idx, 1)
  } else {
    selectedTripIds.value.push(tripId)
  }
}

function clearTripSelection() {
  selectedTripIds.value = []
}

function exportSelectedCarpools() {
  const selected = trips.value.filter((t) => selectedTripIds.value.includes(t.id))
  if (!selected.length) return
  downloadCsv(`${t('carpool.carpoolView.csvFileName')}_${new Date().toISOString().slice(0, 10)}.csv`, carpoolCsvHeaders(), carpoolCsvRows(selected))
}

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    const res = await api.getCarpools(vehicleStore.activeVehicle.id)
    trips.value = res.trips || []
    summary.value = res.summary || summary.value
  } catch (err) {
    console.error('Failed to load carpool trips', err)
  } finally {
    loading.value = false
  }
}

function openCreateModal(options: { driveIds?: string[]; tripGroupId?: string } = {}) {
  editingTrip.value = null
  createOptions.value = options
  openToken.value++
  showModal.value = true
}

function openEditModal(trip: any) {
  editingTrip.value = trip
  openToken.value++
  showModal.value = true
}

async function handleDelete(trip: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('carpool.carpoolView.deleteTitle'),
    message: t('carpool.carpoolView.deleteMessage', { title: trip.title }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteCarpool(vehicleStore.activeVehicle.id, trip.id)
    await loadData()
  } catch (err: any) {
    showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function handleRecalculateSingle(trip: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('carpool.carpoolView.recalcTitle'),
    message: t('carpool.carpoolView.recalcMessage', { title: trip.title }),
    confirmText: t('carpool.carpoolView.recalc'),
    type: 'info',
  })
  if (!ok) return

  recalculating.value = true
  try {
    await api.recalculateCarpools(vehicleStore.activeVehicle.id, [trip.id])
    showAlert(t('carpool.carpoolView.recalcDone', { title: trip.title }), t('carpool.carpoolView.recalcDoneTitle'), 'success')
    await loadData()
  } catch (err: any) {
    showAlert(t('carpool.carpoolView.recalcError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    recalculating.value = false
  }
}

async function handleBatchRecalculate() {
  if (!vehicleStore.activeVehicle || !selectedTripIds.value.length) return
  const count = selectedTripIds.value.length
  const ok = await showConfirm({
    title: t('carpool.carpoolView.recalcSelectedTitle'),
    message: t('carpool.carpoolView.recalcSelectedMessage', { count }),
    confirmText: t('carpool.carpoolView.recalc'),
    type: 'info',
  })
  if (!ok) return

  recalculating.value = true
  try {
    const res = await api.recalculateCarpools(vehicleStore.activeVehicle.id, selectedTripIds.value)
    showAlert(t('carpool.carpoolView.recalcSelectedDone', { count: res.updated_count || count }), t('carpool.carpoolView.recalcDoneTitle'), 'success')
    clearTripSelection()
    await loadData()
  } catch (err: any) {
    showAlert(t('carpool.carpoolView.recalcError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    recalculating.value = false
  }
}

watch(
  () => vehicleStore.activeVehicle?.id,
  () => clearTripSelection()
)

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => loadData()
)

function checkRouteQueryForCarpool() {
  const driveIds = [route.query.new_drive_id, ...String(route.query.new_drive_ids || '').split(',')]
    .map((v) => String(v || '').trim())
    .filter(Boolean)
  const tripGroupId = route.query.new_trip_group_id as string
  if (tripGroupId) openCreateModal({ tripGroupId })
  else if (driveIds.length) openCreateModal({ driveIds })
}

watch(
  () => [route.query.new_drive_id, route.query.new_drive_ids, route.query.new_trip_group_id],
  () => checkRouteQueryForCarpool()
)

onMounted(() => {
  loadData()
  checkRouteQueryForCarpool()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white flex items-center gap-2.5">
          <Users class="w-6 h-6 text-rose-500" />
          {{ $t('carpool.carpoolView.carpoolingAndBlablacar') }}
        </h2>
        <p class="text-sm text-slate-400">
          {{ $t('carpool.carpoolView.multiLegTripsPassengersGetting') }}
        </p>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20 transition-all self-start sm:self-auto"
      >
        <Plus class="w-4 h-4" />
        {{ $t('carpool.carpoolView.newCarpool') }}
      </button>
    </div>

    <!-- Viewer mode banner -->
    <div
      v-if="!vehicleStore.canEdit"
      class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex items-center gap-3 text-xs text-slate-400"
    >
      <Users class="w-4 h-4 text-slate-400 shrink-0" />
      <span>{{ $t('carpool.carpoolView.youAreViewingThisVehicle') }} <strong>{{ $t('carpool.carpoolView.readOnly') }}</strong>{{ $t('carpool.carpoolView.modeCreatingAndEditingCarpools') }}</span>
    </div>

    <!-- KPI Summary Grid -->
    <CarpoolSummaryGrid :summary="summary" />

    <!-- Loading -->
    <div v-if="loading" class="space-y-4 animate-pulse">
      <div v-for="i in 2" :key="i" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 h-48"></div>
    </div>

    <!-- Empty State -->
    <div v-else-if="trips.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center max-w-2xl mx-auto space-y-4">
      <div class="w-16 h-16 bg-rose-500/10 border border-rose-500/20 text-rose-400 rounded-2xl flex items-center justify-center mx-auto">
        <Users class="w-8 h-8" />
      </div>
      <div>
        <h3 class="text-lg font-bold text-white">{{ $t('carpool.carpoolView.noCarpooledTripYet') }}</h3>
        <p class="text-sm text-slate-400 mt-1 max-w-md mx-auto">
          {{ $t('carpool.carpoolView.selectTheLegsOfYour', { APP_NAME }) }}
        </p>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        @click="openCreateModal()"
        class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2.5 rounded-xl inline-flex items-center gap-2 shadow-lg shadow-rose-600/20"
      >
        <Plus class="w-4 h-4" />
        {{ $t('carpool.carpoolView.createMyFirstCarpool') }}
      </button>
    </div>

    <!-- Trips List -->
    <CarpoolTripList
      v-else
      :trips="trips"
      :selected-trip-ids="selectedTripIds"
      :recalculating="recalculating"
      @clear="clearTripSelection"
      @batch-recalculate="handleBatchRecalculate"
      @export="exportSelectedCarpools"
      @toggle-all="toggleSelectAll"
      @toggle="toggleTripSelection"
      @recalculate="handleRecalculateSingle"
      @edit="openEditModal"
      @delete="handleDelete"
    />

    <CarpoolTripModal
      v-model:open="showModal"
      :vehicle-id="vehicleId"
      :editing="editingTrip"
      :create-options="createOptions"
      :open-token="openToken"
      @saved="loadData"
    />
  </div>
</template>
