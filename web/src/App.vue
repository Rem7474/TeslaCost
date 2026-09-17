<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import { useOfflineStore } from '@/stores/offline'
import Navigation from '@/components/Navigation.vue'
import TopBar from '@/components/TopBar.vue'
import ConfirmModal from '@/components/ConfirmModal.vue'

const route = useRoute()
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()

const showDashboardLayout = computed(() => {
  return (
    authStore.isAuthenticated &&
    route.name !== 'onboarding' &&
    route.name !== 'login' &&
    route.name !== 'register'
  )
})

const offlineStore = useOfflineStore()

onMounted(async () => {
  offlineStore.start()
  await authStore.init()
  if (authStore.isAuthenticated) {
    await vehicleStore.fetchVehicles()
    vehicleStore.resumeRunningSync()
  }
})
</script>

<template>
  <div v-if="showDashboardLayout" class="flex h-screen overflow-hidden bg-slate-950">
    <Navigation />
    <div class="flex-1 flex flex-col min-w-0 overflow-y-auto overflow-x-hidden pb-16 md:pb-0">
      <TopBar />
      <main class="flex-1 p-4 md:p-6 max-w-7xl w-full mx-auto">
        <!-- Attente de l'initialisation du store véhicule pour éviter un affichage vide au refresh -->
        <div v-if="!vehicleStore.isInitialized" class="flex flex-col items-center justify-center py-28 space-y-4">
          <div class="w-9 h-9 border-3 border-rose-500 border-t-transparent rounded-full animate-spin"></div>
          <p class="text-xs font-medium text-slate-400">Chargement de votre Tesla...</p>
        </div>
        <router-view v-else />
      </main>
    </div>
  </div>

  <div v-else class="h-full overflow-y-auto bg-slate-950">
    <router-view />
  </div>

  <ConfirmModal />
</template>
