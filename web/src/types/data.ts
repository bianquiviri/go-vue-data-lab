/**
 * Contratos de los endpoints de datos (lab_source / lab_target).
 *
 * ⚠️ REGLA CRÍTICA — IDs OPACOS:
 * Cada `id`, `customer_id`, `order_id` y `product_id` que devuelve la API es un
 * **token opaco cifrado** (AES-256-GCM, formato `v1.<base64url>`), NO un UUID.
 * Hay que:
 *  - Pasarlos tal cual a paths (`/source/customers/{token}`) y query params.
 *  - NUNCA parsearlos, validarlos como UUID ni transformarlos.
 *  - Tratarlos siempre como `string`.
 * `order_items.id` proviene de un int64 interno, pero también viaja como token
 * string. El backend rechaza con 400 cualquier UUID crudo, así que reinterpretar
 * un token rompería las peticiones.
 */

/** Recurso por esquema (lab_source | lab_target): solo cambia el prefijo del path. */
export type DataScope = 'source' | 'target'

/** Parámetros de paginación comunes (defaults del backend: page=1, per_page=20, máx 100). */
export interface PageParams {
  page?: number
  per_page?: number
}

/** Filtros de listado de customers (search en nombre/email, country por ISO-2). */
export interface CustomerListParams extends PageParams {
  search?: string
  country?: string
}

/** Filtros de listado de products (search en nombre/SKU, category). */
export interface ProductListParams extends PageParams {
  search?: string
  category?: string
}

/** Filtros de listado de orders. */
export interface OrderListParams extends PageParams {
  /** Token opaco del customer. */
  customer_id?: string
  status?: string
  /** RFC3339 o YYYY-MM-DD. */
  date_from?: string
  date_to?: string
}

/** Filtros de listado de order-items. */
export interface OrderItemListParams extends PageParams {
  /** Token opaco de la orden. */
  order_id?: string
  /** Token opaco del producto. */
  product_id?: string
}

/** Cliente (lab_source/lab_target). `id` = token opaco (string). */
export interface Customer {
  id: string
  first_name: string
  last_name: string
  email: string
  country_code: string
  city: string
  state?: string | null
  created_at: string
}

/** Producto. `id` = token opaco (string). */
export interface Product {
  id: string
  sku: string
  name: string
  category: string
  price: number
  stock: number
  status: string
  created_at: string
}

/** Item de una orden. `id`/`order_id`/`product_id` = tokens opacos (string). */
export interface OrderItem {
  id: string
  order_id: string
  product_id: string
  quantity: number
  unit_price: number
  subtotal: number
}

/** Orden. Todos los ids son tokens opacos (string). */
export interface Order {
  id: string
  order_number: string
  customer_id: string
  status: string
  total_amount: number
  currency: string
  shipping_address?: Record<string, unknown> | string | null
  order_date: string
  order_items?: OrderItem[]
}

/** Resultado de un listado: filas + metadatos de paginación normalizados. */
export interface PagedResult<T> {
  items: T[]
  page: number
  per_page: number
  total: number
  total_pages: number
}

/** Dirección de orden client-side. */
export type SortOrder = 'asc' | 'desc'

/** Orden activo de una tabla (client-side sobre la página actual). */
export interface SortState {
  key: string
  direction: SortOrder
}

/**
 * Definición declarativa de una columna del DataTable.
 *
 * `labelKey` es una clave i18n (no texto): la tabla la resuelve con `t()` para
 * que el encabezado sea reactivo al idioma. El render custom de una celda se
 * hace por slot con nombre = `cell-<key>`.
 *
 * `key` es un `string` (no `keyof T`) a propósito: las filas provienen de
 * respuestas JSON y la tabla indexa con un cast controlado, lo que evita el
 * friccional variance de los genéricos de Vue en las props.
 */
export interface DataTableColumn {
  /** Nombre de la propiedad de la fila (también nombre del slot `cell-<key>`). */
  key: string
  /** Clave i18n del encabezado. */
  labelKey: string
  /** Alineación de la celda (default: left). */
  align?: 'left' | 'center' | 'right'
  /** Si la columna se puede ordenar client-side (default: true). */
  sortable?: boolean
  /** Ancho opcional (clase Tailwind, p. ej. 'w-32'). */
  class?: string
}

/** Nombre del slot de una columna (`cell-<key>`), tipado para el consumidor. */
export function cellSlotName(key: string): string {
  return `cell-${key}`
}
