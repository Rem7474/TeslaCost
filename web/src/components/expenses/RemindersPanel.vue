<script setup lang="ts">
import { useVehicleStore } from '@/stores/vehicle'
import type { MaintenanceReminder, VehicleWebhook } from '@/services/api'
import { Plus, Pencil, Trash2, AlertTriangle, Bell, Clock, CheckCircle2, Radio, Sparkles } from 'lucide-vue-next'
import { REMINDER_PRESETS, formatDate, type ReminderPreset } from '@/utils/expenses'

defineProps<{
  reminders: MaintenanceReminder[]
  overdueReminders: MaintenanceReminder[]
  dueSoonReminders: MaintenanceReminder[]
  okReminders: MaintenanceReminder[]
  loadingReminders: boolean
  vehicleWebhook: VehicleWebhook | null
}>()
const emit = defineEmits<{
  'open-webhook': []
  add: [preset: ReminderPreset | null]
  edit: [reminder: MaintenanceReminder]
  complete: [reminder: MaintenanceReminder]
  delete: [reminder: MaintenanceReminder]
}>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div class="space-y-4">
    <!-- Webhook homelab info banner -->
    <div class="p-4 bg-slate-900 border border-slate-800 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
      <div class="flex items-center gap-3">
        <div class="p-2 rounded-xl bg-violet-500/10 border border-violet-500/20 text-violet-400 shrink-0">
          <Radio class="w-5 h-5" />
        </div>
        <div>
          <div class="font-bold text-white flex items-center gap-2">
            <span>Webhook Homelab</span>
            <span
              class="px-2 py-0.5 text-[10px] rounded-full font-bold border"
              :class="vehicleWebhook?.enabled ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
            >
              {{ vehicleWebhook?.enabled ? `Actif (${vehicleWebhook.type})` : 'Non configuré' }}
            </span>
          </div>
          <p class="text-slate-400 text-[11px] mt-0.5">
            {{ vehicleWebhook?.enabled ? 'Les alertes sont automatiquement envoyées dès qu\'une synchronisation TeslaMate franchit le seuil.' : 'Recevez automatiquement des alertes sur Discord, Telegram ou Gotify dès qu\'une échéance approche.' }}
          </p>
        </div>
      </div>
      <button
        v-if="vehicleStore.canEdit"
        @click="emit('open-webhook')"
        class="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 shrink-0 transition-colors self-start sm:self-auto"
      >
        {{ vehicleWebhook ? 'Modifier le webhook' : 'Configurer un webhook' }}
      </button>
    </div>

    <!-- Quick summary stats -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
        <span class="text-xs text-slate-400">Total rappels</span>
        <span class="text-xl font-bold text-white mt-1">{{ reminders.length }}</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
        <span class="text-xs text-emerald-400 flex items-center gap-1.5">
          <CheckCircle2 class="w-3.5 h-3.5" />
          À jour
        </span>
        <span class="text-xl font-bold text-emerald-400 mt-1">{{ okReminders.length }}</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
        <span class="text-xs text-amber-400 flex items-center gap-1.5">
          <Clock class="w-3.5 h-3.5" />
          À prévoir
        </span>
        <span class="text-xl font-bold text-amber-400 mt-1">{{ dueSoonReminders.length }}</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex flex-col justify-between">
        <span class="text-xs text-rose-400 flex items-center gap-1.5">
          <AlertTriangle class="w-3.5 h-3.5" />
          En retard
        </span>
        <span class="text-xl font-bold text-rose-400 mt-1">{{ overdueReminders.length }}</span>
      </div>
    </div>

    <div v-if="loadingReminders" class="text-center py-12 text-slate-400">Chargement des rappels...</div>

    <!-- Empty state -->
    <div v-else-if="!reminders.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-4">
      <div class="p-3 bg-violet-500/10 border border-violet-500/20 text-violet-400 w-12 h-12 rounded-2xl mx-auto flex items-center justify-center">
        <Bell class="w-6 h-6" />
      </div>
      <div>
        <h3 class="text-base font-bold text-white">Aucun rappel d'entretien configuré</h3>
        <p class="text-xs text-slate-400 mt-1 max-w-md mx-auto">
          Suivez l'usure de vos pneumatiques, le remplacement du filtre habitacle, les révisions ou le contrôle technique avec des rappels par kilométrage et calendrier.
        </p>
      </div>

      <div v-if="vehicleStore.canEdit" class="pt-2">
        <p class="text-xs font-semibold text-slate-300 mb-3">Ajouter un rappel type en 1 clic :</p>
        <div class="flex flex-wrap justify-center gap-2 max-w-lg mx-auto">
          <button
            v-for="preset in REMINDER_PRESETS"
            :key="preset.title"
            type="button"
            @click="emit('add', preset)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded-xl border border-slate-700 transition-colors flex items-center gap-1.5"
          >
            <Sparkles class="w-3 h-3 text-violet-400" />
            {{ preset.title }}
          </button>
        </div>
      </div>

      <div v-if="vehicleStore.canEdit" class="pt-2">
        <button
          @click="emit('add', null)"
          class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl inline-flex items-center gap-2 shadow-lg shadow-violet-600/20"
        >
          <Plus class="w-4 h-4" />
          Créer un rappel personnalisé
        </button>
      </div>
    </div>

    <!-- Reminders List -->
    <div v-else class="space-y-3">
      <div
        v-for="r in reminders"
        :key="r.id"
        class="bg-slate-900 border p-4 rounded-2xl flex flex-col justify-between gap-3 transition-colors"
        :class="r.status === 'OVERDUE' ? 'border-rose-500/40 bg-rose-500/5' : r.status === 'DUE_SOON' ? 'border-amber-500/40 bg-amber-500/5' : 'border-slate-800'"
      >
        <!-- Card top -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div class="flex items-center gap-2.5 flex-wrap">
            <span
              class="text-xs px-2.5 py-0.5 rounded-full font-bold border flex items-center gap-1"
              :class="r.status === 'OVERDUE' ? 'bg-rose-500/20 text-rose-300 border-rose-500/30' : r.status === 'DUE_SOON' ? 'bg-amber-500/20 text-amber-300 border-amber-500/30' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'"
            >
              <AlertTriangle v-if="r.status === 'OVERDUE'" class="w-3 h-3" />
              <Clock v-else-if="r.status === 'DUE_SOON'" class="w-3 h-3" />
              <CheckCircle2 v-else class="w-3 h-3" />
              {{ r.status === 'OVERDUE' ? 'En retard' : r.status === 'DUE_SOON' ? 'À prévoir bientôt' : 'À jour' }}
            </span>

            <span class="text-xs px-2 py-0.5 rounded-lg bg-slate-800 text-slate-300 border border-slate-700">
              {{ r.category === 'TIRES' ? 'Pneumatiques' : 'Entretien' }}
            </span>

            <h4 class="text-sm font-bold text-white">{{ r.title }}</h4>

            <span
              v-if="r.webhook_enabled"
              class="text-[10px] px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-400 border border-violet-500/20 flex items-center gap-1"
              title="Notification webhook activée pour ce rappel"
            >
              <Radio class="w-2.5 h-2.5" />
              Webhook
            </span>
          </div>

          <!-- Due Badges / Urgency pill -->
          <div class="flex items-center gap-2 text-xs font-semibold">
            <span
              v-if="r.remaining_km !== null && r.remaining_km !== undefined"
              class="px-2 py-0.5 rounded-lg"
              :class="r.remaining_km <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_km <= r.lead_km ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
            >
              {{ r.remaining_km <= 0 ? `Dépassé de ${Math.abs(Math.round(r.remaining_km)).toLocaleString('fr-FR')} km` : `Reste ${Math.round(r.remaining_km).toLocaleString('fr-FR')} km` }}
            </span>
            <span
              v-if="r.remaining_days !== null && r.remaining_days !== undefined"
              class="px-2 py-0.5 rounded-lg"
              :class="r.remaining_days <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_days <= r.lead_days ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
            >
              {{ r.remaining_days <= 0 ? `Dépassé de ${Math.abs(r.remaining_days)} j` : `Reste ${r.remaining_days} j` }}
            </span>
          </div>
        </div>

        <!-- Card details grid -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5 pt-1 text-xs text-slate-300">
          <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
            <span class="text-[11px] text-slate-400 block mb-0.5">Échéance kilométrique</span>
            <span v-if="r.interval_km" class="font-medium text-white">
              Tous les {{ r.interval_km.toLocaleString('fr-FR') }} km
              <span v-if="r.due_odometer" class="text-slate-400 block text-[11px]">
                Échéance : {{ Math.round(r.due_odometer).toLocaleString('fr-FR') }} km
              </span>
            </span>
            <span v-else class="text-slate-400 italic">Non applicable</span>
          </div>

          <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
            <span class="text-[11px] text-slate-400 block mb-0.5">Échéance calendaire</span>
            <span v-if="r.interval_months" class="font-medium text-white">
              Tous les {{ r.interval_months }} mois
              <span v-if="r.due_date" class="text-slate-400 block text-[11px]">
                Échéance : {{ formatDate(r.due_date) }}
              </span>
            </span>
            <span v-else class="text-slate-400 italic">Non applicable</span>
          </div>

          <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
            <span class="text-[11px] text-slate-400 block mb-0.5">Dernière réalisation</span>
            <span class="font-medium text-white">
              {{ r.last_service_date ? formatDate(r.last_service_date) : 'Non renseigné' }}
              <span v-if="r.last_service_odometer" class="text-slate-400 block text-[11px]">
                à {{ Math.round(r.last_service_odometer).toLocaleString('fr-FR') }} km
              </span>
            </span>
          </div>
        </div>

        <!-- Card footer -->
        <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
          <div class="text-[11px] text-slate-400">
            <span v-if="r.last_notified_at">
              Dernière alerte webhook : {{ formatDate(r.last_notified_at) }}
            </span>
            <span v-else>
              Alerte anticipée : {{ r.lead_km.toLocaleString('fr-FR') }} km / {{ r.lead_days }} j avant
            </span>
          </div>

          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('complete', r)"
              class="px-2.5 py-1.5 bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 text-xs font-semibold rounded-xl border border-emerald-500/30 flex items-center gap-1.5 transition-colors"
              title="Marquer cet entretien comme effectué et mettre à jour le rappel"
            >
              <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
              <span>Marquer fait</span>
            </button>
            <button
              @click="emit('edit', r)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-violet-400 rounded-xl transition-colors border border-slate-700/60"
              title="Modifier ce rappel"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', r)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              title="Supprimer ce rappel"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
