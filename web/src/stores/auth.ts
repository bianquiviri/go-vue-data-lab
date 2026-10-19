import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { changePasswordRequest, loginRequest, logoutRequest, meRequest, refreshRequest } from '@/api/auth'
import { ApiError, configureAuthHooks } from '@/api/client'
import { tokenStorage } from '@/lib/tokenStorage'
import type { AuthSession, LoginPayload, User, UserRole } from '@/types/auth'

/** Refresca el access token 60 s antes de que expire. */
const REFRESH_MARGIN_MS = 60_000

/**
 * Store de sesión: el access token vive SOLO en memoria; el refresh token se
 * persiste mediante la capa aislada `tokenStorage` (localStorage por ahora).
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const accessToken = ref<string | null>(null)
  const initialized = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)

  let refreshTimer: ReturnType<typeof setTimeout> | null = null

  const isAuthenticated = computed(() => accessToken.value !== null)
  const role = computed<UserRole | null>(() => user.value?.role ?? null)
  const isAdmin = computed(() => role.value === 'admin')

  function cancelScheduledRefresh(): void {
    if (refreshTimer !== null) {
      clearTimeout(refreshTimer)
      refreshTimer = null
    }
  }

  function clearSession(): void {
    accessToken.value = null
    user.value = null
    tokenStorage.clear()
    cancelScheduledRefresh()
  }

  function scheduleRefresh(expiresIn: number): void {
    cancelScheduledRefresh()
    const delay = Math.max(expiresIn * 1000 - REFRESH_MARGIN_MS, 5_000)
    refreshTimer = setTimeout(() => {
      // Se resuelve vía la instancia del store para poder interceptarlo en tests.
      const instance = useAuthStore()
      instance.refreshTokens().catch(() => clearSession())
    }, delay)
  }

  function applySession(session: AuthSession): void {
    accessToken.value = session.tokens.access_token
    user.value = session.user
    tokenStorage.setRefreshToken(session.tokens.refresh_token)
    scheduleRefresh(session.tokens.expires_in)
  }

  async function login(payload: LoginPayload): Promise<User> {
    loading.value = true
    error.value = null
    try {
      const session = await loginRequest(payload)
      applySession(session)
      return session.user
    } catch (err) {
      clearSession()
      error.value = err instanceof ApiError ? err.message : 'No se pudo conectar con el servidor'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function refreshTokens(): Promise<AuthSession> {
    const refreshToken = tokenStorage.getRefreshToken()
    if (!refreshToken) {
      throw new ApiError({ status: 401, code: 'unauthorized', message: 'No hay sesión activa' })
    }
    const session = await refreshRequest(refreshToken)
    applySession(session)
    return session
  }

  async function fetchMe(): Promise<User> {
    const me = await meRequest()
    user.value = me
    return me
  }

  /** Restaura la sesión desde el refresh token persistido (se llama al arrancar). */
  async function initialize(): Promise<void> {
    if (initialized.value) return
    if (!tokenStorage.getRefreshToken()) {
      initialized.value = true
      return
    }
    try {
      await refreshTokens()
      await fetchMe()
    } catch {
      clearSession()
    } finally {
      initialized.value = true
    }
  }

  async function logout(): Promise<void> {
    if (isAuthenticated.value) {
      try {
        await logoutRequest()
      } catch {
        // best-effort: el backend es stateless; la sesión local se limpia igual
      }
    }
    clearSession()
    error.value = null
  }

  function changePassword(oldPassword: string, newPassword: string): Promise<string> {
    return changePasswordRequest({ old_password: oldPassword, new_password: newPassword })
  }

  // El cliente HTTP delega la renovación del token en este store.
  configureAuthHooks({
    getAccessToken: () => accessToken.value,
    refresh: async () => {
      await refreshTokens()
    },
    onAuthFailure: () => clearSession(),
  })

  return {
    user,
    accessToken,
    initialized,
    loading,
    error,
    isAuthenticated,
    role,
    isAdmin,
    login,
    initialize,
    refreshTokens,
    fetchMe,
    logout,
    changePassword,
  }
})
