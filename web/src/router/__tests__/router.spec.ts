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

  it('/login redirige a / cuando ya hay sesión', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/login')
    expect(router.currentRoute.value.name).toBe('dashboard')
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

  it('una ruta desconocida se resuelve en /', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/esto-no-existe')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('el dashboard es la ruta por defecto', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })
})
