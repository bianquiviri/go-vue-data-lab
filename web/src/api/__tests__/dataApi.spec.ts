import { describe, expect, it } from 'vitest'

import {
  getCustomer,
  getOrder,
  getProduct,
  listCustomers,
  listOrderItems,
  listOrders,
  listProducts,
} from '../data'
import { server } from '@/__tests__/mocks/server'
import { api, errorEnvelope, get, json } from '@/__tests__/fixtures'
import type { Customer, Order, OrderItem, PagedResult, Product } from '@/types/data'

/** Token opaco de ejemplo (formato real del backend: v1.<base64url>). */
const CUSTOMER_TOKEN = 'v1.customer-opaque-token-abc123'
const PRODUCT_TOKEN = 'v1.product-opaque-token-def456'
const ORDER_TOKEN = 'v1.order-opaque-token-ghi789'
const ITEM_TOKEN = 'v1.item-opaque-token-jkl012'

function envelope<T>(data: T[], meta: Record<string, unknown> = {}) {
  return {
    data,
    meta: { page: 1, per_page: 20, total: data.length, total_pages: 1, ...meta },
  }
}

/** Captura la URL completa de la request (para afirmar paths y query params). */
function trackUrl(store: { value: string }) {
  return (info: { request: Request }) => {
    store.value = info.request.url
  }
}

describe('api/data (source)', () => {
  it('listCustomers devuelve los items y normaliza meta de paginación', async () => {
    const customers: Customer[] = [
      {
        id: CUSTOMER_TOKEN,
        first_name: 'Ada',
        last_name: 'Lovelace',
        email: 'ada@example.com',
        country_code: 'GB',
        city: 'London',
        created_at: '2026-01-01T00:00:00Z',
      },
    ]
    server.use(get('/source/customers', () => json(envelope(customers, { total: 42, total_pages: 3 }))))

    const result: PagedResult<Customer> = await listCustomers('source')
    expect(result.items).toEqual(customers)
    expect(result.page).toBe(1)
    expect(result.per_page).toBe(20)
    expect(result.total).toBe(42)
    expect(result.total_pages).toBe(3)
  })

  it('listCustomers pasa search/country/page/per_page como query params', async () => {
    const url = { value: '' }
    server.use(
      get('/source/customers', (info) => {
        trackUrl(url)(info)
        return json(envelope([]))
      }),
    )

    await listCustomers('source', { page: 2, per_page: 50, search: 'ada', country: 'GB' })

    const parsed = new URL(url.value)
    expect(parsed.pathname).toBe('/api/v1/source/customers')
    expect(parsed.searchParams.get('page')).toBe('2')
    expect(parsed.searchParams.get('per_page')).toBe('50')
    expect(parsed.searchParams.get('search')).toBe('ada')
    expect(parsed.searchParams.get('country')).toBe('GB')
  })

  it('listCustomers omite los filtros vacíos de la query string', async () => {
    const url = { value: '' }
    server.use(
      get('/source/customers', (info) => {
        trackUrl(url)(info)
        return json(envelope([]))
      }),
    )

    await listCustomers('source', { search: '', country: '' })

    const parsed = new URL(url.value)
    expect(parsed.searchParams.has('search')).toBe(false)
    expect(parsed.searchParams.has('country')).toBe(false)
  })

  it('getCustomer interpola el token opaco TAL CUAL en el path', async () => {
    const url = { value: '' }
    const customer: Customer = {
      id: CUSTOMER_TOKEN,
      first_name: 'Ada',
      last_name: 'Lovelace',
      email: 'ada@example.com',
      country_code: 'GB',
      city: 'London',
      created_at: '2026-01-01T00:00:00Z',
    }
    server.use(
      get('/source/customers/:id', (info) => {
        trackUrl(url)(info)
        return json(customer)
      }),
    )

    const result = await getCustomer('source', CUSTOMER_TOKEN)
    expect(result).toEqual(customer)
    // El token no se decodifica ni se re-codifica: viaja verbatim.
    expect(new URL(url.value).pathname).toBe(`/api/v1/source/customers/${CUSTOMER_TOKEN}`)
  })

  it('listProducts pasa category y devuelve items', async () => {
    const url = { value: '' }
    const products: Product[] = [
      {
        id: PRODUCT_TOKEN,
        sku: 'SKU-1',
        name: 'Widget',
        category: 'Toys',
        price: 9.99,
        stock: 3,
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
      },
    ]
    server.use(
      get('/source/products', (info) => {
        trackUrl(url)(info)
        return json(envelope(products))
      }),
    )

    const result = await listProducts('source', { category: 'Toys' })
    expect(result.items).toEqual(products)
    expect(new URL(url.value).searchParams.get('category')).toBe('Toys')
  })

  it('listOrders pasa customer_id/status/date_from/date_to con tokens intactos', async () => {
    const url = { value: '' }
    server.use(
      get('/source/orders', (info) => {
        trackUrl(url)(info)
        return json(envelope([]))
      }),
    )

    await listOrders('source', {
      customer_id: CUSTOMER_TOKEN,
      status: 'paid',
      date_from: '2026-01-01',
      date_to: '2026-01-31',
    })

    const parsed = new URL(url.value)
    expect(parsed.searchParams.get('customer_id')).toBe(CUSTOMER_TOKEN)
    expect(parsed.searchParams.get('status')).toBe('paid')
    expect(parsed.searchParams.get('date_from')).toBe('2026-01-01')
    expect(parsed.searchParams.get('date_to')).toBe('2026-01-31')
  })

  it('getOrder devuelve la orden con sus order_items', async () => {
    const item: OrderItem = {
      id: ITEM_TOKEN,
      order_id: ORDER_TOKEN,
      product_id: PRODUCT_TOKEN,
      quantity: 2,
      unit_price: 5,
      subtotal: 10,
    }
    const order: Order = {
      id: ORDER_TOKEN,
      order_number: 'ORD-1',
      customer_id: CUSTOMER_TOKEN,
      status: 'paid',
      total_amount: 10,
      currency: 'USD',
      order_date: '2026-01-01T00:00:00Z',
      order_items: [item],
    }
    server.use(get('/source/orders/:id', () => json(order)))

    const result = await getOrder('source', ORDER_TOKEN)
    expect(result.order_items).toEqual([item])
    expect(result.id).toBe(ORDER_TOKEN)
    expect(result.customer_id).toBe(CUSTOMER_TOKEN)
    expect(result.order_items?.[0]?.id).toBe(ITEM_TOKEN)
  })

  it('listOrderItems pasa order_id/product_id como tokens intactos', async () => {
    const url = { value: '' }
    server.use(
      get('/source/order-items', (info) => {
        trackUrl(url)(info)
        return json(envelope([]))
      }),
    )

    await listOrderItems('source', { order_id: ORDER_TOKEN, product_id: PRODUCT_TOKEN })

    const parsed = new URL(url.value)
    expect(parsed.searchParams.get('order_id')).toBe(ORDER_TOKEN)
    expect(parsed.searchParams.get('product_id')).toBe(PRODUCT_TOKEN)
  })

  it('los tokens se usan verbatim aunque contengan caracteres base64url especiales', async () => {
    const url = { value: '' }
    const tricky = 'v1.a-b_cD-eF'
    server.use(
      get('/source/customers/:id', (info) => {
        trackUrl(url)(info)
        return json({})
      }),
    )

    await getCustomer('source', tricky)
    expect(new URL(url.value).pathname).toBe(`/api/v1/source/customers/${tricky}`)
    // El token NO se codifica ni se escapa en el path.
    expect(url.value).not.toContain('%')
  })
})

