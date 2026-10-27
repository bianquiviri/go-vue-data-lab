import type { ApiEnvelope } from '@/types/api'
import type { PagedResult } from '@/types/data'

/**
 * Helpers HTTP compartidos por los clientes del API (extraídos de `api/data.ts`
 * al añadir el panel de administración, que repite el mismo envelope).
 */

/** Valor admitido en una query string (los booleanos se serializan a true/false). */
export type QueryValue = string | number | boolean | undefined | null

/**
 * Query string solo con valores realmente presentes.
 *
 * Se omiten `undefined`/`null`/`''` (filtros vacíos) pero **no** `false`/`0`:
 * `active=false` y `page=0` son valores con significado para el backend.
 */
export function buildQuery(params?: object): Record<string, QueryValue> {
  const query: Record<string, QueryValue> = {}
  if (!params) return query
  for (const [key, value] of Object.entries(params as Record<string, QueryValue>)) {
    if (value === undefined || value === null || value === '') continue
    query[key] = value
  }
  return query
}

/** Normaliza el envelope paginado a un `PagedResult` estable para la UI. */
export function toPagedResult<T>(body: ApiEnvelope<T[]>): PagedResult<T> {
  const items = Array.isArray(body.data) ? body.data : []
  const meta = body.meta ?? {}
  const perPage = typeof meta.per_page === 'number' ? meta.per_page : items.length || 20
  const total = typeof meta.total === 'number' ? meta.total : items.length
  return {
    items,
    page: typeof meta.page === 'number' ? meta.page : 1,
    per_page: perPage,
    total,
    total_pages:
      typeof meta.total_pages === 'number'
        ? meta.total_pages
        : perPage > 0
          ? Math.ceil(total / perPage)
          : 0,
  }
}
