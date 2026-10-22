<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, Play, RefreshCw, Square, WifiOff } from 'lucide-vue-next'

import { ApiError } from '@/api/client'
import { listJobHistory, startGenerate, startMigrate, stopJob } from '@/api/jobs'
import DataTable from '@/components/DataTable.vue'
import IdChip from '@/components/IdChip.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import TruncatedCell from '@/components/TruncatedCell.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { useDataList } from '@/composables/useDataList'
import { useJobEvents } from '@/composables/useJobEvents'
import { formatDateTime } from '@/lib/datetime'
import { useAuthStore } from '@/stores/auth'
import { type DataTableColumn, type PagedResult } from '@/types/data'
import {
  type GenerateParams,
  type JobExecution,
  type JobStatusKind,
  type MigrateParams,
} from '@/types/jobs'

/**
 * Panel de monitoreo y control de jobs (Issue #21).
 *
 * - **Control**: start generate / start migrate / stop, cada uno con su diálogo
 *   de parámetros (defaults sensatos: se envía solo lo rellenado). Los roles
 *   `viewer` ven los botones DESHABILITADOS con una pista (los POST requieren
 *   admin u operator; el backend devolvería 403 de todos modos).
 * - **Progreso en vivo**: vía SSE (`useJobEvents`); barra general + tabla por
 *   entidad. Si el stream falla, la vista sigue operativa (reconectar o el
 *   fallback REST del composable).
 * - **Historial**: `DataTable` + `useDataList` sobre `GET /jobs/history`.
 *
 * ⚠️ IDs OPACOS: `job_id`/`id` son tokens cifrados; se muestran vía `IdChip`
 * (presentación acortada, valor íntegro en tooltip/copiado) y nunca se parsean.
 */
const auth = useAuthStore()
const { t, te } = useI18n()
const events = useJobEvents()

/** Roles con permiso de escritura sobre jobs (RBAC del backend). */
const canControl = computed(() => auth.role === 'admin' || auth.role === 'operator')

const progress = events.progress
const status = events.status

/** Progreso mostrado: el del job actual o el del snapshot como fallback. */
const displayedProgress = computed(() => progress.value ?? status.value?.progress ?? null)

const statusKind = computed<JobStatusKind>(() => status.value?.status ?? 'idle')
const isRunning = computed(() => status.value?.running === true)
const canStop = computed(() => canControl.value && isRunning.value)
const canStart = computed(() => canControl.value && !isRunning.value)

/** Error terminal del job actual (fallo) para destacarlo. */
const jobError = computed(() => status.value?.error ?? null)

/**
 * Traduce un valor del dominio cuando existe traducción (estados y tipos de
 * job); si el backend devuelve algo desconocido, se muestra tal cual.
 */
function translated(value: unknown, key: string): string {
  const raw = String(value ?? '')
  return te(`${key}.${raw}`) ? t(`${key}.${raw}`) : raw
}

/** Estado del job traducido (p. ej. `running` → "En ejecución"). */
function jobStatusLabel(value: unknown): string {
  return translated(value, 'jobs.status')
}

/** Tipo de job traducido (p. ej. `generate` → "Generación de datos"). */
function jobTypeLabel(value: unknown): string {
  return translated(value, 'jobs.type')
}

// --- Acciones (start/stop) -------------------------------------------------

const actionError = ref<string | null>(null)
const starting = ref(false)
const stopping = ref(false)

/**
 * Traduce el error del API a un mensaje de UI (409/404 con textos propios).
 * El detalle del backend solo se añade en errores de validación (400/403),
 * donde aporta información; para el resto se usa el mensaje traducido.
 */
function actionErrorMessage(
  err: unknown,
  fallbackKey: 'jobs.startError' | 'jobs.stopError',
): string {
  if (err instanceof ApiError) {
    if (err.status === 409) return t('jobs.conflictError')
    if (err.status === 404) return t('jobs.noRunningJob')
    if (err.status === 400 || err.status === 403) {
      const detail = err.message?.trim()
      if (detail) return t('jobs.errorDetail', { message: t(fallbackKey), detail })
    }
  }
  return t(fallbackKey)
}

