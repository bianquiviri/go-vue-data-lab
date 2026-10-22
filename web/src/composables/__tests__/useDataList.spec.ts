import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'

import { useDataList } from '../useDataList'
import { ApiError } from '@/api/client'
import { i18n } from '@/i18n'
import type { PagedResult } from '@/types/data'

interface Row {
  id: string
  name: string
}

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/items', name: 'items', component: { template: '<div />' } }],
  })
}

function makeFetcher(pages: Record<number, PagedResult<Row>> = {}) {
  return vi.fn<(params: { page?: number; per_page?: number }) => Promise<PagedResult<Row>>>(
    async (params) => {
      const page = params.page ?? 1
      return (
        pages[page] ?? {
          items: [
            { id: `v1.row-${page}-a`, name: `Row ${page}.1` },
            { id: `v1.row-${page}-b`, name: `Row ${page}.2` },
          ],
          page,
          per_page: params.per_page ?? 20,
          total: 60,
          total_pages: 3,
        }
      )
    },
  )
}

/** Monta el composable dentro de una vista mínima con el router instalado. */
async function mountComposable(
  fetcher: ReturnType<typeof makeFetcher>,
  query: Record<string, string> = {},
  options: {
    immediate?: boolean
    onError?: (e: unknown) => void
    filterKeys?: string[]
  } = {},
) {
  const router = makeRouter()
  await router.push({ path: '/items', query })
  await router.isReady()

  let api!: ReturnType<typeof useDataList<Row>>
  const Host = defineComponent({
    setup() {
      api = useDataList<Row>(fetcher, {
        immediate: options.immediate,
        onError: options.onError,
        filterKeys: options.filterKeys ?? ['search', 'country'],
      })
      return () => h('div')
    },
  })

  const wrapper = mount(Host, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router, api, fetcher }
}

describe('composables/useDataList', () => {
  it('carga la página 1 por defecto y expone items y meta', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher)

    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(api.result.value?.items).toHaveLength(2)
    expect(api.result.value?.page).toBe(1)
    expect(api.page.value).toBe(1)
    expect(api.loading.value).toBe(false)
  })

  it('hidrata page y per_page desde la query string y los usa en la request', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher, { page: '2', per_page: '50' })

    expect(api.page.value).toBe(2)
    expect(api.perPage.value).toBe(50)
    expect(fetcher).toHaveBeenCalledWith(expect.objectContaining({ page: 2, per_page: 50 }))
  })

  it('goToPage recarga y refleja la página en la URL', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher)

    await api.goToPage(3)
    await flushPromises()

    expect(router.currentRoute.value.query.page).toBe('3')
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3 }))
  })

  it('setPerPage vuelve a la página 1 y la refleja en la URL', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher, { page: '3' })

    await api.setPerPage(50)
    await flushPromises()

    expect(router.currentRoute.value.query.per_page).toBe('50')
    // La página 1 es el default implícito: se omite de la URL (URLs limpias).
    expect(router.currentRoute.value.query.page).toBeUndefined()
    expect(api.page.value).toBe(1)
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, per_page: 50 }))
  })

  it('setFilters guarda los filtros en la URL y los reenvía al fetcher', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher)

    await api.setFilters({ search: 'ada', country: 'GB' })
    await flushPromises()

    expect(router.currentRoute.value.query.search).toBe('ada')
    expect(router.currentRoute.value.query.country).toBe('GB')
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ search: 'ada', country: 'GB' }))
  })

  it('setFilters limpia claves vacías de la URL y reinicia la página', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher, { page: '2', search: 'ada' })

    await api.setFilters({ search: '' })
    await flushPromises()

    expect(router.currentRoute.value.query.search).toBeUndefined()
    expect(api.page.value).toBe(1)
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }))
  })

  it('los filtros sobreviven a un "reload" (nuevo composable, misma query)', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher)
    await api.setFilters({ search: 'grace' })
    await flushPromises()

    // Simula recarga: nueva instancia leyendo la query actual.
    const query = Object.fromEntries(
      Object.entries(router.currentRoute.value.query).map(([k, v]) => [k, String(v)]),
    )
    const reloaded = await mountComposable(makeFetcher(), query)
    expect(reloaded.api.filters.value.search).toBe('grace')
    expect(reloaded.fetcher).toHaveBeenCalledWith(expect.objectContaining({ search: 'grace' }))
  })

  it('sigue cambios externos de la query (back/forward)', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher)

    await router.push({ path: '/items', query: { page: '2', search: 'bob' } })
    await nextTick()
    await flushPromises()

    expect(api.page.value).toBe(2)
    expect(api.filters.value.search).toBe('bob')
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, search: 'bob' }))
  })

  it('expone el ApiError como string y notifica onError', async () => {
    const onError = vi.fn<(error: unknown) => void>()
    const failing = vi.fn<() => Promise<PagedResult<Row>>>(async () => {
      throw new Error('boom')
    })
    const { api } = await mountComposable(failing as never, {}, { onError })

    expect(api.error.value).toBe('boom')
    expect(onError).toHaveBeenCalled()
  })

  it('no carga si immediate=false hasta que se llame refresh', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher, {}, { immediate: false })

    expect(fetcher).not.toHaveBeenCalled()
    await api.refresh()
    await flushPromises()
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('goToPage ignora páginas menores que 1', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher)
    await api.goToPage(0)
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('setPerPage ignora tamaños inválidos', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher)
    await api.setPerPage(0)
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('resetFilters limpia todos los filtros y recarga', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher, { search: 'ada', country: 'GB' })

    await api.resetFilters()
    await flushPromises()

    expect(router.currentRoute.value.query.search).toBeUndefined()
    expect(api.filters.value).toEqual({})
  })

  it('mantiene el per_page por defecto fuera de la URL', async () => {
    const fetcher = makeFetcher()
    const { api, router } = await mountComposable(fetcher)

    await api.setPerPage(20) // igual al default ⇒ no aparece en la URL
    await flushPromises()

    expect(router.currentRoute.value.query.per_page).toBeUndefined()
  })

  it('ignora páginas inválidas de la query (no numéricas)', async () => {
    const fetcher = makeFetcher()
    const { api } = await mountComposable(fetcher, { page: 'abc', per_page: '-4' })

    expect(api.page.value).toBe(1)
    expect(api.perPage.value).toBe(20)
  })

  it('normaliza errores no-Error a un mensaje genérico', async () => {
    const failing = vi.fn<() => Promise<PagedResult<Row>>>(async () => {
      throw 'nope'
    })
    const { api } = await mountComposable(failing as never)
    expect(api.error.value).toBe(i18n.global.t('table.error'))
  })

  it('status 0 (error de red) se expone como mensaje del ApiError', async () => {
    const failing = vi.fn<() => Promise<PagedResult<Row>>>(async () => {
      throw new ApiError({ status: 0, code: 'network_error', message: 'No se pudo conectar' })
    })
    const { api } = await mountComposable(failing as never)
    expect(api.error.value).toBe('No se pudo conectar')
  })
})
