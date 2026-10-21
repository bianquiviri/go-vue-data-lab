import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import AppFooter from '../AppFooter.vue'
import { useLocale } from '@/__tests__/fixtures'

/**
 * Footer compartido por TODAS las vistas (landing, login y panel autenticado).
 * El copy se traduce (ES/EN) salvo la autoría y el código de licencia.
 */
describe('components/AppFooter', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('declara la licencia Creative Commons más restrictiva (CC BY-NC-ND 4.0)', () => {
    useLocale('en')
    const wrapper = mount(AppFooter)
    const license = wrapper.get('[data-test="footer-license"]')
    expect(license.text()).toContain('CC BY-NC-ND 4.0')
    const link = license.get('a')
    expect(link.attributes('href')).toBe('https://creativecommons.org/licenses/by-nc-nd/4.0/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })

  it('identifica a la persona que desarrolló la app', () => {
    useLocale('en')
    const wrapper = mount(AppFooter)
    const author = wrapper.get('[data-test="footer-author"]')
    expect(author.text()).toContain('Jose Antonio Bianco')
  })

  it('mantiene el nombre del proyecto en el footer', () => {
    useLocale('en')
    const wrapper = mount(AppFooter)
    expect(wrapper.get('[data-test="footer-project"]').text()).toContain(
      'Concurrent Data Processing',
    )
  })

  it('traduce el texto del proyecto y de la licencia, pero no la autoría ni el código', () => {
    useLocale('es')
    const wrapper = mount(AppFooter)
    expect(wrapper.get('[data-test="footer-project"]').text()).toContain('plataforma sintética')
    expect(wrapper.get('[data-test="footer-license"]').text()).toContain('Licensed under')
    // No traducibles: autor y código de licencia.
    expect(wrapper.get('[data-test="footer-author"]').text()).toBe('Developed by Jose Antonio Bianco')
    expect(wrapper.get('[data-test="footer-license"]').text()).toContain('CC BY-NC-ND 4.0')
  })

  it('es un elemento footer con data-test estable', () => {
    useLocale('en')
    const wrapper = mount(AppFooter)
    expect(wrapper.find('footer[data-test="app-footer"]').exists()).toBe(true)
  })
})
