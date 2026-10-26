import { describe, expect, it } from 'vitest'

import {
  chartColor,
  customerName,
  revenueTooltipHtml,
  statusPercent,
  statusTooltipHtml,
  statusTotal,
  toBarData,
  toStatusSlices,
} from '../analytics'
import type { OrderStatusCount, TopCustomer, TopProduct } from '@/types/analytics'

const STATUS_COUNTS: OrderStatusCount[] = [
  { status: 'pending', count: 4 },
  { status: 'paid', count: 10 },
  { status: 'shipped', count: 6 },
  { status: 'cancelled', count: 0 },
]

const PRODUCTS: TopProduct[] = [
  {
    id: 'v1.product-a',
    name: 'Wireless Mouse',
    sku: 'SKU-001',
    category: 'Electronics',
    total_quantity: 40,
    total_revenue: 1999.5,
  },
  {
    id: 'v1.product-b',
    name: 'Coffee Beans',
    sku: 'SKU-002',
    category: 'Grocery',
    total_quantity: 120,
    total_revenue: 540.25,
  },
]

describe('lib/analytics', () => {
  it('toStatusSlices descarta ceros/negativos y ordena de mayor a menor', () => {
    expect(toStatusSlices(STATUS_COUNTS)).toEqual([
      { status: 'paid', value: 10 },
      { status: 'shipped', value: 6 },
      { status: 'pending', value: 4 },
    ])
  })

  it('toStatusSlices tolera null/vacío y valores no finitos', () => {
    expect(toStatusSlices(null)).toEqual([])
    expect(toStatusSlices(undefined)).toEqual([])
    expect(toStatusSlices([])).toEqual([])
    expect(
      toStatusSlices([
        { status: 'paid', count: Number.NaN },
        { status: 'pending', count: -2 },
        { status: 'shipped', count: 3 },
      ]),
    ).toEqual([{ status: 'shipped', value: 3 }])
  })

  it('toStatusSlices no muta el array de entrada', () => {
    const input = [...STATUS_COUNTS]
    toStatusSlices(input)
    expect(input).toEqual(STATUS_COUNTS)
  })

  it('statusTotal suma las porciones', () => {
    expect(statusTotal(toStatusSlices(STATUS_COUNTS))).toBe(20)
    expect(statusTotal([])).toBe(0)
  })

  it('statusPercent calcula el porcentaje y evita dividir por cero', () => {
    expect(statusPercent(10, 20)).toBe(50)
    expect(statusPercent(1, 3)).toBeCloseTo(33.333, 3)
    expect(statusPercent(4, 0)).toBe(0)
    expect(statusPercent(Number.NaN, 10)).toBe(0)
  })

  it('toBarData mapea nombre e ingresos para las barras', () => {
    expect(toBarData(PRODUCTS)).toEqual([
      { name: 'Wireless Mouse', value: 1999.5 },
      { name: 'Coffee Beans', value: 540.25 },
    ])
    expect(toBarData([])).toEqual([])
  })

  it('customerName concatena nombre y apellido sin espacios sobrantes', () => {
    const base: TopCustomer = {
      id: 'v1.customer-a',
      first_name: 'Ada',
      last_name: 'Lovelace',
      email: 'ada@example.com',
      total_orders: 3,
      total_spent: 120,
    }
    expect(customerName(base)).toBe('Ada Lovelace')
    expect(customerName({ ...base, last_name: '' })).toBe('Ada')
    expect(customerName({ ...base, first_name: '' })).toBe('Lovelace')
  })

  it('chartColor cicla por las 5 variables de tema y normaliza índices raros', () => {
    expect(chartColor(0)).toBe('var(--chart-1)')
    expect(chartColor(3)).toBe('var(--chart-4)')
    expect(chartColor(5)).toBe('var(--chart-1)')
    expect(chartColor(7)).toBe('var(--chart-3)')
    // Índices no finitos/negativos caen al primer color en vez de generar CSS inválido
    expect(chartColor(-2)).toBe('var(--chart-1)')
    expect(chartColor(Number.NaN)).toBe('var(--chart-1)')
    expect(chartColor(Number.POSITIVE_INFINITY)).toBe('var(--chart-1)')
    // Decimales se truncan (nunca `var(--chart-1.5)`)
    expect(chartColor(2.9)).toBe('var(--chart-3)')
  })

  it('revenueTooltipHtml acepta datum/data/dato suelto y escapa el nombre', () => {
    const format = (value: number): string => `$${value.toFixed(2)}`
    const datum = { name: 'Coffee Beans', value: 540.25 }

    expect(revenueTooltipHtml({ datum }, format)).toContain('Coffee Beans')
    expect(revenueTooltipHtml({ datum }, format)).toContain('$540.25')
    expect(revenueTooltipHtml({ data: datum }, format)).toContain('$540.25')
    expect(revenueTooltipHtml(datum, format)).toContain('$540.25')

    // El nombre sale del dataset: se escapa antes de inyectarlo como HTML
    expect(revenueTooltipHtml({ datum: { name: '<img src=x>', value: 1 } }, format)).toContain(
      '&lt;img src=x&gt;',
    )

    // Payloads inservibles ⇒ cadena vacía (el tooltip no se pinta)
    expect(revenueTooltipHtml(null, format)).toBe('')
    expect(revenueTooltipHtml('nope', format)).toBe('')
    expect(revenueTooltipHtml({ datum: { value: 3 } }, format)).toBe('')
  })

  it('statusTooltipHtml pinta estado, conteo y porcentaje del total', () => {
    const slice = { status: 'paid', value: 10, label: 'Pagado' }
    const html = statusTooltipHtml({ data: slice }, 20, String, (v) => `${Math.round(v)}%`)

    expect(html).toContain('Pagado')
    expect(html).toContain(': 10 (50%)')

    // Sin etiqueta traducida cae al estado crudo del backend
    expect(statusTooltipHtml({ data: { status: 'unknown', value: 1 } }, 4, String, String)).toContain(
      'unknown',
    )

    // Total 0 no divide por cero
    expect(statusTooltipHtml({ data: slice }, 0, String, (v) => `${v}%`)).toContain('0%')

    expect(statusTooltipHtml(undefined, 10, String, String)).toBe('')
    expect(statusTooltipHtml({ data: { status: 'paid' } }, 10, String, String)).toBe('')
  })
})
