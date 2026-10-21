import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import DataFilters from '../DataFilters.vue'
import StatusBadge from '../StatusBadge.vue'
import { useLocale } from '@/__tests__/fixtures'

describe('components/DataFilters', () => {
  it('renderiza la búsqueda y los selects declarados', () => {
    useLocale('es')
    const wrapper = mount(DataFilters, {
      props: {
        modelValue: {},
        searchable: true,
        selects: [{ key: 'status', labelKey: 'filters.status', options: [{ value: 'paid' }] }],
        dateRange: true,
      },
    })
    expect(wrapper.find('[data-test="data-filters-search"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="data-filters-status"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="data-filters-date-from"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="data-filters-date-to"]').exists()).toBe(true)
  })

  it('emite update:modelValue al cambiar un select', async () => {
    useLocale('es')
    const wrapper = mount(DataFilters, {
      props: {
        modelValue: {},
        selects: [{ key: 'country', labelKey: 'filters.country', options: [{ value: 'GB' }] }],
      },
    })
    await wrapper.get('[data-test="data-filters-country"]').setValue('GB')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ country: 'GB' }])
  })

  it('al limpiar un select elimina la clave del diccionario', async () => {
    useLocale('es')
    const wrapper = mount(DataFilters, {
      props: {
        modelValue: { country: 'GB', search: 'ada' },
        selects: [{ key: 'country', labelKey: 'filters.country', options: [{ value: 'GB' }] }],
      },
    })
    await wrapper.get('[data-test="data-filters-country"]').setValue('')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ search: 'ada' }])
  })

  it('debouncea la búsqueda antes de emitir', async () => {
    vi.useFakeTimers()
    useLocale('es')
    const wrapper = mount(DataFilters, { props: { modelValue: {}, searchable: true } })

    await wrapper.get('[data-test="data-filters-search"]').setValue('ada')
    // Aún no ha pasado el debounce: sin emisión.
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    vi.advanceTimersByTime(400)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ search: 'ada' }])
    vi.useRealTimers()
  })

  it('el botón de limpiar aparece solo con filtros activos y los vacía todos', async () => {
    useLocale('es')
    const withFilters = mount(DataFilters, {
      props: { modelValue: { search: 'ada' } },
      attachTo: document.body,
    })
    expect(withFilters.find('[data-test="data-filters-clear"]').exists()).toBe(true)
    await withFilters.get('[data-test="data-filters-clear"]').trigger('click')
    expect(withFilters.emitted('update:modelValue')?.[0]).toEqual([{}])

    const empty = mount(DataFilters, { props: { modelValue: {} } })
    expect(empty.find('[data-test="data-filters-clear"]').exists()).toBe(false)
  })

  it('sincroniza el input de búsqueda cuando la prop cambia por fuera', async () => {
    useLocale('es')
    const wrapper = mount(DataFilters, { props: { modelValue: {}, searchable: true } })
    await wrapper.setProps({ modelValue: { search: 'grace' } })
    expect((wrapper.get('[data-test="data-filters-search"]').element as HTMLInputElement).value).toBe('grace')
  })

  it('emite los filtros de fecha', async () => {
    useLocale('es')
    const wrapper = mount(DataFilters, { props: { modelValue: {}, dateRange: true } })
    await wrapper.get('[data-test="data-filters-date-from"]').setValue('2026-01-01')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ date_from: '2026-01-01' }])
  })

  it('traduce las etiquetas al inglés', () => {
    useLocale('en')
    const wrapper = mount(DataFilters, {
      props: { modelValue: {}, selects: [{ key: 'status', labelKey: 'filters.status', options: [] }] },
    })
    expect(wrapper.text()).toContain('Status')
    expect(wrapper.text()).toContain('All')
  })
})

describe('components/StatusBadge', () => {
  it('muestra el estado del dominio y una variante visual', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'paid' } })
    expect(wrapper.text()).toBe('paid')
    expect(wrapper.get('[data-test="status-badge"]').attributes('data-variant')).toBe('default')
  })

  it('cancelled usa la variante destructiva', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'cancelled' } })
    expect(wrapper.get('[data-test="status-badge"]').attributes('data-variant')).toBe('destructive')
  })

  it('un estado desconocido cae a outline', () => {
    const wrapper = mount(StatusBadge, { props: { status: 'weird' } })
    expect(wrapper.get('[data-test="status-badge"]').attributes('data-variant')).toBe('outline')
  })
})
