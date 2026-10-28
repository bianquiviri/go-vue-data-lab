<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { visibleNavItems } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'
import { listCustomers, listOrders, listProducts } from '@/api/data'
import { useJobEvents } from '@/composables/useJobEvents'
import { isTerminal } from '@/types/jobs'

const auth = useAuthStore()
const { t } = useI18n()
const events = useJobEvents()

/** Secciones del laboratorio (sin el dashboard) visibles para el rol actual. */
const sections = computed(() => visibleNavItems(auth.role).filter((item) => item.routeName !== 'dashboard'))

const firstName = computed(() => {
  const email = auth.user?.email ?? ''
  return email.split('@')[0] || t('dashboard.defaultUser')
})

const counts = ref<Record<string, number | null>>({
  customers: null,
  products: null,
  orders: null,
})

async function loadCounts() {
  try {
    const [customers, products, orders] = await Promise.allSettled([
      listCustomers('source', { page: 1, per_page: 1 }),
      listProducts('source', { page: 1, per_page: 1 }),
      listOrders('source', { page: 1, per_page: 1 }),
    ])

    if (customers.status === 'fulfilled') counts.value.customers = customers.value.total
    if (products.status === 'fulfilled') counts.value.products = products.value.total
    if (orders.status === 'fulfilled') counts.value.orders = orders.value.total
  } catch {
    // Silently ignore errors on dashboard
  }
}

onMounted(() => {
  events.connect()
  loadCounts()
})

watch(
  () => events.status.value?.status,
  (status) => {
    if (status && isTerminal(status)) {
      loadCounts()
    }
  },
)

function getCount(routeName: string) {
  if (routeName === 'customers') return counts.value.customers
  if (routeName === 'products') return counts.value.products
  if (routeName === 'orders') return counts.value.orders
  return null
}
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-6">
    <header>
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('dashboard.title') }}</h1>
      <p class="text-muted-foreground text-sm">
        {{ t('dashboard.greeting', { name: firstName }) }}
      </p>
    </header>

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <RouterLink v-for="item in sections" :key="item.routeName" :to="item.to" class="group">
        <Card class="h-full transition-colors group-hover:border-primary/50">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <component :is="item.icon" class="text-primary size-5" />
              <CardTitle class="text-base">{{ t(item.titleKey) }}</CardTitle>
            </div>
          </CardHeader>
          <CardContent class="space-y-1">
            <div v-if="getCount(item.routeName) !== null" class="text-2xl font-semibold">
              {{ getCount(item.routeName)?.toLocaleString() }}
            </div>
            <CardDescription>{{ t(item.descriptionKey) }}</CardDescription>
          </CardContent>
        </Card>
      </RouterLink>
    </div>
  </div>
</template>

