import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import LanguageToggle from '../LanguageToggle.vue'
import { DEFAULT_LOCALE, LOCALE_STORAGE_KEY, i18n } from '@/i18n'
import { useLocaleStore } from '@/stores/locale'

/**
 * Monta el toggle con la instancia i18n REAL de la app: el selector cambia el
 * idioma del store, y el store aplica ese idioma a `i18n.global` (la única
 * instancia de la app), igual que el ThemeToggle con el store de tema.
 */
function mountToggle() {
  const wrapper = mount(LanguageToggle, {
    global: { plugins: [i18n] },
  })
  return { wrapper }
}

describe('components/LanguageToggle', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    i18n.global.locale.value = DEFAULT_LOCALE
    document.documentElement.lang = ''
  })

  afterEach(() => {
    localStorage.clear()
    i18n.global.locale.value = DEFAULT_LOCALE
    document.documentElement.lang = ''
  })

  it('renderiza los botones ES y EN en un grupo accesible', () => {
    const { wrapper } = mountToggle()
    const group = wrapper.get('[data-test="language-toggle"]')
    expect(group.attributes('role')).toBe('group')
    expect(group.attributes('aria-label')).toBe('Language')
    expect(wrapper.find('[data-test="locale-es"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-en"]').exists()).toBe(true)
  })

  it('marca aria-pressed en el idioma activo', () => {
    const { wrapper } = mountToggle()
    expect(wrapper.get('[data-test="locale-en"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="locale-es"]').attributes('aria-pressed')).toBe('false')
  })

  it('cambia a ES: store, i18n, <html lang> y localStorage', async () => {
    const { wrapper } = mountToggle()

    await wrapper.get('[data-test="locale-es"]').trigger('click')

    const store = useLocaleStore()
    expect(store.locale).toBe('es')
    expect(i18n.global.locale.value).toBe('es')
    expect(document.documentElement.lang).toBe('es')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('es')
  })

  it('vuelve a EN desde ES', async () => {
    const { wrapper } = mountToggle()

    await wrapper.get('[data-test="locale-es"]').trigger('click')
    await wrapper.get('[data-test="locale-en"]').trigger('click')

    expect(useLocaleStore().locale).toBe('en')
    expect(i18n.global.locale.value).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en')
  })

  it('reactualiza aria-pressed y las etiquetas al cambiar de idioma', async () => {
    const { wrapper } = mountToggle()

    await wrapper.get('[data-test="locale-es"]').trigger('click')

    expect(wrapper.get('[data-test="locale-es"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="locale-en"]').attributes('aria-pressed')).toBe('false')
    // El aria-label del grupo se traduce con el idioma activo.
    expect(wrapper.get('[data-test="language-toggle"]').attributes('aria-label')).toBe('Idioma')
  })
})
