<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { KeyRound, Laptop, LogOut, ShieldCheck, Smartphone, UserRound } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useConfirm } from '@/composables/useConfirm'
import { describeRelativeTime, describeUserAgent } from '@/utils/userAgent'

interface Session {
  id: string
  started_at: string
  last_used_at: string
  ip?: string
  user_agent?: string
  current: boolean
}

const router = useRouter()
const authStore = useAuthStore()
const { showConfirm } = useConfirm()

const sessions = ref<Session[]>([])
const loading = ref(true)
const loadError = ref('')
const busyId = ref<string | null>(null)
const busyAll = ref(false)
const sessionError = ref('')

const hasPassword = computed(() => authStore.user?.has_password === true)

const current = ref('')
const next = ref('')
const confirmation = ref('')
const saving = ref(false)
const passwordError = ref('')
const passwordDone = ref('')

const isMobile = (ua?: string) => /Android|iPhone|iPad|iPod|Mobile/i.test(ua ?? '')

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    sessions.value = await api.getSessions()
  } catch (err: any) {
    loadError.value = err?.message || 'Impossible de charger les sessions.'
  } finally {
    loading.value = false
  }
}

// The server has cleared the cookies when the current session is closed: only the local state is left to reset.
async function endLocalSession() {
  await authStore.logout()
  router.push('/login')
}

async function revoke(session: Session) {
  sessionError.value = ''
  const ok = await showConfirm({
    title: session.current ? 'Se déconnecter' : 'Déconnecter cet appareil',
    message: session.current
      ? 'Vous allez être déconnecté de cet appareil.'
      : `${describeUserAgent(session.user_agent)} sera déconnecté. Il garde son accès jusqu'à 15 minutes, la durée du jeton d'accès.`,
    confirmText: 'Déconnecter',
    type: 'warning',
  })
  if (!ok) return
  busyId.value = session.id
  try {
    await api.revokeSession(session.id)
    if (session.current) {
      await endLocalSession()
      return
    }
    sessions.value = sessions.value.filter((s) => s.id !== session.id)
  } catch (err: any) {
    sessionError.value = err?.message || 'Impossible de déconnecter cet appareil.'
  } finally {
    busyId.value = null
  }
}

async function logoutEverywhere() {
  sessionError.value = ''
  const ok = await showConfirm({
    title: 'Déconnecter tous les appareils',
    message: 'Tous les appareils, y compris celui-ci, seront déconnectés. Vous devrez vous reconnecter partout.',
    confirmText: 'Tout déconnecter',
    type: 'danger',
  })
  if (!ok) return
  busyAll.value = true
  try {
    await api.logoutAll()
    await endLocalSession()
  } catch (err: any) {
    sessionError.value = err?.message || 'Impossible de déconnecter les appareils.'
  } finally {
    busyAll.value = false
  }
}

