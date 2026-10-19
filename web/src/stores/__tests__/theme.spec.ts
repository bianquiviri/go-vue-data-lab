import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { THEME_STORAGE_KEY, useThemeStore } from '../theme'
import { installMatchMedia } from '@/__tests__/fixtures'

function isDark(): boolean {
  return document.documentElement.classList.contains('dark')
}

describe('stores/theme', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  it('por defecto usa el modo sistema', () => {
    installMatchMedia(false)
    const store = useThemeStore()
    expect(store.mode).toBe('system')
    expect(store.resolved).toBe('light')
  })

  it('setMode("dark") aplica la clase dark y persiste en localStorage', () => {
    installMatchMedia(false)
    const store = useThemeStore()

    store.setMode('dark')
    expect(store.resolved).toBe('dark')
    expect(isDark()).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('setMode("light") quita la clase dark y persiste', () => {
    installMatchMedia(false)
    const store = useThemeStore()

    store.setMode('light')
    expect(store.resolved).toBe('light')
    expect(isDark()).toBe(false)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })

  it('el modo sistema respeta prefers-color-scheme y reacciona a cambios', () => {
    const media = installMatchMedia(true)
    const store = useThemeStore()
    store.init()

    expect(store.resolved).toBe('dark')
    expect(isDark()).toBe(true)

    media.emitChange(false)
    expect(store.resolved).toBe('light')
    expect(isDark()).toBe(false)

    media.emitChange(true)
    expect(isDark()).toBe(true)
  })

  it('init lee la preferencia persistida', () => {
    installMatchMedia(false)
    localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    const store = useThemeStore()

    store.init()
    expect(store.mode).toBe('dark')
    expect(isDark()).toBe(true)
  })

  it('init ignora valores inválidos del storage y usa sistema', () => {
    installMatchMedia(false)
    localStorage.setItem(THEME_STORAGE_KEY, 'blue')
    const store = useThemeStore()

    store.init()
    expect(store.mode).toBe('system')
  })

  it('en modo light/dark explícito los cambios del sistema no alteran la UI', () => {
    const media = installMatchMedia(false)
    const store = useThemeStore()
    store.init()
    store.setMode('light')

    media.emitChange(true)
    expect(store.resolved).toBe('light')
    expect(isDark()).toBe(false)
  })

  it('init registra un listener del sistema y dispose lo remueve', () => {
    const media = installMatchMedia(false)
    const store = useThemeStore()

    store.init()
    expect(media.listenerCount()).toBe(1)

    store.dispose()
    expect(media.listenerCount()).toBe(0)
  })

  it('init no lanza si matchMedia no está disponible', () => {
    vi.stubGlobal('matchMedia', undefined)
    const store = useThemeStore()
    expect(() => store.init()).not.toThrow()
    expect(store.resolved).toBe('light')
  })
})
