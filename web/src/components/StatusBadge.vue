<script setup lang="ts">
import { computed } from 'vue'

import { Badge } from '@/components/ui/badge'
import type { BadgeVariants } from '@/components/ui/badge'

/**
 * Badge de estado para las tablas de datos (Issue #20). Mapea los estados de
 * order/product del generador a una variante visual del `Badge` de shadcn-vue.
 * El texto del estado NO se traduce: es un valor del dominio que devuelve el
 * backend (p. ej. `paid`, `active`), no una etiqueta de UI.
 */
const props = defineProps<{ status: string }>()

const VARIANTS: Record<string, BadgeVariants['variant']> = {
  paid: 'default',
  active: 'default',
  delivered: 'secondary',
  shipped: 'secondary',
  pending: 'outline',
  refunded: 'outline',
  inactive: 'outline',
  discontinued: 'outline',
  cancelled: 'destructive',
}

const variant = computed<BadgeVariants['variant']>(() => VARIANTS[props.status] ?? 'outline')
</script>

<template>
  <Badge :variant="variant" data-test="status-badge">{{ status }}</Badge>
</template>
