import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

import DashboardView from '@/views/DashboardView.vue'
import DrivesView from '@/views/DrivesView.vue'
import TiresView from '@/views/TiresView.vue'
import ExpensesView from '@/views/ExpensesView.vue'
import VehiclesView from '@/views/VehiclesView.vue'
import LoginView from '@/views/LoginView.vue'
import RegisterView from '@/views/RegisterView.vue'
import OnboardingView from '@/views/OnboardingView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drives',
      name: 'drives',
      component: DrivesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/tires',
      name: 'tires',
      component: TiresView,
      meta: { requiresAuth: true },
    },
    {
      path: '/expenses',
      name: 'expenses',
      component: ExpensesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/vehicles',
      name: 'vehicles',
      component: VehiclesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/register',
      name: 'register',
      component: RegisterView,
    },
    {
      path: '/onboarding',
      name: 'onboarding',
      component: OnboardingView,
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

let checkedOnboarding = false
let needsOnboarding = false

async function checkOnboardingStatus() {
  if (checkedOnboarding) return needsOnboarding
  try {
    const res = await fetch('/api/auth/config')
    if (res.ok) {
      const data = await res.json()
      needsOnboarding = !!data.needs_onboarding
      checkedOnboarding = true
    }
  } catch {
    // fallback
  }
  return needsOnboarding
}

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // On first startup without account, route to onboarding
  if (!authStore.isAuthenticated) {
    const isFirstRun = await checkOnboardingStatus()
    if (isFirstRun && to.name !== 'onboarding') {
      return next({ name: 'onboarding' })
    }
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next({ name: 'login' })
  } else if ((to.name === 'login' || to.name === 'register' || to.name === 'onboarding') && authStore.isAuthenticated) {
    next({ name: 'dashboard' })
  } else {
    next()
  }
})

export default router
