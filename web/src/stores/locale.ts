import { ref } from 'vue'
import { defineStore } from 'pinia'

import { LOCALE_STORAGE_KEY, detectLocale, i18n, type AppLocale } from '@/i18n'

/**
 * Idioma de la interfaz (ES/EN). Sigue el mismo patrón que `stores/theme.ts`:
 * ref reactiva, `init()` que hidrata la preferencia persistida, `setLocale()`
 * que persiste + aplica, y sincronización con `<html lang>`.
 *
 * El idioma real de la UI lo guarda la instancia de vue-i18n
 * (`i18n.global.locale`); este store es la fuente de verdad de la preferencia
 * persistida y el pegamento entre localStorage, i18n y el DOM.
 */
export const useLocaleStore = defineStore('locale', () => {
  const locale = ref<AppLocale>(detectLocale())

  /** Aplica el idioma a vue-i18n y al atributo `lang` de <html>. */
  function apply(next: AppLocale): void {
    i18n.global.locale.value = next
    document.documentElement.lang = next
  }

  /** Cambia el idioma, lo persiste y lo aplica. */
  function setLocale(next: AppLocale): void {
    locale.value = next
    try {
      window.localStorage.setItem(LOCALE_STORAGE_KEY, next)
    } catch {
      // storage no disponible: el idioma sigue funcionando en memoria
    }
    apply(next)
  }

  /** Hidrata la preferencia persistida y sincroniza i18n + <html lang>. */
  function init(): void {
    let stored: string | null = null
    try {
      stored = window.localStorage.getItem(LOCALE_STORAGE_KEY)
    } catch {
      // storage no disponible
    }
    locale.value = stored === 'es' || stored === 'en' ? stored : detectLocale()
    apply(locale.value)
  }

  /** Atajo de traducción para consumidores fuera de plantillas. */
  function t(key: string, params?: Record<string, unknown>): string {
    return params ? i18n.global.t(key, params) : i18n.global.t(key)
  }

  return { locale, init, setLocale, t }
})
