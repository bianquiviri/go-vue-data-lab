<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'

import { Button } from '@/components/ui/button'
import AppFooter from '@/components/AppFooter.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { useAuthStore } from '@/stores/auth'

/**
 * Layout de las rutas públicas (landing y futuras). NO reutiliza AppLayout:
 * ese asume sesión (sidebar RBAC, usuario, logout, tema).
 *
 * La landing se muestra siempre en `/`, así que el acceso del header cambia
 * según la sesión: "Sign in" para visitantes, "Go to dashboard" con sesión.
 * El guard del router ya espera a `auth.initialize()` antes de montar la ruta,
 * por lo que `isAuthenticated` es estable en el primer render.
 */
const auth = useAuthStore()

const cta = computed(() =>
  auth.isAuthenticated
    ? { label: 'Go to dashboard', to: '/dashboard' }
    : { label: 'Sign in', to: '/login' },
)
</script>

<template>
  <div class="bg-background text-foreground flex min-h-screen flex-col">
    <!-- Header mínimo: marca + acceso -->
    <header class="border-b">
      <div class="mx-auto flex h-14 w-full max-w-5xl items-center justify-between px-4">
        <RouterLink to="/" class="tracking-tight font-semibold" data-test="public-brand">
          Go Data Lab
        </RouterLink>
        <div class="flex items-center gap-2">
          <ThemeToggle />
          <Button as-child size="sm" data-test="public-cta">
            <RouterLink :to="cta.to">{{ cta.label }}</RouterLink>
          </Button>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <!-- RouterView (no <slot/>): la landing es un hijo de esta ruta. -->
      <RouterView />
    </main>

    <!-- Footer compartido (licencia CC + autoría) en TODAS las vistas -->
    <AppFooter />
  </div>
</template>
