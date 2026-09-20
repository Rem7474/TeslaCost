<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { Briefcase, User } from 'lucide-vue-next'

// Pro / Perso split of the drives
defineProps<{ tco: any | null }>()
</script>

<template>
  <div v-if="tco?.tag_breakdown?.length" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <h3 class="text-sm font-bold text-white mb-3">{{ $t('dashboard.tagBreakdown.breakdownByDriveCategoryWork') }}</h3>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <div
        v-for="item in tco.tag_breakdown"
        :key="item.tag"
        class="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex items-center justify-between"
      >
        <div>
          <div class="flex items-center gap-2">
            <Briefcase v-if="item.tag === 'Pro'" class="w-4 h-4 text-blue-400" />
            <User v-else class="w-4 h-4 text-emerald-400" />
            <span class="text-sm font-bold text-white">{{ item.tag }}</span>
          </div>
          <p class="text-xs text-slate-400 mt-1">{{ $t('dashboard.tagBreakdown.kmKwh', { distance_km: item.distance_km.toLocaleString(intlLocale()), energy_kwh: item.energy_kwh }) }}</p>
          <p v-if="item.tolls_amount" class="text-xs text-amber-400">{{ $t('dashboard.tagBreakdown.ofTollsAndParking', { value: item.tolls_amount.toFixed(2) }) }}</p>
        </div>
        <div class="text-right">
          <span class="text-lg font-extrabold text-white">{{ item.percentage }}%</span>
        </div>
      </div>
    </div>
  </div>
</template>
