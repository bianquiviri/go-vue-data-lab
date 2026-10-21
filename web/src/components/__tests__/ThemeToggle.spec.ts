import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import ThemeToggle from '../ThemeToggle.vue'
import { THEME_STORAGE_KEY, useThemeStore } from '@/stores/theme'
import { useLocale } from '@/__tests__/fixtures'

describe('components/ThemeToggle', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    useLocale('en')
  })

  it('renderiza los tres modos: Claro, Oscuro y Sistema', () => {
    const wrapper = mount(ThemeToggle)
    expect(wrapper.find('[data-test="theme-light"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-dark"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-system"]').exists()).toBe(true)
  })

  it('el grupo es accesible (role=group con aria-label traducido)', () => {
    const wrapper = mount(ThemeToggle)
    const group = wrapper.get('[role="group"]')
    expect(group.attributes('aria-label')).toBe('Theme')
  })

  it('traduce las etiquetas del grupo y de los botones al cambiar de idioma', async () => {
    const wrapper = mount(ThemeToggle)

    useLocale('es')
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[role="group"]').attributes('aria-label')).toBe('Tema')
    expect(wrapper.get('[data-test="theme-light"]').text()).toBe('Claro')
    expect(wrapper.get('[data-test="theme-dark"]').text()).toBe('Oscuro')
    expect(wrapper.get('[data-test="theme-system"]').text()).toBe('Sistema')
  })

  it('cambia a Oscuro: actualiza el store, <html> y localStorage', async () => {
    const wrapper = mount(ThemeToggle)

    await wrapper.get('[data-test="theme-dark"]').trigger('click')

    const theme = useThemeStore()
    expect(theme.mode).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('cambia a Claro desde Oscuro', async () => {
    const wrapper = mount(ThemeToggle)
    const theme = useThemeStore()

    await wrapper.get('[data-test="theme-dark"]').trigger('click')
    await wrapper.get('[data-test="theme-light"]').trigger('click')

    expect(theme.mode).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })

  it('marca aria-pressed en el modo activo', async () => {
    const wrapper = mount(ThemeToggle)

    await wrapper.get('[data-test="theme-dark"]').trigger('click')
    await wrapper.get('[data-test="theme-light"]').trigger('click')

    expect(wrapper.get('[data-test="theme-light"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="theme-dark"]').attributes('aria-pressed')).toBe('false')
  })
})
