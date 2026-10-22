<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import CustomersView from '@/views/CustomersView.vue'
import ProductsView from '@/views/ProductsView.vue'
import OrdersView from '@/views/OrdersView.vue'
import { Button } from '@/components/ui/button'

/**
 * Sección Target / Analytics (Issue #20): datos procesados de lab_target.
 *
 * Reutiliza EXACTAMENTE las mismas vistas de datos que lab_source, pasándoles
 * `scope="target"` — el contrato de lectura es idéntico, solo cambia el prefijo
 * del path (`/target/*`). La pestaña activa es estado local de UI (no afecta a
 * la URL porque cada vista mantiene sus propios filtros en la query string).
 *
 * Analítica agregada (summary/top-products/top-customers) llega en el Issue #24.
 */
const { t } = useI18n()

type Tab = 'customers' | 'products' | 'orders'

const activeTab = ref<Tab>('customers')

const TABS = computed<{ value: Tab; label: string }[]>(() => [
  { value: 'customers', label: t('data.customers.title') },
  { value: 'products', label: t('data.products.title') },
  { value: 'orders', label: t('data.orders.title') },
])
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header>
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('nav.target.title') }}</h1>
      <p class="text-muted-foreground text-sm">{{ t('data.target') }}</p>
    </header>

    <div class="flex flex-wrap gap-1" role="tablist" data-test="target-tabs">
      <Button
        v-for="tab in TABS"
        :key="tab.value"
        :variant="activeTab === tab.value ? 'secondary' : 'ghost'"
        size="sm"
        role="tab"
        :aria-selected="activeTab === tab.value"
        :data-test="`target-tab-${tab.value}`"
        @click="activeTab = tab.value"
      >
        {{ tab.label }}
      </Button>
    </div>

    <CustomersView v-if="activeTab === 'customers'" scope="target" />
    <ProductsView v-else-if="activeTab === 'products'" scope="target" />
    <OrdersView v-else scope="target" />
  </div>
</template>
