import type { UserRole } from './auth'
import type { JobStatusKind } from './jobs'

/**
 * Contratos del panel de administración (Issue #23).
 *
 * Fuente: `backend/internal/handler/admin_users.go`, `admin_stats.go` y
 * `internal/service/errorlog.go` (el backend es la autoridad del JSON).
 *
 * ⚠️ REGLA CRÍTICA — IDs OPACOS:
 * `AdminUser.id` es un **token opaco cifrado** (`v1.<base64url>`), no un UUID.
 * Se interpola verbatim en los paths de escritura y nunca se parsea ni se
 * compara por valor: la UI usa **índices de fila** como selectores estables.
 */

/** Usuario del panel (GET /admin/users). Nunca incluye el hash del password. */
export interface AdminUser {
  id: string
  email: string
  role: UserRole
  active: boolean
  created_at: string
  updated_at: string
}

/**
 * Filtros de GET /admin/users.
 *
 * `active` llega desde la query string como `'true'`/`'false'`, así que se
 * admite el string tal cual (el API lo reenvía sin transformarlo).
 */
export interface AdminUserListParams {
  page?: number
  per_page?: number
  /** Búsqueda por email, case-insensitive. */
  q?: string
  role?: UserRole | ''
  active?: boolean | string
}

/** Body de POST /admin/users (password ≥ 8; `active` por defecto true). */
export interface AdminUserCreatePayload {
  email: string
  password: string
  role: UserRole
  active: boolean
}

/**
 * Body de PUT /admin/users/:id: **solo los campos que cambian**.
 *
 * El backend responde 400 `no changes` si el body va vacío, así que la UI
 * calcula el diff contra la fila editada antes de enviar.
 */
export interface AdminUserUpdatePayload {
  email?: string
  password?: string
  role?: UserRole
  active?: boolean
}

/** Conteo de filas de un esquema (source | target). */
export interface AdminEntityCounts {
  customers: number
  products: number
  orders: number
  order_items: number
}

/** Bloque de usuarios de GET /admin/stats. */
export interface AdminStatsUsers {
  total: number
  active: number
  by_role: Record<UserRole, number>
}

/** Bloque de ejecuciones de jobs: los cinco estados conocidos. */
export interface AdminStatsJobs {
  total: number
  by_status: Partial<Record<JobStatusKind, number>>
}

/** Snapshot plano de GET /admin/stats (nunca null: los vacíos son 0). */
export interface AdminStats {
  users: AdminStatsUsers
  source: AdminEntityCounts
  target: AdminEntityCounts
  jobs: AdminStatsJobs
  schedules: { total: number; enabled: number }
  server: { uptime_seconds: number; go_version: string; started_at: string }
}

/**
 * Registro del JSON log del backend (GET /admin/logs/errors).
 *
 * Los nombres son `time`/`message` (no `ts`/`msg`): el backend traduce la línea
 * zap a `service.LogEntry` antes de serializar. `time` llega como cadena vacía
 * de valor (`0001-01-01T00:00:00Z`) cuando el timestamp no se pudo parsear.
 */
export interface LogEntry {
  time: string
  level: string
  message: string
  caller?: string
  fields?: Record<string, unknown>
}
