<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  LayoutDashboard,
  Navigation as NavIcon,
  Disc,
  Receipt,
  Car,
  LogOut,
  Zap,
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const navItems = [
  { name: 'dashboard', label: 'Tableau de bord', path: '/', icon: LayoutDashboard },
  { name: 'drives', label: 'Trajets', path: '/drives', icon: NavIcon },
  { name: 'tires', label: 'Pneus', path: '/tires', icon: Disc },
  { name: 'expenses', label: 'Dépenses', path: '/expenses', icon: Receipt },
  { name: 'vehicles', label: 'Véhicules', path: '/vehicles', icon: Car },
]

const currentRouteName = computed(() => route.name)

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<template>
  <!-- Desktop Sidebar -->
  <aside class="hidden md:flex flex-col w-64 bg-slate-900 border-r border-slate-800 p-4 shrink-0">
    <div class="flex items-center gap-3 px-3 py-4 mb-4">
      <div class="p-2 bg-gradient-to-tr from-rose-500 to-amber-500 rounded-xl shadow-lg shadow-rose-500/20">
        <Zap class="w-6 h-6 text-white" />
      </div>
      <div>
        <h1 class="text-xl font-bold tracking-tight bg-gradient-to-r from-white to-slate-400 bg-clip-text text-transparent">
          TeslaCost
        </h1>
        <p class="text-xs text-slate-400">TCO & Fleet Manager</p>
      </div>
    </div>

    <nav class="flex-1 space-y-1">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="item.path"
        class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all"
        :class="
          currentRouteName === item.name
            ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20 shadow-sm'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
        "
      >
        <component :is="item.icon" class="w-5 h-5" />
        {{ item.label }}
      </router-link>
    </nav>

    <div class="pt-4 border-t border-slate-800 mt-auto">
      <div class="flex items-center justify-between px-3 py-2">
        <div class="truncate">
          <p class="text-xs font-semibold text-slate-200 truncate">{{ authStore.user?.email }}</p>
          <p class="text-[10px] text-slate-400">Connecté</p>
        </div>
        <button
          @click="handleLogout"
          class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
          title="Déconnexion"
        >
          <LogOut class="w-4 h-4" />
        </button>
      </div>
    </div>
  </aside>

  <!-- Mobile Bottom Navigation Bar -->
  <nav class="md:hidden fixed bottom-0 left-0 right-0 z-50 bg-slate-900/90 backdrop-blur-md border-t border-slate-800 px-2 py-1 safe-area-pb">
    <div class="flex items-center justify-around">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="item.path"
        class="flex flex-col items-center py-1.5 px-3 rounded-lg text-[10px] font-medium transition-colors"
        :class="
          currentRouteName === item.name
            ? 'text-rose-400 font-semibold'
            : 'text-slate-400 hover:text-slate-200'
        "
      >
        <component :is="item.icon" class="w-5 h-5 mb-0.5" />
        <span>{{ item.label }}</span>
      </router-link>
    </div>
  </nav>
</template>
