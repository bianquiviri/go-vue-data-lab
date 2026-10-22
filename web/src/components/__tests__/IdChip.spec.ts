import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import IdChip from '../IdChip.vue'
import { useLocale } from '@/__tests__/fixtures'

/**
 * Chip para los IDs opacos (Issue #20, feedback de presentación).
 *
 * ⚠️ El token NUNCA se transforma: lo que se copia al portapapeles y lo que
 * viaja en el `title` es EXACTAMENTE el string que devuelve el API. Solo la
 * presentación se acorta.
 */
const TOKEN = 'v1.c_NBk9mZ3nt2Pq7rS4tU6vW8xY0aB1cD2eF3gH4iJ5kL6mN7oP8qR9sT0uV1wX2yZ'

function installClipboard() {
  const writeText = vi.fn<(text: string) => Promise<void>>().mockResolvedValue(undefined)
  Object.assign(navigator, { clipboard: { writeText } })
  return writeText
}

describe('components/IdChip', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useLocale('es')
  })

  it('muestra una versión corta y el token completo en el tooltip nativo', () => {
    const wrapper = mount(IdChip, { props: { id: TOKEN } })
    const el = wrapper.get('[data-test="id-chip"]')

    // La presentación se acorta…
    expect(el.text()).not.toContain(TOKEN)
    expect(el.text().length).toBeLessThan(TOKEN.length)
    // …pero el valor completo está en el title SIN transformar.
    expect(el.attributes('title')).toBe(TOKEN)
  })

  it('copia el token EXACTO al portapapeles y muestra el feedback', async () => {
    const writeText = installClipboard()
    const wrapper = mount(IdChip, { props: { id: TOKEN } })

    await wrapper.get('[data-test="id-chip-copy"]').trigger('click')
    await flushPromises()

    // Regresión: el valor copiado es el token exacto, nunca la versión corta.
    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith(TOKEN)
    expect(wrapper.get('[data-test="id-chip-copy"]').text()).toContain('Copiado')
  })

  it('no rompe si el navegador no soporta navigator.clipboard', async () => {
    Object.assign(navigator, { clipboard: undefined })
    const wrapper = mount(IdChip, { props: { id: TOKEN } })

    await expect(wrapper.get('[data-test="id-chip-copy"]').trigger('click')).resolves.not.toThrow()
    await flushPromises()
  })

  it('se comporta igual si writeText rechaza (permiso denegado)', async () => {
    const writeText = vi.fn<(text: string) => Promise<void>>().mockRejectedValue(new Error('denied'))
    Object.assign(navigator, { clipboard: { writeText } })
    const wrapper = mount(IdChip, { props: { id: TOKEN } })

    await wrapper.get('[data-test="id-chip-copy"]').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith(TOKEN)
  })

  it('revierte el feedback de "copiado" tras el timeout', async () => {
    vi.useFakeTimers()
    installClipboard()
    const wrapper = mount(IdChip, { props: { id: TOKEN } })

    await wrapper.get('[data-test="id-chip-copy"]').trigger('click')
    expect(wrapper.get('[data-test="id-chip-copy"]').text()).toContain('Copiado')

    vi.advanceTimersByTime(2000)
    await flushPromises()
    expect(wrapper.get('[data-test="id-chip-copy"]').text()).not.toContain('Copiado')
    vi.useRealTimers()
  })

  it('no acorta tokens más cortos que el límite', () => {
    const wrapper = mount(IdChip, { props: { id: 'v1.short', shortLength: 20 } })
    expect(wrapper.get('[data-test="id-chip"]').text()).toContain('v1.short')
  })
})
