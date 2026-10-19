import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { createAppRouter } from '../index'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'

describe('router/guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  function seedSession(role: 'admin' | 'operator' | 'viewer' = 'admin') {
    const store = useAuthStore()
    store.user = makeUser({ role })
    store.accessToken = 'access-token'
    store.initialized = true
  }

  it('redirige a /login guardando el destino en query.redirect', async () => {
    const router = createAppRouter()
    await router.push('/customers')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/customers')
  })

  it('las rutas hijas del layout también exigen sesión', async () => {
    const router = createAppRouter()
    await router.push('/admin/logs')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/admin/logs')
  })

  it('el dashboard (/dashboard) exige sesión', async () => {
    const router = createAppRouter()
    await router.push('/dashboard')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/dashboard')
  })

  it('/login redirige a /dashboard cuando ya hay sesión', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/login')
    expect(router.currentRoute.value.name).toBe('dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('/login con query.redirect devuelve al destino original', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push({ path: '/login', query: { redirect: '/jobs' } })
    expect(router.currentRoute.value.name).toBe('jobs')
  })

  it('un rol sin permisos no entra a /admin/logs', async () => {
    seedSession('viewer')
    const router = createAppRouter()
    await router.push('/admin/logs')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('el rol admin sí entra a /admin/logs', async () => {
    seedSession('admin')
    const router = createAppRouter()
    await router.push('/admin/logs')
    expect(router.currentRoute.value.name).toBe('admin-logs')
  })

  // --- Landing pública (Issue #39) ---

  it('/ sin sesión muestra la landing (pública, sin redirect a login)', async () => {
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('landing')
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('/ con sesión redirige a /dashboard', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('la landing no exige sesión y está marcada como pública', async () => {
    const router = createAppRouter()
    await router.push('/')
    const record = router.getRoutes().find((route) => route.name === 'landing')
    expect(record).toBeDefined()
    expect(record?.meta.public).toBe(true)
    expect(record?.meta.requiresAuth).toBeUndefined()
  })

  it('/dashboard con sesión entra al dashboard', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/dashboard')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('una ruta desconocida con sesión se resuelve en /dashboard', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/esto-no-existe')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('una ruta desconocida sin sesión cae en la landing', async () => {
    const router = createAppRouter()
    await router.push('/esto-no-existe')
    expect(router.currentRoute.value.name).toBe('landing')
  })

  it('no hay rutas duplicadas (una sola ruta por nombre)', async () => {
    const router = createAppRouter()
    const names = router.getRoutes().map((route) => route.name)
    const filtered = names.filter((name) => name !== undefined)
    expect(new Set(filtered).size).toBe(filtered.length)
  })
})
