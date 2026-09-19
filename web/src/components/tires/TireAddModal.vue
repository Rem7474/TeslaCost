<script setup lang="ts">
import { ref, watch } from 'vue'
import { Plus, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { todayIso } from '@/utils/tires'

const props = defineProps<{ vehicleId: string; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const addType = ref<'SET_4' | 'SET_4_STORAGE' | 'SET_2_FRONT' | 'SET_2_REAR' | 'SET_2_STORAGE' | 'SINGLE'>('SET_4')
const dimensionPreset = ref('235/40 R19 96W')
const isCustomDimension = ref(false)
const isTotalPrice = ref(true)

const addTireForm = ref({
  brand: 'Michelin',
  model: 'Pilot Sport EV',
  dimension: '235/40 R19 96W',
  season: 'SUMMER',
  purchase_date: todayIso(),
  total_price: 880,
  unit_price: 220,
  initial_depth_mm: 8.0,
  min_legal_depth_mm: 1.6,
  dot_code: '',
  mounted_odometer: 0,
  accumulated_distance_km: 0,
  estimated_lifespan_km: 45000,
  current_position: 'FL',
})

// Tesla predefined tire dimensions
const teslaDimensionPresets = [
  { group: 'Tesla Model 3', label: '18" Aero — 235/45 R18 98Y', value: '235/45 R18 98Y' },
  { group: 'Tesla Model 3', label: '19" Sport — 235/40 R19 96W', value: '235/40 R19 96W' },
  { group: 'Tesla Model 3', label: '20" Performance — 245/35 R20 95Y', value: '245/35 R20 95Y' },
  { group: 'Tesla Model Y', label: '19" Gemini — 255/45 R19 104W', value: '255/45 R19 104W' },
  { group: 'Tesla Model Y', label: '20" Induction — 255/40 R20 101W', value: '255/40 R20 101W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Av — 255/35 R21 98W', value: '255/35 R21 98W' },
  { group: 'Tesla Model Y', label: '21" Überturbine Ar — 275/35 R21 103W', value: '275/35 R21 103W' },
  { group: 'Tesla Model S', label: '19" Tempest — 255/45 R19 104Y', value: '255/45 R19 104Y' },
  { group: 'Tesla Model S', label: '21" Arachnid Av — 265/35 R21', value: '265/35 R21' },
  { group: 'Tesla Model S', label: '21" Arachnid Ar — 295/30 R21', value: '295/30 R21' },
  { group: 'Tesla Model X', label: '20" Cyberstream — 265/45 R20 / 275/45 R20', value: '265/45 R20' },
  { group: 'Autre', label: 'Dimension personnalisée...', value: 'CUSTOM' },
]

function onDimensionPresetChange() {
  if (dimensionPreset.value === 'CUSTOM') {
    isCustomDimension.value = true
    addTireForm.value.dimension = ''
  } else {
    isCustomDimension.value = false
    addTireForm.value.dimension = dimensionPreset.value
  }
}

// Mounted tires start at the vehicle's current odometer
watch(open, (isOpen) => {
  if (isOpen && props.currentOdometer) addTireForm.value.mounted_odometer = Math.round(props.currentOdometer)
})

async function handleCreateTires() {
  if (!props.vehicleId) return
  if (!addTireForm.value.brand || !addTireForm.value.model || !addTireForm.value.dimension) {
    showAlert('Veuillez renseigner la marque, le modèle et la dimension', 'Champs requis', 'warning')
    return
  }

  try {
    if (addType.value === 'SINGLE') {
      await api.createTire(props.vehicleId, {
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        purchase_price: addTireForm.value.unit_price,
        current_position: addTireForm.value.current_position,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addTireForm.value.current_position !== 'STORAGE' ? addTireForm.value.mounted_odometer : null,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    } else {
      await api.batchCreateTires(props.vehicleId, {
        type: addType.value,
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        total_price: isTotalPrice.value ? addTireForm.value.total_price : 0,
        unit_price: !isTotalPrice.value ? addTireForm.value.unit_price : 0,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addType.value.includes('STORAGE') ? null : addTireForm.value.mounted_odometer,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    }

    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Plus class="w-5 h-5 text-rose-500" />
          Ajouter des pneus
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

      <!-- Add Type Selection -->
      <div class="space-y-1.5">
        <span class="block text-xs font-semibold text-slate-300">Format d'enregistrement :</span>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
          <button
            type="button"
            @click="addType = 'SET_4'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_4' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Train complet monté (4 pneus)
          </button>
          <button
            type="button"
            @click="addType = 'SET_4_STORAGE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_4_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Pack au garage (4 pneus hiver/été)
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_FRONT'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_FRONT' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Essieu avant monté (2 pneus)
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_REAR'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_REAR' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Essieu arrière monté (2 pneus)
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_STORAGE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Paire au garage (2 pneus)
          </button>
          <button
            type="button"
            @click="addType = 'SINGLE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SINGLE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            Pneu isolé (achat unique)
          </button>
        </div>
      </div>

      <!-- Position (single tire only) -->
      <div v-if="addType === 'SINGLE'" class="space-y-1">
        <label for="tire-add-tire-position" class="block text-xs font-semibold text-slate-400">Position :</label>
        <select id="tire-add-tire-position"
          v-model="addTireForm.current_position"
          class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
        >
          <option value="FL">Avant Gauche (FL)</option>
          <option value="FR">Avant Droit (FR)</option>
          <option value="RL">Arrière Gauche (RL)</option>
          <option value="RR">Arrière Droit (RR)</option>
          <option value="STORAGE">Stock au garage (non monté)</option>
        </select>
      </div>

      <!-- Dimension Dropdown -->
      <div class="space-y-1">
        <label for="tire-dimension-preset" class="block text-xs font-semibold text-slate-400">Dimension homologuée :</label>
        <select id="tire-dimension-preset"
          v-model="dimensionPreset"
          @change="onDimensionPresetChange"
          class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
        >
          <option v-for="p in teslaDimensionPresets" :key="p.value" :value="p.value">
            {{ p.label }}
          </option>
        </select>
        <div v-if="isCustomDimension" class="pt-1.5">
          <label for="tire-add-tire-dimension" class="sr-only">Ex: 245/40 R19 98Y</label>
          <input id="tire-add-tire-dimension"
            v-model="addTireForm.dimension"
            type="text"
            placeholder="Ex: 245/40 R19 98Y"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 font-mono"
          />
        </div>
      </div>

      <!-- Brand, Model, Season -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label for="tire-add-tire-brand" class="block text-xs font-semibold text-slate-400 mb-1">Marque</label>
          <input id="tire-add-tire-brand"
            v-model="addTireForm.brand"
            type="text"
            placeholder="Michelin, Pirelli, Hankook..."
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>
        <div>
          <label for="tire-add-tire-model" class="block text-xs font-semibold text-slate-400 mb-1">Modèle</label>
          <input id="tire-add-tire-model"
            v-model="addTireForm.model"
            type="text"
            placeholder="Pilot Sport EV, Winter Sottozero..."
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>
        <div>
          <label for="tire-add-tire-season" class="block text-xs font-semibold text-slate-400 mb-1">Saison</label>
          <select id="tire-add-tire-season"
            v-model="addTireForm.season"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          >
            <option value="SUMMER">☀️ Été</option>
            <option value="WINTER">❄️ Hiver</option>
            <option value="ALL_SEASON">🌦️ 4 Saisons</option>
          </select>
        </div>
      </div>

      <!-- Pricing & Lifespan -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-950/60 p-3.5 rounded-2xl border border-slate-800">
        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="tire-add-tire-total-price" class="text-xs font-semibold text-slate-400">
              {{ isTotalPrice ? 'Prix total du lot (€)' : 'Prix par pneu (€)' }}
            </label>
            <button
              type="button"
              @click="isTotalPrice = !isTotalPrice"
              class="text-[10px] text-rose-400 hover:text-rose-300 underline"
            >
              Passer en {{ isTotalPrice ? 'prix unitaire' : 'prix total' }}
            </button>
          </div>
          <input id="tire-add-tire-total-price"
            v-if="isTotalPrice"
            v-model.number="addTireForm.total_price"
            type="number"
            step="10"
            class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
          <input id="tire-add-tire-total-price"
            v-else
            v-model.number="addTireForm.unit_price"
            type="number"
            step="5"
            class="w-full bg-slate-900 text-emerald-400 font-bold text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>

        <div>
          <label for="tire-add-tire-estimated-lifespan-km" class="block text-xs font-semibold text-slate-400 mb-1">Durée de vie estimée (km)</label>
          <input id="tire-add-tire-estimated-lifespan-km"
            v-model.number="addTireForm.estimated_lifespan_km"
            type="number"
            step="5000"
            class="w-full bg-slate-900 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>
      </div>

      <!-- Km already driven (second-hand) -->
      <div>
        <label for="tire-add-tire-accumulated-distance-km" class="block text-xs font-semibold text-slate-400 mb-1">Km déjà parcourus (si occasion)</label>
        <input id="tire-add-tire-accumulated-distance-km"
          v-model.number="addTireForm.accumulated_distance_km"
          type="number"
          class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
        />
      </div>

      <!-- Date & Sculptures -->
      <div class="grid grid-cols-3 gap-3">
        <div>
          <label for="tire-add-tire-purchase-date" class="block text-[11px] text-slate-400 mb-1">Date d'achat</label>
          <AppDatePicker
            id="tire-add-tire-purchase-date"
            v-model="addTireForm.purchase_date"
            size="xs"
            :clearable="true"
          />
        </div>
        <div>
          <label for="tire-add-tire-initial-depth-mm" class="block text-[11px] text-slate-400 mb-1">Gomme neuve (mm)</label>
          <input id="tire-add-tire-initial-depth-mm"
            v-model.number="addTireForm.initial_depth_mm"
            type="number"
            step="0.1"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
          />
        </div>
        <div>
          <label for="tire-add-tire-min-legal-depth-mm" class="block text-[11px] text-slate-400 mb-1">Témoin légal (mm)</label>
          <input id="tire-add-tire-min-legal-depth-mm"
            v-model.number="addTireForm.min_legal_depth_mm"
            type="number"
            step="0.1"
            class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-2.5 py-1.5 border border-slate-700"
          />
        </div>
      </div>

      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-3 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 rounded-xl text-xs font-semibold text-slate-300 transition-colors"
        >
          Annuler
        </button>
        <button
          type="button"
          @click="handleCreateTires"
          class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2 rounded-xl shadow-lg shadow-rose-600/20 transition-colors"
        >
          Créer les pneus
        </button>
      </div>
    </div>
  </div>
</template>
