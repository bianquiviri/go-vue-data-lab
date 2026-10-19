<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { LogOut, Menu, Monitor, Moon, Sun, X, type LucideIcon } from 'lucide-vue-next'

import { visibleNavItems } from '@/config/nav'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

/** Sidebar colapsable en móvil; en desktop (md:) siempre visible. */
const sidebarOpen = ref(false)

const navItems = computed(() => visibleNavItems(auth.role))

const THEME_OPTIONS: { value: ThemeMode; label: string; icon: LucideIcon }[] = [
  { value: 'light', label: 'Claro', icon: Sun },
  { value: 'dark', label: 'Oscuro', icon: Moon },
  { value: 'system', label: 'Sistema', icon: Monitor },
]

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: 'Administrador', operator: 'Operador', viewer: 'Observador' }
  return auth.role ? (map[auth.role] ?? auth.role) : ''
})

function isActive(name: string): boolean {
  return route.name === name
}

async function onLogout(): Promise<void> {
  await auth.logout()
  sidebarOpen.value = false
  // Sin sesión: salir del layout protegido (el guard redirigiría igual).
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="bg-background text-foreground flex min-h-screen">
    <!-- Overlay para cerrar el sidebar en móvil -->
    <div
      v-if="sidebarOpen"
      class="bg-background/60 fixed inset-0 z-30 md:hidden"
      data-test="sidebar-overlay"
      @click="sidebarOpen = false"
    />

    <!-- Sidebar -->
    <aside
      class="bg-sidebar text-sidebar-foreground border-sidebar-border fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r transition-transform md:static md:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
      data-test="sidebar"
    >
      <div class="flex items-center justify-between p-4">
        <span class="text-base font-semibold tracking-tight">Go Data Lab</span>
        <Button variant="ghost" size="icon-sm" class="md:hidden" @click="sidebarOpen = false">
          <X />
          <span class="sr-only">Cerrar menú</span>
        </Button>
      </div>
      <Separator />
      <nav class="flex-1 overflow-y-auto p-2" aria-label="Secciones">
        <RouterLink
          v-for="item in navItems"
          :key="item.routeName"
          :to="item.to"
          class="flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors"
          :class="
            isActive(item.routeName)
              ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
              : 'hover:bg-sidebar-accent/60 text-muted-foreground'
          "
          :aria-current="isActive(item.routeName) ? 'page' : undefined"
          @click="sidebarOpen = false"
        >
          <component :is="item.icon" class="size-4 shrink-0" />
          {{ item.title }}
        </RouterLink>
      </nav>
    </aside>

    <!-- Contenido -->
    <div class="flex min-w-0 flex-1 flex-col">
      <header class="bg-background/95 supports-[backdrop-filter]:bg-background/60 sticky top-0 z-20 border-b backdrop-blur">
        <div class="flex h-14 items-center gap-3 px-4">
          <Button variant="ghost" size="icon-sm" class="md:hidden" @click="sidebarOpen = true">
            <Menu />
            <span class="sr-only">Abrir menú</span>
          </Button>

          <span class="truncate font-medium" data-test="user-email">{{ auth.user?.email }}</span>
          <Badge variant="secondary" data-test="user-role">{{ roleLabel }}</Badge>

          <div class="ml-auto flex items-center gap-1">
            <!-- Selector de tema Claro / Oscuro / Sistema -->
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

            <Separator orientation="vertical" class="mx-1 h-6" />
            <Button variant="ghost" size="sm" data-test="logout" @click="onLogout">
              <LogOut />
              Salir
            </Button>
          </div>
        </div>
      </header>

      <main class="flex-1 p-4 md:p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
