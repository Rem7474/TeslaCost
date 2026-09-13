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

  async function syncActiveVehicle() {
    if (!activeVehicle.value) return
    isSyncing.value = true
    syncResult.value = null
    syncError.value = null
    try {
      const res = await api.syncVehicle(activeVehicle.value.id)
      syncResult.value = res
      await fetchVehicles()
      lastSyncTimestamp.value = Date.now()
    } catch (err: any) {
      syncError.value = err.message || 'Erreur inconnue lors de la synchronisation'
    } finally {
      isSyncing.value = false
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
    isSyncing,
    syncResult,
    syncError,
    lastSyncTimestamp,
    isInitialized,
    isLoading,
    fetchVehicles,
    setActiveVehicle,
    syncActiveVehicle,
    clearSyncStatus,
  }
})