describe('api/data (target)', () => {
  it('usa el prefijo /target para el mismo recurso', async () => {
    const url = { value: '' }
    server.use(
      get('/target/customers', (info) => {
        trackUrl(url)(info)
        return json(envelope([]))
      }),
    )

    await listCustomers('target')
    expect(new URL(url.value).pathname).toBe('/api/v1/target/customers')
  })

  it('getProduct contra target usa /target/products/:token', async () => {
    const url = { value: '' }
    server.use(
      get('/target/products/:id', (info) => {
        trackUrl(url)(info)
        return json({})
      }),
    )

    await getProduct('target', PRODUCT_TOKEN)
    expect(new URL(url.value).pathname).toBe(`/api/v1/target/products/${PRODUCT_TOKEN}`)
  })
})

describe('api/data (errores)', () => {
  it('propaga el ApiError tipado del envelope', async () => {
    server.use(get('/source/customers', () => json(errorEnvelope('not_found', 'gone'), 404)))

    await expect(listCustomers('source')).rejects.toMatchObject({
      name: 'ApiError',
      status: 404,
      code: 'not_found',
      message: 'gone',
    })
  })

  it('un 400 por UUID crudo se propaga como bad_request', async () => {
    server.use(
      get('/source/customers/:id', () =>
        json(errorEnvelope('bad_request', 'id must be an opaque token'), 400),
      ),
    )

    await expect(getCustomer('source', 'a-raw-uuid')).rejects.toMatchObject({
      status: 400,
      code: 'bad_request',
    })
  })

  it('la ruta solicitada es realmente la del backend', async () => {
    expect(api('/source/customers')).toContain('/api/v1/source/customers')
  })
})

describe('api/data (normalización de meta)', () => {
  it('usa defaults razonables cuando el envelope no trae meta', async () => {
    server.use(get('/source/customers', () => json({ data: [{ id: 'v1.x' }] })))
    const result = await listCustomers('source')
    expect(result.page).toBe(1)
    expect(result.total).toBe(1)
    expect(result.per_page).toBe(1)
    expect(result.total_pages).toBe(1)
  })

  it('calcula total_pages si el meta no lo trae', async () => {
    server.use(
      get('/source/customers', () =>
        json({ data: [{ id: 'v1.x' }], meta: { per_page: 10, total: 25 } }),
      ),
    )
    const result = await listCustomers('source')
    expect(result.total_pages).toBe(3)
  })

  it('normaliza data no-array a lista vacía', async () => {
    server.use(get('/source/customers', () => json({ data: null, meta: { total: 0 } })))
    const result = await listCustomers('source')
    expect(result.items).toEqual([])
    expect(result.total_pages).toBe(0)
  })
})
