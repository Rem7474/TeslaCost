<script setup lang="ts">
import { Archive, ClipboardPaste, Copy, Edit2, History, Pencil, Plus, Ruler, Shuffle, Trash2, X, Zap } from 'lucide-vue-next'
import { formatDate, type SessionForm } from '@/utils/tires'

// Full history of one tire: life KPIs, TeslaMate telemetry, mount sessions and tread depth measurements.
// It only displays: every action is emitted for the page to run.
defineProps<{
  selectedTire: any | null
  selectedTireStats: any | null
  tireSessions: any[]
  tireLogs: any[]
  copiedSession: SessionForm | null
}>()
const emit = defineEmits<{
  'edit-tire': []
  'dispose-tire': []
  'delete-tire': []
  'paste-session': []
  'add-session': []
  'copy-session': [session: any]
  'duplicate-session': [session: any]
  'edit-session': [session: any]
  'delete-session': [session: any]
  'add-log': []
  'edit-log': [log: any]
  'delete-log': [log: any]
}>()
const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <div
    v-if="open && selectedTire"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-start justify-between shrink-0 bg-slate-900/95">
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-base font-bold text-white">{{ selectedTire.brand }} {{ selectedTire.model }}</h3>
            <span class="text-xs font-mono bg-slate-800 px-2 py-0.5 rounded text-slate-300">
              {{ selectedTire.dimension }}
            </span>
          </div>
          <div class="text-xs text-slate-400 mt-0.5">
            Acheté le {{ formatDate(selectedTire.purchase_date) }} • {{ selectedTire.purchase_price }} €
          </div>
        </div>
        <div class="flex items-center gap-1.5">
          <button @click="emit('edit-tire')" class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white rounded-lg transition-colors" title="Modifier le pneu">
            <Pencil class="w-4 h-4" />
          </button>
          <button
            v-if="selectedTire.current_position !== 'DISPOSED'"
            @click="emit('dispose-tire')"
            class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-amber-400 rounded-lg transition-colors"
            title="Mettre au rebut (usé, crevé, vendu)"
          >
            <Archive class="w-4 h-4" />
          </button>
          <button @click="emit('delete-tire')" class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-lg transition-colors" title="Supprimer (saisie erronée)">
            <Trash2 class="w-4 h-4" />
          </button>
          <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-6">

      <!-- Life KPI Card -->
      <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-2xl space-y-3">
        <div class="flex items-center justify-between text-xs">
          <span class="text-slate-400">Kilométrage total de vie :</span>
          <span class="text-base font-bold text-white">
            {{ Math.round(selectedTireStats?.total_distance_km || 0).toLocaleString('fr-FR') }} km
            <span class="text-xs text-slate-400 font-normal">/ {{ (selectedTire.estimated_lifespan_km || 45000).toLocaleString('fr-FR') }} km estimés</span>
          </span>
        </div>

        <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all"
            :class="(selectedTireStats?.life_progress_pct || 0) > 80 ? 'bg-rose-500' : 'bg-emerald-500'"
            :style="{ width: `${Math.min(100, selectedTireStats?.life_progress_pct || 0)}%` }"
          ></div>
        </div>

        <div class="grid grid-cols-3 gap-2 text-center text-xs pt-1">
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500">Sculpture actuelle</div>
            <div class="font-bold text-emerald-400">{{ selectedTireStats?.current_depth_mm }} mm</div>
          </div>
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500">Durée de vie consommée</div>
            <div class="font-bold text-slate-200">{{ selectedTireStats?.life_progress_pct }}%</div>
          </div>
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500">Coût réel / km</div>
            <div class="font-bold text-amber-400">{{ Number(selectedTireStats?.cost_per_km).toFixed(4) }} €</div>
          </div>
        </div>
      </div>

      <!-- TeslaMate Driving Telemetry & Stress Analysis Card -->
      <div v-if="selectedTireStats?.driving_stress_index > 0" class="bg-gradient-to-br from-slate-950 to-slate-900 border border-slate-800 p-4 rounded-2xl space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Zap class="w-4 h-4 text-amber-400" />
            <h4 class="text-xs font-bold text-white uppercase tracking-wider">Télémétrie Dynamique TeslaMate</h4>
          </div>
          <span
            class="px-2.5 py-0.5 rounded-full text-xs font-bold border"
            :class="
              selectedTireStats.driving_style === 'SPORT'
                ? 'bg-rose-500/15 text-rose-400 border-rose-500/30'
                : selectedTireStats.driving_style === 'ECO'
                ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                : 'bg-sky-500/15 text-sky-400 border-sky-500/30'
            "
          >
            {{ selectedTireStats.driving_style === 'SPORT' ? 'Contrainte Sportive' : selectedTireStats.driving_style === 'ECO' ? 'Éco-conduite' : 'Conduite Équilibrée' }} (Indice : x{{ selectedTireStats.driving_stress_index }})
          </span>
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-center text-xs">
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500 uppercase">Pointe Accélération</div>
            <div class="font-bold text-rose-400 text-sm mt-0.5">+{{ selectedTireStats.avg_power_max_kw }} kW</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500 uppercase">Pointe Régénération</div>
            <div class="font-bold text-emerald-400 text-sm mt-0.5">{{ selectedTireStats.avg_power_min_kw }} kW</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500 uppercase">Conso moyenne</div>
            <div class="font-bold text-sky-400 text-sm mt-0.5">{{ selectedTireStats.avg_consumption_kwh_100km }} kWh</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-[10px] text-slate-500 uppercase">Longévité ajustée</div>
            <div class="font-bold text-indigo-300 text-sm mt-0.5">~{{ (selectedTireStats.dynamic_lifespan_km || selectedTire.estimated_lifespan_km).toLocaleString('fr-FR') }} km</div>
          </div>
        </div>

        <p v-if="selectedTireStats.wear_explanation" class="text-xs text-slate-300 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/80">
          {{ selectedTireStats.wear_explanation }}
        </p>
      </div>

      <!-- Timeline: Mount/Dismount Sessions -->
      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <History class="w-4 h-4 text-rose-500" />
            Historique des montages, démontages & permutations
          </h4>
          <div class="flex items-center gap-2">
            <button
              v-if="copiedSession"
              @click="emit('paste-session')"
              class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1 transition-colors bg-indigo-950/40 border border-indigo-800/60 px-2 py-1 rounded-lg"
              :title="'Coller la session copiée (' + (copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '') + ')'"
            >
              <ClipboardPaste class="w-3.5 h-3.5" />
              <span>Coller</span>
            </button>
            <button
              @click="emit('add-session')"
              class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              Ajouter une session passée
            </button>
          </div>
        </div>

        <div v-if="tireSessions.length === 0" class="p-6 text-center bg-slate-950/40 rounded-2xl text-xs text-slate-500">
          Aucune session enregistrée pour ce pneu.
        </div>

        <div v-else class="space-y-2.5">
          <div
            v-for="s in tireSessions"
            :key="s.id"
            class="bg-slate-950/80 border border-slate-800/80 rounded-2xl p-3.5 text-xs space-y-2 hover:border-slate-700 transition-all"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span
                  class="px-2 py-0.5 rounded-md font-bold text-[10px]"
                  :class="s.dismounted_date ? 'bg-slate-800 text-slate-300' : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'"
                >
                  {{ s.dismounted_date ? 'Session terminée' : '🟢 Montage en cours' }}
                </span>
                <span class="font-bold text-white">Roue : {{ s.position }}</span>
              </div>

              <div class="flex items-center gap-1.5">
                <button
                  @click="emit('copy-session', s)"
                  class="p-1 rounded transition-colors"
                  :class="copiedSession?.mounted_date === (s.mounted_date ? new Date(s.mounted_date).toISOString().substring(0, 10) : '') && copiedSession?.mounted_odometer === s.mounted_odometer ? 'text-indigo-400 bg-indigo-950/60' : 'text-slate-400 hover:text-indigo-400'"
                  title="Copier les données de cette session"
                >
                  <Copy class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('duplicate-session', s)"
                  class="p-1 text-slate-400 hover:text-sky-400 rounded transition-colors"
                  title="Dupliquer vers d'autres pneus..."
                >
                  <Shuffle class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('edit-session', s)"
                  class="p-1 text-slate-400 hover:text-white rounded"
                  title="Modifier la session"
                >
                  <Edit2 class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('delete-session', s)"
                  class="p-1 text-slate-400 hover:text-rose-400 rounded"
                  title="Supprimer la session"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Session Details -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] text-slate-400 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/60">
              <div>
                <div class="text-slate-500">Montage :</div>
                <div class="text-slate-200 font-medium">{{ formatDate(s.mounted_date) }} à {{ Math.round(s.mounted_odometer).toLocaleString('fr-FR') }} km</div>
              </div>
              <div>
                <div class="text-slate-500">Démontage :</div>
                <div class="text-slate-200 font-medium">
                  {{ s.dismounted_date ? `${formatDate(s.dismounted_date)} à ${Math.round(s.dismounted_odometer).toLocaleString('fr-FR')} km` : 'Actuellement sur le véhicule' }}
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between text-[11px] pt-1">
              <span v-if="s.notes" class="text-slate-400 italic">"{{ s.notes }}"</span>
              <span v-else></span>
              <span class="font-bold text-rose-400">+{{ Math.round(s.distance_km).toLocaleString('fr-FR') }} km parcourus</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Tread Logs (Mesures de gomme) -->
      <div class="space-y-2 pt-3 border-t border-slate-800">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <Ruler class="w-4 h-4 text-emerald-400" />
            Relevés de profondeur de gomme ({{ tireLogs.length }})
          </h4>
          <button
            @click="emit('add-log')"
            class="text-xs text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1 transition-colors"
          >
            <Plus class="w-3.5 h-3.5" />
            Ajouter un relevé
          </button>
        </div>

        <div v-if="tireLogs.length > 0" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
          <div
            v-for="l in tireLogs"
            :key="l.id"
            class="bg-slate-950/60 border border-slate-800 p-2.5 rounded-xl text-xs space-y-0.5"
          >
            <div class="flex items-center justify-between">
              <span class="font-bold text-emerald-400">{{ l.depth_mm }} mm</span>
              <span class="text-[10px] text-slate-500">{{ formatDate(l.date) }}</span>
            </div>
            <div class="flex items-center justify-between text-[10px] text-slate-400">
              <span>à {{ Math.round(l.odometer).toLocaleString('fr-FR') }} km</span>
              <span class="flex items-center gap-1">
                <button @click="emit('edit-log', l)" class="text-slate-500 hover:text-emerald-400" title="Modifier le relevé">
                  <Pencil class="w-3 h-3" />
                </button>
                <button @click="emit('delete-log', l)" class="text-slate-500 hover:text-rose-400" title="Supprimer le relevé">
                  <Trash2 class="w-3 h-3" />
                </button>
              </span>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- Pinned Footer -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
        >
          Fermer
        </button>
      </div>
    </div>
  </div>
</template>
