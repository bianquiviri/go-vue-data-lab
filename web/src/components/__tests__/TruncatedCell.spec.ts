import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import TruncatedCell from '../TruncatedCell.vue'

/**
 * Celda de una línea con truncado por elipsis y el valor completo en tooltip
 * nativo (`title`). Se usa en las columnas potencialmente largas (email, nombre
 * de producto, SKU, ciudad, dirección) para que cada fila ocupe una sola línea
 * sin desbordes (Issue #20, feedback de presentación).
 */
describe('components/TruncatedCell', () => {
  it('muestra el texto y expone el valor completo en el tooltip nativo', () => {
    const wrapper = mount(TruncatedCell, {
      props: { value: 'a-very-long-product-name-that-overflows@example.com' },
    })

    const el = wrapper.get('[data-test="truncated-cell"]')
    expect(el.text()).toBe('a-very-long-product-name-that-overflows@example.com')
    expect(el.attributes('title')).toBe('a-very-long-product-name-that-overflows@example.com')
  })

  it('trunca con elipsis mediante clases de Tailwind (una sola línea)', () => {
    const wrapper = mount(TruncatedCell, { props: { value: 'abc' } })
    const el = wrapper.get('[data-test="truncated-cell"]')
    expect(el.classes()).toContain('truncate')
    expect(el.classes()).toContain('block')
  })

  it('renderiza un guion cuando el valor es nulo o indefinido', () => {
    const wrapper = mount(TruncatedCell, { props: { value: null } })
    const el = wrapper.get('[data-test="truncated-cell"]')
    expect(el.text()).toBe('—')
    expect(el.attributes('title')).toBeUndefined()
  })

  it('permite overrides de clase sin perder el truncado', () => {
    const wrapper = mount(TruncatedCell, {
      props: { value: 'x', class: 'max-w-[10rem]' },
    })
    const el = wrapper.get('[data-test="truncated-cell"]')
    expect(el.classes()).toContain('truncate')
    expect(el.classes()).toContain('max-w-[10rem]')
  })
})
