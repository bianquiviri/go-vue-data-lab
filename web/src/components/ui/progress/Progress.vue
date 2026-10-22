<script setup lang="ts">
import { computed } from 'vue'
import { cn } from 'cn'

/**
 * Barra de progreso mínima (Issue #21) basada en divs: sin dependencias y
 * verificable en jsdom. `value` es un porcentaje 0–100.
 */
const props = withDefaults(defineProps<{ value: number; class?: string }>(), { value: 0 })

/** Porcentaje acotado a 0–100 y sin NaN (un payload corrupto no debe pintar `NaN%`). */
const percent = computed(() => {
  const value = Number(props.value)
  if (!Number.isFinite(value)) return 0
  return Math.min(Math.max(value, 0), 100)
})
</script>

<template>
  <div
    role="progressbar"
    :aria-valuenow="Math.round(percent)"
    aria-valuemin="0"
    aria-valuemax="100"
    :class="cn('bg-muted h-2 w-full overflow-hidden rounded-full', props.class)"
    data-test="progress"
  >
    <div
      class="bg-primary h-full rounded-full transition-[width] duration-300"
      :style="{ width: `${percent}%` }"
    />
  </div>
</template>
