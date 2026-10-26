import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { http } from 'msw'

import SchedulerView from '../SchedulerView.vue'
import { server } from '@/__tests__/mocks/server'
import {
  api,
  errorEnvelope,
  get,
  json,
  makeUser,
  post,
  useLocale,
} from '@/__tests__/fixtures'
import { formatDateTime } from '@/lib/datetime'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import type { CronSchedule } from '@/types/scheduler'

/** Tokens opacos de ejemplo (formato real del backend: v1.<base64url>). */
const SCHEDULE_TOKEN = 'v1.ScHeD-opaque-schedule-token'
const OTHER_TOKEN = 'v1.0tHeR-opaque-schedule-token'
const JOB_TOKEN = 'v1.c_NBk-opaque-job-token'
const ACCESS_TOKEN = 'access-token-1'

function makeSchedule(overrides: Partial<CronSchedule> = {}): CronSchedule {
  return {
    id: SCHEDULE_TOKEN,
    name: 'Nightly migration',
    cron_expr: '0 3 * * *',
    command: 'migrate',
    enabled: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    last_run_at: null,
    next_run_at: '2026-01-02T03:00:00Z',
    ...overrides,
  }
}

/** Handler del listado: el recurso NO está paginado (`{"data":[...]}` sin meta). */
function listHandler(rows: CronSchedule[]) {
  return get('/scheduler/jobs', () => json({ data: rows }))
}

async function mountView(
  role: 'admin' | 'operator' | 'viewer' = 'admin',
  options: { flush?: boolean } = {},
) {
  const auth = useAuthStore()
  auth.user = makeUser({ role })
  auth.accessToken = ACCESS_TOKEN
  auth.initialized = true

  const router = createAppRouter()
  await router.push('/scheduler')
  await router.isReady()

  const wrapper = mount(SchedulerView, { global: { plugins: [router] } })
  if (options.flush !== false) await flushPromises()
  return { wrapper, router }
}

