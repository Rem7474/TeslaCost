import { ref, computed, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'

export function useSelection<T extends Record<string, any>>(options?: {
  idKey?: string
  autoResetOnVehicleChange?: boolean
}) {
  const idKey = options?.idKey || 'id'
  const autoReset = options?.autoResetOnVehicleChange !== false
  const vehicleStore = useVehicleStore()

  const selectedMap = ref<Record<string, T>>({}) as { value: Record<string, T> }

  const selectedIds = computed<string[]>(() => Object.keys(selectedMap.value))
  const selectedList = computed<T[]>(() => Object.values(selectedMap.value))
  const count = computed<number>(() => selectedIds.value.length)

  function isSelected(id: string): boolean {
    return !!selectedMap.value[id]
  }

  function toggle(item: T, customIdKey?: string) {
    const key = customIdKey || idKey
    const id = String(item[key] ?? item)
    const next = { ...selectedMap.value }
    if (next[id]) {
      delete next[id]
    } else {
      next[id] = item
    }
    selectedMap.value = next
  }

  function selectMultiple(items: T[], customIdKey?: string) {
    const key = customIdKey || idKey
    const next = { ...selectedMap.value }
    for (const it of items) {
      const id = String(it[key] ?? it)
      next[id] = it
    }
    selectedMap.value = next
  }

  function deselectMultiple(items: T[], customIdKey?: string) {
    const key = customIdKey || idKey
    const next = { ...selectedMap.value }
    for (const it of items) {
      const id = String(it[key] ?? it)
      delete next[id]
    }
    selectedMap.value = next
  }

  function clear() {
    selectedMap.value = {}
  }

  if (autoReset) {
    watch(
      () => vehicleStore.activeVehicle?.id,
      () => {
        clear()
      }
    )
  }

  return {
    selectedMap,
    selectedIds,
    selectedList,
    count,
    isSelected,
    toggle,
    selectMultiple,
    deselectMultiple,
    clear,
  }
}
