<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Pencil, Play, Plus, Power, PowerOff, RefreshCw, Trash2 } from 'lucide-vue-next'

import { ApiError } from '@/api/client'
import {
  createSchedule,
  deleteSchedule,
  disableSchedule,
  enableSchedule,
  listSchedules,
  runSchedule,
  updateSchedule,
} from '@/api/scheduler'
import IdChip from '@/components/IdChip.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatDateTime } from '@/lib/datetime'
import { useAuthStore } from '@/stores/auth'
import type {
  CronSchedule,
  ScheduleCommand,
  UpdateScheduleParams,
} from '@/types/scheduler'

/**
 * Gestión de schedules cron (Issue #22).
 *
 * - **Lista**: `GET /scheduler/jobs` NO está paginado (envelope sin `meta`), así
 *   que se usan las primitivas de `ui/table` en lugar de `DataTable`.
 * - **Control**: crear/editar/habilitar/ejecutar/borrar exigen `admin` u
 *   `operator`; un `viewer` ve la lista en modo lectura, igual que en `JobsView`.
 * - **Formulario**: `Card` inline (no hay componente `dialog` en `ui/`), con
 *   validación client-side de la FORMA del cron y el backend como autoridad
 *   final: un 400 de cron se muestra junto al campo sin cerrar el formulario.
 * - **Borrado**: confirmación en dos pasos sobre la propia fila (sin
 *   `window.confirm`, que no es testeable ni consistente con el resto de la UI).
 *
 * ⚠️ IDs OPACOS: `id` y `job_id` son tokens cifrados (`v1.*`). Viajan verbatim
 * al API y se muestran con `IdChip`; nunca se parsean ni transforman.
 */
const auth = useAuthStore()
const { t, te } = useI18n()

/** Roles con permiso de escritura sobre el scheduler (RBAC del backend). */
const canControl = computed(() => auth.role === 'admin' || auth.role === 'operator')

// --- Lista ---------------------------------------------------------------

const schedules = ref<CronSchedule[]>([])
const loading = ref(false)
const loadError = ref(false)

/** Mensajes de confirmación (creado/actualizado/borrado) — `aria-live`. */
const statusMessage = ref<string | null>(null)
/** Error de una acción de fila o del formulario (fuera de los campos). */
const actionError = ref<string | null>(null)
/** Id del schedule con una acción en curso (deshabilita sus botones). */
const busyId = ref<string | null>(null)
/** Fila con la confirmación de borrado abierta (dos pasos, sin modal). */
const confirmingDeleteId = ref<string | null>(null)
/** `job_id` opaco del último `run` exitoso. */
const runJobId = ref<string | null>(null)

/**
 * Traduce un valor del dominio cuando existe traducción (los comandos
 * reutilizan las etiquetas de `jobs.type`); si no, se muestra tal cual.
 */
function commandLabel(value: unknown): string {
  const raw = String(value ?? '')
  return te(`jobs.type.${raw}`) ? t(`jobs.type.${raw}`) : raw
}

/** Traduce el fallo de una acción: 409 = ya hay un job corriendo. */
function actionErrorMessage(err: unknown): string {
  if (err instanceof ApiError && err.status === 409) return t('scheduler.jobRunning')
  return t('scheduler.actionError')
}

/** Recarga la lista completa (también es el `retry` del estado de error). */
async function load(): Promise<void> {
  loading.value = true
  loadError.value = false
  try {
    schedules.value = await listSchedules()
  } catch {
    schedules.value = []
    loadError.value = true
  } finally {
    loading.value = false
  }
}

/** Sustituye una fila por la versión devuelta por el backend (o la añade). */
function replaceSchedule(updated: CronSchedule): void {
  const index = schedules.value.findIndex((row) => row.id === updated.id)
  if (index === -1) schedules.value = [...schedules.value, updated]
  else schedules.value[index] = updated
}

// --- Acciones por fila ---------------------------------------------------

async function onToggle(schedule: CronSchedule): Promise<void> {
  if (!canControl.value) return
  busyId.value = schedule.id
  actionError.value = null
  statusMessage.value = null
  confirmingDeleteId.value = null
  try {
    const updated = schedule.enabled
      ? await disableSchedule(schedule.id)
      : await enableSchedule(schedule.id)
    replaceSchedule(updated)
    statusMessage.value = t('scheduler.updated')
  } catch (err) {
    actionError.value = actionErrorMessage(err)
  } finally {
    busyId.value = null
  }
}

