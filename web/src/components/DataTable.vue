<script setup lang="ts" generic="T">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight, RotateCw } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableEmpty,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cellSlotName, type DataTableColumn, type PagedResult, type SortOrder, type SortState } from '@/types/data'

/**
 * Tabla de datos reutilizable (Issue #20).
 *
 * Props: definición declarativa de columnas, el `PagedResult` del API (filas +
 * meta de paginación) y los estados de carga/vacío/error. La paginación se
 * emite como eventos (`update:page`/`update:perPage`) para que la vista controle
 * la query string; el orden es **client-side sobre la página actual** porque el
 * backend ordena de forma fija (ver reporte del issue).
 *
 * Slots por columna: `cell-<key>` recibe `{ row, value }` para render custom
 * (p. ej. un badge de status).
 *
 * ⚠️ IDs OPACOS: los `id` de las filas son tokens; se renderizan como texto tal
 * cual, sin parsear ni transformar.
 */
const props = withDefaults(
  defineProps<{
    columns: DataTableColumn[]
    result?: PagedResult<T> | null
    loading?: boolean
    error?: string | null
    /** Orden activo (v-model:sort). */
    sort?: SortState | null
    /** Tamaños ofrecidos en el selector de filas por página. */
    perPageOptions?: number[]
    /** Resalta las filas como clicables (cursor pointer + emite row-click). */
    clickable?: boolean
  }>(),
  {
    result: null,
    loading: false,
    error: null,
    sort: null,
    perPageOptions: () => [20, 50, 100],
    clickable: false,
  },
)

const emit = defineEmits<{
  (e: 'update:page', page: number): void
  (e: 'update:perPage', perPage: number): void
  (e: 'update:sort', sort: SortState): void
  (e: 'retry'): void
  (e: 'row-click', row: T): void
}>()

const { t } = useI18n()

/**
 * Orden activo. Es un componente controlado (`v-model:sort`) pero mantiene un
 * espejo local para que el toggle asc→desc funcione aunque el consumidor no
 * re-alimente la prop (p. ej. si solo escucha el evento).
 */
const localSort = ref<SortState | null>(props.sort)
watch(
  () => props.sort,
  (next) => {
    localSort.value = next
  },
)

const rows = computed<T[]>(() => props.result?.items ?? [])
const page = computed(() => props.result?.page ?? 1)
const perPage = computed(() => props.result?.per_page ?? props.perPageOptions[0] ?? 20)
const totalPages = computed(() => props.result?.total_pages ?? 0)
const total = computed(() => props.result?.total ?? 0)

const isEmpty = computed(() => !props.loading && !props.error && rows.value.length === 0)
const canPrev = computed(() => page.value > 1)
const canNext = computed(() => page.value < totalPages.value)

/** El `row-click` se emite siempre; `clickable` solo afecta al cursor visual. */
const rowClickable = computed(() => props.clickable)

/** Valor tipado de una celda (clave declarativa de la columna). */
function cellValue(row: T, column: DataTableColumn): unknown {
  return (row as Record<string, unknown>)[column.key]
}

/** Filas ordenadas client-side según el orden activo (sobre la página actual). */
const sortedRows = computed<T[]>(() => {
  const sort = localSort.value
  if (!sort) return rows.value
  return [...rows.value].sort((a, b) => {
    const av = (a as Record<string, unknown>)[sort.key]
    const bv = (b as Record<string, unknown>)[sort.key]
    if (av === bv) return 0
    if (av === null || av === undefined) return 1
    if (bv === null || bv === undefined) return -1
    const cmp = typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv))
    return sort.direction === 'asc' ? cmp : -cmp
  })
})

/** Dirección que tomaría la columna al hacer clic (toggle asc → desc). */
function nextDirection(key: string): SortOrder {
  return localSort.value?.key === key && localSort.value.direction === 'asc' ? 'desc' : 'asc'
}

function onSort(column: DataTableColumn): void {
  if (column.sortable === false) return
  const next: SortState = { key: column.key, direction: nextDirection(column.key) }
  localSort.value = next
  emit('update:sort', next)
}

function sortIcon(key: string) {
  if (localSort.value?.key !== key) return ArrowUpDown
  return localSort.value.direction === 'asc' ? ArrowUp : ArrowDown
}