async function onStop(): Promise<void> {
  if (!canStop.value) return
  stopping.value = true
  actionError.value = null
  try {
    await stopJob()
    await events.refreshStatus()
  } catch (err) {
    actionError.value = actionErrorMessage(err, 'jobs.stopError')
  } finally {
    stopping.value = false
  }
}

// --- Diálogo de parámetros -------------------------------------------------

type DialogKind = 'generate' | 'migrate' | null
const dialog = ref<DialogKind>(null)

// Campos de generación (string para permitir vacío = default del servidor).
const genSeed = ref<string | number>('')
const genCustomers = ref<string | number>('')
const genProducts = ref<string | number>('')
const genOrders = ref<string | number>('')
const genMinItems = ref<string | number>('')
const genMaxItems = ref<string | number>('')
const genBatchSize = ref<string | number>('')
const genReset = ref(false)
const genSkipIfExists = ref(false)

// Campos de migración.
const migEntities = ref<string | number>('')
const migWorkers = ref<string | number>('')
const migBatchSize = ref<string | number>('')
const migExtractSize = ref<string | number>('')
const migFailFast = ref(false)
const migStrictTotals = ref(false)

/**
 * Convierte un campo de texto en número solo si está relleno (si no, se omite).
 *
 * ⚠️ Los `Input` con `type="number"` emiten **números** (Vue castea
 * automáticamente el valor del input), no cadenas: hay que aceptar ambos
 * tipos o `value.trim()` revienta con un TypeError.
 */
function num(value: string | number | undefined | null): number | undefined {
  if (value === undefined || value === null) return undefined
  const trimmed = typeof value === 'number' ? String(value) : value.trim()
  if (trimmed === '') return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : undefined
}

/** Construye el body de generate omitiendo los campos vacíos (defaults del server). */
function buildGenerateParams(): GenerateParams {
  const params: GenerateParams = {}
  const seed = num(genSeed.value)
  const customers = num(genCustomers.value)
  const products = num(genProducts.value)
  const orders = num(genOrders.value)
  const minItems = num(genMinItems.value)
  const maxItems = num(genMaxItems.value)
  const batchSize = num(genBatchSize.value)
  if (seed !== undefined) params.seed = seed
  if (customers !== undefined) params.customers = customers
  if (products !== undefined) params.products = products
  if (orders !== undefined) params.orders = orders
  if (minItems !== undefined) params.min_items = minItems
  if (maxItems !== undefined) params.max_items = maxItems
  if (batchSize !== undefined) params.batch_size = batchSize
  if (genReset.value) params.reset = true
  if (genSkipIfExists.value) params.skip_if_exists = true
  return params
}

/** Construye el body de migrate omitiendo los campos vacíos. */
function buildMigrateParams(): MigrateParams {
  const params: MigrateParams = {}
  const entities = String(migEntities.value ?? '')
    .split(',')
    .map((value) => value.trim())
    .filter((value) => value !== '')
  const workers = num(migWorkers.value)
  const batchSize = num(migBatchSize.value)
  const extractSize = num(migExtractSize.value)
  if (entities.length > 0) params.entities = entities
  if (workers !== undefined) params.workers = workers
  if (batchSize !== undefined) params.batch_size = batchSize
  if (extractSize !== undefined) params.extract_size = extractSize
  if (migFailFast.value) params.fail_fast = true
  if (migStrictTotals.value) params.strict_totals = true
  return params
}

function openDialog(kind: DialogKind): void {
  if (!canStart.value) return
  actionError.value = null
  dialog.value = kind
}

function closeDialog(): void {
  dialog.value = null
}

async function confirmDialog(): Promise<void> {
  const kind = dialog.value
  if (!kind) return
  starting.value = true
  actionError.value = null
  try {
    if (kind === 'generate') {
      await startGenerate(buildGenerateParams())
    } else {
      await startMigrate(buildMigrateParams())
    }
    dialog.value = null
    // Reabre el stream SSE: tras un job terminal el backend cierra el stream
    // (y el `EventSource` no debe reconectar solo), así que el progreso del
    // job nuevo se sigue en vivo gracias a esta reapertura.
    events.connect()
  } catch (err) {
    actionError.value = actionErrorMessage(err, 'jobs.startError')
  } finally {
    starting.value = false
  }
}

