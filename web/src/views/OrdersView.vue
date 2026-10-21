<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Eye } from 'lucide-vue-next'

import { listOrders } from '@/api/data'
import DataFilters from '@/components/DataFilters.vue'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useDataList } from '@/composables/useDataList'
import { type DataTableColumn, type DataScope, type Order } from '@/types/data'

/**
 * Listado de orders de un esquema. Filtros `status` y rango de fechas
 * (`date_from`/`date_to`) + paginación en la query string. Cada fila navega al
 * detalle con el token opaco de la orden en el path.
 */
const props = withDefaults(defineProps<{ scope?: DataScope }>(), { scope: 'source' })

const { t } = useI18n()

const list = useDataList<Order>(
  (params) => listOrders(props.scope, params as never),
  { filterKeys: ['status', 'date_from', 'date_to', 'customer_id'] },
)

const STATUS_OPTIONS = ['pending', 'paid', 'shipped', 'delivered', 'cancelled', 'refunded'].map(
  (value) => ({ value }),
)

const columns = computed<DataTableColumn[]>(() => [
  { key: 'order_number', labelKey: 'data.orders.orderNumber' },
  { key: 'customer_id', labelKey: 'data.orders.customer', class: 'max-w-[12rem]' },
  { key: 'status', labelKey: 'data.orders.status' },
  { key: 'total_amount', labelKey: 'data.orders.total', align: 'right' },
  { key: 'currency', labelKey: 'data.orders.currency' },
  { key: 'order_date', labelKey: 'data.orders.orderDate' },
])

function onFilters(next: Record<string, string>): void {
  void list.setFilters(next)
}

/** Ruta del detalle con el token opaco intacto (no se transforma). */
function detailRoute(order: Order) {
  return {
    name: props.scope === 'target' ? 'target-order-detail' : 'order-detail',
    params: { id: order.id },
  }
}
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header>
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('data.orders.title') }}</h1>
      <p class="text-muted-foreground text-sm">
        {{ scope === 'target' ? t('data.target') : t('data.source') }}
      </p>
    </header>

    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('data.orders.title') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <DataFilters
          :model-value="list.filters.value"
          date-range
          :selects="[{ key: 'status', labelKey: 'filters.status', options: STATUS_OPTIONS }]"
          @update:model-value="onFilters"
        />
        <DataTable
          :columns="columns"
          :result="list.result.value"
          :loading="list.loading.value"
          :error="list.error.value"
          clickable
          @update:page="list.goToPage"
          @update:per-page="list.setPerPage"
          @retry="list.refresh"
        >
          <template #cell-status="{ value }">
            <StatusBadge :status="String(value)" />
          </template>
          <template #cell-customer_id="{ value }">
            <span class="font-mono text-xs">{{ value }}</span>
          </template>
          <template #cell-order_date="{ value }">
            <span class="text-muted-foreground text-xs">{{ value }}</span>
          </template>
          <template #cell-order_number="{ row, value }">
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ value }}</span>
              <Button as-child variant="link" size="sm" class="px-0">
                <RouterLink :to="detailRoute(row as Order)" data-test="order-detail-link">
                  <Eye class="size-3.5" />
                  {{ t('data.orders.viewDetail') }}
                </RouterLink>
              </Button>
            </div>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
