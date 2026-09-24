<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { Coins, Zap, Receipt, TrendingUp } from 'lucide-vue-next'

defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()
// Every TCO figure is in the vehicle's own currency
const money = (v: number, digits = 2) => formatAmount(v || 0, vehicleStore.currency, digits)
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
    <!-- Total Cost -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.totalCost') }}</span>
        <div class="p-2 bg-rose-500/10 text-rose-400 rounded-xl">
          <Coins class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-white">
        {{ money(tco?.total_cost) }}
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-300">
          {{ $t('dashboard.tcoMetricsGrid.fullCost', { full_cost: money(tco?.full_cost) }) }}
          <span v-if="tco?.depreciation_cost" class="text-slate-400 text-[11px]"> {{ $t('dashboard.tcoMetricsGrid.includingDepreciation', { depreciation_cost: money(tco.depreciation_cost, 0) }) }}</span>
        </p>
        <p v-if="tco?.carpool_revenue" class="text-[11px] text-emerald-400">
          {{ $t('dashboard.tcoMetricsGrid.netOfCarpooling', { full_cost_net: money(tco.full_cost_net) }) }}
        </p>
      </div>
    </div>

    <!-- Cost per km -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.fullCostPerKm') }}</span>
        <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
          <TrendingUp class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-emerald-400">
        {{ money(tco?.full_cost_per_km, 3) }}<span class="text-xs font-normal text-slate-400">/km</span>
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.directRunningCostKm', { usage_cost_per_km: money(tco?.usage_cost_per_km, 3) }) }}
        </p>
        <p class="text-[11px] text-slate-500">
          {{ $t('dashboard.tcoMetricsGrid.overKm', { distance_basis_km: Math.round(tco?.distance_basis_km || 0).toLocaleString(intlLocale()) }) }}
          <template v-if="tco?.depreciation_cost_per_km"> {{ $t('dashboard.tcoMetricsGrid.depreciationKm', { value: money(tco.depreciation_cost_per_km, 3) }) }}</template>
        </p>
      </div>
    </div>

    <!-- Energy Cost -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ tco?.powertrain === 'ICE' ? $t('dashboard.tcoMetricsGrid.fuel') : $t('dashboard.tcoMetricsGrid.energy') }}</span>
        <div class="p-2 bg-sky-500/10 text-sky-400 rounded-xl">
          <Zap class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-sky-400">
        {{ money(tco?.energy_cost) }}
      </div>
      <div class="mt-2 space-y-0.5">
        <p v-if="tco?.powertrain === 'ICE'" class="text-xs text-slate-400">
          {{ money(tco?.energy_cost_per_km, 3) }}/km • {{ Math.round(tco?.total_liters || 0).toLocaleString(intlLocale()) }} L<template v-if="tco?.consumption_l_100km"> • {{ tco.consumption_l_100km.toFixed(2) }} L/100</template><template v-if="tco?.avg_cost_per_liter"> • {{ money(tco.avg_cost_per_liter, 3) }}/L</template>
        </p>
        <p v-else class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.kmKwh', { energy_cost_per_km: money(tco?.energy_cost_per_km, 3), total_kwh_added: Math.round(tco?.total_kwh_added || 0).toLocaleString(intlLocale()) }) }}
        </p>
        <p v-if="tco?.completeness?.charges_without_cost" class="text-[11px] text-amber-400">
          {{ $t('dashboard.tcoMetricsGrid.chargeSWithoutACost', { charges_without_cost: tco.completeness.charges_without_cost }) }}
        </p>
      </div>
    </div>

    <!-- Tolls & Parkings -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.tollsAndParking') }}</span>
        <div class="p-2 bg-amber-500/10 text-amber-400 rounded-xl">
          <Receipt class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-amber-400">
        {{ money(tco?.tolls_cost) }}
      </div>
      <div class="mt-2">
        <p class="text-xs text-slate-400">{{ money(tco?.tolls_cost_per_km, 3) }}/km</p>
      </div>
    </div>
  </div>
</template>
