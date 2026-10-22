/**
 * Contratos de los endpoints de jobs (Issue #21).
 *
 * ⚠️ REGLA CRÍTICA — IDS OPACOS:
 * `job_id`, `id` y `schedule_id` son **tokens opacos cifrados** (string, formato
 * `v1.<base64url>`), NO UUIDs. Se pasan verbatim a paths/query params y nunca se
 * parsean ni transforman. Los `job_id` que llegan por SSE vienen sellados igual
 * que en el REST.
 */

/** Estados posibles de un job (el backend añade `idle` cuando no hay ninguno). */
export type JobStatusKind = 'pending' | 'running' | 'completed' | 'failed' | 'stopped' | 'idle'

/** Tipo de job ejecutable desde la API. */
export type JobType = 'generate' | 'migrate'

/** Progreso de una entidad (customers, products, orders, order_items…). */
export interface EntityProgress {
  entity: string
  done: number
  total: number
  percent: number
  processed: number
  failed: number
  skipped: number
}

/** Progreso agregado de un job (payload del evento SSE `progress`). */
export interface JobProgress {
  job_id: string
  job_type: string
  status: JobStatusKind
  entities: EntityProgress[]
  percent: number
  processed: number
  failed: number
  skipped: number
}

/** Snapshot del job actual (o del último), o `idle` si no hay ninguno. */
export interface JobStatus {
  id: string
  type: string
  status: JobStatusKind
  running: boolean
  started_at: string | null
  finished_at?: string | null
  error?: string | null
  progress: JobProgress
}

/** Fila del historial de ejecuciones (`job_executions`). */
export interface JobExecution {
  id: string
  schedule_id?: string | null
  job_name: string
  status: string
  started_at: string | null
  finished_at?: string | null
  error?: string | null
  metrics: Record<string, unknown> | null
}

/** Respuesta de `POST /jobs/{generate,migrate}/start`. */
export interface JobStartResult {
  job_id: string
  type: string
  status: JobStatusKind
}

/** Respuesta de `POST /jobs/stop`. */
export interface JobStopResult {
  stopped: boolean
}

/** Parámetros de generación (todos opcionales; el backend aplica defaults). */
export interface GenerateParams {
  seed?: number
  customers?: number
  products?: number
  orders?: number
  min_items?: number
  max_items?: number
  batch_size?: number
  reset?: boolean
  skip_if_exists?: boolean
}

/** Parámetros de migración ETL (todos opcionales; defaults en el backend). */
export interface MigrateParams {
  entities?: string[]
  workers?: number
  batch_size?: number
  extract_size?: number
  fail_fast?: boolean
  strict_totals?: boolean
}

/** Parámetros de paginación del historial. */
export interface JobHistoryParams {
  page?: number
  per_page?: number
}

/** Estados terminales: el stream SSE se cierra después de emitirlos. */
export const TERMINAL_STATUSES: JobStatusKind[] = ['completed', 'failed', 'stopped']

/** ¿El estado es terminal (el job ya no está corriendo)? */
export function isTerminal(status: JobStatusKind): boolean {
  return TERMINAL_STATUSES.includes(status)
}