function alignClass(align: DataTableColumn['align']): string {
  if (align === 'right') return 'text-right'
  if (align === 'center') return 'text-center'
  return ''
}

function onPerPage(event: Event): void {
  const value = Number((event.target as HTMLSelectElement).value)
  emit('update:perPage', value)
}

/** Texto legible de una celda sin slot (número, string o vacío). */
function display(value: unknown): string {
  if (value === null || value === undefined) return '—'
  return typeof value === 'number' ? String(value) : String(value)
}
</script>

<template>
  <div class="space-y-3" data-test="data-table">
    <div class="bg-card overflow-hidden rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead
              v-for="column in columns"
              :key="column.key"
              :class="[alignClass(column.align), column.class]"
              :data-test="`data-table-head-${column.key}`"
            >
              <button
                v-if="column.sortable !== false"
                type="button"
                class="hover:text-foreground inline-flex items-center gap-1"
                :aria-label="t('table.sortBy', { column: t(column.labelKey) })"
                :data-test="`data-table-sort-${column.key}`"
                @click="onSort(column)"
              >
                {{ t(column.labelKey) }}
                <component :is="sortIcon(column.key)" class="size-3.5 shrink-0" />
              </button>
              <template v-else>{{ t(column.labelKey) }}</template>
            </TableHead>
          </TableRow>
        </TableHeader>

        <TableBody>
          <!-- Carga -->
          <TableEmpty v-if="loading" :colspan="columns.length" data-test="data-table-loading">
            <div class="text-muted-foreground flex items-center gap-2 text-sm">
              <RotateCw class="size-4 animate-spin" />
              {{ t('table.loading') }}
            </div>
          </TableEmpty>

          <!-- Error -->
          <TableEmpty v-else-if="error" :colspan="columns.length" data-test="data-table-error">
            <div class="flex flex-col items-center gap-2 text-sm">
              <span class="text-destructive">{{ error }}</span>
              <Button variant="outline" size="sm" data-test="data-table-retry" @click="emit('retry')">
                <RotateCw />
                {{ t('table.retry') }}
              </Button>
            </div>
          </TableEmpty>

          <!-- Vacío -->
          <TableEmpty v-else-if="isEmpty" :colspan="columns.length" data-test="data-table-empty">
            <span class="text-muted-foreground text-sm">{{ t('table.empty') }}</span>
          </TableEmpty>

          <!-- Filas -->
          <TableRow
            v-for="(row, index) in sortedRows"
            v-else
            :key="String((row as Record<string, unknown>).id ?? index)"
            data-test="data-table-row"
            :class="rowClickable ? 'cursor-pointer' : ''"
            @click="emit('row-click', row)"
          >
            <TableCell
              v-for="column in columns"
              :key="column.key"
              :class="alignClass(column.align)"
            >
              <slot :name="cellSlotName(column.key)" :row="row" :value="cellValue(row, column)">
                {{ display(cellValue(row, column)) }}
              </slot>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Paginación -->
    <div
      class="flex flex-col gap-2 text-sm sm:flex-row sm:items-center sm:justify-between"
      data-test="data-table-pagination"
    >
      <div class="text-muted-foreground flex items-center gap-2">
        <span data-test="data-table-results">{{ t('table.results', total) }}</span>
        <span v-if="totalPages > 0" data-test="data-table-page-info">
          {{ t('table.pageOf', { page, total: totalPages }) }}
        </span>
      </div>

      <div class="flex items-center gap-3">
        <label class="text-muted-foreground flex items-center gap-2">
          <span class="sr-only sm:not-sr-only">{{ t('table.perPage') }}</span>
          <select
            class="border-input bg-background h-8 rounded-lg border px-2 text-sm"
            :value="perPage"
            data-test="data-table-per-page"
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
            data-test="data-table-prev"
            @click="emit('update:page', page - 1)"
          >
            <ChevronLeft />
            <span class="sr-only">{{ t('table.previous') }}</span>
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            :disabled="!canNext"
            data-test="data-table-next"
            @click="emit('update:page', page + 1)"
          >
            <ChevronRight />
            <span class="sr-only">{{ t('table.next') }}</span>
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
