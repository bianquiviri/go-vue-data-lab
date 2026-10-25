import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createAppRouter } from '@/router'

import AnalyticsView from '../AnalyticsView.vue'
import { server } from '@/__tests__/mocks/server'
import { errorEnvelope, get, json, useLocale } from '@/__tests__/fixtures'
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

const PRODUCTS: TopProduct[] = [
  {
    id: 'v1.product-opaque-token',
    name: 'Wireless Mouse',
    sku: 'SKU-001',
    category: 'Electronics',
    total_quantity: 40,
    total_revenue: 1999.5,
  },
]

const CUSTOMERS: TopCustomer[] = [
  {
    id: 'v1.customer-opaque-token',
    first_name: 'Ada',
    last_name: 'Lovelace',
    email: 'ada@example.com',
    total_orders: 8,
    total_spent: 4321.5,
  },
]

function mountAnalytics() {
  const router = createAppRouter()
  return mount(AnalyticsView, { global: { plugins: [router] } })
}

describe('views/AnalyticsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('en')
  })

  it('muestra loading y luego los datos con las 5 cards', async () => {
    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', () => json({ data: PRODUCTS, meta: { limit: 10 } })),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS, meta: { limit: 10 } })),
    )

    const wrapper = mountAnalytics()
    expect(wrapper.text()).toContain('Loading analytics')

    await flushPromises()

    expect(wrapper.find('[data-test="analytics-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="analytics-card-customers"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="analytics-card-products"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="analytics-card-orders"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="analytics-card-revenue"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="analytics-card-aov"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Wireless Mouse')
    expect(wrapper.text()).toContain('Ada Lovelace')
    expect(wrapper.text()).toContain('ada@example.com')
    // IDs opacos con IdChip
    expect(wrapper.find('[data-test="id-chip"]').exists()).toBe(true)
  })

  it('muestra error con reintento cuando todo falla', async () => {
    server.use(
      get('/target/analytics/summary', () => json(errorEnvelope('internal_error', 'boom'), 500)),
      get('/target/analytics/top-products', () => json(errorEnvelope('internal_error', 'boom'), 500)),
      get('/target/analytics/top-customers', () => json(errorEnvelope('internal_error', 'boom'), 500)),
    )
    // contador no necesario, solo reintento

    const wrapper = mountAnalytics()
    await flushPromises()

    expect(wrapper.find('[data-test="analytics-error"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Could not load analytics')
  })

  it('al cambiar limit a 20 hace nuevas peticiones con ?limit=20', async () => {
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

    const wrapper = mountAnalytics()
    await flushPromises()
    limits.length = 0

    const select = wrapper.get('[data-test="analytics-limit"]')
    await select.setValue('20')
    await flushPromises()

    expect(limits).toEqual(['20', '20'])
  })

  it('muestra estado vacío para top si no hay resultados', async () => {
    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', () => json({ data: [] })),
      get('/target/analytics/top-customers', () => json({ data: [] })),
    )

    const wrapper = mountAnalytics()
    await flushPromises()

    expect(wrapper.text()).toContain('No results')
  })

  it('funciona en español', async () => {
    useLocale('es')
    server.use(
      get('/target/analytics/summary', () => json(SUMMARY)),
      get('/target/analytics/top-products', () => json({ data: PRODUCTS })),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS })),
    )

    const wrapper = mountAnalytics()
    await flushPromises()

    expect(wrapper.text()).toContain('Analítica')
    expect(wrapper.text()).toContain('Ingresos')
    expect(wrapper.text()).toContain('Órdenes por estado')
  })

  it('refresca al pulsar el botón', async () => {
    let summaryCalls = 0
    server.use(
      get('/target/analytics/summary', () => {
        summaryCalls += 1
        return json(SUMMARY)
      }),
      get('/target/analytics/top-products', () => json({ data: PRODUCTS })),
      get('/target/analytics/top-customers', () => json({ data: CUSTOMERS })),
    )

    const wrapper = mountAnalytics()
    await flushPromises()
    expect(summaryCalls).toBe(1)

    await wrapper.get('[data-test="analytics-refresh"]').trigger('click')
    await flushPromises()
    expect(summaryCalls).toBe(2)
  })
})
