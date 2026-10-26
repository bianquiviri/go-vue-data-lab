import { describe, expect, it } from 'vitest'

import { formatCount, formatCurrency, formatNumber, formatPercent } from '../format'

/**
 * Formateo localizado (Issue #24). La moneda del dataset es USD; el idioma lo
 * decide i18n (`locale.value`), nunca un string hardcodeado en las vistas.
 */
describe('lib/format', () => {
  it('formatCurrency usa USD y el locale pedido', () => {
    expect(formatCurrency(1234.5, 'en')).toBe('$1,234.50')
    // es: decimal ',', símbolo 'US$' (el agrupador puede variar según el entorno).
    const es = formatCurrency(1234.5, 'es')
    expect(es).toContain('234,50')
    expect(es).toContain('US$')
  })

  it('formatCurrency respeta el cero y los negativos', () => {
    expect(formatCurrency(0, 'en')).toBe('$0.00')
    expect(formatCurrency(-10, 'en')).toBe('-$10.00')
  })

  it('formatNumber agrupa miles y conserva hasta 2 decimales', () => {
    expect(formatNumber(1234567, 'en')).toBe('1,234,567')
    expect(formatNumber(1234.5, 'en')).toBe('1,234.5')
    expect(formatNumber(1234.56, 'es')).toBe('1234,56')
  })

  it('formatCount redondea a entero', () => {
    expect(formatCount(1234, 'en')).toBe('1,234')
    expect(formatCount(1234.6, 'en')).toBe('1,235')
    expect(formatCount(0, 'es')).toBe('0')
  })

  it('formatPercent recibe puntos porcentuales', () => {
    expect(formatPercent(42.4, 'en')).toBe('42%')
    expect(formatPercent(7, 'en')).toBe('7%')
    expect(formatPercent(100, 'en')).toBe('100%')
  })

  it('devuelve un guion para valores no finitos', () => {
    expect(formatCurrency(Number.NaN, 'en')).toBe('—')
    expect(formatNumber(Number.POSITIVE_INFINITY, 'en')).toBe('—')
    expect(formatCount(Number.NaN, 'en')).toBe('—')
    expect(formatPercent(Number.NEGATIVE_INFINITY, 'en')).toBe('—')
  })
})
