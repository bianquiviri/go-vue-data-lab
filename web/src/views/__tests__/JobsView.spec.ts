import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import JobsView from '../JobsView.vue'
import { server } from '@/__tests__/mocks/server'
import { get, installEventSource, json, pagedEnvelope, post, useLocale } from '@/__tests__/fixtures'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'
import type { JobExecution, JobProgress, JobStatus } from '@/types/jobs'

const JOB_TOKEN = 'v1.c_NBk-opaque-job-token'
const ACCESS_TOKEN = 'access-token-1'

const PROGRESS: JobProgress = {
  job_id: JOB_TOKEN,
  job_type: 'generate',
  status: 'running',
  entities: [
    { entity: 'customers', done: 40, total: 100, percent: 40, processed: 40, failed: 2, skipped: 1 },
  ],
  percent: 40,
  processed: 40,
  failed: 2,
  skipped: 1,
}

const IDLE_STATUS: JobStatus = {
  id: JOB_TOKEN,
  type: '',
  status: 'idle',
  running: false,
  started_at: null,
  progress: { job_id: JOB_TOKEN, job_type: '', status: 'idle', entities: [], percent: 0, processed: 0, failed: 0, skipped: 0 },
}

const RUNNING_STATUS: JobStatus = {
  id: JOB_TOKEN,
  type: 'generate',
  status: 'running',
  running: true,
  started_at: '2026-01-01T00:00:00Z',
  progress: PROGRESS,
}

const FAILED_STATUS: JobStatus = {
  ...RUNNING_STATUS,
  status: 'failed',
  running: false,
  error: 'boom: batch 3 failed',
}

const HISTORY: JobExecution[] = [
  {
    id: JOB_TOKEN,
    job_name: 'generate',
    status: 'completed',
    started_at: '2026-01-01T00:00:00Z',
    finished_at: '2026-01-01T00:01:00Z',
    error: null,
    metrics: { customers: 100 },
  },
]

