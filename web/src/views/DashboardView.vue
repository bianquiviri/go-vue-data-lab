<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { visibleNavItems } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { t } = useI18n()

/** Secciones del laboratorio (sin el dashboard) visibles para el rol actual. */
const sections = computed(() => visibleNavItems(auth.role).filter((item) => item.routeName !== 'dashboard'))

const firstName = computed(() => {
  const email = auth.user?.email ?? ''
  return email.split('@')[0] || t('dashboard.defaultUser')
})
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
          <CardContent>
            <CardDescription>{{ t(item.descriptionKey) }}</CardDescription>
          </CardContent>
        </Card>
      </RouterLink>
    </div>
  </div>
</template>

