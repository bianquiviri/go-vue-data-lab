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

/**
 * jsdom no implementa la medición de texto de SVG (`getComputedTextLength`,
 * `getBBox`). Unovis la usa para los ejes y las etiquetas de los charts, así
 * que sin estos stubs lanzaría un TypeError durante el render. La estimación
 * (~7px por carácter) basta para que el layout no falle en los tests; en el
 * navegador se mide de verdad.
 */
interface MeasurableSvgElement extends SVGElement {
  getComputedTextLength?: () => number
  getBBox?: () => { x: number; y: number; width: number; height: number }
}
if (typeof SVGElement !== 'undefined') {
  const proto = SVGElement.prototype as MeasurableSvgElement
  const estimate = (element: SVGElement): number => (element.textContent ?? '').length * 7
  if (typeof proto.getComputedTextLength !== 'function') {
    proto.getComputedTextLength = function getComputedTextLength(this: SVGElement) {
      return estimate(this)
    }
  }
  if (typeof proto.getBBox !== 'function') {
    proto.getBBox = function getBBox(this: SVGElement) {
      return { x: 0, y: 0, width: estimate(this), height: 14 }
    }
  }
}

/**
 * jsdom no implementa `<canvas>`: cada `getContext()` avisa por consola
 * ("Not implemented"). Unovis lo usa solo como *fallback* para medir texto, y
 * jsdom ya devuelve `null` ahí; cambiamos la implementación por una silenciosa
 * que devuelve lo mismo para no ensuciar la salida de los tests.
 */
if (typeof HTMLCanvasElement !== 'undefined') {
  HTMLCanvasElement.prototype.getContext = (() => null) as HTMLCanvasElement['getContext']
}
