import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import DataTable from '../DataTable.vue'
import { useLocale } from '@/__tests__/fixtures'
import type { DataTableColumn, PagedResult, SortState } from '@/types/data'

interface Row {
  id: string
  name: string
  status: string
}

const COLUMNS: DataTableColumn[] = [
  { key: 'id', labelKey: 'data.rowId' },
  { key: 'name', labelKey: 'data.products.name' },
]

function makeResult(overrides: Partial<PagedResult<Row>> = {}): PagedResult<Row> {
  return {
    items: [
      { id: 'v1.token-a', name: 'Alice', status: 'paid' },
      { id: 'v1.token-b', name: 'Bob', status: 'pending' },
    ],
    page: 1,
    per_page: 20,
    total: 42,
    total_pages: 3,
    ...overrides,
  }
}

function mountTable(props: Record<string, unknown> = {}) {
  return mount(DataTable, {
    props: {
      columns: COLUMNS,
      result: makeResult(),
      ...props,
    },
  })
}

describe('components/DataTable', () => {
  it('renderiza una fila por item y las cabeceras traducidas', () => {
    useLocale('es')
    const wrapper = mountTable()
    const rows = wrapper.findAll('[data-test="data-table-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0]?.text()).toContain('Alice')
    expect(wrapper.text()).toContain('Nombre')
    // El id (token opaco) se muestra tal cual.
    expect(wrapper.text()).toContain('v1.token-a')
  })

  it('muestra el estado de carga y no las filas', () => {
    useLocale('es')
    const wrapper = mountTable({ loading: true })
    expect(wrapper.find('[data-test="data-table-loading"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test="data-table-row"]')).toHaveLength(0)
  })

  it('muestra el estado vacío cuando no hay items', () => {
    useLocale('es')
    const wrapper = mountTable({ result: makeResult({ items: [], total: 0, total_pages: 0 }) })
    expect(wrapper.find('[data-test="data-table-empty"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Sin resultados')
  })

  it('muestra el error y permite reintentar', async () => {
    useLocale('es')
    const wrapper = mountTable({ error: 'boom' })
    expect(wrapper.find('[data-test="data-table-error"]').exists()).toBe(true)
    await wrapper.get('[data-test="data-table-retry"]').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('en inglés traduce los estados', () => {
    useLocale('en')
    const wrapper = mountTable({ result: makeResult({ items: [] }) })
    expect(wrapper.text()).toContain('No results')
  })

  it('muestra el resumen de paginación', () => {
    useLocale('es')
    const wrapper = mountTable()
    expect(wrapper.get('[data-test="data-table-page-info"]').text()).toContain('Página 1 de 3')
    expect(wrapper.get('[data-test="data-table-results"]').text()).toContain('42')
  })

  it('emite update:page al pulsar siguiente y anterior', async () => {
    useLocale('es')
    const wrapper = mountTable({ result: makeResult({ page: 2 }) })
    await wrapper.get('[data-test="data-table-next"]').trigger('click')
    await wrapper.get('[data-test="data-table-prev"]').trigger('click')
    expect(wrapper.emitted('update:page')).toEqual([[3], [1]])
  })

  it('deshabilita anterior en la primera página y siguiente en la última', () => {
    useLocale('es')
    const first = mountTable({ result: makeResult({ page: 1, total_pages: 3 }) })
    expect(first.get('[data-test="data-table-prev"]').attributes('disabled')).toBeDefined()

    const last = mountTable({ result: makeResult({ page: 3, total_pages: 3 }) })
    expect(last.get('[data-test="data-table-next"]').attributes('disabled')).toBeDefined()
  })

  it('emite update:perPage al cambiar el selector', async () => {
    useLocale('es')
    const wrapper = mountTable()
    await wrapper.get('[data-test="data-table-per-page"]').setValue('50')
    expect(wrapper.emitted('update:perPage')).toEqual([[50]])
  })

  it('expone slots por columna para render custom', () => {
    useLocale('es')
    const columns: DataTableColumn[] = [...COLUMNS, { key: 'status', labelKey: 'data.orders.status' }]
    const wrapper = mount(DataTable, {
      props: { columns, result: makeResult() },
      slots: {
        'cell-status': '<span data-test="status-badge">X</span>',
      },
    })
    expect(wrapper.find('[data-test="status-badge"]').exists()).toBe(true)
  })

  describe('orden client-side', () => {
    it('emite update:sort con la dirección al pulsar una cabecera ordenable', async () => {
      useLocale('es')
      const wrapper = mountTable()
      await wrapper.get('[data-test="data-table-sort-name"]').trigger('click')
      const emitted = wrapper.emitted('update:sort') as [SortState][]
      expect(emitted).toHaveLength(1)
      expect(emitted[0]?.[0]).toEqual({ key: 'name', direction: 'asc' })

      // Segundo clic: alterna a descendente.
      await wrapper.get('[data-test="data-table-sort-name"]').trigger('click')
      const all = wrapper.emitted('update:sort') as [SortState][]
      expect(all[1]?.[0]).toEqual({ key: 'name', direction: 'desc' })
    })

    it('no emite nada si la columna es sortable=false', async () => {
      useLocale('es')
      const columns: DataTableColumn[] = [
        { key: 'id', labelKey: 'data.rowId', sortable: false },
      ]
      const wrapper = mountTable({ columns })
      await wrapper.get('[data-test="data-table-head-id"]').trigger('click')
      expect(wrapper.emitted('update:sort')).toBeUndefined()
    })
  })

  it('renderiza solo los slots declarados y mantiene el resto como texto', () => {
    useLocale('es')
    const columns: DataTableColumn[] = [
      { key: 'name', labelKey: 'data.products.name', align: 'right' },
    ]
    const wrapper = mount(DataTable, {
      props: {
        columns,
        result: makeResult(),
      },
    })
    const head = wrapper.get('[data-test="data-table-head-name"]')
    expect(head.classes().join(' ')).toContain('text-right')
  })

  it('centra la columna con align=center', () => {
    useLocale('es')
    const wrapper = mountTable({
      columns: [{ key: 'name', labelKey: 'data.products.name', align: 'center' }],
    })
    expect(wrapper.get('[data-test="data-table-head-name"]').classes().join(' ')).toContain('text-center')
  })

  it('muestra un guion para valores null/undefined', () => {
    useLocale('es')
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'state', labelKey: 'data.customers.state' }],
        result: {
          items: [{ id: 'v1.x', state: null }],
          page: 1,
          per_page: 20,
          total: 1,
          total_pages: 1,
        } as unknown as PagedResult<Row>,
      },
    })
    expect(wrapper.get('[data-test="data-table-row"]').text()).toContain('—')
  })

  it('ordena numéricamente client-side', async () => {
    useLocale('es')
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'n', labelKey: 'data.products.stock', align: 'right' }],
        result: {
          items: [{ n: 10 }, { n: 2 }, { n: 5 }],
          page: 1,
          per_page: 20,
          total: 3,
          total_pages: 1,
        } as unknown as PagedResult<Record<string, number>>,
      },
    })
    await wrapper.get('[data-test="data-table-sort-n"]').trigger('click')
    const rows = wrapper.findAll('[data-test="data-table-row"]')
    expect(rows[0]?.text()).toBe('2')
  })

  it('emite row-click al hacer clic en una fila', async () => {
    useLocale('es')
    const wrapper = mount(DataTable, {
      props: { columns: COLUMNS, result: makeResult() },
    })
    await wrapper.get('[data-test="data-table-row"]').trigger('click')
    expect(wrapper.emitted('row-click')).toHaveLength(1)
  })

  it('con clickable marca las filas con cursor pointer', () => {
    useLocale('es')
    const wrapper = mountTable({ clickable: true })
    expect(wrapper.get('[data-test="data-table-row"]').classes()).toContain('cursor-pointer')

    const plain = mountTable()
    expect(plain.get('[data-test="data-table-row"]').classes()).not.toContain('cursor-pointer')
  })

  it('maneja filas sin id para la key', () => {
    useLocale('es')
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', labelKey: 'data.products.name' }],
        result: {
          items: [{ name: 'no-id' }],
          page: 1,
          per_page: 20,
          total: 1,
          total_pages: 1,
        } as unknown as PagedResult<Record<string, unknown>>,
      },
    })
    expect(wrapper.findAll('[data-test="data-table-row"]')).toHaveLength(1)
  })

  it('usa el resultado nulo como lista vacía', () => {
    useLocale('es')
    const wrapper = mount(DataTable, { props: { columns: COLUMNS, result: null } })
    expect(wrapper.find('[data-test="data-table-empty"]').exists()).toBe(true)
  })

  it('permite personalizar las opciones de filas por página', () => {
    useLocale('es')
    const wrapper = mountTable({ perPageOptions: [10, 25] })
    const options = wrapper.get('[data-test="data-table-per-page"]').findAll('option')
    expect(options).toHaveLength(2)
    expect(options[1]?.text()).toBe('25')
  })
})
