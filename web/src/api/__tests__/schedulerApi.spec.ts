import { describe, expect, it } from 'vitest'
import { http } from 'msw'

import {
  createSchedule,
  deleteSchedule,
  disableSchedule,
  enableSchedule,
  listSchedules,
  runSchedule,
  updateSchedule,
} from '../scheduler'
import { server } from '@/__tests__/mocks/server'
import { api, errorEnvelope, get, json, post } from '@/__tests__/fixtures'
import { ApiError } from '@/api/client'
import type { CronSchedule } from '@/types/scheduler'

/** Tokens opacos de ejemplo (formato real del backend: v1.<base64url>). */
const SCHEDULE_TOKEN = 'v1.ScHeD-opaque-schedule-token'
const JOB_TOKEN = 'v1.c_NBk-opaque-job-token'

const SCHEDULE: CronSchedule = {
  id: SCHEDULE_TOKEN,
  name: 'Nightly migration',
  cron_expr: '0 3 * * *',
  command: 'migrate',
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  last_run_at: null,
  next_run_at: '2026-01-02T03:00:00Z',
}

describe('api/scheduler', () => {
  it('listSchedules hace GET y devuelve la lista plana (sin meta)', async () => {
    let url = ''
    let method = ''
    server.use(
      get('/scheduler/jobs', (info) => {
        url = info.request.url
        method = info.request.method
        return json({ data: [SCHEDULE] })
      }),
    )

    const rows = await listSchedules()

    expect(method).toBe('GET')
    expect(url).toContain('/api/v1/scheduler/jobs')
    // El recurso no está paginado: no se añaden query params de paginación.
    expect(new URL(url).search).toBe('')
    expect(rows).toEqual([SCHEDULE])
    // El id opaco llega verbatim.
    expect(rows[0]!.id).toBe(SCHEDULE_TOKEN)
  })

  it('createSchedule hace POST con el body exacto y devuelve el schedule', async () => {
    let url = ''
    let method = ''
    let body: unknown = null
    server.use(
      post('/scheduler/jobs', async (info) => {
        url = info.request.url
        method = info.request.method
        body = await info.request.json()
        return json({ data: SCHEDULE }, 201)
      }),
    )

    const created = await createSchedule({
      name: 'Nightly migration',
      cron_expr: '0 3 * * *',
      command: 'migrate',
      enabled: true,
    })

    expect(method).toBe('POST')
    expect(url).toContain('/api/v1/scheduler/jobs')
    expect(body).toEqual({
      name: 'Nightly migration',
      cron_expr: '0 3 * * *',
      command: 'migrate',
      enabled: true,
    })
    expect(created).toEqual(SCHEDULE)
  })

  it('updateSchedule hace PUT con el token verbatim en el path', async () => {
    let url = ''
    let method = ''
    let body: unknown = null
    server.use(
      http.put(api(`/scheduler/jobs/${SCHEDULE_TOKEN}`), async (info) => {
        url = info.request.url
        method = info.request.method
        body = await info.request.json()
        return json({ data: { ...SCHEDULE, name: 'Renamed' } })
      }),
    )

    const updated = await updateSchedule(SCHEDULE_TOKEN, {
      name: 'Renamed',
      cron_expr: '0 3 * * *',
      command: 'migrate',
      enabled: false,
    })

    expect(method).toBe('PUT')
    expect(url).toContain(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}`)
    // El token no se escapa ni transforma: viaja tal cual en el path.
    expect(new URL(url).pathname).toBe(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}`)
    expect(body).toEqual({
      name: 'Renamed',
      cron_expr: '0 3 * * *',
      command: 'migrate',
      enabled: false,
    })
    expect(updated.name).toBe('Renamed')
  })

  it('deleteSchedule hace DELETE y acepta el 204 sin body', async () => {
    let url = ''
    let method = ''
    server.use(
      http.delete(api(`/scheduler/jobs/${SCHEDULE_TOKEN}`), (info) => {
        url = info.request.url
        method = info.request.method
        return new Response(null, { status: 204 })
      }),
    )

    const result = await deleteSchedule(SCHEDULE_TOKEN)

    expect(method).toBe('DELETE')
    expect(url).toContain(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}`)
    // Sin envelope: la función no devuelve nada.
    expect(result).toBeUndefined()
  })

  it('enableSchedule y disableSchedule hacen POST a sus subrutas', async () => {
    const seen: string[] = []
    server.use(
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/enable`, (info) => {
        seen.push(info.request.url)
        return json({ data: { ...SCHEDULE, enabled: true } })
      }),
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/disable`, (info) => {
        seen.push(info.request.url)
        return json({ data: { ...SCHEDULE, enabled: false } })
      }),
    )

    const enabled = await enableSchedule(SCHEDULE_TOKEN)
    const disabled = await disableSchedule(SCHEDULE_TOKEN)

    expect(seen[0]).toContain(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}/enable`)
    expect(seen[1]).toContain(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}/disable`)
    expect(enabled.enabled).toBe(true)
    expect(disabled.enabled).toBe(false)
  })

  it('runSchedule hace POST y devuelve el job_id opaco verbatim', async () => {
    let url = ''
    server.use(
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/run`, (info) => {
        url = info.request.url
        return json({ data: { job_id: JOB_TOKEN } })
      }),
    )

    const result = await runSchedule(SCHEDULE_TOKEN)

    expect(url).toContain(`/api/v1/scheduler/jobs/${SCHEDULE_TOKEN}/run`)
    expect(result.job_id).toBe(JOB_TOKEN)
  })

  it('propaga el 400 de cron como ApiError con el mensaje del backend', async () => {
    server.use(
      post('/scheduler/jobs', () =>
        json(errorEnvelope('validation_failed', 'invalid cron expression: expected 5 fields'), 400),
      ),
    )

    const error = await createSchedule({
      name: 'Bad cron',
      cron_expr: 'nope',
      command: 'generate',
    }).catch((err: unknown) => err)

    expect(error).toBeInstanceOf(ApiError)
    const apiError = error as ApiError
    expect(apiError.status).toBe(400)
    expect(apiError.code).toBe('validation_failed')
    expect(apiError.message).toContain('invalid cron expression')
  })

  it('propaga el 409 de run (job ya en ejecución) como ApiError conflict', async () => {
    server.use(
      post(`/scheduler/jobs/${SCHEDULE_TOKEN}/run`, () =>
        json(errorEnvelope('conflict', 'a job is already running'), 409),
      ),
    )

    const error = await runSchedule(SCHEDULE_TOKEN).catch((err: unknown) => err)

    expect(error).toBeInstanceOf(ApiError)
    const apiError = error as ApiError
    expect(apiError.status).toBe(409)
    expect(apiError.code).toBe('conflict')
  })

  it('propaga el 404 de un token desconocido', async () => {
    server.use(
      http.delete(api('/scheduler/jobs/v1.unknown-token'), () =>
        json(errorEnvelope('not_found', 'schedule not found'), 404),
      ),
    )

    const error = await deleteSchedule('v1.unknown-token').catch((err: unknown) => err)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(404)
  })
})
