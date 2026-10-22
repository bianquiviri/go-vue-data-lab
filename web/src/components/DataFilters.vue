<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, X } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

/**
 * Barra de filtros de las vistas de datos (Issue #20): búsqueda con debounce
 * + selects de filtro opcionales. Es un componente `v-model`-like: recibe los
 * valores actuales y emite `update` con el diccionario de filtros completo.
 *
 * La búsqueda se debouncea (350 ms) para no disparar una request por tecla; el
 * resto de filtros se aplican al cambiar. Los valores viajan verbatim al API
 * (incluidos los tokens opacos de `customer_id`/`product_id`).
 */
interface SelectFilter {
  key: string
  labelKey: string
  options: { value: string; label?: string }[]
}

const props = withDefaults(
  defineProps<{
    /** Valor actual de los filtros (claves = query params). */
    modelValue: Record<string, string>
    /** Soporta búsqueda libre (debounced) sobre el campo `search`. */
    searchable?: boolean
    /** Filtros de tipo select. */
    selects?: SelectFilter[]
    /** Filtros de tipo fecha (date_from/date_to). */
    dateRange?: boolean
    /** Milisegundos de debounce de la búsqueda. */
    debounceMs?: number
  }>(),
  {
    searchable: false,
    selects: () => [],
    dateRange: false,
    debounceMs: 350,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', filters: Record<string, string>): void
}>()

const { t } = useI18n()

const search = ref(props.modelValue.search ?? '')
let timer: ReturnType<typeof setTimeout> | null = null

// Mantiene el input sincronizado si la query cambia por fuera (back/forward).
watch(
  () => props.modelValue.search,
  (next) => {
    if ((next ?? '') !== search.value) search.value = next ?? ''
  },
)

/** Aplica un filtro sobre una copia y emite el diccionario completo. */
function apply(key: string, value: string): void {
  const next = { ...props.modelValue }
  if (value === '') delete next[key]
  else next[key] = value
  emit('update:modelValue', next)
}

function onSearchInput(value: string | number): void {
  search.value = String(value)
  if (timer !== null) clearTimeout(timer)
  timer = setTimeout(() => {
    apply('search', search.value.trim())
  }, props.debounceMs)
}

function onSelect(key: string, event: Event): void {
  apply(key, (event.target as HTMLSelectElement).value)
}

function onDate(key: string, event: Event): void {
  apply(key, (event.target as HTMLInputElement).value)
}

const hasActiveFilters = computed(() =>
  Object.values(props.modelValue).some((value) => value !== undefined && value !== null && value !== ''),
)

function clearAll(): void {
  search.value = ''
  if (timer !== null) clearTimeout(timer)
  emit('update:modelValue', {})
}
</script>

<template>
  <div class="flex flex-wrap items-end gap-3" data-test="data-filters">
    <div v-if="searchable" class="min-w-48 flex-1">
      <label class="text-muted-foreground mb-1 block text-xs font-medium" for="data-filters-search">
        {{ t('filters.search') }}
      </label>
      <div class="relative">
        <Search class="text-muted-foreground pointer-events-none absolute top-1/2 left-2 size-4 -translate-y-1/2" />
        <Input
          id="data-filters-search"
          :model-value="search"
          :placeholder="t('filters.searchPlaceholder')"
          class="pl-8"
          data-test="data-filters-search"
          @update:model-value="onSearchInput"
        />
      </div>
    </div>

    <label
      v-for="select in selects"
      :key="select.key"
      class="flex flex-col gap-1 text-xs font-medium"
    >
      <span class="text-muted-foreground">{{ t(select.labelKey) }}</span>
      <select
        class="border-input bg-background h-8 min-w-32 rounded-lg border px-2 text-sm"
        :value="modelValue[select.key] ?? ''"
        :data-test="`data-filters-${select.key}`"
        @change="onSelect(select.key, $event)"
      >
        <option value="">{{ t('filters.all') }}</option>
        <option v-for="option in select.options" :key="option.value" :value="option.value">
          {{ option.label ?? option.value }}
        </option>
      </select>
    </label>

    <template v-if="dateRange">
      <label class="flex flex-col gap-1 text-xs font-medium">
        <span class="text-muted-foreground">{{ t('filters.dateFrom') }}</span>
        <input
          type="date"
          class="border-input bg-background h-8 rounded-lg border px-2 text-sm"
          :value="modelValue.date_from ?? ''"
          data-test="data-filters-date-from"
          @change="onDate('date_from', $event)"
        >
      </label>
      <label class="flex flex-col gap-1 text-xs font-medium">
        <span class="text-muted-foreground">{{ t('filters.dateTo') }}</span>
        <input
          type="date"
          class="border-input bg-background h-8 rounded-lg border px-2 text-sm"
          :value="modelValue.date_to ?? ''"
          data-test="data-filters-date-to"
          @change="onDate('date_to', $event)"
        >
      </label>
    </template>

    <Button
      v-if="hasActiveFilters"
      variant="ghost"
      size="sm"
      data-test="data-filters-clear"
      @click="clearAll"
    >
      <X />
      {{ t('filters.clear') }}
    </Button>
  </div>
</template>