async function onRun(schedule: CronSchedule): Promise<void> {
  if (!canControl.value) return
  busyId.value = schedule.id
  actionError.value = null
  statusMessage.value = null
  runJobId.value = null
  try {
    const result = await runSchedule(schedule.id)
    runJobId.value = result.job_id
  } catch (err) {
    actionError.value = actionErrorMessage(err)
  } finally {
    busyId.value = null
  }
}

/** Abre la confirmación en dos pasos de esa fila (nunca `window.confirm`). */
function askDelete(schedule: CronSchedule): void {
  if (!canControl.value) return
  actionError.value = null
  confirmingDeleteId.value = schedule.id
}

async function onDelete(schedule: CronSchedule): Promise<void> {
  busyId.value = schedule.id
  actionError.value = null
  statusMessage.value = null
  try {
    await deleteSchedule(schedule.id)
    schedules.value = schedules.value.filter((row) => row.id !== schedule.id)
    statusMessage.value = t('scheduler.deleted')
  } catch (err) {
    actionError.value = actionErrorMessage(err)
  } finally {
    busyId.value = null
    confirmingDeleteId.value = null
  }
}

// --- Formulario crear/editar ---------------------------------------------

const formOpen = ref(false)
/** `null` = alta; token opaco = edición de ese schedule. */
const editingId = ref<string | null>(null)
const formName = ref('')
const formCron = ref('')
const formCommand = ref<ScheduleCommand>('generate')
const formEnabled = ref(true)
const formError = ref<string | null>(null)
const nameError = ref<string | null>(null)
const cronError = ref<string | null>(null)
const saving = ref(false)

/** Un campo de cron admite dígitos, letras (meses/días) y los operadores habituales. */
const CRON_FIELD = /^[0-9A-Za-z*/,-]+$/

/**
 * Valida la FORMA de la expresión cron: exactamente 5 campos separados por
 * espacios y cada campo solo con dígitos, letras (meses/días), asterisco, barra,
 * coma y guion. La semántica (rangos, pasos válidos) la decide el backend, que
 * es la autoridad.
 */
function isValidCron(value: string): boolean {
  const fields = value.trim().split(/\s+/)
  if (fields.length !== 5) return false
  return fields.every((field) => CRON_FIELD.test(field))
}

function resetFormErrors(): void {
  formError.value = null
  nameError.value = null
  cronError.value = null
}

function openCreate(): void {
  if (!canControl.value) return
  editingId.value = null
  formName.value = ''
  formCron.value = ''
  formCommand.value = 'generate'
  formEnabled.value = true
  resetFormErrors()
  actionError.value = null
  formOpen.value = true
}

function openEdit(schedule: CronSchedule): void {
  if (!canControl.value) return
  editingId.value = schedule.id
  formName.value = schedule.name
  formCron.value = schedule.cron_expr
  formCommand.value = schedule.command
  formEnabled.value = schedule.enabled
  resetFormErrors()
  actionError.value = null
  formOpen.value = true
}

function closeForm(): void {
  formOpen.value = false
}

/** Body común de alta y edición (el checkbox viaja siempre explícito). */
function buildParams(): UpdateScheduleParams {
  return {
    name: formName.value.trim(),
    cron_expr: formCron.value.trim(),
    command: formCommand.value,
    enabled: formEnabled.value,
  }
}

