import type { ResponseResolverInfo } from 'msw'
import { http } from 'msw'
import { vi } from 'vitest'

import { i18n } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'
import type { AuthSession, Tokens, User } from '@/types/auth'

/** Factory para un usuario de prueba (contrato de GET /api/v1/auth/me). */
export function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: '4c1f5f42-0000-4000-8000-000000000001',
    email: 'root@gmail.com',
    role: 'admin',
    active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

/** Factory para tokens (contrato de login/refresh). */
export function makeTokens(overrides: Partial<Tokens> = {}): Tokens {
  return {
    access_token: 'access-token-1',
    refresh_token: 'refresh-token-1',
    token_type: 'Bearer',
    expires_in: 900,
    ...overrides,
  }
}

export function makeSession(user: User = makeUser(), tokens: Tokens = makeTokens()): AuthSession {
  return { user, tokens }
}

/** Envelope de error del backend: {"error":{"code","message","details"}}. */
export function errorEnvelope(code: string, message: string, details?: Record<string, unknown>) {
  return { error: { code, message, ...(details ? { details } : {}) } }
}

/** Envelope plano de los endpoints de auth del backend: {"error":"mensaje"}. */
export function flatError(message: string) {
  return { error: message }
}

/** Respuesta JSON para handlers de MSW. */
export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** Ruta comodín con origen absoluto (jsdom usa http://localhost:3000). */
export function api(path: string): string {
  return `*/api/v1${path}`
}

type MatchMediaListener = (event: { matches: boolean }) => void

export interface MatchMediaMock {
  matches: () => boolean
  setMatches: (next: boolean) => void
  emitChange: (next: boolean) => void
  listenerCount: () => number
}

/**
 * Instala un window.matchMedia controlable en jsdom y devuelve un manejador
 * para simular cambios de `prefers-color-scheme`.
 */
export function installMatchMedia(initial = false): MatchMediaMock {
  let current = initial
  const listeners = new Set<MatchMediaListener>()
  const mql = {
    media: '(prefers-color-scheme: dark)',
    get matches() {
      return current
    },
    addEventListener: (_type: string, cb: MatchMediaListener) => listeners.add(cb),
    removeEventListener: (_type: string, cb: MatchMediaListener) => listeners.delete(cb),
  }
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation(() => mql),
  )
  return {
    matches: () => current,
    setMatches: (next: boolean) => {
      current = next
    },
    emitChange: (next: boolean) => {
      current = next
      listeners.forEach((cb) => cb({ matches: next }))
    },
    listenerCount: () => listeners.size,
  }
}

/** Tipado auxiliar para resolvers de MSW que inspeccionan la request. */
export type ApiResolverInfo = ResponseResolverInfo<{ headers: Headers; method: string }>

/** Guarda el header Authorization visto por un handler de MSW. */
export function trackAuthHeader(store: { value: string | null }) {
  return (info: ApiResolverInfo) => {
    store.value = info.request.headers.get('Authorization')
  }
}

/** Handler POST de conveniencia. */
export function post(path: string, resolver: Parameters<typeof http.post>[1]) {
  return http.post(api(path), resolver)
}

/** Handler GET de conveniencia. */
export function get(path: string, resolver: Parameters<typeof http.get>[1]) {
  return http.get(api(path), resolver)
}

/**
 * Fija el idioma de forma determinista para los specs: actualiza i18n (para
 * `useI18n()`) y, si hay pinia activa, también el store (para los toggles y
 * `<html lang>`). Llamar en `beforeEach` de las suites que afirman cadenas.
 */
export function useLocale(locale: 'es' | 'en' = 'es'): void {
  i18n.global.locale.value = locale
  try {
    useLocaleStore().setLocale(locale)
  } catch {
    // Sin pinia activa (specs que montan con su propio pinia): basta con i18n.
  }
}
