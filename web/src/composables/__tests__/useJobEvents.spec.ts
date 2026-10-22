import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'

import { useJobEvents } from '../useJobEvents'
import { server } from '@/__tests__/mocks/server'
import { get, installEventSource, json, useLocale } from '@/__tests__/fixtures'
import { useAuthStore } from '@/stores/auth'
import type { JobProgress, JobStatus } from '@/types/jobs'

const JOB_TOKEN = 'v1.c_NBk-opaque-job-token'
const ACCESS_TOKEN = 'access-token+jwt/with=chars'

const PROGRESS: JobProgress = {
  job_id: JOB_TOKEN,
  job_type: 'generate',
  status: 'running',
  entities: [{ entity: 'customers', done: 40, total: 100, percent: 40, processed: 40, failed: 0, skipped: 0 }],
  percent: 40,
  processed: 40,
  failed: 0,
  skipped: 0,
}

const STATUS: JobStatus = {
  id: JOB_TOKEN,
  type: 'generate',
  status: 'running',
  running: true,
  started_at: '2026-01-01T00:00:00Z',
  progress: PROGRESS,
}

/** Componente host que monta el composable y expone su retorno. */
function mountComposable() {
  let api!: ReturnType<typeof useJobEvents>
  const Host = defineComponent({
    setup() {
      api = useJobEvents()
      return () => h('div')
    },
  })
  const wrapper = mount(Host)
  return { wrapper, api: () => api }
}

describe('composables/useJobEvents', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
    const auth = useAuthStore()
    auth.accessToken = ACCESS_TOKEN
  })

  it('crea el EventSource con la URL y el token (codificado) del store', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))

    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    const source = es.latest()
    expect(source).toBeDefined()
    expect(source!.url).toBe(`/api/v1/jobs/events?token=${encodeURIComponent(ACCESS_TOKEN)}`)
    es.restore()
  })

  it('parsea el evento progress y actualiza el estado', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    // En el flujo real, el snapshot llega al abrir la conexión.
    es.latest()!.open()
    await flushPromises()
    es.latest()!.emit('progress', PROGRESS)
    await nextTick()

    expect(api().progress.value?.percent).toBe(40)
    expect(api().progress.value?.job_id).toBe(JOB_TOKEN)
    expect(api().status.value?.status).toBe('running')
    es.restore()
  })

  it('refresca el snapshot vía GET /jobs/status al crear y al reconectar', async () => {
    const es = installEventSource()
    let statusCalls = 0
    server.use(
      get('/jobs/status', () => {
        statusCalls += 1
        return json({ data: STATUS })
      }),
    )
    const { api } = mountComposable()
    api().connect()
    await flushPromises()
    // Snapshot inmediato al conectar (no se espera al primer frame).
    expect(statusCalls).toBe(1)
    expect(api().status.value?.running).toBe(true)

    // La (re)apertura de la conexión vuelve a refrescar para no perder estado.
    es.latest()!.open()
    await flushPromises()
    expect(statusCalls).toBe(2)
    es.restore()
  })

  it('cierra el EventSource al desmontar (sin fugas)', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { wrapper, api } = mountComposable()
    api().connect()
    await flushPromises()

    wrapper.unmount()
    expect(es.latest()!.closed).toBe(true)
    es.restore()
  })

  it('ante un fallo marca desconectado y no entra en bucle de reconexión', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    es.latest()!.fail()
    await flushPromises()

    expect(api().disconnected.value).toBe(true)
    // Solo se creó una instancia: el composable no reintenta indefinidamente.
    expect(es.instances).toHaveLength(1)
    es.restore()
  })

  it('reconectar manualmente crea una nueva instancia y limpia el fallo', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()
    es.latest()!.fail()
    await flushPromises()
    expect(api().disconnected.value).toBe(true)

    api().reconnect()
    await flushPromises()
    expect(api().disconnected.value).toBe(false)
    expect(es.instances.length).toBeGreaterThan(1)
    es.restore()
  })

  it('usa el fallback GET /jobs/progress si el SSE no está disponible', async () => {
    const es = installEventSource()
    vi.stubGlobal('EventSource', undefined)
    let progressCalls = 0
    server.use(
      get('/jobs/progress', () => {
        progressCalls += 1
        return json({ data: PROGRESS })
      }),
    )
    const { api } = mountComposable()

    await api().refreshProgress()
    await flushPromises()

    expect(progressCalls).toBe(1)
    expect(api().progress.value?.percent).toBe(40)
    es.restore()
  })

  it('el cierre tras un evento terminal NO se marca como desconexión', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    // Frame terminal (el job terminó) seguido del cierre del stream por parte
    // del backend: es el final normal, no un fallo de conexión.
    es.latest()!.emit('progress', { ...PROGRESS, status: 'completed', percent: 100 })
    es.latest()!.fail()
    await flushPromises()

    expect(api().disconnected.value).toBe(false)
    es.restore()
  })

  it('cierra el stream al recibir el frame terminal (el backend lo cierra después)', async () => {
    // El backend cierra el stream tras el evento terminal y, además, cierra de
    // inmediato a cualquier suscriptor nuevo de un job ya terminal. Si el
    // `EventSource` se deja reconectar solo, eso se convierte en un bucle de
    // reconexiones (~3 s) contra el backend.
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    es.latest()!.emit('progress', { ...PROGRESS, status: 'completed', percent: 100 })
    await nextTick()

    expect(es.latest()!.closed).toBe(true)
    expect(api().connected.value).toBe(false)
    expect(api().disconnected.value).toBe(false)
    es.restore()
  })

  it('no deja que el EventSource reconecte solo cuando el navegador lo haría', async () => {
    // readyState CONNECTING: el navegador real reconectaría con la MISMA URL
    // (token viejo) una y otra vez. El composable cierra la fuente y ofrece
    // la reconexión manual.
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    es.latest()!.drop()
    await flushPromises()

    expect(es.latest()!.closed).toBe(true)
    expect(api().disconnected.value).toBe(true)
    // Ninguna reconexión automática: sigue habiendo una sola instancia.
    expect(es.instances).toHaveLength(1)
    es.restore()
  })

  it('cierra el stream si la sesión se pierde (token a null)', async () => {
    // Logout o expiración: el EventSource no puede seguir consumiendo el job
    // con un token muerto.
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    useAuthStore().accessToken = null
    await nextTick()

    expect(es.latest()!.closed).toBe(true)
    expect(api().connected.value).toBe(false)
    es.restore()
  })

  it('reabre el stream con el token rotado (refresh de sesión)', async () => {
    const es = installEventSource()
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const { api } = mountComposable()
    api().connect()
    await flushPromises()

    useAuthStore().accessToken = 'token-rotado'
    await nextTick()
    await flushPromises()

    expect(es.instances).toHaveLength(2)
    expect(es.latest()!.url).toContain(`token=${encodeURIComponent('token-rotado')}`)
    es.restore()
  })

  it('sin token no abre el stream (no hay EventSource colgado)', async () => {
    const es = installEventSource()
    useAuthStore().accessToken = null
    const { api } = mountComposable()

    api().connect()
    await flushPromises()

    expect(es.instances).toHaveLength(0)
    expect(api().disconnected.value).toBe(true)
    es.restore()
  })
})
