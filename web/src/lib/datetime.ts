/**
 * Helpers de fecha compartidos por las vistas (Issue #22).
 *
 * Se extrajeron de `JobsView.vue` al añadir el scheduler para que el formato sea
 * idéntico en ambas pantallas (y para poder testearlo aislado).
 */

/** Guion largo usado cuando la fecha no existe (mismo criterio que las tablas). */
export const EMPTY_DATE = '—'

/**
 * Formatea una fecha ISO en algo legible para el usuario.
 *
 * - `null`/`undefined`/`''` ⇒ `'—'` (el backend devuelve `null` en `*_at`).
 * - Fecha inválida ⇒ el string original (no se pierde el dato recibido).
 * - Fecha válida ⇒ `Date#toLocaleString()` en la zona del navegador.
 */
export function formatDateTime(value: unknown): string {
  if (value === null || value === undefined || value === '') return EMPTY_DATE
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}
