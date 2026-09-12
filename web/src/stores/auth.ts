import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/services/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('teslacost_token'))
  const user = ref<any | null>(null)
  const isAuthenticated = computed(() => !!token.value)

  async function init() {
    if (token.value && !user.value) {
      try {
        user.value = await api.getMe()
      } catch (err) {
        logout()
      }
    }
  }

  async function login(credentials: { email: string; password: string }) {
    const res = await api.login(credentials)
    token.value = res.token
    user.value = res.user
    localStorage.setItem('teslacost_token', res.token)
  }

  async function register(payload: { email: string; password: string }) {
    const res = await api.register(payload)
    token.value = res.token
    user.value = res.user
    localStorage.setItem('teslacost_token', res.token)
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('teslacost_token')
  }

  return {
    token,
    user,
    isAuthenticated,
    init,
    login,
    register,
    logout,
  }
})
