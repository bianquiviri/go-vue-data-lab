<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RefreshCw } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import IdChip from '@/components/IdChip.vue'
import StatusDonut from '@/components/analytics/StatusDonut.vue'
import RevenueBars from '@/components/analytics/RevenueBars.vue'
import { useAnalytics } from '@/composables/useAnalytics'
import { formatCount, formatCurrency } from '@/lib/format'
import { customerName } from '@/lib/analytics'
import { ANALYTICS_LIMITS } from '@/types/analytics'

const { t, locale } = useI18n()
const {
  limit,
  summary,
  products,
  customers,
  loading,
  error,
  partial,
  hasData,
  setLimit,
  reload,
} = useAnalytics()

function onLimitChange(event: Event): void {
  const value = Number((event.target as HTMLSelectElement).value)
  if (ANALYTICS_LIMITS.includes(value as (typeof ANALYTICS_LIMITS)[number])) {
    setLimit(value as (typeof ANALYTICS_LIMITS)[number])
  }
}

function currency(value: number): string {
  return formatCurrency(value, locale.value)
}

function count(value: number): string {
  return formatCount(value, locale.value)
}
</script>

<template>
  <section class="space-y-4" data-test="analytics-view">
    <header class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold tracking-tight">{{ t('analytics.title') }}</h2>
        <p class="text-muted-foreground text-sm">{{ t('analytics.subtitle') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <label class="text-muted-foreground flex items-center gap-2 text-sm">
          <span>{{ t('analytics.topN') }}</span>
          <select
            class="border-input bg-background h-9 rounded-md border px-2 text-sm"
            :value="limit"
            data-test="analytics-limit"
            @change="onLimitChange"
          >
            <option v-for="option in ANALYTICS_LIMITS" :key="option" :value="option">{{ option }}</option>
          </select>
        </label>
        <Button variant="outline" size="sm" data-test="analytics-refresh" @click="reload">
          <RefreshCw class="size-4" />
          {{ t('analytics.refresh') }}
        </Button>
      </div>
    </header>

    <p v-if="partial && !loading" class="text-muted-foreground text-sm" aria-live="polite" data-test="analytics-partial">
      {{ error }}
    </p>

    <div v-if="loading && !hasData" class="text-muted-foreground text-sm" aria-live="polite" data-test="analytics-loading">
      {{ t('analytics.loading') }}
    </div>

    <div v-else-if="error && !hasData" class="flex flex-col items-start gap-2" data-test="analytics-error">
      <p class="text-destructive text-sm">{{ error }}</p>
      <Button variant="outline" size="sm" @click="reload">
        <RefreshCw class="size-4" />
        {{ t('analytics.retry') }}
      </Button>
    </div>

    <template v-else>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <Card data-test="analytics-card-customers">
          <CardHeader class="pb-2">
            <CardTitle class="text-muted-foreground text-sm font-medium">{{ t('analytics.customers') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-semibold">{{ summary ? count(summary.total_customers) : '—' }}</div>
          </CardContent>
        </Card>
        <Card data-test="analytics-card-products">
          <CardHeader class="pb-2">
            <CardTitle class="text-muted-foreground text-sm font-medium">{{ t('analytics.products') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-semibold">{{ summary ? count(summary.total_products) : '—' }}</div>
          </CardContent>
        </Card>
        <Card data-test="analytics-card-orders">
          <CardHeader class="pb-2">
            <CardTitle class="text-muted-foreground text-sm font-medium">{{ t('analytics.orders') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-semibold">{{ summary ? count(summary.total_orders) : '—' }}</div>
          </CardContent>
        </Card>
        <Card data-test="analytics-card-revenue">
          <CardHeader class="pb-2">
            <CardTitle class="text-muted-foreground text-sm font-medium">{{ t('analytics.revenue') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-semibold">{{ summary ? currency(summary.total_revenue) : '—' }}</div>
          </CardContent>
        </Card>
        <Card data-test="analytics-card-aov">
          <CardHeader class="pb-2">
            <CardTitle class="text-muted-foreground text-sm font-medium">{{ t('analytics.averageOrderValue') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-semibold">{{ summary ? currency(summary.average_order_value) : '—' }}</div>
          </CardContent>
        </Card>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{{ t('analytics.ordersByStatus') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <StatusDonut :summary="summary" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{{ t('analytics.topProducts') }}</CardTitle>
          </CardHeader>
          <CardContent class="space-y-4">
            <RevenueBars :products="products" />
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b">
                    <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('analytics.productName') }}</th>
                    <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('analytics.productSku') }}</th>
                    <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('analytics.productCategory') }}</th>
                    <th class="text-muted-foreground px-2 py-2 text-right font-medium">{{ t('analytics.quantitySold') }}</th>
                    <th class="text-muted-foreground px-2 py-2 text-right font-medium">{{ t('analytics.revenueAmount') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="products.length === 0">
                    <td colspan="5" class="text-muted-foreground px-2 py-4 text-center">{{ t('analytics.noResults') }}</td>
                  </tr>
                  <tr v-for="product in products" :key="product.id" class="border-b last:border-0">
                    <td class="px-2 py-2">{{ product.name }}</td>
                    <td class="px-2 py-2">{{ product.sku }}</td>
                    <td class="px-2 py-2">{{ product.category }}</td>
                    <td class="px-2 py-2 text-right tabular-nums">{{ count(product.total_quantity) }}</td>
                    <td class="px-2 py-2 text-right tabular-nums">{{ currency(product.total_revenue) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('analytics.topCustomers') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b">
                  <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('data.rowId') }}</th>
                  <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('analytics.customer') }}</th>
                  <th class="text-muted-foreground px-2 py-2 text-left font-medium">{{ t('analytics.customerEmail') }}</th>
                  <th class="text-muted-foreground px-2 py-2 text-right font-medium">{{ t('analytics.totalOrders') }}</th>
                  <th class="text-muted-foreground px-2 py-2 text-right font-medium">{{ t('analytics.totalSpent') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="customers.length === 0">
                  <td colspan="5" class="text-muted-foreground px-2 py-4 text-center">{{ t('analytics.noResults') }}</td>
                </tr>
                <tr v-for="customer in customers" :key="customer.id" class="border-b last:border-0">
                  <td class="px-2 py-2"><IdChip :id="customer.id" /></td>
                  <td class="px-2 py-2">{{ customerName(customer) }}</td>
                  <td class="px-2 py-2">{{ customer.email }}</td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ count(customer.total_orders) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ currency(customer.total_spent) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </template>
  </section>
</template>
