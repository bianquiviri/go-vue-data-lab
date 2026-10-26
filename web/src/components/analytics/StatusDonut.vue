<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { VisDonut, VisDonutSelectors, VisSingleContainer, VisTooltip } from '@unovis/vue'

import {
  chartColor,
  statusPercent,
  statusTooltipHtml,
  statusTotal,
  toStatusSlices,
} from '@/lib/analytics'
import { formatCount, formatPercent } from '@/lib/format'
import type { AnalyticsSummary, LabeledStatusSlice } from '@/types/analytics'

/**
 * Donut de distribución de órdenes por estado (Issue #24).
 *
 * Usa Unovis v1.7 (`VisSingleContainer` + `VisDonut`) con datos puros. Las
 * etiquetas se traducen aquí con `t('analytics.status.<status>')` y, si el
 * estado no está traducido, se muestra el valor crudo del backend: la vista
 * nunca se rompe por un estado nuevo.
 *
 * Animaciones desactivadas (`:duration="0"`): las transiciones de d3 no aportan
 * información en un panel que se re-renderiza al cambiar el top-N, y rompen en
 * jsdom (`SVGElement.transform.baseVal` no está implementado).
 */
const props = defineProps<{
  summary: AnalyticsSummary | null
}>()

const { t, locale, te } = useI18n()

const slices = computed(() => toStatusSlices(props.summary?.orders_by_status))

const total = computed(() => statusTotal(slices.value))

const labeledSlices = computed<LabeledStatusSlice[]>(() =>
  slices.value.map((slice) => {
    const key = `analytics.status.${slice.status}`
    return { ...slice, label: te(key) ? t(key) : slice.status }
  }),
)

function percentFor(value: number): string {
  return formatPercent(statusPercent(value, total.value), locale.value)
}

function countFor(value: number): string {
  return formatCount(value, locale.value)
}

function tooltipTemplate(payload: unknown): string {
  return statusTooltipHtml(
    payload,
    total.value,
    countFor,
    (value) => formatPercent(value, locale.value),
  )
}
</script>

<template>
  <div data-test="analytics-status-donut">
    <div v-if="slices.length === 0" class="text-muted-foreground flex h-64 items-center justify-center text-sm">
      {{ t('analytics.statusEmpty') }}
    </div>
    <div v-else class="space-y-3">
      <VisSingleContainer :data="labeledSlices" :duration="0" class="h-64">
        <VisDonut
          :value="(d: LabeledStatusSlice) => d.value"
          :color="(_d: LabeledStatusSlice, i: number) => chartColor(i)"
          :angle-range="[0, 2 * Math.PI]"
          :arc-width="28"
        />
        <VisTooltip :triggers="{ [VisDonutSelectors.segment]: tooltipTemplate }" />
      </VisSingleContainer>
      <ul
        class="grid gap-2 text-sm sm:grid-cols-2"
        :aria-label="t('analytics.statusLegend')"
        data-test="analytics-status-legend"
      >
        <li
          v-for="(slice, index) in labeledSlices"
          :key="slice.status"
          class="flex items-center justify-between gap-2"
          data-test="analytics-status-legend-item"
        >
          <span class="flex min-w-0 items-center gap-2">
            <span
              aria-hidden="true"
              class="size-2.5 shrink-0 rounded-full"
              :style="{ backgroundColor: chartColor(index) }"
              data-test="analytics-status-legend-swatch"
            />
            <span class="truncate">{{ slice.label }}</span>
          </span>
          <span class="text-muted-foreground tabular-nums">
            {{ countFor(slice.value) }} · {{ percentFor(slice.value) }}
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>
