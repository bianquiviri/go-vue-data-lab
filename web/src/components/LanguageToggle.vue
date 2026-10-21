<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Languages } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { useLocaleStore } from '@/stores/locale'
import type { AppLocale } from '@/i18n'

/**
 * Selector de idioma ES/EN compartido por TODAS las vistas (header del panel
 * autenticado, header público de la landing y login). Mismo patrón visual que
 * `ThemeToggle`: grupo de botones con `aria-pressed` y `Button` de shadcn-vue.
 */
const { t } = useI18n()
const localeStore = useLocaleStore()

const OPTIONS: { value: AppLocale; key: string }[] = [
  { value: 'es', key: 'language.es' },
  { value: 'en', key: 'language.en' },
]
</script>

<template>
  <div
    class="flex items-center gap-0.5 rounded-lg border p-0.5"
    role="group"
    :aria-label="t('language.label')"
    data-test="language-toggle"
  >
    <Languages class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
    <Button
      v-for="option in OPTIONS"
      :key="option.value"
      variant="ghost"
      size="icon-sm"
      :aria-pressed="localeStore.locale === option.value"
      :class="localeStore.locale === option.value ? 'bg-accent' : ''"
      :data-test="`locale-${option.value}`"
      @click="localeStore.setLocale(option.value)"
    >
      <span class="text-xs font-medium">{{ t(option.key) }}</span>
    </Button>
  </div>
</template>