// --- Progreso por entidad --------------------------------------------------

const entities = computed(() => displayedProgress.value?.entities ?? [])

// --- Historial -------------------------------------------------------------

const list = useDataList<JobExecution>(
  (params) => listJobHistory(params as { page?: number; per_page?: number }),
)

const historyColumns = computed<DataTableColumn[]>(() => [
  { key: 'id', labelKey: 'data.rowId', class: 'w-[9rem]' },
  { key: 'job_name', labelKey: 'jobs.historyName', class: 'min-w-[8rem]' },
  { key: 'status', labelKey: 'jobs.historyStatus', class: 'min-w-[7rem]' },
  { key: 'started_at', labelKey: 'jobs.historyStartedAt', class: 'min-w-[12rem]' },
  { key: 'finished_at', labelKey: 'jobs.historyFinishedAt', class: 'min-w-[12rem]' },
  { key: 'error', labelKey: 'jobs.historyError', class: 'min-w-[12rem]' },
])

/**
 * Historial con las fechas ya formateadas para la tabla.
 *
 * `formatDateTime` es el helper compartido (Issue #22): `started_at` es
 * `string | null` en el contrato, así que se le pasa como `unknown`.
 */
const formattedHistory = computed<PagedResult<JobExecution> | null>(() => {
  const result = list.result.value
  if (!result) return null
  return {
    ...result,
    items: result.items.map((row) => ({
      ...row,
      started_at: formatDateTime(row.started_at),
      finished_at: formatDateTime(row.finished_at),
    })),
  }
})

