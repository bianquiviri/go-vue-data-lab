import { describe, expect, it } from 'vitest'

import {
  buildJobEventsUrl,
  getJobProgress,
  getJobStatus,
  listJobHistory,
  startGenerate,
  startMigrate,
  stopJob,
} from '../jobs'
import { server } from '@/__tests__/mocks/server'
import { get, json, pagedEnvelope, post } from '@/__tests__/fixtures'
import { api } from '@/api/client'
import type { JobExecution, JobProgress, JobStatus } from '@/types/jobs'

/** Token opaco de ejemplo (formato real del backend: v1.<base64url>). */
const JOB_TOKEN = 'v1.c_NBk-opaque-job-token'

const STATUS: JobStatus = {
  id: JOB_TOKEN,
  type: 'generate',
  status: 'running',
  running: true,
  started_at: '2026-01-01T00:00:00Z',
  progress: {
    job_id: JOB_TOKEN,
    job_type: 'generate',
    status: 'running',
    entities: [],
    percent: 40,
    processed: 4000,
    failed: 0,
    skipped: 0,
  },
}

describe('api/jobs', () => {
  it('startGenerate hace POST y devuelve el job_id verbatim', async () => {
    let url = ''
    let body: unknown = null
    server.use(
      post('/jobs/generate/start', async (info) => {
        url = info.request.url
        body = await info.request.json()
        return json({ data: { job_id: JOB_TOKEN, type: 'generate', status: 'running' } }, 202)
      }),
    )

    const result = await startGenerate({ customers: 100, seed: 7 })

    expect(url).toContain('/api/v1/jobs/generate/start')
    expect(body).toEqual({ customers: 100, seed: 7 })
    // El token opaco viaja verbatim, sin transformar.
    expect(result.job_id).toBe(JOB_TOKEN)
    expect(result.type).toBe('generate')
    expect(result.status).toBe('running')
  })

  it('startGenerate sin params envía un body vacío', async () => {
    let body: unknown = 'unset'
    server.use(
      post('/jobs/generate/start', async (info) => {
        body = await info.request.json().catch(() => null)
        return json({ data: { job_id: JOB_TOKEN, type: 'generate', status: 'running' } }, 202)
      }),
    )

    await startGenerate()
    expect(body).toEqual({})
  })

  it('startMigrate hace POST con los parámetros de migración', async () => {
    let url = ''
    let body: unknown = null
    server.use(
      post('/jobs/migrate/start', async (info) => {
        url = info.request.url
        body = await info.request.json()
        return json({ data: { job_id: JOB_TOKEN, type: 'migrate', status: 'running' } }, 202)
      }),
    )

    const result = await startMigrate({ entities: ['customers'], workers: 4 })

    expect(url).toContain('/api/v1/jobs/migrate/start')
    expect(body).toEqual({ entities: ['customers'], workers: 4 })
    expect(result.type).toBe('migrate')
  })

  it('stopJob hace POST y devuelve si se detuvo', async () => {
    let method = ''
    server.use(
      post('/jobs/stop', (info) => {
        method = info.request.method
        return json({ data: { stopped: true } })
      }),
    )

    const result = await stopJob()
    expect(method).toBe('POST')
    expect(result.stopped).toBe(true)
  })

  it('getJobStatus devuelve el snapshot con el id opaco', async () => {
    server.use(get('/jobs/status', () => json({ data: STATUS })))
    const status = await getJobStatus()
    expect(status.id).toBe(JOB_TOKEN)
    expect(status.status).toBe('running')
  })

  it('getJobProgress devuelve el progreso agregado', async () => {
    const progress: JobProgress = STATUS.progress
    server.use(get('/jobs/progress', () => json({ data: progress })))
    const result = await getJobProgress()
    expect(result.job_id).toBe(JOB_TOKEN)
    expect(result.percent).toBe(40)
  })

  it('listJobHistory pagina el envelope y omite params vacíos', async () => {
    const rows: JobExecution[] = [
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
    const urls: string[] = []
    server.use(
      get('/jobs/history', (info) => {
        urls.push(info.request.url)
        return json(pagedEnvelope(rows, { total: 42 }))
      }),
    )

    const result = await listJobHistory({ page: 2, per_page: 50 })
    expect(result.items).toEqual(rows)
    expect(result.total).toBe(42)
    const parsed = new URL(urls[0]!)
    expect(parsed.searchParams.get('page')).toBe('2')
    expect(parsed.searchParams.get('per_page')).toBe('50')
  })

  it('buildJobEventsUrl codifica el token en el query param y escapa caracteres especiales', () => {
    const url = buildJobEventsUrl('a+b/c=d?e')
    expect(url).toBe('/api/v1/jobs/events?token=a%2Bb%2Fc%3Dd%3Fe')
  })

  it('buildJobEventsUrl normaliza el baseURL con barra final', () => {
    // Un baseURL absoluto con "/" al final produciría "//jobs/events".
    const previous = api.defaults.baseURL
    api.defaults.baseURL = 'https://api.example.com/api/v1/'
    try {
      expect(buildJobEventsUrl('tok')).toBe('https://api.example.com/api/v1/jobs/events?token=tok')
    } finally {
      api.defaults.baseURL = previous
    }
  })
})
