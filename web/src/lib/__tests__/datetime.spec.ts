import { describe, expect, it } from 'vitest'

import { EMPTY_DATE, formatDateTime } from '../datetime'

describe('lib/datetime', () => {
  it('devuelve el guion cuando la fecha no existe', () => {
    expect(formatDateTime(null)).toBe(EMPTY_DATE)
    expect(formatDateTime(undefined)).toBe(EMPTY_DATE)
    expect(formatDateTime('')).toBe(EMPTY_DATE)
    expect(EMPTY_DATE).toBe('—')
  })

  it('formatea una fecha ISO válida con toLocaleString', () => {
    const iso = '2026-01-02T03:04:05Z'
    expect(formatDateTime(iso)).toBe(new Date(iso).toLocaleString())
  })

  it('conserva el string original cuando la fecha es inválida', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
  })
})
