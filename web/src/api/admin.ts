import { api } from './client'
import { buildQuery, toPagedResult } from './query'
import type { ApiEnvelope } from '@/types/api'
import type {
  AdminStats,
  AdminUser,
  AdminUserCreatePayload,
  AdminUserListParams,
  AdminUserUpdatePayload,
  LogEntry,
} from '@/types/admin'
import type { PagedResult } from '@/types/data'

/**
 * Cliente del panel de administración (Issue #23). Todo el grupo
 * `/api/v1/admin` está guardado con auth + RBAC **admin**: un operator/viewer
 * recibe 403 (el guard del router ya evita montar la vista, esto es la defensa
 * del API).
 *
 * ⚠️ IDs OPACOS: el `id` de cada usuario es un token (`v1.*`) y se interpola en
 * el path **tal cual**; nunca se parsea ni se transforma.
 */

/** Lista usuarios paginados (envelope con `meta`). */
export async function listUsers(params: AdminUserListParams = {}): Promise<PagedResult<AdminUser>> {
  const { data } = await api.get<ApiEnvelope<AdminUser[]>>('/admin/users', {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}

/** Obtiene un usuario por token opaco (404 si ya no existe). */
export async function getUser(id: string): Promise<AdminUser> {
  const { data } = await api.get<ApiEnvelope<AdminUser>>(`/admin/users/${id}`)
  return data.data
}

/** Crea un usuario. 409 si el email ya está registrado. */
export async function createUser(payload: AdminUserCreatePayload): Promise<AdminUser> {
  const { data } = await api.post<ApiEnvelope<AdminUser>>('/admin/users', payload)
  return data.data
}

/**
 * Actualiza un usuario con **solo los campos cambiados**.
 *
 * El backend responde 400 `no changes` si el body va vacío y 409 cuando el
 * cambio rompe una regla de negocio (email duplicado, auto-mutación, último
 * admin activo).
 */
export async function updateUser(
  id: string,
  payload: AdminUserUpdatePayload,
): Promise<AdminUser> {
  const { data } = await api.put<ApiEnvelope<AdminUser>>(`/admin/users/${id}`, payload)
  return data.data
}

/** Borra un usuario. El backend responde 204 sin body (409 en los guards). */
export async function deleteUser(id: string): Promise<void> {
  await api.delete(`/admin/users/${id}`)
}

/** Contadores del dashboard (usuarios, filas por esquema, jobs, servidor). */
export async function getStats(): Promise<AdminStats> {
  const { data } = await api.get<ApiEnvelope<AdminStats>>('/admin/stats')
  return data.data
}

/** Errores del JSON log del backend, newest first y paginados. */
export async function listErrorLogs(
  params: { page?: number; per_page?: number } = {},
): Promise<PagedResult<LogEntry>> {
  const { data } = await api.get<ApiEnvelope<LogEntry[]>>('/admin/logs/errors', {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}
