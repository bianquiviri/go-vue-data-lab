<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
/** Se activa al intentar enviar: revela los errores de validación. */
const submitted = ref(false)

const emailError = computed(() => {
  const value = email.value.trim()
  if (!value) return 'El email es obligatorio'
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) return 'El formato del email no es válido'
  return ''
})

const passwordError = computed(() => (password.value ? '' : 'La contraseña es obligatoria'))

const formError = computed(() => (submitted.value ? emailError.value || passwordError.value : ''))

/** Mensaje del envelope de error del backend (o de red) que dejó el store. */
const serverError = computed(() => auth.error)

async function onSubmit(): Promise<void> {
  submitted.value = true
  if (emailError.value || passwordError.value) return

  try {
    await auth.login({ email: email.value.trim(), password: password.value })
    // Devuelve al usuario a la ruta que originó la redirección al login.
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch {
    // El error ya quedó expuesto en auth.error (envelope {"error":...}).
  }
}
</script>

<template>
  <div class="bg-background flex min-h-screen items-center justify-center p-4">
    <Card class="w-full max-w-sm">
      <CardHeader>
        <CardTitle class="text-xl">Iniciar sesión</CardTitle>
        <CardDescription>Acceso al Go Concurrent Data Processing Lab</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="grid gap-4" novalidate @submit.prevent="onSubmit">
          <div class="grid gap-2">
            <Label for="email">Email</Label>
            <Input
              id="email"
              v-model="email"
              type="email"
              name="email"
              autocomplete="username"
              placeholder="root@gmail.com"
              :aria-invalid="submitted && emailError !== ''"
              @blur="submitted = true"
            />
            <p v-if="submitted && emailError" class="text-destructive text-sm" data-test="email-error">
              {{ emailError }}
            </p>
          </div>

          <div class="grid gap-2">
            <Label for="password">Contraseña</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              name="password"
              autocomplete="current-password"
              placeholder="••••••••"
              :aria-invalid="submitted && passwordError !== ''"
              @blur="submitted = true"
            />
            <p
              v-if="submitted && passwordError"
              class="text-destructive text-sm"
              data-test="password-error"
            >
              {{ passwordError }}
            </p>
          </div>

          <p
            v-if="serverError"
            class="border-destructive/30 bg-destructive/10 text-destructive rounded-md border px-3 py-2 text-sm"
            data-test="server-error"
            role="alert"
          >
            {{ serverError }}
          </p>
          <p v-else-if="formError" class="sr-only" data-test="form-error" role="alert">
            {{ formError }}
          </p>

          <Button type="submit" class="w-full" :disabled="auth.loading">
            <Loader2 v-if="auth.loading" class="animate-spin" />
            {{ auth.loading ? 'Entrando…' : 'Entrar' }}
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
