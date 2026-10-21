import { createI18n } from 'vue-i18n'

import en from './locales/en'
import es from './locales/es'

export type AppLocale = 'es' | 'en'

/** Clave de localStorage de la preferencia de idioma (patrón `gdl.*`). */
export const LOCALE_STORAGE_KEY = 'gdl.locale'

/** Idioma por defecto cuando no hay preferencia guardada ni pista del navegador. */
export const DEFAULT_LOCALE: AppLocale = 'en'

/** Idiomas soportados (`as const` para el narrowing de `detectLocale`). */
export const SUPPORTED_LOCALES = ['es', 'en'] as const

/** Valida y normaliza el valor de una preferencia guardada. */
export function isAppLocale(value: unknown): value is AppLocale {
  return value === 'es' || value === 'en'
}

/**
 * Resuelve el idioma inicial en este orden:
 *   1. localStorage['gdl.locale'] si es válido (preferencia explícita).
 *   2. `navigator.language` que empiece por `es` ⇒ 'es'.
 *   3. Default final: 'en' (coherente con el Issue #37, UI en inglés).
 */
export function detectLocale(): AppLocale {
  let stored: string | null = null
  try {
    stored = window.localStorage.getItem(LOCALE_STORAGE_KEY)
  } catch {
    // storage no disponible: se cae a la detección del navegador
  }
  if (isAppLocale(stored)) return stored

  const navigatorLanguage =
    typeof navigator !== 'undefined' && typeof navigator.language === 'string'
      ? navigator.language
      : ''
  if (navigatorLanguage.toLowerCase().startsWith('es')) return 'es'

  return DEFAULT_LOCALE
}

/**
 * Instancia i18n de la app (Composition API). `locale` se resuelve con
 * `detectLocale()`; `stores/locale.ts` la hidrata y la persiste.
 */
export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: { en, es },
})

export default i18n
