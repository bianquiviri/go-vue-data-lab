import { describe, expect, it } from 'vitest'

import { getAnalyticsSummary, getTopCustomers, getTopProducts } from '../analytics'
import { server } from '@/__tests__/mocks/server'
import { errorEnvelope, get, json } from '@/__tests__/fixtures'
import { ApiError } from '@/api/client'
import type { AnalyticsSummary, TopCustomer, TopProduct } from '@/types/analytics'

const SUMMARY: AnalyticsSummary = {
  total_customers: 120,
  total_products: 80,
  total_orders: 340,
  total_revenue: 123456.78,
  average_order_value: 363.11,
  orders_by_status: [
    { status: 'paid', count: 200 },
    { status: 'pending', count: 140 },
  ],
}

const PRODUCT: TopProduct = {
  id: 'v1.product-opaque-token',
  name: 'Wireless Mouse',
  sku: 'SKU-001',
  category: 'Electronics',
  total_quantity: 40,
  total_revenue: 1999.5,
}

const CUSTOMER: TopCustomer = {
  id: 'v1.customer-opaque-token',
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  total_orders: 8,
  total_spent: 4321.5,
}

describe('api/analytics', () => {
  it('getAnalyticsSummary devuelve el objeto plano (sin envelope data)', async () => {
    let url = ''
    let method = ''
    server.use(
      get('/target/analytics/summary', (info) => {
        url = info.request.url
        method = info.request.method
        // Contrato real: RespondJSON, sin {"data":...}.
        return json(SUMMARY)
      }),
    )

    const summary = await getAnalyticsSummary()

    expect(method).toBe('GET')
    expect(new URL(url).pathname).toBe('/api/v1/target/analytics/summary')
    expect(new URL(url).search).toBe('')
    expect(summary).toEqual(SUMMARY)
  })

  it('getTopProducts envía siempre ?limit=N y devuelve el array', async () => {
    let url = ''
    server.use(
      get('/target/analytics/top-products', (info) => {
        url = info.request.url
        return json({ data: [PRODUCT], meta: { limit: 20 } })
      }),
    )

    const products = await getTopProducts(20)

    const parsed = new URL(url)
    expect(parsed.pathname).toBe('/api/v1/target/analytics/top-products')
    expect(parsed.searchParams.get('limit')).toBe('20')
    expect(products).toEqual([PRODUCT])
    // El token opaco viaja verbatim: nunca se transforma.
    expect(products[0]!.id).toBe('v1.product-opaque-token')
  })

  it('getTopCustomers envía ?limit=N y devuelve el array', async () => {
    let url = ''
    server.use(
      get('/target/analytics/top-customers', (info) => {
        url = info.request.url
        return json({ data: [CUSTOMER], meta: { limit: 5 } })
      }),
    )

    const customers = await getTopCustomers(5)

    expect(new URL(url).searchParams.get('limit')).toBe('5')
    expect(customers).toEqual([CUSTOMER])
    expect(customers[0]!.id).toBe('v1.customer-opaque-token')
  })

  it('tolera respuestas sin data (null) devolviendo listas vacías', async () => {
    server.use(
      get('/target/analytics/top-products', () => json({ data: null })),
      get('/target/analytics/top-customers', () => json({})),
    )

    await expect(getTopProducts(10)).resolves.toEqual([])
    await expect(getTopCustomers(10)).resolves.toEqual([])
  })

  it('propaga el ApiError del backend', async () => {
    server.use(
      get('/target/analytics/summary', () => json(errorEnvelope('internal_error', 'boom'), 500)),
    )

    await expect(getAnalyticsSummary()).rejects.toBeInstanceOf(ApiError)
  })
})
