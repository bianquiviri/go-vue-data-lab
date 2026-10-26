import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import RevenueBars from '../RevenueBars.vue'
import { useLocale } from '@/__tests__/fixtures'
import type { TopProduct } from '@/types/analytics'

const PRODUCTS: TopProduct[] = [
  {
    id: 'v1.product-a',
    name: 'Wireless Mouse',
    sku: 'SKU-001',
    category: 'Electronics',
    total_quantity: 40,
    total_revenue: 1999.5,
  },
  {
    id: 'v1.product-b',
    name: 'Coffee Beans',
    sku: 'SKU-002',
    category: 'Grocery',
    total_quantity: 120,
    total_revenue: 540.25,
  },
]

/**
 * Unovis mide el contenedor con `clientWidth`/`clientHeight`; en jsdom valen 0 y
 * el chart no dibuja nada (el SVG sale vacío), así que el test no podría detectar
 * un mapeo de ejes roto. Forzamos un tamaño para que renderice de verdad.
 */
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, 'clientWidth', {
    configurable: true,
    get: () => 640,
  })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
    configurable: true,
    get: () => 256,
  })
})

afterAll(() => {
  delete (HTMLElement.prototype as { clientWidth?: number }).clientWidth
  delete (HTMLElement.prototype as { clientHeight?: number }).clientHeight
})

describe('components/analytics/RevenueBars', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useLocale('en')
  })

  it('renderiza el gráfico con datos', async () => {
    const wrapper = mount(RevenueBars, { props: { products: PRODUCTS } })
    await flushPromises()

    expect(wrapper.find('[data-test="analytics-revenue-bars"]').exists()).toBe(true)
  })

  it('dibuja una barra por producto con el color del tema (sin coordenadas NaN)', async () => {
    const wrapper = mount(RevenueBars, { props: { products: PRODUCTS } })
    await flushPromises()

    const barPaths = () =>
      wrapper.findAll('path').filter((path) => (path.attributes('style') ?? '').includes('chart-1'))

    // Unovis pinta las barras en un tick posterior al mount.
    await vi.waitFor(() => expect(barPaths()).toHaveLength(PRODUCTS.length))

    // Regresión: con la categoría como string en la escala numérica de Unovis
    // las barras se dibujaban con `d="MNaN,..."` (o directamente no se dibujaban).
    // (Los `dy="NaNem"` de las etiquetas de eje sí son un artefacto de jsdom:
    // no hay métricas de fuente reales.)
    barPaths().forEach((path) => expect(path.attributes('d')).not.toContain('NaN'))
    expect(barPaths()[0]?.attributes('d')).toMatch(/^M[\d.]/)
  })

  it('muestra estado vacío cuando no hay productos', async () => {
    const wrapper = mount(RevenueBars, { props: { products: [] } })
    await flushPromises()

    expect(wrapper.text()).toContain('No results')
  })
})
