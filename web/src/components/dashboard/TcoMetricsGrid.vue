<script setup lang="ts">
import { Coins, Zap, Receipt, TrendingUp } from 'lucide-vue-next'

defineProps<{ tco: any | null }>()
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
    <!-- Total Cost -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût Total</span>
        <div class="p-2 bg-rose-500/10 text-rose-400 rounded-xl">
          <Coins class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-white">
        {{ (tco?.total_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-300">
          Coût complet : {{ (tco?.full_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
          <span v-if="tco?.depreciation_cost" class="text-slate-400 text-[11px]"> (dont décote {{ tco.depreciation_cost.toLocaleString('fr-FR', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) }} €)</span>
        </p>
        <p v-if="tco?.carpool_revenue" class="text-[11px] text-emerald-400">
          Net covoiturage : {{ (tco.full_cost_net || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
        </p>
      </div>
    </div>

    <!-- Cost per km -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Coût complet au km</span>
        <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
          <TrendingUp class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-emerald-400">
        {{ (tco?.full_cost_per_km || 0).toLocaleString('fr-FR', { minimumFractionDigits: 3, maximumFractionDigits: 3 }) }} €<span class="text-xs font-normal text-slate-400">/km</span>
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-400">
          Usage direct : {{ (tco?.usage_cost_per_km || 0).toFixed(3) }} €/km
        </p>
        <p class="text-[11px] text-slate-500">
          Sur {{ Math.round(tco?.distance_basis_km || 0).toLocaleString('fr-FR') }} km
          <template v-if="tco?.depreciation_cost_per_km"> • décote {{ tco.depreciation_cost_per_km.toFixed(3) }} €/km</template>
        </p>
      </div>
    </div>

    <!-- Energy Cost -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ tco?.powertrain === 'ICE' ? 'Carburant (pleins)' : 'Énergie (Charges)' }}</span>
        <div class="p-2 bg-sky-500/10 text-sky-400 rounded-xl">
          <Zap class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-sky-400">
        {{ (tco?.energy_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
      </div>
      <div class="mt-2 space-y-0.5">
        <p v-if="tco?.powertrain === 'ICE'" class="text-xs text-slate-400">
          {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km • {{ Math.round(tco?.total_liters || 0).toLocaleString('fr-FR') }} L<template v-if="tco?.consumption_l_100km"> • {{ tco.consumption_l_100km.toFixed(2) }} L/100</template><template v-if="tco?.avg_cost_per_liter"> • {{ tco.avg_cost_per_liter.toFixed(3) }} €/L</template>
        </p>
        <p v-else class="text-xs text-slate-400">
          {{ (tco?.energy_cost_per_km || 0).toFixed(3) }} €/km • {{ Math.round(tco?.total_kwh_added || 0).toLocaleString('fr-FR') }} kWh
        </p>
        <p v-if="tco?.completeness?.charges_without_cost" class="text-[11px] text-amber-400">
          {{ tco.completeness.charges_without_cost }} recharge(s) sans coût
        </p>
      </div>
    </div>

    <!-- Tolls & Parkings -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Péages & Parkings</span>
        <div class="p-2 bg-amber-500/10 text-amber-400 rounded-xl">
          <Receipt class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-amber-400">
        {{ (tco?.tolls_cost || 0).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
      </div>
      <div class="mt-2">
        <p class="text-xs text-slate-400">{{ (tco?.tolls_cost_per_km || 0).toFixed(3) }} €/km</p>
      </div>
    </div>
  </div>
</template>
