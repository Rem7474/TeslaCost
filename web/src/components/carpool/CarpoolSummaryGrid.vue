<script setup lang="ts">
import { Users, TrendingUp, CreditCard, Receipt } from 'lucide-vue-next'
import { fmt } from '@/utils/carpool'

// Totals of all the carpool trips of the vehicle
defineProps<{ summary: any }>()
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 sm:gap-4">
    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">Trajets covoiturés</span>
        <div class="p-2 bg-slate-800 rounded-xl text-slate-300"><Users class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-white">{{ summary.total_trips }}</span>
        <span class="text-xs text-slate-400">voyages</span>
      </div>
      <div class="mt-1 text-[11px] text-slate-400">{{ Math.round(summary.total_distance_km || 0).toLocaleString('fr-FR') }} km partagés</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">Passagers transportés</span>
        <div class="p-2 bg-blue-500/10 rounded-xl text-blue-400"><Users class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-blue-400">{{ summary.total_passengers }}</span>
        <span class="text-xs text-slate-400">personnes</span>
      </div>
      <div class="mt-1 text-[11px] text-slate-400">Part équitable due : {{ fmt(summary.total_passengers_share) }} €</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">Total perçu passagers</span>
        <div class="p-2 bg-emerald-500/10 rounded-xl text-emerald-400"><CreditCard class="w-4 h-4" /></div>
      </div>
      <div class="mt-2"><span class="text-2xl font-bold text-emerald-400">{{ fmt(summary.total_revenue) }} €</span></div>
      <div class="mt-1 text-[11px]" :class="summary.total_revenue >= summary.total_passengers_share ? 'text-emerald-500/80' : 'text-amber-400'">
        {{ summary.total_revenue >= summary.total_passengers_share ? 'Parts des passagers couvertes' : `${fmt(summary.total_passengers_share - summary.total_revenue)} € sous leur part` }}
      </div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">Taux d'amortissement</span>
        <div class="p-2 bg-rose-500/10 rounded-xl text-rose-400"><TrendingUp class="w-4 h-4" /></div>
      </div>
      <div class="mt-2"><span class="text-2xl font-bold text-rose-400">{{ summary.coverage_rate_pct || 0 }} %</span></div>
      <div class="mt-1 text-[11px] text-slate-400">du coût réel total ({{ fmt(summary.total_real_cost) }} €)</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">Coût net conducteur</span>
        <div class="p-2 bg-amber-500/10 rounded-xl text-amber-400"><Receipt class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-amber-400">{{ Number(summary.net_cost_per_km || 0).toFixed(3) }} €</span>
        <span class="text-xs text-slate-400">/ km</span>
      </div>
      <div class="mt-1 text-[11px] text-slate-400">Part équitable conducteur : {{ fmt(summary.total_driver_share) }} €</div>
    </div>
  </div>
</template>
