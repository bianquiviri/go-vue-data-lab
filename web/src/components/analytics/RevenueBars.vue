<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { VisAxis, VisStackedBar, VisStackedBarSelectors, VisTooltip, VisXYContainer } from '@unovis/vue'
import { Orientation } from '@unovis/ts'

import { chartColor, revenueTooltipHtml, toBarData } from '@/lib/analytics'
import { formatCurrency } from '@/lib/format'
import type { BarDatum, TopProduct } from '@/types/analytics'

/**
 * Barras de ingresos del top de productos (Issue #24).
 *
 * Se elige **barras horizontales** (`orientation: Orientation.Horizontal`) porque
 * los nombres de producto (`name`) suelen ser largos: en vertical se truncarían
 * mucho los labels del eje X. Altura fija `h-64` para mantener el layout estable
 * entre estados de carga/lleno.
 *
 * ⚠️ Unovis alinea `x`/`y` con los **ejes de pantalla** y las escalas del contenedor
 * son **numéricas** (`d3.extent`), así que:
 * - `x` = categoría (el producto), expresada como **índice** porque los nombres
 *   no son un dominio válido para la escala (con strings las coordenadas salen
 *   `NaN` y no se dibuja ninguna barra);
 * - `y` = valor (los ingresos), que con `Orientation.Horizontal` crece a lo largo
 *   del eje X de la pantalla.
 * El eje Y se formatea al nombre real con `tickValues`/`tickFormat`.
 *
 * Animaciones desactivadas (`:duration="0"`): panel que se re-renderiza al
 * cambiar el top-N y transiciones de d3 no soportadas por jsdom.
 */
const props = defineProps<{
  products: TopProduct[]
}>()

const { t, locale } = useI18n()

const data = computed(() => toBarData(props.products))

/** Posiciones del eje de categorías: una por producto (escala numérica). */
const indexes = computed(() => data.value.map((_, index) => index))

function nameAt(index: number): string {
  return data.value[Math.round(index)]?.name ?? ''
}

function currency(value: number): string {
  return formatCurrency(value, locale.value)
}

function tooltipTemplate(payload: unknown): string {
  return revenueTooltipHtml(payload, currency)
}
</script>

<template>
  <div data-test="analytics-revenue-bars">
    <div v-if="data.length === 0" class="text-muted-foreground flex h-64 items-center justify-center text-sm">
      {{ t('analytics.noResults') }}
    </div>
    <VisXYContainer v-else :data="data" :duration="0" class="h-64">
      <VisStackedBar
        :x="(_d: BarDatum, index: number) => index"
        :y="(d: BarDatum) => d.value"
        :color="() => chartColor(0)"
        :orientation="Orientation.Horizontal"
      />
      <VisAxis type="x" :tick-format="(v: number) => currency(v)" />
      <VisAxis type="y" :tick-values="indexes" :tick-format="nameAt" :grid-line="false" />
      <VisTooltip :triggers="{ [VisStackedBarSelectors.bar]: tooltipTemplate }" />
    </VisXYContainer>
  </div>
</template>
