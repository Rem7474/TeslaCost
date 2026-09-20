<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type VehicleWebhook } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { X, AlertTriangle, Loader2, CheckCircle2, Radio } from 'lucide-vue-next'

// The vehicle's notification webhook. `webhook` is the saved one (null when none); changes are reported through update:webhook.
const props = defineProps<{ vehicleId: string; webhook: VehicleWebhook | null }>()
const emit = defineEmits<{ 'update:webhook': [webhook: VehicleWebhook | null] }>()
const open = defineModel<boolean>('open', { required: true })
const { showConfirm, showAlert } = useConfirm()

const vehicleWebhook = computed(() => props.webhook)
const webhookForm = ref({
  url: '',
  type: 'DISCORD',
  enabled: true,
})
const isTestingWebhook = ref(false)
const isSavingWebhook = ref(false)
const webhookTestResult = ref<{ success: boolean; message: string } | null>(null)

watch(open, (isOpen) => {
  if (!isOpen) return
  webhookTestResult.value = null
  const wh = props.webhook
  webhookForm.value = wh ? { url: wh.url, type: wh.type, enabled: wh.enabled } : { url: '', type: 'DISCORD', enabled: true }
})

async function handleSaveWebhook() {
  if (!props.vehicleId) return
  if (!webhookForm.value.url.trim()) {
    showAlert('Veuillez saisir une URL de webhook valide', 'Champ requis', 'warning')
    return
  }
  isSavingWebhook.value = true
  try {
    const saved = await api.saveVehicleWebhook(props.vehicleId, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
      enabled: webhookForm.value.enabled,
    })
    emit('update:webhook', saved)
    showAlert('Configuration du webhook enregistrée avec succès', 'Succès', 'success')
    open.value = false
  } catch (err: any) {
    showAlert(`Erreur : ${err.message}`, 'Erreur', 'danger')
  } finally {
    isSavingWebhook.value = false
  }
}

async function handleTestWebhook() {
  if (!props.vehicleId) return
  if (!webhookForm.value.url.trim()) {
    showAlert('Saisissez d\'abord une URL de webhook', 'Champ requis', 'warning')
    return
  }
  isTestingWebhook.value = true
  webhookTestResult.value = null
  try {
    const res = await api.testVehicleWebhook(props.vehicleId, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
    })
    webhookTestResult.value = res
  } catch (err: any) {
    webhookTestResult.value = { success: false, message: err.message || 'Erreur inattendue' }
  } finally {
    isTestingWebhook.value = false
  }
}

async function handleDeleteWebhook() {
  if (!props.vehicleId) return
  const ok = await showConfirm({
    title: 'Supprimer le webhook',
    message: 'Êtes-vous sûr de vouloir supprimer l\'intégration webhook pour ce véhicule ?',
    confirmText: 'Supprimer',
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteVehicleWebhook(props.vehicleId)
    emit('update:webhook', null)
    webhookForm.value = { url: '', type: 'DISCORD', enabled: true }
    showAlert('Webhook supprimé', 'Succès', 'success')
    open.value = false
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
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Radio class="w-5 h-5 text-violet-400" />
          Notifications Webhook Homelab
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="webhook-modal-form" @submit.prevent="handleSaveWebhook" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <p class="text-xs text-slate-400">
          Configurez un webhook sortant vers votre serveur domotique ou de messagerie (Discord, Telegram, Gotify). Dès que TeslaMate remonte un kilométrage franchissant le seuil d'alerte, une notification est automatiquement expédiée.
        </p>

        <div>
          <label for="webhook-form-type" class="block text-xs font-semibold text-slate-300 mb-1">Plateforme / Connecteur</label>
          <select
            id="webhook-form-type"
            v-model="webhookForm.type"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
          >
            <option value="DISCORD">Discord (Webhook Embed)</option>
            <option value="TELEGRAM">Telegram Bot (API sendMessage)</option>
            <option value="GOTIFY">Gotify (Push notification)</option>
            <option value="GENERIC">Générique (JSON standard)</option>
          </select>
        </div>

        <div>
          <label for="webhook-form-url" class="block text-xs font-semibold text-slate-300 mb-1">URL cible du Webhook</label>
          <input
            id="webhook-form-url"
            v-model="webhookForm.url"
            type="url"
            required
            placeholder="https://discord.com/api/webhooks/... ou https://api.telegram.org/bot<token>/sendMessage?chat_id=<id>"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white"
          />
          <p class="text-[11px] text-slate-400 mt-1">
            Pour Telegram, l'URL doit contenir le token du bot et le chat_id en paramètre.
          </p>
        </div>

        <div class="flex items-center gap-2 pt-1">
          <input
            id="webhook-form-enabled"
            v-model="webhookForm.enabled"
            type="checkbox"
            class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
          />
          <label for="webhook-form-enabled" class="text-xs text-slate-300 cursor-pointer">
            Activer les notifications automatiques en tâche de fond
          </label>
        </div>

        <!-- Test result banner -->
        <div
          v-if="webhookTestResult"
          class="p-3 rounded-xl border text-xs flex items-center gap-2"
          :class="webhookTestResult.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300' : 'bg-rose-500/10 border-rose-500/30 text-rose-300'"
        >
          <CheckCircle2 v-if="webhookTestResult.success" class="w-4 h-4 shrink-0 text-emerald-400" />
          <AlertTriangle v-else class="w-4 h-4 shrink-0 text-rose-400" />
          <span>{{ webhookTestResult.message }}</span>
        </div>

        <!-- Test button -->
        <div class="pt-2">
          <button
            type="button"
            @click="handleTestWebhook"
            :disabled="isTestingWebhook || !webhookForm.url"
            class="w-full px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
          >
            <Loader2 v-if="isTestingWebhook" class="w-4 h-4 animate-spin text-violet-400" />
            <Radio v-else class="w-4 h-4 text-violet-400" />
            <span>{{ isTestingWebhook ? 'Envoi du test en cours...' : 'Envoyer un message de test maintenant' }}</span>
          </button>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div>
          <button
            v-if="vehicleWebhook"
            type="button"
            @click="handleDeleteWebhook"
            class="px-3 py-1.5 bg-rose-900/20 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl border border-rose-800/40 transition-colors"
          >
            Supprimer le webhook
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
            Annuler
          </button>
          <button
            type="submit"
            form="webhook-modal-form"
            :disabled="isSavingWebhook"
            class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl transition-colors disabled:opacity-50 flex items-center gap-1.5"
          >
            <Loader2 v-if="isSavingWebhook" class="w-3.5 h-3.5 animate-spin" />
            <span>Enregistrer</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
