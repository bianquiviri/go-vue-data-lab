<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { listProducts } from '@/api/data'
import DataFilters from '@/components/DataFilters.vue'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useDataList } from '@/composables/useDataList'
import { type DataTableColumn, type DataScope, type Product } from '@/types/data'

/**
 * Listado de products de un esquema (lab_source o lab_target). Filtros `search`
 * y `category` + paginación persistidos en la query string.
 * El `status` se renderiza con `StatusBadge` mediante el slot `cell-status`.
 */
const props = withDefaults(defineProps<{ scope?: DataScope }>(), { scope: 'source' })

const { t } = useI18n()

const list = useDataList<Product>(
  (params) => listProducts(props.scope, params as never),
  { filterKeys: ['search', 'category'] },
)

const CATEGORY_OPTIONS = [
  'Automotive',
  'Beauty',
  'Books',
  'Clothing',
  'Electronics',
  'Garden',
  'Grocery',
  'Home & Kitchen',
  'Sports',
  'Toys',
].map((value) => ({ value }))

const columns = computed<DataTableColumn[]>(() => [
  { key: 'id', labelKey: 'data.rowId', class: 'max-w-[12rem]' },
  { key: 'sku', labelKey: 'data.products.sku' },
  { key: 'name', labelKey: 'data.products.name' },
  { key: 'category', labelKey: 'data.products.category' },
  { key: 'price', labelKey: 'data.products.price', align: 'right' },
  { key: 'stock', labelKey: 'data.products.stock', align: 'right' },
  { key: 'status', labelKey: 'data.products.status' },
])

function onFilters(next: Record<string, string>): void {
  void list.setFilters(next)
}
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header>
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('data.products.title') }}</h1>
      <p class="text-muted-foreground text-sm">
        {{ scope === 'target' ? t('data.target') : t('data.source') }}
      </p>
    </header>

    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('data.products.title') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <DataFilters
          :model-value="list.filters.value"
          searchable
          :selects="[{ key: 'category', labelKey: 'filters.category', options: CATEGORY_OPTIONS }]"
          @update:model-value="onFilters"
        />
        <DataTable
          :columns="columns"
          :result="list.result.value"
          :loading="list.loading.value"
          :error="list.error.value"
          @update:page="list.goToPage"
          @update:per-page="list.setPerPage"
          @retry="list.refresh"
        >
          <template #cell-status="{ value }">
            <StatusBadge :status="String(value)" />
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