async function onSave(): Promise<void> {
  resetFormErrors()
  const name = formName.value.trim()
  const cron = formCron.value.trim()

  // Validación client-side de forma: evita un round-trip y da feedback inmediato.
  if (name === '') nameError.value = t('scheduler.form.nameRequired')
  if (!isValidCron(cron)) cronError.value = t('scheduler.form.cronInvalid')
  if (nameError.value !== null || cronError.value !== null) return

  saving.value = true
  actionError.value = null
  statusMessage.value = null
  try {
    if (editingId.value !== null) {
      const updated = await updateSchedule(editingId.value, buildParams())
      replaceSchedule(updated)
      statusMessage.value = t('scheduler.updated')
    } else {
      const created = await createSchedule(buildParams())
      replaceSchedule(created)
      statusMessage.value = t('scheduler.created')
    }
    formOpen.value = false
  } catch (err) {
    // El backend es la autoridad del cron: su mensaje se muestra junto al campo
    // y el formulario sigue abierto para corregirlo.
    if (err instanceof ApiError && err.status === 400) {
      const detail = err.message?.trim()
      cronError.value = detail ? detail : t('scheduler.form.cronInvalid')
    } else {
      formError.value = t('scheduler.form.saveError')
    }
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-4">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">{{ t('scheduler.title') }}</h1>
        <p class="text-muted-foreground text-sm">{{ t('scheduler.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="sm" data-test="scheduler-refresh" @click="load">
          <RefreshCw />
          {{ t('scheduler.refresh') }}
        </Button>
        <Button v-if="canControl" size="sm" data-test="scheduler-new" @click="openCreate">
          <Plus />
          {{ t('scheduler.new') }}
        </Button>
      </div>
    </header>

    <p
      v-if="!canControl"
      class="bg-muted/50 text-muted-foreground rounded-lg border px-3 py-2 text-sm"
      data-test="scheduler-viewer-hint"
    >
      {{ t('scheduler.viewerHint') }}
    </p>

    <!-- Mensajes de estado (aria-live para lectores de pantalla) -->
    <p
      v-if="statusMessage"
      class="text-muted-foreground text-sm"
      aria-live="polite"
      data-test="scheduler-status-message"
    >
      {{ statusMessage }}
    </p>

    <p
      v-if="actionError"
      class="text-destructive text-sm"
      aria-live="polite"
      data-test="scheduler-action-error"
    >
      {{ actionError }}
    </p>

    <!-- Resultado de "ejecutar ahora": job_id opaco + acceso al monitoreo -->
    <div
      v-if="runJobId"
      class="bg-muted/50 flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-sm"
      aria-live="polite"
      data-test="scheduler-run-result"
    >
      <span>{{ t('scheduler.runStarted') }}</span>
      <IdChip :id="runJobId" />
      <RouterLink
        to="/jobs"
        class="text-primary text-sm underline-offset-4 hover:underline"
        data-test="scheduler-go-to-jobs"
      >
        {{ t('scheduler.goToJobs') }}
      </RouterLink>
    </div>

    <!-- Formulario inline de alta/edición -->
    <Card v-if="formOpen" data-test="scheduler-form">
      <CardHeader>
        <CardTitle class="text-base">
          {{ editingId ? t('scheduler.form.editTitle') : t('scheduler.form.createTitle') }}
        </CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <p v-if="formError" class="text-destructive text-sm" data-test="scheduler-form-error">
          {{ formError }}
        </p>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('scheduler.form.name') }}</span>
            <Input
              v-model="formName"
              :placeholder="t('scheduler.form.namePlaceholder')"
              data-test="scheduler-form-name"
            />
            <span
              v-if="nameError"
              class="text-destructive"
              data-test="scheduler-form-name-error"
            >
              {{ nameError }}
            </span>
          </label>

          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('scheduler.form.cron') }}</span>
            <Input
              v-model="formCron"
              class="font-mono"
              :placeholder="t('scheduler.form.cronPlaceholder')"
              data-test="scheduler-form-cron"
            />
            <span class="text-muted-foreground">{{ t('scheduler.form.cronHint') }}</span>
            <span
              v-if="cronError"
              class="text-destructive"
              data-test="scheduler-form-cron-error"
            >
              {{ cronError }}
            </span>
          </label>

          <label class="flex flex-col gap-1 text-xs font-medium">
            <span class="text-muted-foreground">{{ t('scheduler.form.command') }}</span>
            <select
              v-model="formCommand"
              class="border-input bg-background h-8 rounded-lg border px-2 text-sm"
              data-test="scheduler-form-command"
            >
              <option value="generate">{{ t('jobs.type.generate') }}</option>
              <option value="migrate">{{ t('jobs.type.migrate') }}</option>
            </select>
          </label>

          <label class="flex items-center gap-2 text-xs font-medium">
            <input v-model="formEnabled" type="checkbox" data-test="scheduler-form-enabled">
            <span>{{ t('scheduler.form.enabled') }}</span>
          </label>
        </div>

        <div class="flex items-center gap-2">
          <Button :disabled="saving" data-test="scheduler-form-save" @click="onSave">
            {{ saving ? t('scheduler.form.saving') : t('scheduler.form.save') }}
          </Button>
          <Button variant="outline" data-test="scheduler-form-cancel" @click="closeForm">
            {{ t('scheduler.form.cancel') }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Lista -->
    <Card>
      <CardContent>
        <p
          v-if="loading"
          class="text-muted-foreground text-sm"
          data-test="scheduler-loading"
        >
          {{ t('scheduler.loading') }}
        </p>

        <div
          v-else-if="loadError"
          class="flex flex-col items-start gap-2"
          data-test="scheduler-load-error"
        >
          <p class="text-destructive text-sm">{{ t('scheduler.loadError') }}</p>
          <Button variant="outline" size="sm" data-test="scheduler-retry" @click="load">
            <RefreshCw />
            {{ t('scheduler.retry') }}
          </Button>
        </div>

        <p
          v-else-if="schedules.length === 0"
          class="text-muted-foreground text-sm"
          data-test="scheduler-empty"
        >
          {{ t('scheduler.empty') }}
        </p>

        <Table v-else data-test="scheduler-table">
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('scheduler.columns.name') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.cron') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.command') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.enabled') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.nextRun') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.lastRun') }}</TableHead>
              <TableHead>{{ t('scheduler.columns.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="(schedule, index) in schedules"
              :key="schedule.id"
              :data-test="`scheduler-row-${index}`"
            >
              <TableCell>
                <div class="flex flex-col gap-1">
                  <span class="font-medium">{{ schedule.name }}</span>
                  <IdChip :id="schedule.id" class="text-[0.7rem] opacity-70" />
                </div>
              </TableCell>
              <TableCell>
                <code
                  class="bg-muted/60 rounded px-1.5 py-0.5 font-mono text-xs"
                  data-test="scheduler-cron"
                >
                  {{ schedule.cron_expr }}
                </code>
              </TableCell>
              <TableCell>
                <Badge variant="secondary">{{ commandLabel(schedule.command) }}</Badge>
              </TableCell>
              <TableCell>
                <Badge
                  :variant="schedule.enabled ? 'default' : 'outline'"
                  data-test="scheduler-enabled-badge"
                >
                  {{ schedule.enabled ? t('scheduler.enabled') : t('scheduler.disabled') }}
                </Badge>
              </TableCell>
              <TableCell class="text-muted-foreground text-xs">
                {{ schedule.next_run_at ? formatDateTime(schedule.next_run_at) : t('scheduler.never') }}
              </TableCell>
              <TableCell class="text-muted-foreground text-xs">
                {{ schedule.last_run_at ? formatDateTime(schedule.last_run_at) : t('scheduler.never') }}
              </TableCell>
              <TableCell>
                <div v-if="canControl" class="flex flex-wrap items-center gap-1">
                  <Button
                    variant="ghost"
                    size="xs"
                    :data-test="`scheduler-edit-${index}`"
                    @click="openEdit(schedule)"
                  >
                    <Pencil />
                    {{ t('scheduler.actions.edit') }}
                  </Button>

                  <Button
                    variant="ghost"
                    size="xs"
                    :disabled="busyId === schedule.id"
                    :data-test="schedule.enabled ? `scheduler-disable-${index}` : `scheduler-enable-${index}`"
                    @click="onToggle(schedule)"
                  >
                    <PowerOff v-if="schedule.enabled" />
                    <Power v-else />
                    {{ schedule.enabled ? t('scheduler.actions.disable') : t('scheduler.actions.enable') }}
                  </Button>

                  <Button
                    variant="ghost"
                    size="xs"
                    :disabled="busyId === schedule.id"
                    :data-test="`scheduler-run-${index}`"
                    @click="onRun(schedule)"
                  >
                    <Play />
                    {{ t('scheduler.actions.run') }}
                  </Button>

                  <template v-if="confirmingDeleteId === schedule.id">
                    <Button
                      variant="destructive"
                      size="xs"
                      :disabled="busyId === schedule.id"
                      :data-test="`scheduler-delete-confirm-${index}`"
                      @click="onDelete(schedule)"
                    >
                      <Trash2 />
                      {{ t('scheduler.actions.confirmDelete') }}
                    </Button>
                    <Button
                      variant="ghost"
                      size="xs"
                      :data-test="`scheduler-delete-cancel-${index}`"
                      @click="confirmingDeleteId = null"
                    >
                      {{ t('scheduler.actions.cancel') }}
                    </Button>
                  </template>
                  <Button
                    v-else
                    variant="destructive"
                    size="xs"
                    :disabled="busyId === schedule.id"
                    :data-test="`scheduler-delete-${index}`"
                    @click="askDelete(schedule)"
                  >
                    <Trash2 />
                    {{ t('scheduler.actions.delete') }}
                  </Button>
                </div>

                <span v-else class="text-muted-foreground text-xs">—</span>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </div>
</template>
