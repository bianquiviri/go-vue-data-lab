<script setup lang="ts">
import { RouterLink } from 'vue-router'

import { Button } from '@/components/ui/button'

/**
 * Layout de las rutas públicas (landing y futuras). NO reutiliza AppLayout:
 * ese asume sesión (sidebar RBAC, usuario, logout, tema).
 */
</script>

<template>
  <div class="bg-background text-foreground flex min-h-screen flex-col">
    <!-- Header mínimo: marca + acceso -->
    <header class="border-b">
      <div class="mx-auto flex h-14 w-full max-w-5xl items-center justify-between px-4">
        <RouterLink to="/" class="tracking-tight font-semibold" data-test="public-brand">
          Go Data Lab
        </RouterLink>
        <Button as-child size="sm" data-test="public-signin">
          <RouterLink to="/login">Sign in</RouterLink>
        </Button>
      </div>
    </header>

    <main class="flex-1">
      <slot />
    </main>

    <footer class="border-t" data-test="public-footer">
      <div class="text-muted-foreground mx-auto w-full max-w-5xl px-4 py-6 text-sm">
        <p>Go Concurrent Data Processing Lab — synthetic e-commerce data platform.</p>
      </div>
    </footer>
  </div>
</template>
