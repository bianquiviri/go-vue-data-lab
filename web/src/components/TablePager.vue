<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'

/**
 * Barra de paginación de tablas construidas a mano con las primitivas
 * `ui/table` (Issue #23).
 *
 * `DataTable` ya trae su propia paginación, pero solo sirve cuando las filas
 * exponen un `id` y no se necesitan acciones por fila indexadas. El panel de
 * admin necesita selectores **estables por índice** (los `id` son tokens
 * opacos), así que las tablas se escriben a mano y comparten este paginador.
 *
 * Es un componente controlado: no guarda estado, solo emite `update:page` /
 * `update:per-page`. Los textos son las claves `table.*` ya existentes.
 */
const props = withDefaults(
  defineProps<{
    page: number
    perPage: number
    total: number
    totalPages: number
    perPageOptions?: number[]
    /** Prefijo de los `data-test` (`<prefix>-prev`, `-next`, …). */
    testPrefix?: string
  }>(),
  { perPageOptions: () => [20, 50, 100], testPrefix: 'table-pager' },
)

const emit = defineEmits<{
  (e: 'update:page', page: number): void
  (e: 'update:perPage', perPage: number): void
}>()

const { t } = useI18n()

const canPrev = computed(() => props.page > 1)
const canNext = computed(() => props.page < props.totalPages)

function onPerPage(event: Event): void {
  const value = Number((event.target as HTMLSelectElement).value)
  if (!Number.isFinite(value) || value < 1) return
  emit('update:perPage', value)
}
</script>

<template>
  <div
    class="flex flex-col gap-2 text-sm sm:flex-row sm:items-center sm:justify-between"
    :data-test="`${props.testPrefix}-pagination`"
  >
    <div class="text-muted-foreground flex items-center gap-2">
      <span :data-test="`${props.testPrefix}-results`">{{ t('table.results', total) }}</span>
      <span v-if="totalPages > 0" :data-test="`${props.testPrefix}-page-info`">
        {{ t('table.pageOf', { page, total: totalPages }) }}
      </span>
    </div>

    <div class="flex items-center gap-3">
      <label class="text-muted-foreground flex items-center gap-2">
        <span class="sr-only sm:not-sr-only">{{ t('table.perPage') }}</span>
        <select
          class="border-input bg-background h-8 rounded-lg border px-2 text-sm"
          :value="perPage"
          :data-test="`${props.testPrefix}-per-page`"
          @change="onPerPage"
        >
          <option v-for="option in perPageOptions" :key="option" :value="option">{{ option }}</option>
        </select>
      </label>

      <div class="flex items-center gap-1">
        <Button
          variant="outline"
          size="icon-sm"
          :disabled="!canPrev"
          :data-test="`${props.testPrefix}-prev`"
          @click="emit('update:page', page - 1)"
        >
          <ChevronLeft />
          <span class="sr-only">{{ t('table.previous') }}</span>
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          :disabled="!canNext"
          :data-test="`${props.testPrefix}-next`"
          @click="emit('update:page', page + 1)"
        >
          <ChevronRight />
          <span class="sr-only">{{ t('table.next') }}</span>
        </Button>
      </div>
    </div>
  </div>
</template>
