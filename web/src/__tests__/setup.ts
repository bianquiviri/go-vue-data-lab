import { afterAll, afterEach, beforeAll } from 'vitest'
import { config } from '@vue/test-utils'

import { DEFAULT_LOCALE, i18n } from '@/i18n'
import { server } from './mocks/server'

beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' })
})

afterEach(() => {
  server.resetHandlers()
  // Todas las suites parten del idioma por defecto: los specs que necesiten
  // otro idioma lo fijan de forma explícita (i18n.global.locale o el store).
  i18n.global.locale.value = DEFAULT_LOCALE
})

afterAll(() => {
  server.close()
})

// i18n disponible en TODOS los mounts de la suite (igual que en main.ts). Los
// specs no tienen que declarar el plugin en cada `mount()`.
config.global.plugins.push(i18n)