describe('views/SchedulerView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('renderiza la lista con los datos del backend', async () => {
    server.use(listHandler([makeSchedule()]))
    const { wrapper } = await mountView()

    const row = wrapper.get('[data-test="scheduler-row-0"]')
    expect(row.text()).toContain('Nightly migration')
    expect(row.get('[data-test="scheduler-cron"]').text()).toBe('0 3 * * *')
    // El comando reutiliza las etiquetas ya existentes de jobs.
    expect(row.text()).toContain('Migración ETL')
    expect(row.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Habilitado')
    expect(row.text()).toContain(formatDateTime('2026-01-02T03:00:00Z'))
    // Sin última ejecución ⇒ el texto traducido de "nunca".
    expect(row.text()).toContain('Nunca')
    // El chip expone el token íntegro: nunca se parsea ni transforma.
    expect(row.get('[data-test="id-chip"]').attributes('title')).toBe(SCHEDULE_TOKEN)
  })

  it('muestra el estado vacío cuando no hay schedules', async () => {
    server.use(listHandler([]))
    const { wrapper } = await mountView()

    expect(wrapper.get('[data-test="scheduler-empty"]').text()).toContain('Aún no hay programaciones')
    expect(wrapper.find('[data-test="scheduler-table"]').exists()).toBe(false)
  })

  it('muestra el estado de carga mientras llega la respuesta', async () => {
    server.use(listHandler([makeSchedule()]))
    const { wrapper } = await mountView('admin', { flush: false })

    await nextTick()
    expect(wrapper.get('[data-test="scheduler-loading"]').text()).toContain('Cargando programaciones')

    await flushPromises()
    expect(wrapper.find('[data-test="scheduler-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-table"]').exists()).toBe(true)
  })

  it('muestra el error de carga y permite reintentar', async () => {
    let calls = 0
    server.use(
      get('/scheduler/jobs', () => {
        calls += 1
        return calls === 1
          ? json(errorEnvelope('internal_error', 'boom'), 500)
          : json({ data: [makeSchedule()] })
      }),
    )
    const { wrapper } = await mountView()

    expect(wrapper.get('[data-test="scheduler-load-error"]').text()).toContain(
      'No se pudieron cargar las programaciones',
    )

    await wrapper.get('[data-test="scheduler-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="scheduler-load-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-row-0"]').exists()).toBe(true)
    expect(calls).toBe(2)
  })

  it('un viewer ve la lista en modo lectura, sin acciones', async () => {
    server.use(listHandler([makeSchedule()]))
    const { wrapper } = await mountView('viewer')

    expect(wrapper.get('[data-test="scheduler-viewer-hint"]').text()).toContain('solo lectura')
    expect(wrapper.find('[data-test="scheduler-new"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-edit-0"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-disable-0"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-run-0"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-delete-0"]').exists()).toBe(false)
    // La fila sigue visible (el GET lo puede hacer cualquier rol).
    expect(wrapper.get('[data-test="scheduler-row-0"]').text()).toContain('Nightly migration')
  })

  it('crea un schedule y lo añade a la lista', async () => {
    let body: unknown = null
    let method = ''
    server.use(
      listHandler([]),
      post('/scheduler/jobs', async (info) => {
        method = info.request.method
        body = await info.request.json()
        return json(
          {
            data: makeSchedule({
              id: OTHER_TOKEN,
              name: 'Fresh schedule',
              cron_expr: '*/5 * * * *',
              command: 'generate',
              next_run_at: null,
            }),
          },
          201,
        )
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-new"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="scheduler-form"]').text()).toContain('Nueva programación')

    await wrapper.get('[data-test="scheduler-form-name"]').setValue('Fresh schedule')
    await wrapper.get('[data-test="scheduler-form-cron"]').setValue('*/5 * * * *')
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()

    expect(method).toBe('POST')
    // El checkbox viaja siempre explícito (default true).
    expect(body).toEqual({
      name: 'Fresh schedule',
      cron_expr: '*/5 * * * *',
      command: 'generate',
      enabled: true,
    })
    // El formulario se cierra y la fila nueva aparece sin refetch.
    expect(wrapper.find('[data-test="scheduler-form"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="scheduler-row-0"]').text()).toContain('Fresh schedule')
    expect(wrapper.get('[data-test="scheduler-status-message"]').text()).toContain(
      'Programación creada',
    )
  })

  it('valida la forma del cron y el nombre sin llamar a la API', async () => {
    let called = false
    server.use(
      listHandler([]),
      post('/scheduler/jobs', () => {
        called = true
        return json({ data: makeSchedule() }, 201)
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-new"]').trigger('click')
    await flushPromises()

    // Formulario vacío: nombre obligatorio + cron inválido.
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="scheduler-form-name-error"]').text()).toContain(
      'El nombre es obligatorio',
    )
    expect(wrapper.get('[data-test="scheduler-form-cron-error"]').text()).toContain(
      'exactamente 5 campos',
    )
    expect(called).toBe(false)
    expect(wrapper.find('[data-test="scheduler-form"]').exists()).toBe(true)

    // Cuatro campos: sigue siendo inválido.
    await wrapper.get('[data-test="scheduler-form-name"]').setValue('Almost')
    await wrapper.get('[data-test="scheduler-form-cron"]').setValue('0 3 * *')
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="scheduler-form-name-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="scheduler-form-cron-error"]').text()).toContain(
      'exactamente 5 campos',
    )

    // Caracteres fuera del alfabeto permitido: inválido también.
    await wrapper.get('[data-test="scheduler-form-cron"]').setValue('0 3 * * MON#2')
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="scheduler-form-cron-error"]').exists()).toBe(true)
    expect(called).toBe(false)
  })

  it('muestra el 400 del backend junto al campo del cron y no cierra el formulario', async () => {
    server.use(
      listHandler([makeSchedule()]),
      post('/scheduler/jobs', () =>
        json(errorEnvelope('validation_failed', 'invalid cron expression: day out of range'), 400),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-new"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="scheduler-form-name"]').setValue('Bad range')
    await wrapper.get('[data-test="scheduler-form-cron"]').setValue('0 3 40 * *')
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-form-cron-error"]').text()).toContain(
      'invalid cron expression',
    )
    expect(wrapper.find('[data-test="scheduler-form"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="scheduler-form-error"]').exists()).toBe(false)
  })

  it('un error no relacionado con el cron muestra el error general sin cerrar el formulario', async () => {
    server.use(
      listHandler([]),
      post('/scheduler/jobs', () => json(errorEnvelope('internal_error', 'boom'), 500)),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-new"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="scheduler-form-name"]').setValue('Nightly run')
    await wrapper.get('[data-test="scheduler-form-cron"]').setValue('0 3 * * *')
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-form-error"]').text()).toContain(
      'No se pudo guardar la programación',
    )
    expect(wrapper.find('[data-test="scheduler-form"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="scheduler-form-cron-error"]').exists()).toBe(false)
  })

  it('un fallo al habilitar/deshabilitar muestra el error genérico de la acción', async () => {
    server.use(
      listHandler([makeSchedule()]),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/disable`, () =>
        json(errorEnvelope('internal_error', 'boom'), 500),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-disable-0"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-action-error"]').text()).toContain(
      'No se pudo completar la acción',
    )
    // El estado de la fila no cambia si el backend falla.
    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Habilitado')
  })

  it('edita un schedule: precarga los campos y guarda con PUT', async () => {
    let putBody: unknown = null
    server.use(
      listHandler([makeSchedule()]),
      http.put(api(`/scheduler/jobs/${SCHEDULE_TOKEN}`), async (info) => {
        putBody = await info.request.json()
        return json({ data: makeSchedule({ name: 'Renamed', enabled: false }) })
      }),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-edit-0"]').trigger('click')
    await flushPromises()

    const name = wrapper.get('[data-test="scheduler-form-name"]').element as HTMLInputElement
    const cron = wrapper.get('[data-test="scheduler-form-cron"]').element as HTMLInputElement
    const command = wrapper.get('[data-test="scheduler-form-command"]').element as HTMLSelectElement
    const enabled = wrapper.get('[data-test="scheduler-form-enabled"]').element as HTMLInputElement

    expect(wrapper.get('[data-test="scheduler-form"]').text()).toContain('Editar programación')
    expect(name.value).toBe('Nightly migration')
    expect(cron.value).toBe('0 3 * * *')
    expect(command.value).toBe('migrate')
    expect(enabled.checked).toBe(true)

    await wrapper.get('[data-test="scheduler-form-name"]').setValue('Renamed')
    await wrapper.get('[data-test="scheduler-form-enabled"]').setValue(false)
    await wrapper.get('[data-test="scheduler-form-save"]').trigger('click')
    await flushPromises()

    expect(putBody).toEqual({
      name: 'Renamed',
      cron_expr: '0 3 * * *',
      command: 'migrate',
      enabled: false,
    })
    expect(wrapper.get('[data-test="scheduler-row-0"]').text()).toContain('Renamed')
    expect(wrapper.get('[data-test="scheduler-status-message"]').text()).toContain(
      'Programación actualizada',
    )
  })

  it('habilita y deshabilita un schedule desde la fila', async () => {
    let enabled = true
    server.use(
      get('/scheduler/jobs', () => json({ data: [makeSchedule({ enabled })] })),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/disable`, () => {
        enabled = false
        return json({ data: makeSchedule({ enabled }) })
      }),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/enable`, () => {
        enabled = true
        return json({ data: makeSchedule({ enabled }) })
      }),
    )
    const { wrapper } = await mountView()

    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Habilitado')
    await wrapper.get('[data-test="scheduler-disable-0"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Deshabilitado')
    expect(wrapper.find('[data-test="scheduler-enable-0"]').exists()).toBe(true)

    await wrapper.get('[data-test="scheduler-enable-0"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Habilitado')
    expect(wrapper.find('[data-test="scheduler-disable-0"]').exists()).toBe(true)
  })

  it('ejecuta un schedule ahora y muestra el job_id con enlace al monitoreo', async () => {
    server.use(
      listHandler([makeSchedule()]),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/run`, () => json({ data: { job_id: JOB_TOKEN } })),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-run-0"]').trigger('click')
    await flushPromises()

    const result = wrapper.get('[data-test="scheduler-run-result"]')
    expect(result.text()).toContain('Job iniciado')
    // El job_id es un token opaco: se muestra íntegro en el chip.
    expect(result.get('[data-test="id-chip"]').attributes('title')).toBe(JOB_TOKEN)
    expect(wrapper.get('[data-test="scheduler-go-to-jobs"]').attributes('href')).toBe('/jobs')
  })

  it('un 409 al ejecutar muestra el aviso de job en ejecución (no un error genérico)', async () => {
    server.use(
      listHandler([makeSchedule()]),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/run`, () =>
        json(errorEnvelope('conflict', 'a job is already running'), 409),
      ),
    )
    const { wrapper } = await mountView()

    await wrapper.get('[data-test="scheduler-run-0"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="scheduler-action-error"]').text()).toContain(
      'Ya hay un job en ejecución',
    )
    expect(wrapper.find('[data-test="scheduler-run-result"]').exists()).toBe(false)
  })

  it('borra con confirmación en dos pasos (y cancelar no borra)', async () => {
    let deleted = 0
    server.use(
      listHandler([makeSchedule(), makeSchedule({ id: OTHER_TOKEN, name: 'Second schedule' })]),
      http.delete(api(`/scheduler/jobs/${SCHEDULE_TOKEN}`), () => {
        deleted += 1
        return new Response(null, { status: 204 })
      }),
    )
    const { wrapper } = await mountView()

    // Paso 1: abre la confirmación en la propia fila, sin borrar todavía.
    await wrapper.get('[data-test="scheduler-delete-0"]').trigger('click')
    expect(wrapper.find('[data-test="scheduler-delete-confirm-0"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="scheduler-delete-0"]').exists()).toBe(false)
    expect(deleted).toBe(0)

    // Cancelar cierra la confirmación y deja la fila intacta.
    await wrapper.get('[data-test="scheduler-delete-cancel-0"]').trigger('click')
    expect(wrapper.find('[data-test="scheduler-delete-confirm-0"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="scheduler-delete-0"]').exists()).toBe(true)
    expect(deleted).toBe(0)

    // Paso 2: confirmar borra y quita la fila de la lista.
    await wrapper.get('[data-test="scheduler-delete-0"]').trigger('click')
    await wrapper.get('[data-test="scheduler-delete-confirm-0"]').trigger('click')
    await flushPromises()

    expect(deleted).toBe(1)
    expect(wrapper.findAll('[data-test^="scheduler-row-"]').length).toBe(1)
    expect(wrapper.get('[data-test="scheduler-row-0"]').text()).toContain('Second schedule')
    expect(wrapper.get('[data-test="scheduler-status-message"]').text()).toContain(
      'Programación borrada',
    )
  })

  it('cambia los textos al cambiar de idioma (paridad ES/EN)', async () => {
    server.use(listHandler([makeSchedule()]))
    const { wrapper } = await mountView()

    expect(wrapper.text()).toContain('Programaciones cron ejecutadas por el runner del backend')
    expect(wrapper.get('[data-test="scheduler-refresh"]').text()).toContain('Actualizar')
    expect(wrapper.get('[data-test="scheduler-new"]').text()).toContain('Nuevo schedule')
    expect(wrapper.get('[data-test="scheduler-edit-0"]').text()).toContain('Editar')
    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Habilitado')

    useLocale('en')
    await nextTick()

    expect(wrapper.text()).toContain('Cron schedules executed by the backend runner')
    expect(wrapper.get('[data-test="scheduler-refresh"]').text()).toContain('Refresh')
    expect(wrapper.get('[data-test="scheduler-new"]').text()).toContain('New schedule')
    expect(wrapper.get('[data-test="scheduler-edit-0"]').text()).toContain('Edit')
    expect(wrapper.get('[data-test="scheduler-row-0"]').text()).toContain('ETL migration')
    expect(wrapper.get('[data-test="scheduler-enabled-badge"]').text()).toBe('Enabled')
  })
})
