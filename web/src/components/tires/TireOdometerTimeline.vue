<script setup lang="ts">
import { computed, ref } from 'vue'
import { buildTireTimeline, tickStep, type TimelineSegment } from '../../utils/tireTimeline'

const props = defineProps<{ tires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ (e: 'select-tire', tireId: string): void }>()

const timeline = computed(() => buildTireTimeline(props.tires, Math.round(props.currentOdometer || 0)))

const hovered = ref<TimelineSegment | null>(null)
const pinned = ref<TimelineSegment | null>(null)
const shown = computed(() => hovered.value ?? pinned.value)

const pct = (km: number) => (timeline.value.maxKm > 0 ? (km / timeline.value.maxKm) * 100 : 0)
const fmtKm = (km: number) => `${Math.round(km).toLocaleString('fr-FR')} km`

const hasData = computed(() => timeline.value.lanes.some((l) => l.segments.length > 0))
const hasGaps = computed(() => timeline.value.lanes.some((l) => l.gaps.length > 0))
const laneHeight = computed(() => (timeline.value.lanes.length > 1 ? 'h-7' : 'h-9'))

const ticks = computed(() => {
  const step = tickStep(timeline.value.maxKm)
  const out: number[] = []
  for (let km = 0; km <= timeline.value.maxKm; km += step) out.push(km)
  return out
})

function onClick(seg: TimelineSegment) {
  pinned.value = pinned.value?.id === seg.id ? null : seg
}
</script>

<template>
  <div class="bg-slate-900 border border-slate-800 rounded-3xl p-5 space-y-4 shadow-sm">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <h3 class="text-sm font-bold text-white">Pneus montés selon le kilométrage</h3>
      <span class="text-[11px] text-slate-400">0 → {{ fmtKm(timeline.maxKm) }}</span>
    </div>

    <div v-if="!hasData" class="p-6 text-center text-xs text-slate-500 bg-slate-950/40 rounded-2xl">
      Aucune session de montage avec kilométrage enregistrée.
    </div>

    <template v-else>
      <div class="space-y-1.5">
        <div v-for="lane in timeline.lanes" :key="lane.id" class="flex items-center gap-3">
          <span v-if="timeline.lanes.length > 1" class="w-32 shrink-0 text-[11px] text-slate-400 truncate" :title="lane.label">{{ lane.label }}</span>
          <div class="relative flex-1 rounded-xl overflow-hidden bg-slate-950 border border-slate-800" :class="laneHeight">
            <div
              v-for="(gap, i) in lane.gaps"
              :key="`gap-${i}`"
              class="absolute top-0 h-full"
              :style="{
                left: pct(gap.startKm) + '%',
                width: pct(gap.endKm - gap.startKm) + '%',
                backgroundImage: 'repeating-linear-gradient(45deg, #334155 0 4px, transparent 4px 8px)',
              }"
              :title="`Aucun pneu enregistré (${fmtKm(gap.startKm)} → ${fmtKm(gap.endKm)})`"
            ></div>
            <button
              v-for="seg in lane.segments"
              :key="seg.id"
              type="button"
              class="absolute top-0 h-full border-r border-slate-950 transition-opacity hover:opacity-80 focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
              :class="{ 'ring-2 ring-white ring-inset': pinned?.id === seg.id }"
              :style="{ left: pct(seg.startKm) + '%', width: pct(seg.endKm - seg.startKm) + '%', backgroundColor: seg.color }"
              :aria-label="`${lane.label} : ${seg.tires[0].label}, de ${fmtKm(seg.startKm)} à ${fmtKm(seg.endKm)}`"
              @mouseenter="hovered = seg"
              @mouseleave="hovered = null"
              @focus="hovered = seg"
              @blur="hovered = null"
              @click="onClick(seg)"
            ></button>
          </div>
        </div>
      </div>

      <div class="relative h-4 text-[10px] text-slate-500" :class="timeline.lanes.length > 1 ? 'ml-[8.75rem]' : ''">
        <span v-for="km in ticks" :key="km" class="absolute -translate-x-1/2 first:translate-x-0" :style="{ left: pct(km) + '%' }">
          {{ (km / 1000).toLocaleString('fr-FR') }}k
        </span>
      </div>

      <div class="flex flex-wrap gap-x-4 gap-y-1">
        <span v-for="l in timeline.legend" :key="l.key" class="flex items-center gap-1.5 text-[11px] text-slate-300">
          <span class="w-2.5 h-2.5 rounded-sm" :style="{ backgroundColor: l.color }"></span>{{ l.key }}
        </span>
        <span v-if="hasGaps" class="flex items-center gap-1.5 text-[11px] text-slate-400">
          <span class="w-2.5 h-2.5 rounded-sm bg-slate-600"></span>Aucun pneu enregistré
        </span>
      </div>

      <div class="min-h-[4.5rem] bg-slate-950/60 border border-slate-800/80 rounded-2xl p-3 text-xs">
        <p v-if="!shown" class="text-slate-500">Survolez ou cliquez sur un segment pour voir le détail.</p>
        <div v-else class="space-y-2">
          <div class="text-slate-200 font-semibold">
            {{ fmtKm(shown.startKm) }} → {{ fmtKm(shown.endKm) }}
            <span class="text-slate-400 font-normal">({{ fmtKm(shown.endKm - shown.startKm) }})</span>
          </div>
          <ul class="space-y-1">
            <li v-for="t in shown.tires" :key="t.tireId + t.position" class="flex items-center justify-between gap-2 text-slate-300">
              <span>
                <span class="font-mono text-rose-400 mr-1">{{ t.position }}</span>{{ t.label }}
                <span class="text-slate-500 font-mono">{{ t.dimension }}</span>
                <span v-if="t.ongoing" class="ml-1 text-emerald-400">en cours</span>
              </span>
              <button type="button" class="text-rose-400 hover:text-rose-300 font-semibold shrink-0" @click="emit('select-tire', t.tireId)">
                Fiche
              </button>
            </li>
          </ul>
        </div>
      </div>
    </template>
  </div>
</template>
