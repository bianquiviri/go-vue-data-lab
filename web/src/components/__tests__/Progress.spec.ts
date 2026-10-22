import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import { Progress } from '@/components/ui/progress'

/**
 * Barra de progreso (Issue #21): expone el valor a los lectores de pantalla
 * (`role="progressbar"` + `aria-valuenow`) y acota el porcentaje a 0–100.
 */
describe('components/Progress', () => {
  it('expone el valor como progressbar', () => {
    const wrapper = mount(Progress, { props: { value: 42.4 } })
    const el = wrapper.get('[data-test="progress"]')

    expect(el.attributes('role')).toBe('progressbar')
    expect(el.attributes('aria-valuenow')).toBe('42')
    expect(el.attributes('aria-valuemin')).toBe('0')
    expect(el.attributes('aria-valuemax')).toBe('100')
  })

  it('acota el ancho de la barra al rango 0–100', () => {
    const over = mount(Progress, { props: { value: 140 } })
    expect(over.get('[data-test="progress"] > div').attributes('style')).toContain('width: 100%')

    const under = mount(Progress, { props: { value: -20 } })
    expect(under.get('[data-test="progress"] > div').attributes('style')).toContain('width: 0%')
  })

  it('tolera un valor no numérico (no genera NaN en el DOM)', () => {
    const wrapper = mount(Progress, { props: { value: Number.NaN } })
    const el = wrapper.get('[data-test="progress"]')

    expect(el.attributes('aria-valuenow')).toBe('0')
    expect(el.get('div').attributes('style')).toContain('width: 0%')
  })
})
