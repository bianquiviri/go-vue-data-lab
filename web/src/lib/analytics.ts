import type {
  BarDatum,
  LabeledStatusSlice,
  OrderStatusCount,
  StatusSlice,
  TopCustomer,
  TopProduct,
} from '@/types/analytics'

/**
 * Transformaciones puras de la analítica (Issue #24).
 *
 * Separan la forma que devuelve el backend de la que consumen los charts y las
 * tablas. Las etiquetas traducidas NO se calculan aquí: dependen de i18n y se
 * añaden en la vista (`LabeledStatusSlice`), para que estos helpers sean puros y
 * testables sin Vue.
 */

/**
 * Convierte `orders_by_status` en porciones para el donut.
 *
 * - Descarta filas con `count` no finito, cero o negativo (no aportan área).
 * - Ordena de mayor a menor para que la lectura del donut sea estable (el
 *   backend agrupa sin `ORDER BY`).
 * - Devuelve un array nuevo: no muta la entrada.
 */
export function toStatusSlices(rows: readonly OrderStatusCount[] | null | undefined): StatusSlice[] {
  if (!rows || rows.length === 0) return []
  return rows
    .filter((row) => Number.isFinite(row.count) && row.count > 0)
    .map((row) => ({ status: row.status, value: row.count }))
    .sort((a, b) => b.value - a.value)
}

/** Suma de las porciones (total de órdenes del desglose). */
export function statusTotal(slices: readonly StatusSlice[]): number {
  return slices.reduce((total, slice) => total + slice.value, 0)
}

/**
 * Porcentaje (0–100) de una porción sobre el total. Total 0 o valores no
 * finitos devuelven 0 para no romper la leyenda.
 */
export function statusPercent(value: number, total: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(total) || total <= 0) return 0
  return (value / total) * 100
}

/**
 * Datos para el gráfico de barras del top de productos: el eje de categorías
 * usa el nombre y el valor son los ingresos (`total_revenue`, USD).
 */
export function toBarData(products: readonly TopProduct[]): BarDatum[] {
  return products.map((product) => ({ name: product.name, value: product.total_revenue }))
}

/** Nombre completo de un cliente sin espacios sobrantes si falta un apellido. */
export function customerName(
  customer: Pick<TopCustomer, 'first_name' | 'last_name'>,
): string {
  return `${customer.first_name} ${customer.last_name}`.trim()
}

/**
 * Número de colores del tema disponibles para los charts (`--chart-1..5`,
 * definidos en `assets/main.css` para claro y oscuro — Issue #38).
 */
export const CHART_COLOR_COUNT = 5

/**
 * Color del tema para la porción/barra `index` (cíclico).
 *
 * Devuelve una **referencia a la variable CSS** (`var(--chart-3)`), no un color
 * resuelto: así el chart sigue el tema activo y alterna claro/oscuro sin
 * recalcular nada (`prefers-color-scheme`/`.dark` los define el CSS).
 * El ciclo evita quedarse sin paleta cuando hay más porciones que colores.
 */
export function chartColor(index: number): string {
  const safeIndex = Number.isFinite(index) && index >= 0 ? Math.floor(index) : 0
  return `var(--chart-${(safeIndex % CHART_COLOR_COUNT) + 1})`
}

/**
 * Extrae el dato de un payload de tooltip de Unovis.
 *
 * Según el selector, Unovis entrega el evento con `datum` (barras) o `data`
 * (donut), y en tests puede llegar el dato suelto. Devuelve `undefined` si el
 * payload no trae nada utilizable (el tooltip se renderiza vacío).
 */
function tooltipDatum<T>(payload: unknown): T | undefined {
  if (payload === null || typeof payload !== 'object') return undefined
  const record = payload as Record<string, unknown>
  const candidate = record.datum ?? record.data ?? record
  return candidate !== null && typeof candidate === 'object' ? (candidate as T) : undefined
}

/** Escapa texto para inyectarlo en el HTML del tooltip. */
function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

/**
 * HTML del tooltip de las barras de ingresos: `<nombre>: <importe>`.
 *
 * `formatValue` inyecta el formateo localizado (la lib no conoce i18n). El
 * nombre se escapa porque sale del dataset.
 */
export function revenueTooltipHtml(
  payload: unknown,
  formatValue: (value: number) => string,
): string {
  const datum = tooltipDatum<BarDatum>(payload)
  if (!datum || typeof datum.name !== 'string') return ''
  const value = typeof datum.value === 'number' ? datum.value : 0
  return `<div class="px-2 py-1 text-sm"><strong>${escapeHtml(datum.name)}</strong>: ${formatValue(value)}</div>`
}

/**
 * HTML del tooltip del donut: `<estado>: <conteo> (<porcentaje>)`.
 *
 * `formatCount`/`formatPercent` inyectan el formateo localizado y el porcentaje
 * sale de `statusPercent` sobre el total del desglose.
 */
export function statusTooltipHtml(
  payload: unknown,
  total: number,
  formatCount: (value: number) => string,
  formatPercent: (value: number) => string,
): string {
  const slice = tooltipDatum<LabeledStatusSlice>(payload)
  if (!slice || typeof slice.value !== 'number') return ''
  const label = typeof slice.label === 'string' ? slice.label : slice.status
  const percent = formatPercent(statusPercent(slice.value, total))
  return `<div class="px-2 py-1 text-sm"><strong>${escapeHtml(label)}</strong>: ${formatCount(slice.value)} (${percent})</div>`
}
