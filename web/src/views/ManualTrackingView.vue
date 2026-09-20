<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ClipboardList, Gauge, Fuel, Zap } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import OdometerReadingsPanel from '@/components/manual/OdometerReadingsPanel.vue'
import FuelLogsPanel from '@/components/manual/FuelLogsPanel.vue'
import EstimatedEnergyPanel from '@/components/manual/EstimatedEnergyPanel.vue'

type Tab = 'KM' | 'FUEL' | 'ENERGY'

const route = useRoute()
const router = useRouter()
const vehicleStore = useVehicleStore()

// Readings are shared by every vehicle; fill-ups only exist for combustion vehicles, the energy estimate for electric ones
const tabs = computed<{ key: Tab; label: string; icon: any }[]>(() => {
  const list: { key: Tab; label: string; icon: any }[] = [{ key: 'KM', label: 'Kilométrage', icon: Gauge }]
  if (vehicleStore.isIce) list.push({ key: 'FUEL', label: 'Pleins', icon: Fuel })
  else list.push({ key: 'ENERGY', label: 'Énergie estimée', icon: Zap })
  return list
})

const requested = String(route.query.tab || '').toUpperCase()
const activeTab = ref<Tab>('KM')

function resolveTab(value: string): Tab {
  return (tabs.value.find((t) => t.key === value)?.key) || 'KM'
}

activeTab.value = resolveTab(requested)

watch(() => route.query.tab, (q) => {
  activeTab.value = resolveTab(String(q || '').toUpperCase())
})

// A tab that does not exist for the newly selected vehicle falls back to the readings
watch(() => vehicleStore.isIce, () => {
  activeTab.value = resolveTab(activeTab.value)
})

function select(tab: Tab) {
  activeTab.value = tab
  if (route.query.tab !== tab) router.replace({ query: { ...route.query, tab } })
}
</script>

<template>
  <div class="space-y-5">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center">
        <ClipboardList class="w-5 h-5 text-cyan-400" />
      </div>
      <div>
        <h1 class="text-xl font-bold text-white">Suivi manuel</h1>
        <p class="text-xs text-slate-400">
          Kilométrage et {{ vehicleStore.isIce ? 'pleins' : 'énergie' }} saisis à la main{{ vehicleStore.activeVehicle ? ` · ${vehicleStore.activeVehicle.name}` : '' }}. Chaque saisie est indépendante.
        </p>
      </div>
    </div>

    <div v-if="!vehicleStore.activeVehicle" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 text-sm text-slate-400">
      Ajoutez d'abord un véhicule.
    </div>

    <template v-else>
      <div class="flex items-center bg-slate-900 border border-slate-800 rounded-2xl p-1 gap-1 w-fit max-w-full overflow-x-auto" role="tablist">
        <button
          v-for="t in tabs"
          :key="t.key"
          type="button"
          role="tab"
          :aria-selected="activeTab === t.key"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
          :class="activeTab === t.key ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          @click="select(t.key)"
        >
          <component :is="t.icon" class="w-3.5 h-3.5" />
          <span>{{ t.label }}</span>
        </button>
      </div>

      <OdometerReadingsPanel v-if="activeTab === 'KM'" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
      <FuelLogsPanel
        v-else-if="activeTab === 'FUEL'"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :can-edit="vehicleStore.canEdit"
      />
      <EstimatedEnergyPanel v-else-if="activeTab === 'ENERGY'" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
    </template>
  </div>
</template>
