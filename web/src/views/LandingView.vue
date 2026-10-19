<script setup lang="ts">
import { RouterLink } from 'vue-router'
import {
  CalendarClock,
  Cpu,
  FileCode2,
  KeyRound,
  RefreshCcw,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-vue-next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

/**
 * Landing pública (Issue #39). Copy en inglés por decisión: el Issue #37
 * migrará el resto de la UI; esta vista se escribe en inglés desde ya.
 * Sin datos dinámicos: todo el contenido es estático.
 */
interface Feature {
  title: string
  description: string
  icon: LucideIcon
}

const FEATURES: Feature[] = [
  {
    title: 'Concurrent worker pool',
    description: 'Goroutine worker pools with panic recovery, live metrics and batched job control.',
    icon: Cpu,
  },
  {
    title: 'Cron scheduler',
    description: 'Schedules persisted in PostgreSQL and executed with robfig/cron.',
    icon: CalendarClock,
  },
  {
    title: 'Resumable ETL',
    description: 'Checkpoint-based, idempotent migration between the source and target schemas.',
    icon: RefreshCcw,
  },
  {
    title: 'Role-based access control',
    description: 'RBAC roles — admin, operator and viewer — guarding every read and write endpoint.',
    icon: ShieldCheck,
  },
  {
    title: 'REST API + Swagger',
    description: 'A documented OpenAPI surface under /api/v1 with stable envelopes.',
    icon: FileCode2,
  },
  {
    title: 'Encrypted opaque IDs',
    description: 'AES-256-GCM tokens keep internal database identifiers out of every response.',
    icon: KeyRound,
  },
]
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-16 px-4 py-16 md:py-24">
    <!-- Hero -->
    <section class="mx-auto max-w-2xl space-y-6 text-center">
      <h1
        class="text-4xl font-semibold tracking-tight sm:text-5xl"
        data-test="landing-title"
      >
        Go Data Lab
      </h1>
      <p class="text-muted-foreground text-lg">
        Go Concurrent Data Processing Lab
      </p>
      <p class="text-muted-foreground mx-auto max-w-xl">
        Concurrent data processing, massive simulation and real-time analytics over a synthetic
        e-commerce dataset — customers, products, orders and order items, from ingestion to
        insight.
      </p>
      <Button as-child size="lg" data-test="cta-login">
        <RouterLink to="/login">Sign in</RouterLink>
      </Button>
    </section>

    <!-- Features -->
    <section aria-label="Features">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card v-for="feature in FEATURES" :key="feature.title" data-test="feature">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <component :is="feature.icon" class="text-primary size-5" />
              <CardTitle class="text-base">{{ feature.title }}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <CardDescription>{{ feature.description }}</CardDescription>
          </CardContent>
        </Card>
      </div>
    </section>
  </div>
</template>
