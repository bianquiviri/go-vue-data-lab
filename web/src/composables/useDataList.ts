import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { i18n } from '@/i18n'
import type { PagedResult } from '@/types/data'

/** Valor admitido en la query string (solo strings; los números se serializan). */
type FilterValue = string | number | undefined | null

/**
 * Estado de un listado de datos reflejado en la **query string** de la URL
 * (Issue #20). Paginación (`page`, `per_page`) y filtros viven en la URL para
 * que un enlace sea compartible y sobreviva a un reload; el composable reacciona
 * a cambios externos (botones atrás/adelante del navegador).
 *
 * El llamador aporta el `fetcher` (una función de `api/data.ts` ya ligada a un
 * scope/filtros base) y declara qué claves de filtro gestionar.
 *
 * ⚠️ IDs OPACOS: los valores de filtro (p. ej. `customer_id`) son tokens que se
 * pasan verbatim a la query string y al API; nunca se parsean.
 */
export interface UseDataListOptions {
  /** Claves de la query string tratadas como filtros (no como paginación). */
  filterKeys?: string[]
  /** Tamaño de página por defecto (si la URL no lo trae). */
  defaultPerPage?: number
  /** Carga inicial automática (default true). */
  immediate?: boolean
  /** Hook de error (p. ej. logging). La UI lee `error`. */
  onError?: (error: unknown) => void
}

export type ListFetcher<T> = (params: Record<string, FilterValue>) => Promise<PagedResult<T>>

const PAGE_KEY = 'page'
const PER_PAGE_KEY = 'per_page'

/** Convierte un valor de query (string | string[]) en un string plano. */
function asString(value: unknown): string {
  if (Array.isArray(value)) return value[0] ?? ''
  return typeof value === 'string' ? value : ''
}

/** Convierte un valor de query en número entero válido, o `undefined`. */
function asInt(value: unknown, min = 1): number | undefined {
  const raw = asString(value)
  const parsed = Number.parseInt(raw, 10)
  if (!Number.isFinite(parsed) || parsed < min) return undefined
  return parsed
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  // Fallback traducido (ES/EN): el composable vive fuera de plantillas, así que
  // usa la instancia global de vue-i18n.
  return i18n.global.t('table.error')
}

export function useDataList<T>(fetcher: ListFetcher<T>, options: UseDataListOptions = {}) {
  const route = useRoute()
  const router = useRouter()
  const filterKeys = options.filterKeys ?? []
  const defaultPerPage = options.defaultPerPage ?? 20
  // Por defecto: cargar, salvo que la vista pida agenda manual (immediate=false).
  const autoLoad = options.immediate ?? true

  const result = ref<PagedResult<T> | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  /** Lee la paginación desde la query actual (con defaults saneados). */
  function readPage(): number {
    return asInt(route.query[PAGE_KEY]) ?? 1
  }

  function readPerPage(): number {
    return asInt(route.query[PER_PAGE_KEY]) ?? defaultPerPage
  }

  /** Lee los filtros declarados desde la query actual. */
  function readFilters(): Record<string, string> {
    const filters: Record<string, string> = {}
    for (const key of filterKeys) {
      const value = asString(route.query[key])
      if (value !== '') filters[key] = value
    }
    return filters
  }

  const page = ref(readPage())
  const perPage = ref(readPerPage())
  const filters = ref<Record<string, string>>(readFilters())

  /** Params finales para el fetcher: filtros + paginación (sin claves vacías). */
  const params = computed<Record<string, FilterValue>>(() => ({
    ...filters.value,
    page: page.value,
    per_page: perPage.value,
  }))

  async function refresh(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      result.value = await fetcher(params.value)
    } catch (err) {
      error.value = errorMessage(err)
      options.onError?.(err)
    } finally {
      loading.value = false
    }
  }

  /**
   * Sincroniza el estado local desde la query string. Devuelve `true` cuando
   * algo cambió (el watcher decide si recargar).
   */
  function syncFromRoute(): boolean {
    const nextPage = readPage()
    const nextPerPage = readPerPage()
    const nextFilters = readFilters()
    const changed =
      nextPage !== page.value ||
      nextPerPage !== perPage.value ||
      JSON.stringify(nextFilters) !== JSON.stringify(filters.value)
    page.value = nextPage
    perPage.value = nextPerPage
    filters.value = nextFilters
    return changed
  }

  /**
   * Reemplaza la query manteniendo solo lo declarado (paginación + filtros),
   * para no arrastrar claves ajenas al listado.
   */
  async function pushQuery(next: { page?: number; per_page?: number; filters?: Record<string, string> }): Promise<void> {
    const query: Record<string, string> = {}
    const mergedFilters = next.filters ?? filters.value
    for (const [key, value] of Object.entries(mergedFilters)) {
      if (value !== '' && value !== undefined && value !== null) query[key] = String(value)
    }
    const nextPage = next.page ?? page.value
    const nextPerPage = next.per_page ?? perPage.value
    if (nextPage > 1) query[PAGE_KEY] = String(nextPage)
    if (nextPerPage !== defaultPerPage) query[PER_PAGE_KEY] = String(nextPerPage)

    await router.replace({ query })
  }

  async function goToPage(nextPage: number): Promise<void> {
    if (nextPage < 1) return
    await pushQuery({ page: nextPage })
    await refresh()
  }

  async function setPerPage(nextPerPage: number): Promise<void> {
    if (nextPerPage < 1) return
    // Cambiar el tamaño de página reinicia a la primera (evita páginas fuera de rango).
    await pushQuery({ per_page: nextPerPage, page: 1 })
    await refresh()
  }

  async function setFilters(nextFilters: Record<string, string>): Promise<void> {
    // Cualquier cambio de filtro reinicia a la página 1.
    await pushQuery({ filters: nextFilters, page: 1 })
    await refresh()
  }

  async function resetFilters(): Promise<void> {
    await setFilters({})
  }

  // Reacciona a cambios de la query (back/forward, enlaces externos).
  watch(
    () => route.fullPath,
    async () => {
      if (syncFromRoute()) {
        await refresh()
      }
    },
  )

  if (autoLoad) {
    void refresh()
  }

  return {
    result,
    loading,
    error,
    page,
    perPage,
    filters,
    params,
    refresh,
    goToPage,
    setPerPage,
    setFilters,
    resetFilters,
  }
}