onMounted(() => {
  events.connect()
})
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">{{ t('jobs.title') }}</h1>
        <p class="text-muted-foreground text-sm">{{ t('jobs.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <span
          v-if="events.connected.value"
          class="text-muted-foreground inline-flex items-center gap-1 text-xs"
          data-test="jobs-live-connected"
        >
          <span class="bg-primary size-2 animate-pulse rounded-full" />
          {{ t('jobs.liveConnected') }}
        </span>
        <Button variant="outline" size="sm" data-test="jobs-refresh" @click="events.refreshStatus()">
          <RefreshCw />
          {{ t('jobs.refresh') }}
        </Button>
      </div>
    </header>

    <p
      v-if="!canControl"
      class="bg-muted/50 text-muted-foreground rounded-lg border px-3 py-2 text-sm"
      data-test="jobs-viewer-hint"
    >
      {{ t('jobs.viewerHint') }}
    </p>

    <p v-if="events.authExpired.value" class="text-destructive text-sm" data-test="jobs-session-expired">
      {{ t('jobs.sessionExpired') }}
    </p>

    <!-- Estado + panel de control -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('jobs.control') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <Button data-test="jobs-generate" :disabled="!canStart" @click="openDialog('generate')">
            <Play />
            {{ t('jobs.generate') }}
          </Button>
          <Button
            variant="secondary"
            data-test="jobs-migrate"
            :disabled="!canStart"
            @click="openDialog('migrate')"
          >
            <Play />
            {{ t('jobs.migrate') }}
          </Button>
          <Button
            variant="destructive"
            data-test="jobs-stop"
            :disabled="!canStop"
            @click="onStop"
          >
            <Square />
            {{ stopping ? t('jobs.stopping') : t('jobs.stop') }}
          </Button>

          <div class="ml-auto flex items-center gap-2 text-sm">
            <span class="text-muted-foreground">{{ t('jobs.statusLabel') }}</span>
            <StatusBadge
              :status="statusKind"
              :label="jobStatusLabel(statusKind)"
              data-test="jobs-status-badge"
            />
          </div>
        </div>

        <dl class="grid gap-4 sm:grid-cols-3">
          <div>
            <dt class="text-muted-foreground text-xs">{{ t('jobs.statusLabel') }}</dt>
            <dd data-test="jobs-status">{{ jobStatusLabel(statusKind) }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground text-xs">{{ t('jobs.jobType') }}</dt>
            <dd class="text-sm" data-test="jobs-type">{{ jobTypeLabel(status?.type) }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground text-xs">{{ t('jobs.startedAt') }}</dt>
            <dd class="text-sm" data-test="jobs-started-at">{{ formatDateTime(status?.started_at) }}</dd>
          </div>
        </dl>

        <!--
          El error se muestra en el panel y, si el diálogo está abierto, también
          dentro del diálogo (que es lo que el usuario tiene delante).
        -->
        <p
          v-if="actionError && !dialog"
          class="text-destructive text-sm"
          data-test="jobs-action-error"
        >
          {{ actionError }}
        </p>

        <!-- Error de job fallido, destacado -->
        <div
          v-if="jobError || statusKind === 'failed'"
          class="border-destructive/40 bg-destructive/10 text-destructive flex items-start gap-2 rounded-lg border p-3 text-sm"
          data-test="jobs-error"
        >
          <AlertTriangle class="mt-0.5 size-4 shrink-0" />
          <div>
            <p class="font-medium">{{ t('jobs.jobError') }}</p>
            <p data-test="jobs-error-message">{{ jobError ?? '—' }}</p>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Diálogo de parámetros -->
    <Card v-if="dialog" data-test="jobs-params-dialog">
      <CardHeader>
        <CardTitle class="text-base">
          {{ dialog === 'generate' ? t('jobs.params.generateTitle') : t('jobs.params.migrateTitle') }}
        </CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <p class="text-muted-foreground text-xs">{{ t('jobs.params.defaultHint') }}</p>

        <p
          v-if="actionError"
          class="text-destructive text-sm"
          data-test="jobs-action-error"
        >
          {{ actionError }}
        </p>

        <div v-if="dialog === 'generate'" class="grid gap-4 sm:grid-cols-3">
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.seed') }}</span>
            <Input v-model="genSeed" type="number" data-test="jobs-param-seed" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.customers') }}</span>
            <Input v-model="genCustomers" type="number" data-test="jobs-param-customers" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.products') }}</span>
            <Input v-model="genProducts" type="number" data-test="jobs-param-products" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.orders') }}</span>
            <Input v-model="genOrders" type="number" data-test="jobs-param-orders" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.minItems') }}</span>
            <Input v-model="genMinItems" type="number" data-test="jobs-param-min-items" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.maxItems') }}</span>
            <Input v-model="genMaxItems" type="number" data-test="jobs-param-max-items" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.batchSize') }}</span>
            <Input v-model="genBatchSize" type="number" data-test="jobs-param-batch-size" />
          </label>
          <label class="flex items-center gap-2 text-xs font-medium">
            <input v-model="genReset" type="checkbox" data-test="jobs-param-reset">
            <span>{{ t('jobs.params.reset') }}</span>
          </label>
          <label class="flex items-center gap-2 text-xs font-medium">
            <input v-model="genSkipIfExists" type="checkbox" data-test="jobs-param-skip-if-exists">
            <span>{{ t('jobs.params.skipIfExists') }}</span>
          </label>
        </div>

        <div v-else class="grid gap-4 sm:grid-cols-3">
          <label class="flex flex-col gap-1 text-xs font-medium sm:col-span-3">
            <span class="text-muted-foreground">{{ t('jobs.params.workers') }}</span>
            <Input v-model="migWorkers" type="number" data-test="jobs-param-workers" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.batchSize') }}</span>
            <Input v-model="migBatchSize" type="number" data-test="jobs-param-mig-batch-size" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.extractSize') }}</span>
            <Input v-model="migExtractSize" type="number" data-test="jobs-param-extract-size" />
          </label>
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('jobs.params.entities') }}</span>
            <Input v-model="migEntities" data-test="jobs-param-entities" />
          </label>
          <label class="flex items-center gap-2 text-xs font-medium">
            <input v-model="migFailFast" type="checkbox" data-test="jobs-param-fail-fast">
            <span>{{ t('jobs.params.failFast') }}</span>
          </label>
          <label class="flex items-center gap-2 text-xs font-medium">
            <input v-model="migStrictTotals" type="checkbox" data-test="jobs-param-strict-totals">
            <span>{{ t('jobs.params.strictTotals') }}</span>
          </label>
        </div>

        <div class="flex items-center gap-2">
          <Button data-test="jobs-params-confirm" :disabled="starting" @click="confirmDialog">
            {{ starting ? t('jobs.starting') : t('jobs.params.confirm') }}
          </Button>
          <Button variant="outline" data-test="jobs-params-cancel" @click="closeDialog">
            {{ t('jobs.params.cancel') }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Progreso en vivo -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('jobs.progressTitle') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div
          v-if="events.disconnected.value"
          class="border-muted-foreground/30 bg-muted/50 flex items-center gap-2 rounded-lg border p-3 text-sm"
          data-test="jobs-live-disconnected"
        >
          <WifiOff class="size-4 shrink-0" />
          <span>{{ t('jobs.liveDisconnected') }}</span>
          <Button variant="outline" size="sm" data-test="jobs-reconnect" @click="events.reconnect()">
            <RefreshCw />
            {{ t('jobs.reconnect') }}
          </Button>
        </div>

        <template v-if="displayedProgress">
          <div class="space-y-1">
            <div class="flex items-center justify-between text-sm">
              <span class="text-muted-foreground">{{ t('jobs.overallProgress') }}</span>
              <span class="font-medium" data-test="jobs-percent">
                {{ Math.round(displayedProgress.percent) }}%
              </span>
            </div>
            <Progress
              :value="displayedProgress.percent"
              :aria-label="t('jobs.overallProgress')"
            />
          </div>

          <dl class="grid gap-4 sm:grid-cols-3">
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('jobs.processed') }}</dt>
              <dd class="text-sm font-medium" data-test="jobs-processed">
                {{ displayedProgress.processed }}
              </dd>
            </div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('jobs.failed') }}</dt>
              <dd class="text-sm font-medium" data-test="jobs-failed">
                {{ displayedProgress.failed }}
              </dd>
            </div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('jobs.skipped') }}</dt>
              <dd class="text-sm font-medium" data-test="jobs-skipped">
                {{ displayedProgress.skipped }}
              </dd>
            </div>
          </dl>

          <table v-if="entities.length > 0" class="w-full text-sm" data-test="jobs-entities">
            <thead>
              <tr class="text-muted-foreground text-left text-xs">
                <th class="py-1 pr-4 font-medium">{{ t('jobs.entity') }}</th>
                <th class="py-1 pr-4 font-medium">{{ t('jobs.done') }}</th>
                <th class="py-1 pr-4 font-medium">{{ t('jobs.processed') }}</th>
                <th class="py-1 pr-4 font-medium">{{ t('jobs.failed') }}</th>
                <th class="py-1 font-medium">{{ t('jobs.skipped') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="entity in entities"
                :key="entity.entity"
                class="border-t"
                data-test="jobs-entity-row"
              >
                <td class="py-1 pr-4">{{ entity.entity }}</td>
                <td class="py-1 pr-4">{{ entity.done }} / {{ entity.total }}</td>
                <td class="py-1 pr-4">{{ entity.processed }}</td>
                <td class="py-1 pr-4">{{ entity.failed }}</td>
                <td class="py-1">{{ entity.skipped }}</td>
              </tr>
            </tbody>
          </table>
        </template>

        <p v-else class="text-muted-foreground text-sm" data-test="jobs-progress-empty">
          {{ t('jobs.emptyProgress') }}
        </p>
      </CardContent>
    </Card>

    <!-- Historial -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('jobs.historyTitle') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="historyColumns"
          :result="formattedHistory"
          :loading="list.loading.value"
          :error="list.error.value"
          @update:page="list.goToPage"
          @update:per-page="list.setPerPage"
          @retry="list.refresh"
        >
          <template #cell-id="{ value }">
            <IdChip :id="String(value)" />
          </template>
          <template #cell-status="{ value }">
            <StatusBadge :status="String(value)" :label="jobStatusLabel(value)" />
          </template>
          <template #cell-error="{ value }">
            <TruncatedCell :value="value" class="text-destructive max-w-[16rem] text-xs" />
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
