/**
 * Contratos de la analítica de `lab_target` (Issue #24).
 *
 * El backend (ya terminado) expone tres endpoints de solo lectura bajo
 * `/api/v1/target/analytics/*`. Modelo de e-commerce sintético: clientes,
 * productos y órdenes; la moneda del dataset es **USD**.
 *
 * ⚠️ IDs OPACOS: el `id` de `TopProduct`/`TopCustomer` es un token cifrado
 * (`v1.<base64url>`). Viaja **verbatim** desde el API y se muestra con `IdChip`
 * (presentación acortada, valor íntegro en el tooltip y el portapapeles): nunca
 * se parsea, transforma ni convierte en número.
 *
 * ⚠️ `GET /analytics/summary` responde un **objeto plano**, SIN envelope `data`
 * (el handler usa `RespondJSON`, no `RespondData`). Los dos top-N sí llegan con
 * `{"data":[...],"meta":{"limit":N}}`.
 */

/** Fila agregada de `orders_by_status` (puede llegar `null` si no hay órdenes). */
export interface OrderStatusCount {
  status: string
  count: number
}

/** Resumen plano de `GET /target/analytics/summary`. */
export interface AnalyticsSummary {
  total_customers: number
  total_products: number
  total_orders: number
  total_revenue: number
  average_order_value: number
  /** El backend puede devolver `null` cuando no hay órdenes. */
  orders_by_status: OrderStatusCount[] | null
}

/** Producto del top-N por ingresos (`GET /target/analytics/top-products`). */
export interface TopProduct {
  /** Token opaco del producto. */
  id: string
  name: string
  sku: string
  category: string
  total_quantity: number
  total_revenue: number
}

/** Cliente del top-N por gasto (`GET /target/analytics/top-customers`). */
export interface TopCustomer {
  /** Token opaco del cliente. */
  id: string
  first_name: string
  last_name: string
  email: string
  total_orders: number
  total_spent: number
}

/**
 * Datos puros de una porción del donut (lo que produce `toStatusSlices`).
 * `status` es el valor del dominio tal cual lo devuelve el backend.
 */
export interface StatusSlice {
  status: string
  value: number
}

/**
 * Porción ya preparada para el chart: el dato puro más la etiqueta **ya
 * traducida** (la traducción depende de i18n y vive en la vista, no en `lib/`).
 */
export interface LabeledStatusSlice extends StatusSlice {
  label: string
}

/** Dato de una barra del top de productos (`name` en el eje, `value` = ingresos). */
export interface BarDatum {
  name: string
  value: number
}

/** Límites de top-N que ofrece la UI (el backend normaliza 10 por defecto, máx 100). */
export const ANALYTICS_LIMITS = [5, 10, 20] as const

/** Límite de top-N seleccionable en la vista. */
export type AnalyticsLimit = (typeof ANALYTICS_LIMITS)[number]

/** Límite por defecto de la UI (= default del backend). */
export const DEFAULT_ANALYTICS_LIMIT: AnalyticsLimit = 10
