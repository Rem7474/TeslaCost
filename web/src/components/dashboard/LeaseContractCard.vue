<script setup lang="ts">
import { computed } from 'vue'
import { Coins, Gauge, Calendar, FileText, CheckCircle2 } from 'lucide-vue-next'
import { buildLeaseSummary } from '@/utils/dashboard'

// Follow-up of a LOA / LLD contract: duration, mileage against the allowance, costs and included services
const props = defineProps<{ tco: any | null }>()
const leaseContract = computed(() => buildLeaseSummary(props.tco))
</script>

<template>
  <div
    v-if="leaseContract"
    class="bg-gradient-to-br from-slate-900/90 to-indigo-950/20 border border-indigo-500/20 p-4 sm:p-5 rounded-2xl shadow-sm space-y-4"
  >
    <!-- Card Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
      <div class="flex items-center gap-2.5 flex-wrap">
        <span class="px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider bg-indigo-500/15 text-indigo-400 border border-indigo-500/30 flex items-center gap-1.5">
          <FileText class="w-3.5 h-3.5" />
          Contrat {{ leaseContract.acquisitionType }}
        </span>
        <span
          class="px-2.5 py-0.5 rounded-full text-xs font-semibold border"
          :class="leaseContract.status.class"
        >
          {{ leaseContract.status.label }}
        </span>
      </div>

      <!-- Monthly rent / downpayment -->
      <div v-if="leaseContract.monthlyRent" class="self-start sm:self-auto text-left sm:text-right">
        <span class="text-base sm:text-lg font-extrabold text-white">
          {{ Number(leaseContract.monthlyRent).toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} €
        </span>
        <span class="text-xs text-slate-400 font-normal"> / mois</span>
        <span v-if="leaseContract.downPayment && leaseContract.downPayment > 0" class="block text-[11px] text-slate-400">
          (Apport : {{ Number(leaseContract.downPayment).toLocaleString('fr-FR', { maximumFractionDigits: 0 }) }} €)
        </span>
      </div>
    </div>

    <!-- Progress Bars Section: 1 col on mobile, 2 cols on desktop if mileage allowance configured -->
    <div
      class="grid gap-4"
      :class="leaseContract.hasMileageAllowance ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1'"
    >
      <!-- Bar 1: Contract Duration Progress -->
      <div class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Calendar class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
            Durée du contrat
          </span>
          <span class="font-bold text-indigo-300">
            {{ leaseContract.elapsedMonths }} / {{ leaseContract.totalMonths }} mois
            <span class="text-slate-400 font-normal">({{ leaseContract.durationProgressPct }} %)</span>
          </span>
        </div>

        <!-- Progress Bar Track -->
        <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
          <div
            class="h-full bg-gradient-to-r from-indigo-500 to-violet-500 rounded-full transition-all duration-500"
            :style="{ width: `${leaseContract.durationProgressPct}%` }"
          ></div>
        </div>

        <!-- Sub-info: start date, remaining, end date -->
        <div class="flex items-center justify-between text-[11px] text-slate-400 flex-wrap gap-1">
          <span v-if="leaseContract.startDate">
            Début : {{ leaseContract.startDate.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' }) }}
          </span>
          <span class="font-medium text-slate-300">
            {{ leaseContract.isEnded ? 'Contrat terminé' : leaseContract.isNotStarted ? 'Contrat à venir' : `${leaseContract.remainingMonths} mois restants` }}
          </span>
          <span v-if="leaseContract.endDate">
            Fin : {{ leaseContract.endDate.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' }) }}
          </span>
        </div>
      </div>

      <!-- Bar 2: Mileage Allowance Progress (if configured) -->
      <div v-if="leaseContract.hasMileageAllowance" class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Gauge class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
            Forfait kilométrique
          </span>
          <span class="font-bold text-slate-200">
            {{ Math.round(leaseContract.kmDriven).toLocaleString('fr-FR') }} km
            <span v-if="leaseContract.kmAllowanceTotal" class="text-slate-400 font-normal">
              / {{ Math.round(leaseContract.kmAllowanceTotal).toLocaleString('fr-FR') }} km
            </span>
            <span v-else class="text-slate-400 font-normal">
              / {{ Math.round(leaseContract.kmAllowanceToDate).toLocaleString('fr-FR') }} km à date
            </span>
          </span>
        </div>

        <!-- Progress Bar Track -->
        <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500"
            :class="leaseContract.mileageColor"
            :style="{ width: `${Math.min(100, leaseContract.mileageProgressPct)}%` }"
          ></div>
        </div>

        <!-- Sub-info: Pace & Diff -->
        <div class="flex items-center justify-between text-[11px] flex-wrap gap-1">
          <span class="text-slate-400">
            Rythme : <strong>{{ leaseContract.actualPaceKmMonth }} km/mois</strong>
            <template v-if="leaseContract.contractualPaceKmMonth"> (prévu : {{ leaseContract.contractualPaceKmMonth }} km/mois)</template>
          </span>
          <span
            v-if="leaseContract.kmDiff !== undefined"
            :class="leaseContract.kmDiff > 0 ? 'text-rose-400 font-semibold' : 'text-emerald-400 font-semibold'"
          >
            {{ leaseContract.kmDiff > 0 ? `+${leaseContract.kmDiff.toLocaleString('fr-FR')} km de dépassement` : `${Math.abs(leaseContract.kmDiff).toLocaleString('fr-FR')} km d'avance` }}
          </span>
        </div>
      </div>
    </div>

    <!-- Footer: Buyout option, penalties, included services -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-800/60 text-xs text-slate-400">
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Purchase option (LOA) -->
        <span
          v-if="leaseContract.purchaseOptionPrice"
          class="px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 font-medium border border-slate-700/60 flex items-center gap-1.5"
        >
          <Coins class="w-3.5 h-3.5 text-amber-400" />
          Option d'achat finale : <strong class="text-white">{{ Number(leaseContract.purchaseOptionPrice).toLocaleString('fr-FR', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) }} €</strong>
        </span>

        <!-- Included Services Badges -->
        <span
          v-if="leaseContract.includesMaintenance"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> Entretien inclus
        </span>
        <span
          v-if="leaseContract.includesInsurance"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> Assurance incluse
        </span>
        <span
          v-if="leaseContract.includesTires"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> Pneus inclus
        </span>
      </div>

      <!-- Penalty Warnings if applicable -->
      <div class="flex items-center gap-3">
        <span v-if="leaseContract.excessKmCost > 0" class="text-rose-400 font-semibold">
          Dépassement à date : {{ leaseContract.excessKmCost.toFixed(2) }} €
        </span>
        <span v-if="leaseContract.excessKmProjected > 0" class="text-amber-400 font-semibold">
          Pénalité estimée fin de contrat : {{ leaseContract.excessKmProjected.toFixed(2) }} €
        </span>
      </div>
    </div>
  </div>
</template>
