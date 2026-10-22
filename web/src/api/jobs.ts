import { api } from './client'
import type { ApiEnvelope } from '@/types/api'
import type { PagedResult } from '@/types/data'
import type {
  GenerateParams,
  JobExecution,
  JobHistoryParams,
  JobProgress,
  JobStartResult,
  JobStatus,
  JobStopResult,
  MigrateParams,
} from '@/types/jobs'

/**
 * Endpoints de jobs (Issue #21): control, estado, progreso e historial.
 *
 * ⚠️ IDs OPACOS: los `job_id`/`id` de las respuestas son tokens cifrados
 * (string, `v1.<base64url>`); se devuelven tal cual y nunca se transforman.
 */

/** Query string solo con valores realmente presentes (omite undefined/''/null). */
function buildQuery(params?: object): Record<string, string | number> {
  const query: Record<string, string | number> = {}
  if (!params) return query
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    query[key] = value as string | number
  }
  return query
}

/** Normaliza el envelope paginado a un `PagedResult` estable para la UI. */
function toPagedResult<T>(body: ApiEnvelope<T[]>): PagedResult<T> {
  const items = Array.isArray(body.data) ? body.data : []
  const meta = body.meta ?? {}
  const per_page = typeof meta.per_page === 'number' ? meta.per_page : items.length || 20
  const total = typeof meta.total === 'number' ? meta.total : items.length
  return {
    items,
    page: typeof meta.page === 'number' ? meta.page : 1,
    per_page,
    total,
    total_pages:
      typeof meta.total_pages === 'number'
        ? meta.total_pages
        : per_page > 0
          ? Math.ceil(total / per_page)
          : 0,
  }
}

/** Arranca un job de generación de datos sintéticos. */
export async function startGenerate(params: GenerateParams = {}): Promise<JobStartResult> {
  const { data } = await api.post<ApiEnvelope<JobStartResult>>('/jobs/generate/start', params)
  return data.data
}

/** Arranca un job de migración ETL (lab_source → lab_target). */
export async function startMigrate(params: MigrateParams = {}): Promise<JobStartResult> {
  const { data } = await api.post<ApiEnvelope<JobStartResult>>('/jobs/migrate/start', params)
  return data.data
}

/** Detiene el job en ejecución (404 si no hay ninguno). */
export async function stopJob(): Promise<JobStopResult> {
  const { data } = await api.post<ApiEnvelope<JobStopResult>>('/jobs/stop')
  return data.data
}

/** Snapshot del job actual (o del último), o `idle` si no hay ninguno. */
export async function getJobStatus(): Promise<JobStatus> {
  const { data } = await api.get<ApiEnvelope<JobStatus>>('/jobs/status')
  return data.data
}

/** Progreso agregado del job actual (fallback cuando el SSE no está disponible). */
export async function getJobProgress(): Promise<JobProgress> {
  const { data } = await api.get<ApiEnvelope<JobProgress>>('/jobs/progress')
  return data.data
}

/** Historial de ejecuciones paginado (newest-first), desde `job_executions`. */
export async function listJobHistory(
  params: JobHistoryParams = {},
): Promise<PagedResult<JobExecution>> {
  const { data } = await api.get<ApiEnvelope<JobExecution[]>>('/jobs/history', {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}

/**
 * URL del stream SSE de progreso (`GET /jobs/events`).
 *
 * `EventSource` nativo NO permite enviar la cabecera `Authorization`, así que el
 * access token viaja en el query param `token` (decisión acordada). Se codifica
 * con `encodeURIComponent` porque un JWT contiene `+`, `/` y `=` que romperían
 * la query string. El resto del grupo `/jobs` sigue usando `Bearer` normal.
 *
 * Trade-off documentado: el token puede quedar registrado en logs/proxies; es el
 * precio de no poder usar cabeceras con `EventSource`.
 */
export function buildJobEventsUrl(token: string): string {
  // Normaliza el baseURL (una barra final produciría "//jobs/events").
  const base = (api.defaults.baseURL ?? '/api/v1').replace(/\/+$/, '')
  return `${base}/jobs/events?token=${encodeURIComponent(token)}`
}
