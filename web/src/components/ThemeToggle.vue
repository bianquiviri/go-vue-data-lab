<script setup lang="ts">
import { Monitor, Moon, Sun, type LucideIcon } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

/**
 * Selector de tema Claro / Oscuro / Sistema compartido por TODAS las vistas
 * (header del panel autenticado, header público de la landing y login).
 * Extraído de AppLayout para no repetir el bloque en tres sitios.
 */
const theme = useThemeStore()

const THEME_OPTIONS: { value: ThemeMode; label: string; icon: LucideIcon }[] = [
  { value: 'light', label: 'Claro', icon: Sun },
  { value: 'dark', label: 'Oscuro', icon: Moon },
  { value: 'system', label: 'Sistema', icon: Monitor },
]
</script>

<template>
  <div class="flex items-center gap-0.5 rounded-lg border p-0.5" role="group" aria-label="Tema">
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
      <span class="sr-only">{{ option.label }}</span>
    </Button>
  </div>
</template>
