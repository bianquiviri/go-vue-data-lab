import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import StatusDonut from '../StatusDonut.vue'
import { useLocale } from '@/__tests__/fixtures'
import type { AnalyticsSummary } from '@/types/analytics'

const SUMMARY: AnalyticsSummary = {
  total_customers: 10,
  total_products: 5,
  total_orders: 20,
  total_revenue: 1000,
  average_order_value: 50,
  orders_by_status: [
    { status: 'paid', count: 12 },
    { status: 'pending', count: 8 },
  ],
}

describe('components/analytics/StatusDonut', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useLocale('es')
  })

  it('muestra el donut y la leyenda cuando hay datos', async () => {
    const wrapper = mount(StatusDonut, { props: { summary: SUMMARY } })
    await flushPromises()

    expect(wrapper.find('[data-test="analytics-status-donut"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="analytics-status-legend"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Pagado')
    expect(wrapper.text()).toContain('Pendiente')
  })

  it('muestra estado vacío cuando orders_by_status es null', async () => {
    const wrapper = mount(StatusDonut, { props: { summary: { ...SUMMARY, orders_by_status: null } } })
    await flushPromises()

    expect(wrapper.text()).toContain('No hay órdenes para mostrar')
    expect(wrapper.find('[data-test="analytics-status-legend"]').exists()).toBe(false)
  })

  it('muestra estado vacío cuando orders_by_status está vacío', async () => {
    const wrapper = mount(StatusDonut, { props: { summary: { ...SUMMARY, orders_by_status: [] } } })
    await flushPromises()

    expect(wrapper.text()).toContain('No hay órdenes para mostrar')
  })

  it('traduce estados conocidos y hace fallback al crudo si no existe traducción', async () => {
    useLocale('en')
    const wrapper = mount(StatusDonut, {
      props: { summary: { ...SUMMARY, orders_by_status: [{ status: 'unknown', count: 3 }] } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('unknown')
  })

  it('colorea la leyenda con la paleta del tema y la anuncia a lectores de pantalla', async () => {
    const wrapper = mount(StatusDonut, { props: { summary: SUMMARY } })
    await flushPromises()

    // paid:12 va primero (ordenado) ⇒ --chart-1; pending:8 ⇒ --chart-2
    const swatches = wrapper.findAll('[data-test="analytics-status-legend-swatch"]')
    expect(swatches).toHaveLength(2)
    expect(swatches[0]?.attributes('style')).toContain('var(--chart-1)')
    expect(swatches[1]?.attributes('style')).toContain('var(--chart-2)')

    expect(wrapper.get('[data-test="analytics-status-legend"]').attributes('aria-label')).toBe(
      'Distribución por estado',
    )
  })
})
