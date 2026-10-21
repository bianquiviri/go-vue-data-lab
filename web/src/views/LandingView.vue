<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
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
 * Landing pública (Issue #39) con doble idioma ES/EN. Todo el copy vive en
 * `i18n/locales/*` bajo `landing.*`; aquí solo quedan las claves y los iconos.
 *
 * La vista se renderiza SIEMPRE en `/` (con o sin sesión): el guard del router
 * ya espera a `auth.initialize()` antes de montarla, así que `isAuthenticated`
 * es estable en el primer render y no hay parpadeo.
 */
const auth = useAuthStore()
const { t } = useI18n()

const cta = computed(() =>
  auth.isAuthenticated
    ? { label: t('publicHeader.goToDashboard'), to: '/dashboard' }
    : { label: t('publicHeader.signIn'), to: '/login' },
)

interface Feature {
  titleKey: string
  descriptionKey: string
  icon: LucideIcon
}

const FEATURES: Feature[] = [
  {
    titleKey: 'landing.features.workerPoolTitle',
    descriptionKey: 'landing.features.workerPoolDetail',
    icon: Cpu,
  },
  {
    titleKey: 'landing.features.schedulerTitle',
    descriptionKey: 'landing.features.schedulerDetail',
    icon: CalendarClock,
  },
  {
    titleKey: 'landing.features.etlTitle',
    descriptionKey: 'landing.features.etlDetail',
    icon: RefreshCcw,
  },
  {
    titleKey: 'landing.features.rbacTitle',
    descriptionKey: 'landing.features.rbacDetail',
    icon: ShieldCheck,
  },
  {
    titleKey: 'landing.features.apiTitle',
    descriptionKey: 'landing.features.apiDetail',
    icon: FileCode2,
  },
  {
    titleKey: 'landing.features.opaqueIdsTitle',
    descriptionKey: 'landing.features.opaqueIdsDetail',
    icon: KeyRound,
  },
]

interface PipelineStep {
  titleKey: string
  /** Esquema (o concepto) al que corresponde el paso, renderizado como chip mono. */
  schema: string
  detailKey: string
  icon: LucideIcon
}

/** Los tres tramos del pipeline: ingesta → transformación → analítica. */
const PIPELINE: PipelineStep[] = [
  {
    titleKey: 'landing.pipeline.ingestTitle',
    schema: 'lab_source',
    detailKey: 'landing.pipeline.ingestDetail',
    icon: Database,
  },
  {
    titleKey: 'landing.pipeline.transformTitle',
    schema: 'resumable ETL',
    detailKey: 'landing.pipeline.transformDetail',
    icon: Workflow,
  },
  {
    titleKey: 'landing.pipeline.serveTitle',
    schema: 'lab_target',
    detailKey: 'landing.pipeline.serveDetail',
    icon: LineChart,
  },
]

interface Capability {
  titleKey: string
  detailKey: string
}

const CAPABILITIES: Capability[] = [
  {
    titleKey: 'landing.capabilities.simulateTitle',
    detailKey: 'landing.capabilities.simulateDetail',
  },
  {
    titleKey: 'landing.capabilities.scheduleTitle',
    detailKey: 'landing.capabilities.scheduleDetail',
  },
  {
    titleKey: 'landing.capabilities.exploreTitle',
    detailKey: 'landing.capabilities.exploreDetail',
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
        {{ t('common.brand') }}
      </h1>
      <p class="text-muted-foreground text-lg">
        {{ t('landing.tagline') }}
      </p>
      <p class="text-muted-foreground mx-auto max-w-xl">
        {{ t('landing.hero') }}
      </p>
      <Button as-child size="lg" data-test="landing-cta">
        <RouterLink :to="cta.to">{{ cta.label }}</RouterLink>
      </Button>
    </section>

    <!-- What is this? Propósito, pipeline y usos (Issue #39) -->
    <section class="space-y-10" aria-labelledby="what-is-this-title" data-test="what-is-this">
      <div class="mx-auto max-w-2xl space-y-4 text-center">
        <p class="text-muted-foreground text-xs font-medium tracking-widest uppercase">
          {{ t('landing.overview') }}
        </p>
        <h2
          id="what-is-this-title"
          class="text-2xl font-semibold tracking-tight sm:text-3xl"
          data-test="what-is-this-title"
        >
          {{ t('landing.whatIsThisTitle') }}
        </h2>
        <p class="text-muted-foreground">
          {{ t('landing.whatIsThisP1') }}
        </p>
        <p class="text-muted-foreground">
          {{ t('landing.whatIsThisP2') }}
        </p>
      </div>

      <!-- Pipeline: lab_source → ETL con checkpoints → lab_target -->
      <div class="space-y-4">
        <div class="space-y-1 text-center">
          <h3 class="text-lg font-medium tracking-tight">{{ t('landing.pipelineTitle') }}</h3>
          <p class="text-muted-foreground text-sm">
            {{ t('landing.pipelineSubtitle') }}
          </p>
        </div>
        <ol class="grid gap-4 md:grid-cols-3" data-test="pipeline-step-list">
          <li v-for="(step, index) in PIPELINE" :key="step.titleKey">
            <Card class="h-full" data-test="pipeline-step">
              <CardHeader class="pb-2">
                <div class="flex items-center gap-2">
                  <Badge variant="secondary" class="w-5 justify-center px-0 tabular-nums">
                    {{ index + 1 }}
                  </Badge>
                  <component :is="step.icon" class="text-primary size-4" aria-hidden="true" />
                  <CardTitle class="text-base">{{ t(step.titleKey) }}</CardTitle>
                </div>
              </CardHeader>
              <CardContent class="space-y-2">
                <code
                  class="bg-muted text-foreground inline-block rounded px-1.5 py-0.5 font-mono text-xs"
                >
                  {{ step.schema }}
                </code>
                <CardDescription>{{ t(step.detailKey) }}</CardDescription>
              </CardContent>
            </Card>
          </li>
        </ol>
      </div>

      <!-- Usos -->
      <Card data-test="capabilities">
        <CardHeader>
          <CardTitle class="text-lg">{{ t('landing.capabilitiesTitle') }}</CardTitle>
          <CardDescription>{{ t('landing.capabilitiesSubtitle') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <ul class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <li v-for="capability in CAPABILITIES" :key="capability.titleKey" class="space-y-1.5">
              <p class="flex items-center gap-2 text-sm font-medium">
                <Check class="text-primary size-4 shrink-0" aria-hidden="true" />
                {{ t(capability.titleKey) }}
              </p>
              <p class="text-muted-foreground text-sm">{{ t(capability.detailKey) }}</p>
            </li>
          </ul>
        </CardContent>
      </Card>
    </section>

    <!-- Features -->
    <section :aria-label="t('landing.featuresAriaLabel')">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card v-for="feature in FEATURES" :key="feature.titleKey" data-test="feature">
          <CardHeader class="pb-2">
            <div class="flex items-center gap-2">
              <component :is="feature.icon" class="text-primary size-5" />
              <CardTitle class="text-base">{{ t(feature.titleKey) }}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <CardDescription>{{ t(feature.descriptionKey) }}</CardDescription>
          </CardContent>
        </Card>
      </div>
    </section>
  </div>
</template>