async function mountView(role: 'admin' | 'operator' | 'viewer' = 'admin') {
  const auth = useAuthStore()
  auth.user = makeUser({ role })
  auth.accessToken = ACCESS_TOKEN
  auth.initialized = true

  const router = createAppRouter()
  await router.push('/jobs')
  await router.isReady()

  const wrapper = mount(JobsView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

/** Handlers por defecto para que la vista monte sin errores. */
function defaultHandlers() {
  server.use(
    get('/jobs/status', () => json({ data: IDLE_STATUS })),
    get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
  )
}

describe('views/JobsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('renderiza el estado idle y el historial paginado', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView()

    expect(wrapper.get('[data-test="jobs-status"]').text()).toContain('Inactivo')
    expect(wrapper.findAll('[data-test="data-table-row"]').length).toBeGreaterThan(0)
    es.restore()
  })

  it('un viewer ve los controles de start/stop deshabilitados con una pista', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView('viewer')

    expect(wrapper.get('[data-test="jobs-generate"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="jobs-migrate"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="jobs-stop"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="jobs-viewer-hint"]').text()).toContain('solo lectura')
    es.restore()
  })

  it('un operator puede iniciar y detener (según el estado)', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView('operator')

    expect(wrapper.get('[data-test="jobs-generate"]').attributes('disabled')).toBeUndefined()
    // Stop solo con job activo: idle ⇒ deshabilitado.
    expect(wrapper.get('[data-test="jobs-stop"]').attributes('disabled')).toBeDefined()
    es.restore()
  })

  it('muestra el progreso en vivo (percent y entidades) desde el SSE', async () => {
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: RUNNING_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
    )
    const { wrapper } = await mountView()
    es.latest()!.open()
    await flushPromises()

    es.latest()!.emit('progress', PROGRESS)
    await flushPromises()

    expect(wrapper.get('[data-test="jobs-percent"]').text()).toContain('40')
    expect(wrapper.text()).toContain('customers')
    // Los contadores por entidad se reflejan.
    expect(wrapper.get('[data-test="jobs-processed"]').text()).toContain('40')
    es.restore()
  })

  it('destaca el error de un job fallido', async () => {
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: FAILED_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
    )
    const { wrapper } = await mountView()
    await flushPromises()

    const error = wrapper.get('[data-test="jobs-error"]')
    expect(error.text()).toContain('boom: batch 3 failed')
    es.restore()
  })

  it('iniciar generate dispara POST /jobs/generate/start con los params', async () => {
    const es = installEventSource()
    let called = false
    let body: unknown = null
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/generate/start', async (info) => {
        called = true
        body = await info.request.json()
        return json({ data: { job_id: JOB_TOKEN, type: 'generate', status: 'running' } }, 202)
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-generate"]').trigger('click')
    await flushPromises()
    // El diálogo de params se abre; se confirma el envío.
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    expect(called).toBe(true)
    expect(body).toBeTypeOf('object')
    es.restore()
  })

  it('un 409 al iniciar muestra el mensaje de conflicto', async () => {
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/generate/start', () =>
        json({ error: { code: 'conflict', message: 'a job is already running' } }, 409),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-generate"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="jobs-action-error"]').text()).toContain('Ya hay un job en ejecución')
    es.restore()
  })

  it('detener dispara POST /jobs/stop cuando hay job activo', async () => {
    const es = installEventSource()
    let stopped = false
    server.use(
      get('/jobs/status', () => json({ data: RUNNING_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/stop', () => {
        stopped = true
        return json({ data: { stopped: true } })
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-stop"]').trigger('click')
    await flushPromises()
    expect(stopped).toBe(true)
    es.restore()
  })

  it('muestra el aviso cuando el stream SSE se pierde y ofrece reconectar', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView()

    es.latest()!.fail()
    await flushPromises()

    expect(wrapper.find('[data-test="jobs-live-disconnected"]').exists()).toBe(true)
    await wrapper.get('[data-test="jobs-reconnect"]').trigger('click')
    await flushPromises()
    expect(es.instances.length).toBeGreaterThan(1)
    es.restore()
  })

  it('iniciar migrate envía los params parseados (entidades por coma)', async () => {
    const es = installEventSource()
    let body: unknown = null
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/migrate/start', async (info) => {
        body = await info.request.json()
        return json({ data: { job_id: JOB_TOKEN, type: 'migrate', status: 'running' } }, 202)
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-migrate"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="jobs-param-entities"]').setValue('customers, products')
    await wrapper.get('[data-test="jobs-param-workers"]').setValue('4')
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    expect(body).toEqual({ entities: ['customers', 'products'], workers: 4 })
    es.restore()
  })

  it('iniciar generate envía los campos numéricos rellenados (casting number de Vue)', async () => {
    // Regresión: los `Input type="number"` emiten NÚMEROS (Vue castea el valor),
    // no cadenas. Los helpers de params deben aceptar ambos tipos.
    const es = installEventSource()
    let body: unknown = null
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/generate/start', async (info) => {
        body = await info.request.json()
        return json({ data: { job_id: JOB_TOKEN, type: 'generate', status: 'running' } }, 202)
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-generate"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="jobs-param-seed"]').setValue('42')
    await wrapper.get('[data-test="jobs-param-customers"]').setValue('100')
    await wrapper.get('[data-test="jobs-param-batch-size"]').setValue('3000')
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    expect(body).toEqual({ seed: 42, customers: 100, batch_size: 3000 })
    // Sin errores de UI: el diálogo se cerró tras el arranque.
    expect(wrapper.find('[data-test="jobs-action-error"]').exists()).toBe(false)
    es.restore()
  })

  it('el diálogo de migrate etiqueta el campo entities con i18n (sin texto duro)', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-migrate"]').trigger('click')
    await flushPromises()

    const label = wrapper
      .get('[data-test="jobs-param-entities"]')
      .element.closest('label') as HTMLElement
    expect(label.textContent).toContain('Entidades')
    es.restore()
  })

  it('el job_id del start viaja verbatim: el chip del historial lo expone íntegro', async () => {
    const es = installEventSource()
    defaultHandlers()
    const { wrapper } = await mountView()

    // Regresión de IDs opacos: el chip del historial conserva el token exacto.
    const chip = wrapper.get('[data-test="id-chip"]')
    expect(chip.attributes('title')).toBe(JOB_TOKEN)
    es.restore()
  })

  it('al iniciar un job reabre el stream SSE (el anterior se cerró al terminar el job previo)', async () => {
    // Tras un job terminal el backend cierra el stream y el `EventSource` no
    // debe reconectar solo (bucle) ⇒ la vista reabre el stream al arrancar un
    // job nuevo para que el progreso siga llegando en vivo.
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/generate/start', () =>
        json({ data: { job_id: JOB_TOKEN, type: 'generate', status: 'running' } }, 202),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-generate"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    // Stream inicial + stream reabierto para el job nuevo.
    expect(es.instances.length).toBeGreaterThan(1)
    expect(es.latest()!.url).toContain(`token=${encodeURIComponent(ACCESS_TOKEN)}`)
    es.restore()
  })

  it('muestra los estados de job traducidos (no el valor crudo del backend)', async () => {
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: RUNNING_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
    )
    const { wrapper } = await mountView()

    // El badge y el detalle usan la traducción (no 'running' en inglés).
    expect(wrapper.get('[data-test="jobs-status-badge"]').text()).toBe('En ejecución')
    expect(wrapper.get('[data-test="jobs-status"]').text()).toBe('En ejecución')
    // El tipo de job también se traduce.
    expect(wrapper.get('[data-test="jobs-type"]').text()).toBe('Generación de datos')
    es.restore()
  })

  it('el error de la acción se muestra dentro del diálogo de parámetros', async () => {
    const es = installEventSource()
    server.use(
      get('/jobs/status', () => json({ data: IDLE_STATUS })),
      get('/jobs/history', () => json(pagedEnvelope(HISTORY))),
      post('/jobs/generate/start', () =>
        json({ error: { code: 'conflict', message: 'a job is already running' } }, 409),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="jobs-generate"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="jobs-params-confirm"]').trigger('click')
    await flushPromises()

    // El error sigue visible y, además, dentro del diálogo (que es lo que el
    // usuario tiene delante en ese momento).
    const error = wrapper.get('[data-test="jobs-action-error"]')
    expect(error.text()).toContain('Ya hay un job en ejecución')
    expect(wrapper.get('[data-test="jobs-params-dialog"]').text()).toContain('Ya hay un job en ejecución')
    es.restore()
  })
})
