/**
 * Formateo localizado para la analítica (Issue #24).
 *
 * La moneda del dataset es **USD** (fija). El idioma llega desde i18n
 * (`locale.value`), nunca se hardcodea en las vistas: los importes y conteos se
 * adaptan al idioma activo (separadores, símbolo de moneda, posición).
 *
 * Los helpers son puros y no dependen de Vue/i18n para poder testearlos de
 * forma aislada.
 */

/** Locale mínimo que necesitamos: el idioma activo de i18n (`en`/`es`). */
export type FormatLocale = 'en' | 'es' | string

/** Placeholder único para valores no finitos (NaN, Infinity). */
const NOT_AVAILABLE = '—'

function isFiniteNumber(value: number): boolean {
  return Number.isFinite(value)
}

/**
 * Formatea un importe en USD con 2 decimales.
 * `formatCurrency(1234.5, 'en')` → `$1,234.50`.
 */
export function formatCurrency(value: number, locale: FormatLocale = 'en'): string {
  if (!isFiniteNumber(value)) return NOT_AVAILABLE
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

/** Formatea un número con separador de miles y hasta 2 decimales. */
export function formatNumber(value: number, locale: FormatLocale = 'en'): string {
  if (!isFiniteNumber(value)) return NOT_AVAILABLE
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  }).format(value)
}

/** Formatea un conteo entero (sin decimales). */
export function formatCount(value: number, locale: FormatLocale = 'en'): string {
  if (!isFiniteNumber(value)) return NOT_AVAILABLE
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(value)
}

/**
 * Formatea **puntos porcentuales** (0–100) sin decimales.
 * `formatPercent(42.4, 'en')` → `42%`.
 */
export function formatPercent(value: number, locale: FormatLocale = 'en'): string {
  if (!isFiniteNumber(value)) return NOT_AVAILABLE
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(value / 100)
}
