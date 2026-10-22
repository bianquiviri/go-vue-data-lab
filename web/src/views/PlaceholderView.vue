<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Construction } from 'lucide-vue-next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { NAV_ITEMS } from '@/config/nav'

/**
 * Vista genérica para las secciones aún no implementadas (Issues #20-#24).
 * El router la parametriza con `meta.titleKey`/`meta.descriptionKey` (ver
 * router/index.ts); el icono se resuelve por `route.name` contra NAV_ITEMS.
 * Las claves se resuelven con `t()` en el computed ⇒ el texto es reactivo al
 * cambio de idioma sin re-montar la vista.
 */
const route = useRoute()
const { t } = useI18n()

const section = computed(() => NAV_ITEMS.find((item) => item.routeName === route.name))

const titleKey = computed(() =>
  typeof route.meta.titleKey === 'string'
    ? route.meta.titleKey
    : (section.value?.titleKey ?? 'placeholder.fallbackTitle'),
)
const descriptionKey = computed(() =>
  typeof route.meta.descriptionKey === 'string'
    ? route.meta.descriptionKey
    : (section.value?.descriptionKey ?? ''),
)

const title = computed(() => t(titleKey.value))
const description = computed(() => (descriptionKey.value ? t(descriptionKey.value) : ''))
</script>

<template>
  <div class="mx-auto max-w-2xl" data-test="placeholder">
    <Card>
      <CardHeader>
        <div class="flex items-center gap-3">
          <component :is="section?.icon ?? Construction" class="text-muted-foreground size-6" />
          <div>
            <CardTitle class="flex items-center gap-2 text-xl">
              {{ title }}
              <Badge variant="outline">{{ t('placeholder.upcoming') }}</Badge>
            </CardTitle>
            <CardDescription>{{ description }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <p class="text-muted-foreground text-sm">
          {{ t('placeholder.body') }}
        </p>
      </CardContent>
    </Card>
  </div>
</template>
