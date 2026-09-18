import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/services/api'

export const useVehicleStore = defineStore('vehicle', () => {
  const vehicles = ref<any[]>([])
  const activeVehicleId = ref<string | null>(localStorage.getItem('teslacost_active_vehicle'))
  const isInitialized = ref(false)
  const isLoading = ref(false)
  const isSyncing = ref(false)
  const syncResult = ref<any | null>(null)
  const syncError = ref<string | null>(null)
  const lastSyncTimestamp = ref<number>(Date.now())

  const activeVehicle = computed(() => {
    if (!vehicles.value.length) return null
    return vehicles.value.find((v) => v.id === activeVehicleId.value) || vehicles.value[0]
  })

  const userRole = computed<'OWNER' | 'EDITOR' | 'VIEWER'>(() => activeVehicle.value?.role || 'OWNER')
  const isOwner = computed(() => userRole.value === 'OWNER')
  const isEditor = computed(() => userRole.value === 'EDITOR')
  const isViewer = computed(() => userRole.value === 'VIEWER')
  const canEdit = computed(() => isOwner.value || isEditor.value)
  // Combustion vehicles are tracked manually (fuel fill-ups) and have no TeslaMate link
  const isIce = computed(() => activeVehicle.value?.powertrain === 'ICE')

  async function fetchVehicles() {
    isLoading.value = true
    try {
      const list = await api.getVehicles()
      vehicles.value = list
      if (list.length > 0 && (!activeVehicleId.value || !list.some((v) => v.id === activeVehicleId.value))) {
        setActiveVehicle(list[0].id)
      }
      lastSyncTimestamp.value = Date.now()
    } catch (err) {
      console.error('Failed to fetch vehicles', err)
    } finally {
      isLoading.value = false
      isInitialized.value = true
    }
  }

  function setActiveVehicle(id: string) {
    activeVehicleId.value = id
    localStorage.setItem('teslacost_active_vehicle', id)
  }

  const SYNC_POLL_INTERVAL_MS = 1500
  const SYNC_POLL_MAX_MS = 20 * 60 * 1000
  const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

  // Follows a background synchronization job until it finishes
  async function followSyncJob(vehicleId: string, job: any) {
    isSyncing.value = true
    syncResult.value = null
    syncError.value = null
    const startedAt = Date.now()
    try {
      while (job?.status === 'RUNNING') {
        if (Date.now() - startedAt > SYNC_POLL_MAX_MS) {
          throw new Error('La synchronisation prend plus de temps que prévu, elle continue en arrière-plan')
        }
        await sleep(SYNC_POLL_INTERVAL_MS)
        job = await api.getSyncStatus(vehicleId)
      }
      if (job?.status === 'FAILED') {
        syncError.value = job.error || 'Erreur inconnue lors de la synchronisation'
      } else if (job?.status === 'SUCCEEDED') {
        syncResult.value = job.result
        await fetchVehicles()
        lastSyncTimestamp.value = Date.now()
      }
    } catch (err: any) {
      syncError.value = err.message || 'Erreur inconnue lors de la synchronisation'
    } finally {
      isSyncing.value = false
    }
  }

  async function syncActiveVehicle() {
    if (!activeVehicle.value || isSyncing.value) return
    const vehicleId = activeVehicle.value.id
    try {
      const job = await api.syncVehicle(vehicleId)
      await followSyncJob(vehicleId, job)
    } catch (err: any) {
      syncError.value = err.message || 'Erreur inconnue lors de la synchronisation'
    }
  }

  // Resumes the progress indicator when a synchronization (manual or scheduled) is already running
  async function resumeRunningSync() {
    if (!activeVehicle.value || isSyncing.value) return
    try {
      const job = await api.getSyncStatus(activeVehicle.value.id)
      if (job?.status === 'RUNNING') {
        await followSyncJob(activeVehicle.value.id, job)
      }
    } catch {
      // No sync information: nothing to resume
    }
  }

  function clearSyncStatus() {
    syncResult.value = null
    syncError.value = null
  }

  return {
    vehicles,
    activeVehicleId,
    activeVehicle,
    userRole,
    isOwner,
    isEditor,
    isViewer,
    canEdit,
    isIce,
    isSyncing,
    syncResult,
    syncError,
    lastSyncTimestamp,
    isInitialized,
    isLoading,
    fetchVehicles,
    setActiveVehicle,
    syncActiveVehicle,
    resumeRunningSync,
    clearSyncStatus,
  }
})
