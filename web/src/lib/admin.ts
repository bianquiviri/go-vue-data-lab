import { ApiError } from '@/api/client'
import type { BadgeVariants } from '@/components/ui/badge'
import { EMPTY_DATE, formatDateTime } from '@/lib/datetime'
import { API_ERROR_CODES } from '@/types/api'
import type { UserRole } from '@/types/auth'

/**
 * Helpers puros del panel de administración (Issue #23).
 *
 * Aquí vive la **traducción de errores del backend a claves i18n** (nunca a
 * texto): la UI resuelve la clave con `t()` en el render, así un cambio de
 * idioma se refleja sin re-pedir datos.
 *
 * Motivo de mapear por `message` en los 409: el envelope solo trae
 * `{code:"conflict",message}` y el handler usa un mensaje fijo por guard
 * (`respondAdminUserError` en `handler/admin_users.go`). Esos textos son el
 * contrato que distingue "email duplicado" de "último admin".
 */

/** Claves i18n de los campos del formulario de usuario (400 validation_failed). */
const FIELD_MESSAGE_KEYS: Record<string, string> = {
  email: 'admin.users.errors.emailInvalid',
  password: 'admin.users.errors.passwordTooShort',
  role: 'admin.users.errors.roleInvalid',
}

/** Clave de fallback para un campo desconocido en `details.fields`. */
export const INVALID_FIELD_KEY = 'admin.users.errors.invalidField'

/** Mensajes fijos de los 409 del handler ⇒ clave i18n propia. */
const CONFLICT_MESSAGE_KEYS: Record<string, string> = {
  'email is already registered': 'admin.users.errors.emailConflict',
  'cannot remove your own access': 'admin.users.errors.selfMutation',
  'cannot remove the last active admin': 'admin.users.errors.lastAdmin',
}

const EMAIL_CONFLICT_KEY = 'admin.users.errors.emailConflict'

/** Errores de un guardado, ya como claves i18n. */
export interface AdminUserSaveErrors {
  /** Campo del formulario ⇒ clave i18n (se muestra junto al input). */
  fields: Record<string, string>
  /** Aviso del formulario (guard de negocio o fallo genérico). */
  form: string | null
}

/** `details.fields` del 400: `{"Email":"failed on 'email' validation",…}`. */
function rawFields(details: Record<string, unknown> | undefined): Record<string, unknown> | null {
  const fields = details?.fields
  if (fields === null || typeof fields !== 'object' || Array.isArray(fields)) return null
  return fields as Record<string, unknown>
}

/**
 * Traduce el fallo de un alta/edición a claves i18n.
 *
 * - 400 con `details.fields` ⇒ un mensaje por campo (el backend usa el **nombre
 *   del campo Go**, por eso se normaliza a minúsculas: `Email` ⇒ `email`).
 * - 400 `bad_request` ⇒ el backend rechazó un body vacío (`no changes`).
 * - 409 ⇒ guard de negocio; el email duplicado se ancla al campo `email`.
 */
export function mapUserSaveError(error: unknown): AdminUserSaveErrors {
  if (!(error instanceof ApiError)) {
    return { fields: {}, form: 'admin.users.errors.saveError' }
  }

  if (error.status === 400) {
    const fields = rawFields(error.details)
    if (fields) {
      const mapped: Record<string, string> = {}
      for (const name of Object.keys(fields)) {
        const field = name.toLowerCase()
        mapped[field] = FIELD_MESSAGE_KEYS[field] ?? INVALID_FIELD_KEY
      }
      return { fields: mapped, form: null }
    }
    // `RespondBadRequest(c, "no changes")` ⇒ code bad_request sin details.
    if (error.code === API_ERROR_CODES.badRequest) {
      return { fields: {}, form: 'admin.users.errors.noChanges' }
    }
    return { fields: {}, form: 'admin.users.errors.saveError' }
  }

  if (error.status === 409) {
    const key = CONFLICT_MESSAGE_KEYS[error.message] ?? 'admin.users.errors.conflict'
    if (key === EMAIL_CONFLICT_KEY) return { fields: { email: key }, form: null }
    return { fields: {}, form: key }
  }

  if (error.status === 404) {
    return { fields: {}, form: 'admin.users.errors.notFound' }
  }

  return { fields: {}, form: 'admin.users.errors.saveError' }
}

/** Traduce el fallo de un borrado a una clave i18n. */
export function mapUserDeleteError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'admin.users.errors.deleteError'
  if (error.status === 404) return 'admin.users.errors.notFound'
  if (error.status === 409) {
    return CONFLICT_MESSAGE_KEYS[error.message] ?? 'admin.users.errors.deleteConflict'
  }
  return 'admin.users.errors.deleteError'
}

/** Prefijo del `time` cero de Go (`0001-01-01T00:00:00Z` ⇒ timestamp ilegible). */
const ZERO_TIME_PREFIX = '0001-01-01'

/**
 * Fecha de un registro de log. El backend devuelve el *zero time* de Go cuando
 * no pudo parsear el `ts` de la línea zap; eso se muestra como `—`, no como una
 * fecha del año 1.
 */
export function logTime(value: unknown): string {
  if (typeof value === 'string' && value.startsWith(ZERO_TIME_PREFIX)) return EMPTY_DATE
  return formatDateTime(value)
}

/** Niveles zap que el endpoint considera errores (handler de referencia). */
const ERROR_LEVELS = new Set(['ERROR', 'FATAL', 'PANIC', 'DPANIC'])

/** Variante visual del badge de nivel de log. */
export function logLevelVariant(level: string): BadgeVariants['variant'] {
  const upper = (level ?? '').toUpperCase()
  if (ERROR_LEVELS.has(upper)) return 'destructive'
  if (upper.startsWith('WARN')) return 'secondary'
  return 'outline'
}

const ROLE_VARIANTS: Record<UserRole, BadgeVariants['variant']> = {
  admin: 'default',
  operator: 'secondary',
  viewer: 'outline',
}

/** Variante visual del badge de rol (desconocido ⇒ `outline`). */
export function roleBadgeVariant(role: string): BadgeVariants['variant'] {
  return ROLE_VARIANTS[role as UserRole] ?? 'outline'
}

const MINUTE = 60
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/**
 * Uptime legible a partir de segundos.
 *
 * Se usan sufijos cortos (`d`/`h`/`m`/`s`) idénticos en ES y EN: el dato es
 * técnico y no se traduce (mismo criterio que el nombre del autor o el código
 * de licencia). Negativo o no finito ⇒ `0s` (nunca una duración inventada).
 */
export function formatUptime(seconds: number): string {
  const total = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0
  const days = Math.floor(total / DAY)
  const hours = Math.floor((total % DAY) / HOUR)
  const minutes = Math.floor((total % HOUR) / MINUTE)
  const secs = total % MINUTE
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m ${secs}s`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

/** Campos extra de un log como JSON indentado (vacío si no hay ninguno). */
export function fieldsJson(fields: Record<string, unknown> | undefined): string {
  if (!fields || Object.keys(fields).length === 0) return ''
  return JSON.stringify(fields, null, 2)
}
