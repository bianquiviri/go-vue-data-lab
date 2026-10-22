<script setup lang="ts">
import { computed } from 'vue'

/**
 * Celda de una sola línea con truncado por elipsis y el valor completo en un
 * tooltip nativo (`title`) — feedback de presentación del Issue #20.
 *
 * Motivo del `title` nativo frente al `Tooltip` de shadcn-vue: no añade
 * dependencias (Reka UI ya está, pero el Tooltip requiere portal, focus
 * management y no se renderiza en el DOM del propio nodo), funciona sin JS
 * adicional y es verificable en jsdom. Con `truncate` (overflow hidden +
 * text-overflow ellipsis + white-space nowrap) la fila queda en una línea.
 */
const props = defineProps<{
  value: unknown
  /** Clases extra (p. ej. un ancho máximo para la columna). */
  class?: string
}>()

const hasValue = computed(() => props.value !== null && props.value !== undefined)

const text = computed(() => (hasValue.value ? String(props.value) : '—'))

/** Sin valor real no tiene sentido un tooltip. */
const title = computed(() => (hasValue.value ? text.value : undefined))
</script>

<template>
  <span :class="['block truncate', props.class]" :title="title" data-test="truncated-cell">
    {{ text }}
  </span>
</template>
