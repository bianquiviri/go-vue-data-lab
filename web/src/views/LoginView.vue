<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Loader2 } from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import AppFooter from '@/components/AppFooter.vue'
import LanguageToggle from '@/components/LanguageToggle.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
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
const { t } = useI18n()

const email = ref('')
const password = ref('')
/** Se activa al intentar enviar: revela los errores de validación. */
const submitted = ref(false)

const emailError = computed(() => {
  const value = email.value.trim()
  if (!value) return t('auth.emailRequired')
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) return t('auth.emailInvalid')
  return ''
})

const passwordError = computed(() =>
  password.value ? '' : t('auth.passwordRequired'),
)

const formError = computed(() => (submitted.value ? emailError.value || passwordError.value : ''))

/** Mensaje del envelope de error del backend (o de red) que dejó el store. */
const serverError = computed(() => auth.error)

async function onSubmit(): Promise<void> {
  submitted.value = true
  if (emailError.value || passwordError.value) return

  try {
    await auth.login({ email: email.value.trim(), password: password.value })
    // Devuelve al usuario a la ruta que originó la redirección al login
    // (o al dashboard por defecto; la landing no es para usuarios con sesión).
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : { name: 'dashboard' as const }
    await router.push(redirect)
  } catch {
    // El error ya quedó expuesto en auth.error (envelope {"error":...}).
  }
}
</script>

<template>
  <div class="bg-background flex min-h-screen flex-col">
    <!-- Selectores de idioma y tema también en el login (esquina superior derecha) -->
    <header class="flex items-center justify-end gap-2 p-4">
      <LanguageToggle />
      <ThemeToggle />
    </header>

    <main class="flex flex-1 items-center justify-center p-4">
      <Card class="w-full max-w-sm">
      <CardHeader>
        <CardTitle class="text-xl">{{ t('auth.title') }}</CardTitle>
        <CardDescription>{{ t('auth.subtitle') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="grid gap-4" novalidate @submit.prevent="onSubmit">
          <div class="grid gap-2">
            <Label for="email">{{ t('auth.email') }}</Label>
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
            <Label for="password">{{ t('auth.password') }}</Label>
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
            {{ auth.loading ? t('auth.submitting') : t('auth.submit') }}
          </Button>
        </form>
      </CardContent>
    </Card>
    </main>

    <!-- Footer compartido (licencia CC + autoría) en TODAS las vistas -->
    <AppFooter />
  </div>
</template>
