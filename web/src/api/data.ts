import { api } from './client'
import type { ApiEnvelope } from '@/types/api'
import type {
  Customer,
  CustomerListParams,
  DataScope,
  Order,
  OrderItem,
  OrderItemListParams,
  OrderListParams,
  PagedResult,
  Product,
  ProductListParams,
} from '@/types/data'

/**
 * Endpoints de datos de lab_source y lab_target.
 *
 * El backend expone los mismos recursos bajo `/source/*` y `/target/*`, así que
 * cada función recibe el `scope` y compone el path. Las respuestas paginadas
 * llegan con el envelope `{"data":[...],"meta":{page,per_page,total,total_pages}}`.
 *
 * ⚠️ IDs OPACOS: cada `id`/`customer_id`/`order_id`/`product_id` es un token
 * cifrado (string). Se interpola en el path y se envía como query param **tal
 * cual**: nunca se parsea ni se transforma. Ver `types/data.ts`.
 */

/** Query string solo con valores realmente presentes (omite undefined/''/null). */
function buildQuery(params?: object): Record<string, string | number> {
  const query: Record<string, string | number> = {}
  if (!params) return query
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    query[key] = value as string | number
  }
  return query
}

/** Normaliza el envelope paginado a un `PagedResult` estable para la UI. */
function toPagedResult<T>(body: ApiEnvelope<T[]>): PagedResult<T> {
  const items = Array.isArray(body.data) ? body.data : []
  const meta = body.meta ?? {}
  const per_page = typeof meta.per_page === 'number' ? meta.per_page : items.length || 20
  const total = typeof meta.total === 'number' ? meta.total : items.length
  return {
    items,
    page: typeof meta.page === 'number' ? meta.page : 1,
    per_page,
    total,
    total_pages:
      typeof meta.total_pages === 'number'
        ? meta.total_pages
        : per_page > 0
          ? Math.ceil(total / per_page)
          : 0,
  }
}

/** Lista clientes paginados de un esquema. */
export async function listCustomers(
  scope: DataScope,
  params: CustomerListParams = {},
): Promise<PagedResult<Customer>> {
  const { data } = await api.get<ApiEnvelope<Customer[]>>(`/${scope}/customers`, {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}

/** Obtiene un cliente por token opaco. */
export async function getCustomer(scope: DataScope, id: string): Promise<Customer> {
  const { data } = await api.get<Customer>(`/${scope}/customers/${id}`)
  return data
}

/** Lista productos paginados de un esquema. */
export async function listProducts(
  scope: DataScope,
  params: ProductListParams = {},
): Promise<PagedResult<Product>> {
  const { data } = await api.get<ApiEnvelope<Product[]>>(`/${scope}/products`, {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}

/** Obtiene un producto por token opaco. */
export async function getProduct(scope: DataScope, id: string): Promise<Product> {
  const { data } = await api.get<Product>(`/${scope}/products/${id}`)
  return data
}

/** Lista órdenes paginadas de un esquema. */
export async function listOrders(
  scope: DataScope,
  params: OrderListParams = {},
): Promise<PagedResult<Order>> {
  const { data } = await api.get<ApiEnvelope<Order[]>>(`/${scope}/orders`, {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}

/** Obtiene una orden por token opaco, con sus `order_items`. */
export async function getOrder(scope: DataScope, id: string): Promise<Order> {
  const { data } = await api.get<Order>(`/${scope}/orders/${id}`)
  return data
}

/** Lista items de orden paginados de un esquema. */
export async function listOrderItems(
  scope: DataScope,
  params: OrderItemListParams = {},
): Promise<PagedResult<OrderItem>> {
  const { data } = await api.get<ApiEnvelope<OrderItem[]>>(`/${scope}/order-items`, {
    params: buildQuery(params),
  })
  return toPagedResult(data)
}
