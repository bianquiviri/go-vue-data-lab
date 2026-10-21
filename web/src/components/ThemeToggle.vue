<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Monitor, Moon, Sun, type LucideIcon } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

/**
 * Selector de tema Claro / Oscuro / Sistema compartido por TODAS las vistas
 * (header del panel autenticado, header público de la landing y login).
 * Extraído de AppLayout para no repetir el bloque en tres sitios.
 * Las etiquetas se traducen con `t()` (reactivo al idioma activo).
 */
const { t } = useI18n()
const theme = useThemeStore()

const THEME_OPTIONS: { value: ThemeMode; labelKey: string; icon: LucideIcon }[] = [
  { value: 'light', labelKey: 'theme.light', icon: Sun },
  { value: 'dark', labelKey: 'theme.dark', icon: Moon },
  { value: 'system', labelKey: 'theme.system', icon: Monitor },
]
</script>

<template>
  <div
    class="flex items-center gap-0.5 rounded-lg border p-0.5"
    role="group"
    :aria-label="t('theme.label')"
  >
    <Button
      v-for="option in THEME_OPTIONS"
      :key="option.value"
      variant="ghost"
      size="icon-sm"
      :aria-pressed="theme.mode === option.value"
      :class="theme.mode === option.value ? 'bg-accent' : ''"
      :data-test="`theme-${option.value}`"
      @click="theme.setMode(option.value)"
    >
      <component :is="option.icon" />
      <span class="sr-only">{{ t(option.labelKey) }}</span>
    </Button>
  </div>
</template>
