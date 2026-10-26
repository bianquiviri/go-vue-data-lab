import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

import { useAnalytics } from '../useAnalytics'
import { server } from '@/__tests__/mocks/server'
import { errorEnvelope, get, json, useLocale } from '@/__tests__/fixtures'
import type { AnalyticsLimit, AnalyticsSummary, TopCustomer, TopProduct } from '@/types/analytics'

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

const PRODUCTS: TopProduct[] = [
  {
    id: 'v1.product-a',
    name: 'Mouse',
    sku: 'SKU-001',
    category: 'Electronics',
    total_quantity: 40,
    total_revenue: 1999.5,
  },
]

const CUSTOMERS: TopCustomer[] = [
  {
    id: 'v1.customer-a',
    first_name: 'Ada',
    last_name: 'Lovelace',
    email: 'ada@example.com',
    total_orders: 8,
    total_spent: 4321.5,
  },
]

function mountComposable(initialLimit: AnalyticsLimit = 10) {
  let api!: ReturnType<typeof useAnalytics>
  const Host = defineComponent({
    setup() {
      api = useAnalytics({ initialLimit })
      return () => h('div')
    },
  })
  const wrapper = mount(Host, { global: { plugins: [createPinia()] } })
  return { wrapper, api }
}

describe('composables/useAnalytics', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('carga summary, top-products y top-customers en paralelo', async () => {
    const calls: string[] = []
    server.use(
      get('/target/analytics/summary', (info) => {
        calls.push(new URL(info.request.url).pathname)
        return json(SUMMARY)
      }),
      get('/target/analytics/top-products', (info) => {
        calls.push(new URL(info.request.url).pathname + new URL(info.request.url).search)
        return json({ data: PRODUCTS, meta: { limit: 10 } })
      }),
      get('/target/analytics/top-customers', (info) => {
        calls.push(new URL(info.request.url).pathname + new URL(info.request.url).search)
        return json({ data: CUSTOMERS, meta: { limit: 10 } })
      }),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    expect(api.loading.value).toBe(false)
    expect(api.summary.value).toEqual(SUMMARY)
    expect(api.products.value).toEqual(PRODUCTS)
    expect(api.customers.value).toEqual(CUSTOMERS)
    expect(calls).toContain('/api/v1/target/analytics/summary')
    expect(calls.some((c) => c.includes('/api/v1/target/analytics/top-products?limit=10'))).toBe(true)
    expect(calls.some((c) => c.includes('/api/v1/target/analytics/top-customers?limit=10'))).toBe(true)
    expect(api.error.value).toBeNull()
  })

  it('vuelve a cargar al cambiar el limit y emite nuevas peticiones con ?limit=N', async () => {
    const limits: string[] = []
    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', (info) => {
        limits.push(new URL(info.request.url).searchParams.get('limit') ?? '')
        return json({ data: PRODUCTS, meta: { limit: Number(new URL(info.request.url).searchParams.get('limit')) } })
      }),
      get('/target/analytics/top-customers', (info) => {
        limits.push(new URL(info.request.url).searchParams.get('limit') ?? '')
        return json({ data: CUSTOMERS, meta: { limit: Number(new URL(info.request.url).searchParams.get('limit')) } })
      }),
    )

    const { api } = mountComposable(10)
    await flushPromises()
    limits.length = 0

    api.setLimit(20)
    await flushPromises()

    expect(limits).toEqual(['20', '20'])
  })

  it('reload() vuelve a pedir los tres endpoints', async () => {
    let summaryCalls = 0
    server.use(
      get('/target/analytics/summary', () => {
        summaryCalls += 1
        return json(SUMMARY)
      }),
      get('/target/analytics/top-products', () => json({ data: PRODUCTS })),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS })),
    )

    const { api } = mountComposable(10)
    await flushPromises()
    expect(summaryCalls).toBe(1)

    await api.reload()
    await flushPromises()
    expect(summaryCalls).toBe(2)
  })

  it('si falla el summary pero los top responden, muestra los top y un aviso sutil', async () => {
    server.use(
      get('/target/analytics/summary', () => json(errorEnvelope('internal_error', 'boom'), 500)),
      get('/target/analytics/top-products', () => json({ data: PRODUCTS })),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS })),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    expect(api.loading.value).toBe(false)
    expect(api.summary.value).toBeNull()
    expect(api.products.value).toEqual(PRODUCTS)
    expect(api.customers.value).toEqual(CUSTOMERS)
    expect(api.error.value).toBeTruthy()
    expect(api.partial.value).toBe(true)
  })

  it('error si fallan todas las peticiones', async () => {
    server.use(
      get('/target/analytics/summary', () => json(errorEnvelope('internal_error', 'boom'), 500)),
      get('/target/analytics/top-products', () => json(errorEnvelope('internal_error', 'x'), 500)),
      get('/target/analytics/top-customers', () => json(errorEnvelope('internal_error', 'y'), 500)),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    expect(api.loading.value).toBe(false)
    expect(api.products.value).toEqual([])
    expect(api.customers.value).toEqual([])
    expect(api.summary.value).toBeNull()
    expect(api.error.value).toBeTruthy()
    expect(api.partial.value).toBe(false)
  })

  it('tiene hasData true cuando hay algo que mostrar', async () => {
    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', () => json({ data: [] })),
      get('/target/analytics/top-customers', () => json({ data: [] })),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    expect(api.hasData.value).toBe(true)
  })

  it('el mensaje de error sigue al idioma activo (no se congela al fallar)', async () => {
    server.use(
      get('/target/analytics/summary', () => json(errorEnvelope('internal_error', 'boom'), 500)),
      get('/target/analytics/top-products', () => json(errorEnvelope('internal_error', 'x'), 500)),
      get('/target/analytics/top-customers', () => json(errorEnvelope('internal_error', 'y'), 500)),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    expect(api.errorKey.value).toBe('analytics.error')
    expect(api.error.value).toBe('Could not load analytics')

    useLocale('es')
    await flushPromises()
    expect(api.error.value).toBe('No se pudo cargar la analítica')
  })

  it('descarta la respuesta obsoleta cuando el límite cambia con una carga en vuelo', async () => {
    let resolveFirst: (() => void) | undefined
    let productCalls = 0
    const PRODUCT: TopProduct = {
      id: 'v1.product-a',
      name: 'base',
      sku: 'SKU-001',
      category: 'Electronics',
      total_quantity: 1,
      total_revenue: 10,
    }

    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', async (info) => {
        productCalls += 1
        const limit = new URL(info.request.url).searchParams.get('limit')
        if (limit === '10') {
          // La petición del límite viejo se resuelve tarde y con datos distintos.
          await new Promise<void>((resolve) => {
            resolveFirst = resolve
          })
          return json({ data: [{ ...PRODUCT, name: 'stale-10' }], meta: { limit: 10 } })
        }
        return json({ data: [{ ...PRODUCT, name: 'fresh-20' }], meta: { limit: 20 } })
      }),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS })),
    )

    const { api } = mountComposable(10)
    await flushPromises()

    api.setLimit(20)
    await flushPromises()

    // La vieja llega después: no debe pisar la tabla del límite nuevo
    resolveFirst?.()
    await flushPromises()

    expect(productCalls).toBeGreaterThanOrEqual(2)
    expect(api.products.value[0]?.name).toBe('fresh-20')
    expect(api.loading.value).toBe(false)
  })
})
