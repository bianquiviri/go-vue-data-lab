<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { listCustomers } from '@/api/data'
import DataFilters from '@/components/DataFilters.vue'
import DataTable from '@/components/DataTable.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useDataList } from '@/composables/useDataList'
import type { Customer, DataTableColumn, DataScope } from '@/types/data'

/**
 * Listado de customers de un esquema (lab_source o lab_target). La misma vista
 * sirve a ambas secciones; el `scope` lo fija la ruta vía `meta.scope`.
 *
 * Filtros (`search`, `country`) y paginación viven en la query string gracias a
 * `useDataList`, de modo que la URL es compartible y sobrevive a un reload.
 *
 * ⚠️ El `id` de cada fila es un token opaco: se muestra/copia tal cual.
 */
const props = withDefaults(defineProps<{ scope?: DataScope }>(), { scope: 'source' })

const { t } = useI18n()

const list = useDataList<Customer>(
  (params) => listCustomers(props.scope, params as never),
  { filterKeys: ['search', 'country'] },
)

const COUNTRY_OPTIONS = ['AU', 'BR', 'CA', 'DE', 'ES', 'FR', 'GB', 'IN', 'JP', 'MX', 'US'].map(
  (value) => ({ value }),
)

const columns = computed<DataTableColumn[]>(() => [
  { key: 'id', labelKey: 'data.rowId', class: 'max-w-[12rem]' },
  { key: 'first_name', labelKey: 'data.customers.firstName' },
  { key: 'last_name', labelKey: 'data.customers.lastName' },
  { key: 'email', labelKey: 'data.customers.email' },
  { key: 'country_code', labelKey: 'data.customers.country' },
  { key: 'city', labelKey: 'data.customers.city' },
])

function onFilters(next: Record<string, string>): void {
  void list.setFilters(next)
}
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">{{ t('data.customers.title') }}</h1>
        <p class="text-muted-foreground text-sm">
          {{ scope === 'target' ? t('data.target') : t('data.source') }}
        </p>
      </div>
    </header>

    <Card>
      <CardHeader class="gap-3">
        <CardTitle class="text-base">{{ t('data.customers.title') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <DataFilters :model-value="list.filters.value" searchable :selects="[{ key: 'country', labelKey: 'filters.country', options: COUNTRY_OPTIONS }]" @update:model-value="onFilters" />
        <DataTable
          :columns="columns"
          :result="list.result.value"
          :loading="list.loading.value"
          :error="list.error.value"
          @update:page="list.goToPage"
          @update:per-page="list.setPerPage"
          @retry="list.refresh"
        />
      </CardContent>
    </Card>
  </div>
</template>
