import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

export type ThemeMode = 'light' | 'dark' | 'system'

export const THEME_STORAGE_KEY = 'gdl.theme'

const DARK_MEDIA_QUERY = '(prefers-color-scheme: dark)'

function hasMatchMedia(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
}

function prefersDark(): boolean {
  return hasMatchMedia() ? window.matchMedia(DARK_MEDIA_QUERY).matches : false
}

/**
 * Tema Claro/Oscuro/Sistema. La clase `dark` se aplica en <html> (Tailwind
 * dark variant) y también se replica en `color-scheme` para los controles nativos.
 * El flash inicial lo evita el script inline de index.html con esta misma clave.
 */
export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>('system')
  // Estado reactivo del sistema: Vue no puede rastrear window.matchMedia, así
  // que el listener de cambios lo mantiene al día.
  const systemDark = ref(prefersDark())
  const resolved = computed<'light' | 'dark'>(() =>
    mode.value === 'system' ? (systemDark.value ? 'dark' : 'light') : mode.value,
  )

  function apply(): void {
    document.documentElement.classList.toggle('dark', resolved.value === 'dark')
    document.documentElement.style.colorScheme = resolved.value
  }

  function setMode(next: ThemeMode): void {
    mode.value = next
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, next)
    } catch {
      // storage no disponible: el tema sigue funcionando en memoria
    }
    apply()
  }

  function onSystemChange(): void {
    systemDark.value = prefersDark()
    if (mode.value === 'system') apply()
  }

  /** Hidrata la preferencia persistida y sincroniza con el sistema. */
  function init(): void {
    let stored: string | null = null
    try {
      stored = window.localStorage.getItem(THEME_STORAGE_KEY)
    } catch {
      // storage no disponible
    }
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      mode.value = stored
    }
    systemDark.value = prefersDark()
    apply()
    if (hasMatchMedia()) {
      window.matchMedia(DARK_MEDIA_QUERY).addEventListener('change', onSystemChange)
    }
  }

  /** Limpia el listener del sistema (usado por pinia y los tests). */
  function dispose(): void {
    if (hasMatchMedia()) {
      window.matchMedia(DARK_MEDIA_QUERY).removeEventListener('change', onSystemChange)
    }
  }

  return { mode, resolved, init, setMode, dispose }
})
