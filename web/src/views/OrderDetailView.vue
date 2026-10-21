<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowLeft } from 'lucide-vue-next'

import { getOrder } from '@/api/data'
import { ApiError } from '@/api/client'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { type DataTableColumn, type DataScope, type Order } from '@/types/data'

/**
 * Detalle de una orden con su tabla de `order_items` (Issue #20).
 *
 * El `id` de la ruta es el token opaco de la orden: se pasa tal cual a
 * `getOrder` sin parsear ni validar como UUID (el backend responde 400 a un UUID
 * crudo). Los tokens de los items también se muestran verbatim.
 */
const props = withDefaults(defineProps<{ scope?: DataScope }>(), { scope: 'source' })

const route = useRoute()
const { t } = useI18n()

const order = ref<Order | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

/** Token opaco de la orden, tal cual viene de la ruta. */
const orderId = computed(() => String(route.params.id ?? ''))

const itemColumns = computed<DataTableColumn[]>(() => [
  { key: 'id', labelKey: 'data.rowId', class: 'max-w-[12rem]' },
  { key: 'product_id', labelKey: 'data.orders.product', class: 'max-w-[12rem]' },
  { key: 'quantity', labelKey: 'data.orders.quantity', align: 'right' },
  { key: 'unit_price', labelKey: 'data.orders.unitPrice', align: 'right' },
  { key: 'subtotal', labelKey: 'data.orders.subtotal', align: 'right' },
])

/** `PagedResult` sintético para reutilizar el DataTable con los items del detalle. */
const itemsResult = computed(() => {
  const items = order.value?.order_items ?? []
  return {
    items,
    page: 1,
    per_page: items.length || 20,
    total: items.length,
    total_pages: items.length > 0 ? 1 : 0,
  }
})

async function load(): Promise<void> {
  if (!orderId.value) return
  loading.value = true
  error.value = null
  try {
    order.value = await getOrder(props.scope, orderId.value)
  } catch (err) {
    order.value = null
    error.value = err instanceof ApiError ? err.message : t('table.error')
  } finally {
    loading.value = false
  }
}

watch(orderId, load, { immediate: true })

/** Dirección de vuelta al listado (mismo esquema). */
const backRoute = computed(() => (props.scope === 'target' ? '/target' : '/orders'))
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-4">
    <Button as-child variant="ghost" size="sm" class="px-0">
      <RouterLink :to="backRoute" data-test="order-detail-back">
        <ArrowLeft />
        {{ t('data.orders.back') }}
      </RouterLink>
    </Button>

    <Card>
      <CardHeader class="gap-2">
        <div class="flex flex-wrap items-center gap-2">
          <CardTitle class="text-xl">
            {{ order ? t('data.orders.detailTitle', { number: order.order_number }) : t('data.orders.title') }}
          </CardTitle>
          <StatusBadge v-if="order" :status="order.status" />
        </div>
        <p v-if="order" class="text-muted-foreground font-mono text-xs">{{ order.id }}</p>
      </CardHeader>

      <CardContent class="space-y-6">
        <p v-if="loading" class="text-muted-foreground text-sm" data-test="order-detail-loading">
          {{ t('table.loading') }}
        </p>

        <p v-else-if="error" class="text-destructive text-sm" data-test="order-detail-error">
          {{ error }}
        </p>

        <template v-else-if="order">
          <dl class="grid gap-4 sm:grid-cols-3">
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('data.orders.customer') }}</dt>
              <dd class="font-mono text-xs">{{ order.customer_id }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('data.orders.total') }}</dt>
              <dd class="text-sm font-medium">{{ order.total_amount }} {{ order.currency }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('data.orders.orderDate') }}</dt>
              <dd class="text-sm">{{ order.order_date }}</dd>
            </div>
          </dl>

          <div v-if="order.shipping_address">
            <p class="text-muted-foreground mb-1 text-xs">{{ t('data.orders.shippingAddress') }}</p>
            <pre class="bg-muted/40 overflow-x-auto rounded-md p-3 text-xs">{{ JSON.stringify(order.shipping_address, null, 2) }}</pre>
          </div>

          <div class="space-y-2">
            <h2 class="flex items-center gap-2 text-sm font-medium">
              {{ t('data.orders.itemsTitle') }}
              <Badge variant="secondary">{{ order.order_items?.length ?? 0 }}</Badge>
            </h2>
            <DataTable :columns="itemColumns" :result="itemsResult">
              <template #cell-product_id="{ value }">
                <span class="font-mono text-xs">{{ value }}</span>
              </template>
            </DataTable>
          </div>
        </template>
      </CardContent>
    </Card>
  </div>
</template>
