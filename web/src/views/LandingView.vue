<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  CalendarClock,
  Check,
  Cpu,
  Database,
  FileCode2,
  KeyRound,
  LineChart,
  RefreshCcw,
  ShieldCheck,
  Workflow,
  type LucideIcon,
} from 'lucide-vue-next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuthStore } from '@/stores/auth'

/**
 * Landing pública (Issue #39). Copy en inglés por decisión: el Issue #37
 * migrará el resto de la UI; esta vista se escribe en inglés desde ya.
 * Todo el contenido es estático salvo el CTA, que depende de si hay sesión.
 *
 * La vista se renderiza SIEMPRE en `/` (con o sin sesión): el guard del router
 * ya espera a `auth.initialize()` antes de montarla, así que `isAuthenticated`
 * es estable en el primer render y no hay parpadeo.
 */
const auth = useAuthStore()

const cta = computed(() =>
  auth.isAuthenticated
    ? { label: 'Go to dashboard', to: '/dashboard' }
    : { label: 'Sign in', to: '/login' },
)

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

interface PipelineStep {
  title: string
  /** Esquema (o concepto) al que corresponde el paso, renderizado como chip mono. */
  schema: string
  detail: string
  icon: LucideIcon
}

/** Los tres tramos del pipeline: ingesta → transformación → analítica. */
const PIPELINE: PipelineStep[] = [
  {
    title: 'Ingest',
    schema: 'lab_source',
    detail: 'Raw schema. Generated records land here untouched — the ground truth for every run.',
    icon: Database,
  },
  {
    title: 'Transform',
    schema: 'resumable ETL',
    detail:
      'Checkpoint-based, idempotent migration in batches with panic recovery. Interrupted runs resume from their last checkpoint.',
    icon: Workflow,
  },
  {
    title: 'Serve',
    schema: 'lab_target',
    detail: 'Processed records plus the aggregates that feed real-time analytics.',
    icon: LineChart,
  },
]

interface Capability {
  title: string
  detail: string
}

const CAPABILITIES: Capability[] = [
  {
    title: 'Run massive simulations',
    detail:
      'Worker pools process batches of 3,000–4,000 records concurrently and report progress as they go.',
  },
  {
    title: 'Schedule recurring work',
    detail: 'Cron definitions are persisted in PostgreSQL and executed by robfig/cron.',
  },
  {
    title: 'Explore the results',
    detail:
      'A REST API under /api/v1, documented in Swagger and guarded by RBAC roles (admin, operator, viewer).',
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
      <Button as-child size="lg" data-test="landing-cta">
        <RouterLink :to="cta.to">{{ cta.label }}</RouterLink>
      </Button>
    </section>

    <!-- What is this? Propósito, pipeline y usos (Issue #39) -->
    <section class="space-y-10" aria-labelledby="what-is-this-title" data-test="what-is-this">
      <div class="mx-auto max-w-2xl space-y-4 text-center">
        <p class="text-muted-foreground text-xs font-medium tracking-widest uppercase">
          Overview
        </p>
        <h2
          id="what-is-this-title"
          class="text-2xl font-semibold tracking-tight sm:text-3xl"
          data-test="what-is-this-title"
        >
          What is this?
        </h2>
        <p class="text-muted-foreground">
          Go Data Lab is a laboratory for exploring concurrent data processing in Go over a
          synthetic e-commerce model — customers, products, orders and order items. Every dataset
          is generated on demand inside a real PostgreSQL database, at whatever volume the
          experiment requires.
        </p>
        <p class="text-muted-foreground">
          Its purpose is to simulate massive volumes while keeping the whole pipeline visible,
          from ingestion to analytics, so concurrency strategies, batching and resumability can be
          measured end to end on data you control.
        </p>
      </div>

      <!-- Pipeline: lab_source → ETL con checkpoints → lab_target -->
      <div class="space-y-4">
        <div class="space-y-1 text-center">
          <h3 class="text-lg font-medium tracking-tight">How the pipeline works</h3>
          <p class="text-muted-foreground text-sm">
            Two independent PostgreSQL schemas, so a failed run never touches the source data.
          </p>
        </div>
        <ol class="grid gap-4 md:grid-cols-3" data-test="pipeline-step-list">
          <li v-for="(step, index) in PIPELINE" :key="step.title">
            <Card class="h-full" data-test="pipeline-step">
              <CardHeader class="pb-2">
                <div class="flex items-center gap-2">
                  <Badge variant="secondary" class="w-5 justify-center px-0 tabular-nums">
                    {{ index + 1 }}
                  </Badge>
                  <component :is="step.icon" class="text-primary size-4" aria-hidden="true" />
                  <CardTitle class="text-base">{{ step.title }}</CardTitle>
                </div>
              </CardHeader>
              <CardContent class="space-y-2">
                <code
                  class="bg-muted text-foreground inline-block rounded px-1.5 py-0.5 font-mono text-xs"
                >
                  {{ step.schema }}
                </code>
                <CardDescription>{{ step.detail }}</CardDescription>
              </CardContent>
            </Card>
          </li>
        </ol>
      </div>

      <!-- Usos -->
      <Card data-test="capabilities">
        <CardHeader>
          <CardTitle class="text-lg">What can you do with it?</CardTitle>
          <CardDescription>Three entry points into the lab.</CardDescription>
        </CardHeader>
        <CardContent>
          <ul class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <li v-for="capability in CAPABILITIES" :key="capability.title" class="space-y-1.5">
              <p class="flex items-center gap-2 text-sm font-medium">
                <Check class="text-primary size-4 shrink-0" aria-hidden="true" />
                {{ capability.title }}
              </p>
              <p class="text-muted-foreground text-sm">{{ capability.detail }}</p>
            </li>
          </ul>
        </CardContent>
      </Card>
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
