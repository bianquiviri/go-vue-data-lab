import { api } from './client'
import type { ApiEnvelope } from '@/types/api'
import type { AnalyticsSummary, TopCustomer, TopProduct } from '@/types/analytics'

/**
 * Cliente de la analítica de `lab_target` (Issue #24): solo lectura.
 *
 * ⚠️ `GET /target/analytics/summary` responde un **objeto plano**, SIN envelope
 * `data` (el handler usa `RespondJSON`). Los dos top-N sí llegan con
 * `{"data":[...],"meta":{"limit":N}}`.
 *
 * ⚠️ IDs OPACOS: el `id` de productos y clientes es un token cifrado (`v1.*`).
 * Se devuelve **verbatim**; la UI lo muestra con `IdChip`. Nunca se parsea.
 *
 * `limit` va **siempre** en la query (`?limit=N`): el backend lo normaliza
 * (default 10, máx 100), la UI ofrece 5/10/20. No hace falta un `buildQuery`
 * genérico porque es el único parámetro.
 */

/** Resumen agregado de lab_target. */
export async function getAnalyticsSummary(): Promise<AnalyticsSummary> {
  const { data } = await api.get<AnalyticsSummary>('/target/analytics/summary')
  return data
}

/** Top-N productos por ingresos. */
export async function getTopProducts(limit: number): Promise<TopProduct[]> {
  const { data } = await api.get<ApiEnvelope<TopProduct[] | null>>(
    '/target/analytics/top-products',
    { params: { limit } },
  )
  return Array.isArray(data.data) ? data.data : []
}

/** Top-N clientes por gasto. */
export async function getTopCustomers(limit: number): Promise<TopCustomer[]> {
  const { data } = await api.get<ApiEnvelope<TopCustomer[] | null>>(
    '/target/analytics/top-customers',
    { params: { limit } },
  )
  return Array.isArray(data.data) ? data.data : []
}