async function changePassword() {
  passwordError.value = ''
  passwordDone.value = ''
  if (next.value.length < 8) {
    passwordError.value = 'Le nouveau mot de passe doit faire au moins 8 caractères.'
    return
  }
  if (new TextEncoder().encode(next.value).length > 72) {
    passwordError.value = 'Le nouveau mot de passe ne doit pas dépasser 72 octets.'
    return
  }
  if (next.value !== confirmation.value) {
    passwordError.value = 'Les deux mots de passe ne correspondent pas.'
    return
  }
  saving.value = true
  try {
    const res = await api.changePassword({ current_password: current.value, new_password: next.value })
    current.value = next.value = confirmation.value = ''
    const others = res?.sessions_revoked ?? 0
    passwordDone.value = others > 0
      ? `Mot de passe modifié. ${others} autre(s) appareil(s) déconnecté(s).`
      : 'Mot de passe modifié.'
    await load()
  } catch (err: any) {
    passwordError.value = err?.message || 'Impossible de modifier le mot de passe.'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6">
    <div class="flex items-center gap-3">
      <div class="flex h-10 w-10 items-center justify-center rounded-xl border border-emerald-500/20 bg-emerald-500/10">
        <ShieldCheck class="h-5 w-5 text-emerald-400" aria-hidden="true" />
      </div>
      <div>
        <h1 class="text-xl font-bold text-white">Compte &amp; sécurité</h1>
        <p class="text-xs text-slate-400">Connexion, mot de passe et appareils connectés.</p>
      </div>
    </div>

    <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-identity">
      <h2 id="account-identity" class="mb-3 flex items-center gap-2 text-sm font-bold text-white">
        <UserRound class="h-4 w-4 text-slate-400" aria-hidden="true" />
        Identité
      </h2>
      <dl class="space-y-1 text-sm">
        <div class="flex flex-wrap gap-x-2">
          <dt class="text-slate-400">Adresse e-mail</dt>
          <dd class="break-all font-medium text-white">{{ authStore.user?.email }}</dd>
        </div>
        <div class="flex flex-wrap gap-x-2">
          <dt class="text-slate-400">Connexion</dt>
          <dd class="text-slate-200">{{ hasPassword ? 'Mot de passe local' : 'Fournisseur SSO' }}</dd>
        </div>
      </dl>
    </section>

    <section v-if="hasPassword" class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-password">
      <h2 id="account-password" class="mb-3 flex items-center gap-2 text-sm font-bold text-white">
        <KeyRound class="h-4 w-4 text-slate-400" aria-hidden="true" />
        Mot de passe
      </h2>
      <form class="space-y-4" novalidate @submit.prevent="changePassword">
        <div>
          <label for="pw-current" class="quick-label">Mot de passe actuel</label>
          <input id="pw-current" v-model="current" type="password" autocomplete="current-password" class="quick-input" />
        </div>
        <div>
          <label for="pw-new" class="quick-label">Nouveau mot de passe</label>
          <input id="pw-new" v-model="next" type="password" autocomplete="new-password" class="quick-input" aria-describedby="pw-hint" />
          <p id="pw-hint" class="mt-1 text-[11px] text-slate-400">8 caractères au minimum, 72 octets au maximum.</p>
        </div>
        <div>
          <label for="pw-confirm" class="quick-label">Confirmer le nouveau mot de passe</label>
          <input id="pw-confirm" v-model="confirmation" type="password" autocomplete="new-password" class="quick-input" />
        </div>
        <p v-if="passwordError" role="alert" class="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">{{ passwordError }}</p>
        <p v-if="passwordDone" role="status" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-300">{{ passwordDone }}</p>
        <button
          type="submit"
          :disabled="saving || !current || !next || !confirmation"
          class="min-h-12 rounded-xl bg-rose-600 px-5 text-sm font-bold text-white hover:bg-rose-500 disabled:opacity-50"
        >
          {{ saving ? 'Enregistrement…' : 'Modifier le mot de passe' }}
        </button>
        <p class="text-[11px] text-slate-400">Les autres appareils sont déconnectés après un changement de mot de passe.</p>
      </form>
    </section>

    <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-sessions">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 id="account-sessions" class="flex items-center gap-2 text-sm font-bold text-white">
          <Laptop class="h-4 w-4 text-slate-400" aria-hidden="true" />
          Appareils connectés
        </h2>
        <button
          type="button"
          :disabled="busyAll || loading"
          class="min-h-11 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 text-xs font-semibold text-rose-300 hover:bg-rose-500/20 disabled:opacity-50"
          @click="logoutEverywhere"
        >
          Tout déconnecter
        </button>
      </div>

      <p v-if="loading" class="text-sm text-slate-400">Chargement…</p>
      <p v-else-if="loadError" role="alert" class="text-sm text-rose-300">{{ loadError }}</p>
      <ul v-else class="space-y-2">
        <li
          v-for="s in sessions"
          :key="s.id"
          class="flex items-center justify-between gap-3 rounded-xl border bg-slate-950/60 p-3"
          :class="s.current ? 'border-emerald-500/30' : 'border-slate-800'"
        >
          <div class="flex min-w-0 items-start gap-3">
            <component :is="isMobile(s.user_agent) ? Smartphone : Laptop" class="mt-0.5 h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" />
            <div class="min-w-0">
              <p class="flex flex-wrap items-center gap-2 text-sm font-semibold text-white">
                {{ describeUserAgent(s.user_agent) }}
                <span v-if="s.current" class="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-emerald-300">Cet appareil</span>
              </p>
              <p class="text-xs text-slate-400">
                Actif {{ describeRelativeTime(s.last_used_at) }}<template v-if="s.ip"> · {{ s.ip }}</template>
              </p>
              <p class="text-[11px] text-slate-500">Connecté {{ describeRelativeTime(s.started_at) }}</p>
            </div>
          </div>
          <button
            type="button"
            :disabled="busyId === s.id"
            class="flex min-h-11 shrink-0 items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 px-3 text-xs font-semibold text-slate-200 hover:bg-slate-700 disabled:opacity-50"
            :aria-label="s.current ? 'Se déconnecter de cet appareil' : `Déconnecter ${describeUserAgent(s.user_agent)}`"
            @click="revoke(s)"
          >
            <LogOut class="h-3.5 w-3.5" aria-hidden="true" />
            <span class="hidden sm:inline">{{ s.current ? 'Se déconnecter' : 'Déconnecter' }}</span>
          </button>
        </li>
        <li v-if="sessions.length === 0" class="text-sm text-slate-400">Aucune session active.</li>
      </ul>
      <p v-if="sessionError" role="alert" class="mt-3 text-xs text-rose-300">{{ sessionError }}</p>
      <p class="mt-3 text-[11px] text-slate-500">Un appareil déconnecté garde son accès jusqu'à 15 minutes, la durée de vie du jeton d'accès.</p>
    </section>
  </div>
</template>
