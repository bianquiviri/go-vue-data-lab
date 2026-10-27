import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { RouterView } from 'vue-router'

import { createAppRouter } from '../index'
import { APP_ROUTES } from '../index'
import { useAuthStore } from '@/stores/auth'
import { get, installEventSource, json, makeUser, pagedEnvelope } from '@/__tests__/fixtures'
import { server } from '@/__tests__/mocks/server'

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
    expect(router.currentRoute.value.query.redirect).toBe('/admin')
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

  it('un rol sin permisos no entra a /admin', async () => {
    seedSession('viewer')
    const router = createAppRouter()
    await router.push('/admin')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('el rol admin sí entra a /admin', async () => {
    seedSession('admin')
    const router = createAppRouter()
    await router.push('/admin')
    expect(router.currentRoute.value.name).toBe('admin')
  })

  // --- Landing pública (Issue #39) ---

  it('/ sin sesión muestra la landing (pública, sin redirect a login)', async () => {
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('landing')
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('/ con sesión sigue mostrando la landing (la landing no se redirige)', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/')
    expect(router.currentRoute.value.name).toBe('landing')
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('la landing no exige sesión y está marcada como pública', async () => {
    const router = createAppRouter()
    await router.push('/')
    const record = router.getRoutes().find((route) => route.name === 'landing')
    expect(record).toBeDefined()
    expect(record?.meta.public).toBe(true)
    expect(record?.meta.requiresAuth).toBeUndefined()
  })

  it('con sesión, `/` monta la landing (no el dashboard) y su CTA lleva al dashboard', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/')
    await router.isReady()

    const wrapper = mount(RouterView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.find('[data-test="landing-title"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="landing-cta"]').attributes('href')).toBe('/dashboard')
    // El layout protegido (sidebar) no se renderiza: no se redirige al dashboard.
    expect(wrapper.find('[data-test="sidebar"]').exists()).toBe(false)
  })

  it('/dashboard con sesión entra al dashboard', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/dashboard')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('una ruta desconocida con sesión cae en la landing (catch-all ⇒ `/`)', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/esto-no-existe')
    expect(router.currentRoute.value.name).toBe('landing')
    expect(router.currentRoute.value.path).toBe('/')
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

  // --- Issue #20: rutas reales de datos ---

  it('registra las vistas reales de datos con sesión', async () => {
    seedSession()
    const router = createAppRouter()
    for (const name of ['customers', 'products', 'orders', 'target', 'order-detail']) {
      expect(router.getRoutes().some((route) => route.name === name)).toBe(true)
    }
  })

  it('/orders/:id resuelve al detalle y NO al listado', async () => {
    seedSession()
    const router = createAppRouter()
    const token = 'v1.order-opaque-token-abc'
    await router.push(`/orders/${token}`)

    expect(router.currentRoute.value.name).toBe('order-detail')
    // El token opaco llega intacto al param de la ruta (nunca se parsea).
    expect(router.currentRoute.value.params.id).toBe(token)
  })

  it('/orders con query extra sigue siendo el listado', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push({ path: '/orders', query: { status: 'paid', page: '2' } })

    expect(router.currentRoute.value.name).toBe('orders')
    expect(router.currentRoute.value.query.status).toBe('paid')
  })

  it('/target es la vista de destino y exige sesión', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/target')
    expect(router.currentRoute.value.name).toBe('target')
  })

  it('las secciones de datos pasan scope=source|target por props', () => {
    const protectedLayout = APP_ROUTES.find((route) => route.meta?.requiresAuth === true)
    const children = protectedLayout?.children ?? []
    const customers = children.find((child) => child.name === 'customers')
    expect(customers?.props).toEqual({ scope: 'source' })
    // El detalle también fija el scope del esquema.
    const detail = children.find((child) => child.name === 'order-detail')
    expect(detail?.props).toEqual({ scope: 'source' })
    // TargetView resuelve el scope internamente (no recibe props).
    const target = children.find((child) => child.name === 'target')
    expect(target?.props).toBeUndefined()
  })

  it('jobs y scheduler tienen vista real (ya no son placeholder)', async () => {
    seedSession()
    const router = createAppRouter()
    await router.push('/jobs')
    expect(router.currentRoute.value.name).toBe('jobs')
    const jobs = router.getRoutes().find((route) => route.name === 'jobs')
    expect(jobs?.meta.titleKey).toBe('nav.jobs.title')

    await router.push('/scheduler')
    expect(router.currentRoute.value.name).toBe('scheduler')
    const scheduler = router.getRoutes().find((route) => route.name === 'scheduler')
    expect(scheduler?.meta.titleKey).toBe('nav.scheduler.title')
    // Sin `descriptionKey`: esa meta solo la generan los placeholders.
    expect(scheduler?.meta.descriptionKey).toBeUndefined()
  })

  it('/scheduler monta la vista real del scheduler (no el placeholder)', async () => {
    seedSession()
    server.use(get('/scheduler/jobs', () => json({ data: [] })))
    const router = createAppRouter()
    await router.push('/scheduler')
    await router.isReady()
    const wrapper = mount(RouterView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.find('[data-test="scheduler-empty"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="placeholder"]').exists()).toBe(false)
  })

  it('/jobs monta la vista real de monitoreo (no el placeholder)', async () => {
    seedSession()
    const es = installEventSource()
    server.use(
      get('/jobs/status', () =>
        json({
          data: {
            id: 'v1.job',
            type: 'generate',
            status: 'idle',
            running: false,
            started_at: null,
            progress: { job_id: 'v1.job', job_type: '', status: 'idle', entities: [], percent: 0, processed: 0, failed: 0, skipped: 0 },
          },
        }),
      ),
      get('/jobs/history', () => json(pagedEnvelope([]))),
    )
    const router = createAppRouter()
    await router.push('/jobs')
    await router.isReady()
    const wrapper = mount(RouterView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.find('[data-test="jobs-status"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="placeholder"]').exists()).toBe(false)
    es.restore()
  })
})
