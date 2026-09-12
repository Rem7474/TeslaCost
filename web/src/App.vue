<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import Navigation from '@/components/Navigation.vue'
import TopBar from '@/components/TopBar.vue'

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

onMounted(async () => {
  await authStore.init()
  if (authStore.isAuthenticated) {
    await vehicleStore.fetchVehicles()
  }
})
</script>

<template>
  <div v-if="showDashboardLayout" class="flex h-screen overflow-hidden bg-slate-950">
    <Navigation />
    <div class="flex-1 flex flex-col min-w-0 overflow-y-auto pb-16 md:pb-0">
      <TopBar />
      <main class="flex-1 p-4 md:p-6 max-w-7xl w-full mx-auto">
        <router-view />
      </main>
    </div>
  </div>

  <div v-else class="min-h-screen bg-slate-950">
    <router-view />
  </div>
</template>
