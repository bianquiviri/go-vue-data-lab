import { computed, ref, watch } from 'vue'

import { getAnalyticsSummary, getTopCustomers, getTopProducts } from '@/api/analytics'
import { i18n } from '@/i18n'
import { DEFAULT_ANALYTICS_LIMIT, type AnalyticsLimit } from '@/types/analytics'

export interface UseAnalyticsOptions {
  initialLimit?: AnalyticsLimit
}

/** Claves i18n de error (nunca texto): el idioma puede cambiar en caliente. */
type AnalyticsErrorKey = 'analytics.error' | 'analytics.partialError'

export function useAnalytics(options: UseAnalyticsOptions = {}) {
  const limit = ref<AnalyticsLimit>(options.initialLimit ?? DEFAULT_ANALYTICS_LIMIT)
  const summary = ref<Awaited<ReturnType<typeof getAnalyticsSummary>> | null>(null)
  const products = ref<Awaited<ReturnType<typeof getTopProducts>>>([])
  const customers = ref<Awaited<ReturnType<typeof getTopCustomers>>>([])
  const loading = ref(false)
  const errorKey = ref<AnalyticsErrorKey | null>(null)
  const partial = ref(false)

  /**
   * Guardia de respuestas obsoletas: cambiar el top-N mientras hay una carga en
   * vuelo lanzaría dos peticiones y la primera podría llegar después de la
   * segunda, sobrescribiendo la tabla con datos del límite anterior. Cada
   * `load()` se queda con un número de secuencia y solo el último escribe estado.
   */
  let requestId = 0

  /** Mensaje de error ya traducido; se recalcula si cambia el idioma. */
  const error = computed<string | null>(() => (errorKey.value ? i18n.global.t(errorKey.value) : null))

  async function load(): Promise<void> {
    const id = (requestId += 1)
    loading.value = true
    errorKey.value = null
    partial.value = false

    const results = await Promise.allSettled([
      getAnalyticsSummary(),
      getTopProducts(limit.value),
      getTopCustomers(limit.value),
    ])

    // Otra carga más reciente ya tomó el control: esta respuesta se descarta.
    if (id !== requestId) return

    const [summaryResult, productsResult, customersResult] = results
    let failed = 0
    let succeeded = 0

    if (summaryResult.status === 'fulfilled') {
      summary.value = summaryResult.value
      succeeded += 1
    } else {
      summary.value = null
      failed += 1
    }

    if (productsResult.status === 'fulfilled') {
      products.value = productsResult.value
      succeeded += 1
    } else {
      products.value = []
      failed += 1
    }

    if (customersResult.status === 'fulfilled') {
      customers.value = customersResult.value
      succeeded += 1
    } else {
      customers.value = []
      failed += 1
    }

    if (failed === 0) {
      errorKey.value = null
      partial.value = false
    } else if (succeeded === 0) {
      errorKey.value = 'analytics.error'
      partial.value = false
    } else {
      errorKey.value = 'analytics.partialError'
      partial.value = true
    }

    loading.value = false
  }

  function setLimit(next: AnalyticsLimit): void {
    if (limit.value === next) return
    limit.value = next
  }

  async function reload(): Promise<void> {
    await load()
  }

  watch(
    limit,
    () => {
      void load()
    },
    { immediate: true },
  )

  const hasData = computed(
    () =>
      summary.value !== null || products.value.length > 0 || customers.value.length > 0,
  )

  return {
    limit,
    summary,
    products,
    customers,
    loading,
    error,
    errorKey,
    partial,
    hasData,
    setLimit,
    reload,
    load,
  }
}
