<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Construction } from 'lucide-vue-next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { NAV_ITEMS } from '@/config/nav'

/**
 * Vista genérica para las secciones aún no implementadas (Issues #20-#24).
 * El router la parametriza con `meta.title`/`meta.description` (ver
 * router/index.ts); el icono se resuelve por `route.name` contra NAV_ITEMS.
 */
const route = useRoute()

const section = computed(() => NAV_ITEMS.find((item) => item.routeName === route.name))

const title = computed(() => (typeof route.meta.title === 'string' ? route.meta.title : section.value?.title ?? 'Sección'))
const description = computed(
  () => (typeof route.meta.description === 'string' ? route.meta.description : section.value?.description ?? ''),
)
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <Card>
      <CardHeader>
        <div class="flex items-center gap-3">
          <component :is="section?.icon ?? Construction" class="text-muted-foreground size-6" />
          <div>
            <CardTitle class="flex items-center gap-2 text-xl">
              {{ title }}
              <Badge variant="outline">Próximamente</Badge>
            </CardTitle>
            <CardDescription>{{ description }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <p class="text-muted-foreground text-sm">
          Esta sección está planificada en los Issues #20–#24 (tablas de datos, monitoreo de
          jobs, scheduler, admin y analítica). Mientras tanto, el resto del panel ya está
          disponible.
        </p>
      </CardContent>
    </Card>
  </div>
</template>
