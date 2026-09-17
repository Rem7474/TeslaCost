<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Zap } from 'lucide-vue-next'

const router = useRouter()

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    localStorage.setItem('teslacost_token', token)
    router.replace('/')
  } else {
    router.replace('/login?error=oidc_failed')
  }
})
</script>

<template>
  <div class="min-h-screen flex flex-col items-center justify-center gap-4 bg-slate-950">
    <div class="inline-flex p-3 bg-gradient-to-tr from-rose-500 to-amber-500 rounded-2xl shadow-lg shadow-rose-500/20 animate-pulse">
      <Zap class="w-8 h-8 text-white" />
    </div>
    <p class="text-slate-400 text-sm">Connexion SSO en cours…</p>
  </div>
</template>