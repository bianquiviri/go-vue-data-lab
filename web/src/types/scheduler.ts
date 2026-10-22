/**
 * Contratos de los endpoints del scheduler (Issue #22).
 *
 * ⚠️ REGLA CRÍTICA — IDS OPACOS:
 * `CronSchedule.id` y el `job_id` de `POST /scheduler/jobs/{id}/run` son
 * **tokens opacos cifrados** (string, formato `v1.<base64url>`), NO UUIDs. Se
 * pasan verbatim a los paths (`/scheduler/jobs/{token}`, `.../{token}/enable`,
 * …) y nunca se parsean ni transforman. El backend rechaza con 400 un UUID
 * crudo, así que reinterpretar un token rompería todas las peticiones.
 */

/** Comandos que el backend acepta en `command` (validados server-side). */
export type ScheduleCommand = 'generate' | 'migrate'

/** Schedule cron persistido en `lab_target.cron_schedules`. */
export interface CronSchedule {
  /** Token opaco (`v1.*`): clave de fila y de URL, nunca se parsea. */
  id: string
  name: string
  /** Expresión cron de 5 campos (minuto hora día mes día-semana). */
  cron_expr: string
  command: ScheduleCommand
  enabled: boolean
  created_at: string
  updated_at: string
  last_run_at: string | null
  next_run_at: string | null
}

/**
 * Parámetros de alta. `enabled` es opcional: el backend asume `true` cuando se
 * omite, pero el formulario siempre lo envía con el valor del checkbox.
 */
export interface CreateScheduleParams {
  name: string
  cron_expr: string
  command: ScheduleCommand
  enabled?: boolean
}

/**
 * Parámetros de actualización. Mismo shape que el alta; si `enabled` se omite
 * el backend conserva el valor actual (`*bool` en el DTO de Go).
 */
export interface UpdateScheduleParams {
  name: string
  cron_expr: string
  command: ScheduleCommand
  enabled?: boolean
}

/** Respuesta de `POST /scheduler/jobs/{id}/run` (`job_id` = token opaco). */
export interface ScheduleRunResult {
  job_id: string
}
