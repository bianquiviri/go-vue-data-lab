import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import StatusBadge from '../StatusBadge.vue'

/**
 * Badge de estado: por defecto muestra el valor del dominio tal cual (p. ej.
 * `paid`), pero admite una etiqueta traducida (`label`) para los dominios
 * cerrados y conocidos (estados de job).
 */
describe('components/StatusBadge', () => {
  it('muestra el valor del dominio cuando no hay etiqueta', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'paid' } })
    expect(wrapper.get('[data-test="status-badge"]').text()).toBe('paid')
  })

  it('usa la etiqueta traducida cuando se proporciona', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'running', label: 'En ejecución' } })
    expect(wrapper.get('[data-test="status-badge"]').text()).toBe('En ejecución')
  })

  it('mantiene la variante visual a partir del valor del dominio', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'cancelled', label: 'Cancelado' } })
    const badge = wrapper.get('[data-test="status-badge"]')
    expect(badge.text()).toBe('Cancelado')
    expect(badge.classes().join(' ')).toContain('text-destructive')
  })
})
