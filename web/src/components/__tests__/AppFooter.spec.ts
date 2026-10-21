import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import AppFooter from '../AppFooter.vue'

/**
 * Footer compartido por TODAS las vistas (landing, login y panel autenticado).
 * Afecta al Issue #37 (copy en inglés): la cadenas se escriben en inglés desde ya.
 */
describe('components/AppFooter', () => {
  it('declara la licencia Creative Commons más restrictiva (CC BY-NC-ND 4.0)', () => {
    const wrapper = mount(AppFooter)
    const license = wrapper.get('[data-test="footer-license"]')
    expect(license.text()).toContain('CC BY-NC-ND 4.0')
    const link = license.get('a')
    expect(link.attributes('href')).toBe('https://creativecommons.org/licenses/by-nc-nd/4.0/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })

  it('identifica a la persona que desarrolló la app', () => {
    const wrapper = mount(AppFooter)
    const author = wrapper.get('[data-test="footer-author"]')
    expect(author.text()).toContain('Jose Antonio Bianco')
  })

  it('mantiene el nombre del proyecto en el footer', () => {
    const wrapper = mount(AppFooter)
    expect(wrapper.get('[data-test="footer-project"]').text()).toContain(
      'Concurrent Data Processing',
    )
  })

  it('es un elemento footer con data-test estable', () => {
    const wrapper = mount(AppFooter)
    expect(wrapper.find('footer[data-test="app-footer"]').exists()).toBe(true)
  })
})
