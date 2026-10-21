import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { DEFAULT_LOCALE, LOCALE_STORAGE_KEY, detectLocale, i18n, isAppLocale } from '..'
import en from '../locales/en'
import es from '../locales/es'
import { useLocaleStore } from '@/stores/locale'

/** Cambia el idioma del navegador (jsdom lo expone en `navigator.language`). */
function stubNavigatorLanguage(language: string): void {
  vi.stubGlobal(
    'navigator',
    Object.create(navigator, {
      language: { get: () => language, configurable: true },
    }),
  )
}

describe('i18n/detección', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    // Restaura el idioma de trabajo de la suite para no contaminar otros specs.
    i18n.global.locale.value = DEFAULT_LOCALE
  })

  it('usa el default (en) sin preferencia guardada ni pista del navegador', () => {
    stubNavigatorLanguage('fr-FR')
    expect(DEFAULT_LOCALE).toBe('en')
    expect(detectLocale()).toBe('en')
  })

  it('detecta es cuando navigator.language empieza por "es"', () => {
    stubNavigatorLanguage('es-AR')
    expect(detectLocale()).toBe('es')
  })

  it('ignora la pista del navegador si su variante no es española', () => {
    stubNavigatorLanguage('en-US')
    expect(detectLocale()).toBe('en')
  })

  it('la preferencia guardada válida gana sobre el navegador', () => {
    stubNavigatorLanguage('en-US')
    localStorage.setItem(LOCALE_STORAGE_KEY, 'es')
    expect(detectLocale()).toBe('es')

    localStorage.setItem(LOCALE_STORAGE_KEY, 'en')
    stubNavigatorLanguage('es-ES')
    expect(detectLocale()).toBe('en')
  })

  it('ignora valores inválidos del storage', () => {
    stubNavigatorLanguage('en-US')
    localStorage.setItem(LOCALE_STORAGE_KEY, 'fr')
    expect(detectLocale()).toBe('en')
  })

  it('si el storage no está disponible cae a la detección del navegador', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage bloqueado')
    })
    stubNavigatorLanguage('es-MX')
    expect(detectLocale()).toBe('es')
    getItem.mockRestore()
  })

  it('sin navigator usa el default (en)', () => {
    vi.stubGlobal('navigator', undefined)
    expect(detectLocale()).toBe('en')
  })

  it('isAppLocale valida solo es/en', () => {
    expect(isAppLocale('es')).toBe(true)
    expect(isAppLocale('en')).toBe(true)
    expect(isAppLocale('fr')).toBe(false)
    expect(isAppLocale(null)).toBe(false)
  })

  it('la instancia i18n arranca en modo Composition y con fallback en', () => {
    expect(i18n.global.locale.value).toBe('en')
    expect(i18n.global.fallbackLocale.value).toBe('en')
    // Composición habilitada: new Function evita que el lint marque `this` implícito.
    expect(i18n.global.t).toBeInstanceOf(Function)
  })
})

describe('i18n/formatos', () => {
  it('en.ts y es.ts comparten el mismo esqueleto de claves', () => {
    const skeleton = (value: unknown): string[] =>
      Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
        child !== null && typeof child === 'object'
          ? skeleton(child).map((path) => `${key}.${path}`)
          : [key],
      )

    expect(skeleton(es).sort()).toEqual(skeleton(en).sort())
  })

  it('traduce el mismo mensaje en ambos idiomas', () => {
    expect(en.auth.title).toBe('Sign in')
    expect(es.auth.title).toBe('Iniciar sesión')
    expect(en.roles.admin).toBe('Administrator')
    expect(es.roles.admin).toBe('Administrador')
    // Lo que NO se traduce: autoría y código de licencia.
    expect(es.footer.developedBy).toBe(en.footer.developedBy)
  })

  it('interpola parámetros nombrados en el saludo', () => {
    setActivePinia(createPinia())
    const store = useLocaleStore()
    store.setLocale('es')
    expect(store.t('dashboard.greeting', { name: 'root' })).toContain('root')
    expect(store.t('dashboard.greeting', { name: 'root' })).toContain('Hola')
    i18n.global.locale.value = DEFAULT_LOCALE
  })
})

describe('stores/locale', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    stubNavigatorLanguage('en-US')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    i18n.global.locale.value = DEFAULT_LOCALE
    document.documentElement.lang = ''
  })

  it('por defecto arranca en el idioma inicial (en)', () => {
    const store = useLocaleStore()
    expect(store.locale).toBe('en')
    expect(store.t('auth.title')).toBe('Sign in')
  })

  it('init hidrata la preferencia persistida y actualiza i18n + <html lang>', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'es')
    const store = useLocaleStore()

    store.init()

    expect(store.locale).toBe('es')
    expect(i18n.global.locale.value).toBe('es')
    expect(document.documentElement.lang).toBe('es')
  })

  it('init ignora valores inválidos del storage y mantiene el idioma detectado', () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'de')
    const store = useLocaleStore()

    store.init()

    expect(store.locale).toBe('en')
    expect(i18n.global.locale.value).toBe('en')
  })

  it('setLocale("es") traduce, persiste y actualiza <html lang>', () => {
    const store = useLocaleStore()

    store.setLocale('es')

    expect(store.locale).toBe('es')
    expect(store.t('auth.title')).toBe('Iniciar sesión')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('es')
    expect(document.documentElement.lang).toBe('es')
    expect(i18n.global.locale.value).toBe('es')
  })

  it('setLocale("en") vuelve al inglés y lo persiste', () => {
    const store = useLocaleStore()
    store.setLocale('es')

    store.setLocale('en')

    expect(store.locale).toBe('en')
    expect(store.t('auth.title')).toBe('Sign in')
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en')
    expect(document.documentElement.lang).toBe('en')
  })

  it('el idioma persiste tras "recargar" (nuevo store + init)', () => {
    useLocaleStore().setLocale('es')

    // Simula una recarga: pinia nuevo y store nuevo leyendo localStorage.
    setActivePinia(createPinia())
    const reloaded = useLocaleStore()
    reloaded.init()

    expect(reloaded.locale).toBe('es')
    expect(reloaded.t('auth.title')).toBe('Iniciar sesión')
  })

  it('no lanza si localStorage no está disponible', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage bloqueado')
    })
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage bloqueado')
    })

    const store = useLocaleStore()
    expect(() => store.init()).not.toThrow()
    expect(store.locale).toBe('en')
    expect(() => store.setLocale('es')).not.toThrow()
    expect(store.locale).toBe('es')

    getItem.mockRestore()
    setItem.mockRestore()
  })
})
