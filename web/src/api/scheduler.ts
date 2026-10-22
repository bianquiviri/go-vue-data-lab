import { api } from './client'
import type { ApiEnvelope } from '@/types/api'
import type {
  CreateScheduleParams,
  CronSchedule,
  ScheduleRunResult,
  UpdateScheduleParams,
} from '@/types/scheduler'

/**
 * Endpoints del scheduler de cron (Issue #22).
 *
 * `GET /scheduler/jobs` devuelve la lista plana (`{"data":[...]}`), SIN `meta`:
 * el backend no pagina este recurso, así que no hay `buildQuery` ni
 * normalización a `PagedResult`.
 *
 * ⚠️ IDs OPACOS: el `id` de cada schedule es un token cifrado (`v1.*`) y se
 * interpola en el path **tal cual**, igual que en `api/data.ts`. El `job_id`
 * que devuelve `run` también es un token y se entrega verbatim a la UI.
 */

/** Lista todos los schedules (sin paginación). */
export async function listSchedules(): Promise<CronSchedule[]> {
  const { data } = await api.get<ApiEnvelope<CronSchedule[]>>('/scheduler/jobs')
  return data.data
}

/** Crea un schedule; el backend valida el cron y calcula `next_run_at`. */
export async function createSchedule(params: CreateScheduleParams): Promise<CronSchedule> {
  const { data } = await api.post<ApiEnvelope<CronSchedule>>('/scheduler/jobs', params)
  return data.data
}

/** Reemplaza los campos de un schedule (`id` = token opaco). */
export async function updateSchedule(
  id: string,
  params: UpdateScheduleParams,
): Promise<CronSchedule> {
  const { data } = await api.put<ApiEnvelope<CronSchedule>>(`/scheduler/jobs/${id}`, params)
  return data.data
}

/** Borra un schedule. El backend responde 204 sin body. */
export async function deleteSchedule(id: string): Promise<void> {
  await api.delete(`/scheduler/jobs/${id}`)
}

/** Habilita un schedule (lo registra en el runner de cron). */
export async function enableSchedule(id: string): Promise<CronSchedule> {
  const { data } = await api.post<ApiEnvelope<CronSchedule>>(`/scheduler/jobs/${id}/enable`)
  return data.data
}

/** Deshabilita un schedule (lo desregistra del runner de cron). */
export async function disableSchedule(id: string): Promise<CronSchedule> {
  const { data } = await api.post<ApiEnvelope<CronSchedule>>(`/scheduler/jobs/${id}/disable`)
  return data.data
}

/**
 * Ejecuta un schedule ahora. 409 (`conflict`) si ya hay un job en ejecución.
 * El `job_id` devuelto es el token opaco del job recién arrancado.
 */
export async function runSchedule(id: string): Promise<ScheduleRunResult> {
  const { data } = await api.post<ApiEnvelope<ScheduleRunResult>>(`/scheduler/jobs/${id}/run`)
  return data.data
}
