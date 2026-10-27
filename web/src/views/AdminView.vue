<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'

import { ApiError } from '@/api/client'
import {
  createUser,
  deleteUser,
  getStats,
  listErrorLogs,
  listUsers,
  updateUser,
} from '@/api/admin'
import IdChip from '@/components/IdChip.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import TablePager from '@/components/TablePager.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useAdminUsers } from '@/composables/useAdminUsers'
import { useAuthStore } from '@/stores/auth'
import { fieldsJson, formatUptime, logLevelVariant, logTime, mapUserDeleteError, mapUserSaveError, roleBadgeVariant } from '@/lib/admin'
import { formatDateTime } from '@/lib/datetime'
import type { AdminStats, AdminUser, LogEntry } from '@/types/admin'
import type { UserRole } from '@/types/auth'

/**
 * Panel de administración (Issue #23).
 *
 * Tres pestañas: Usuarios (CRUD con guards del backend), Logs de error (JSON log
 * del zap) y Stats (métricas). Solo accesible a `admin` (router + backend).
 *
 * IDs OPACOS: los tokens `v1.*` nunca se parsean; las acciones usan el índice
 * de fila como `data-test` (`admin-user-edit-0`) y el token íntegro va en `IdChip`.
 */
const { t } = useI18n()
const auth = useAuthStore()

type Tab = 'users' | 'logs' | 'stats'
const activeTab = ref<Tab>('users')

// --- Usuarios ---
const users = useAdminUsers()
const roleOptions: (UserRole | '')[] = ['', 'admin', 'operator', 'viewer']
const activeOptions = [
  { value: '', label: '' },
  { value: 'true', label: 'true' },
  { value: 'false', label: 'false' },
]

type FormMode = 'create' | 'edit'
const formOpen = ref(false)
const formMode = ref<FormMode>('create')
const editingId = ref<string | null>(null)
const formSaving = ref(false)
const form = reactive({
  email: '',
  password: '',
  role: 'viewer' as UserRole,
  active: true,
})
const fieldErrors = ref<Record<string, string>>({})
const formError = ref<string | null>(null)

function resetForm(): void {
  form.email = ''
  form.password = ''
  form.role = 'viewer'
  form.active = true
  editingId.value = null
  fieldErrors.value = {}
  formError.value = null
}

function openCreate(): void {
  formMode.value = 'create'
  resetForm()
  formOpen.value = true
}

function openEdit(row: AdminUser, index: number): void {
  formMode.value = 'edit'
  editingId.value = row.id
  form.email = row.email
  form.password = ''
  form.role = row.role
  form.active = row.active
  fieldErrors.value = {}
  formError.value = null
  formOpen.value = true
  ;(document.querySelector(`[data-test="admin-user-edit-${index}"]`) as HTMLElement | null)?.focus()
}

function closeForm(): void {
  formOpen.value = false
  resetForm()
}

function buildUpdatePayload(): Record<string, unknown> {
  const payload: Record<string, unknown> = {}
  const current = users.result.value?.items.find((u: AdminUser) => u.id === editingId.value)
  if (!current) return payload
  if (form.email.trim() !== current.email) payload.email = form.email.trim()
  if (form.password.trim() !== '') payload.password = form.password
  if (form.role !== current.role) payload.role = form.role
  if (form.active !== current.active) payload.active = form.active
  return payload
}

async function submitForm(): Promise<void> {
  fieldErrors.value = {}
  formError.value = null
  formSaving.value = true
  try {
    if (formMode.value === 'create') {
      await createUser({
        email: form.email.trim(),
        password: form.password,
        role: form.role,
        active: form.active,
      })
      formOpen.value = false
      resetForm()
      await users.refresh()
    } else {
      const payload = buildUpdatePayload()
      if (Object.keys(payload).length === 0) {
        const mapped = mapUserSaveError(
          new ApiError({ status: 400, code: 'bad_request', message: 'no changes' }),
        )
        fieldErrors.value = mapped.fields
        formError.value = mapped.form
        formSaving.value = false
        return
      }
      await updateUser(editingId.value as string, payload)
      formOpen.value = false
      resetForm()
      await users.refresh()
    }
  } catch (err) {
    const mapped = mapUserSaveError(err)
    fieldErrors.value = mapped.fields
    formError.value = mapped.form
  } finally {
    formSaving.value = false
  }
}

const deletingId = ref<string | null>(null)
const deleteError = ref<string | null>(null)
const confirmDeleteId = ref<string | null>(null)

function askDelete(id: string): void {
  confirmDeleteId.value = id
  deleteError.value = null
}

function cancelDelete(): void {
  confirmDeleteId.value = null
}

async function doDelete(id: string): Promise<void> {
  deletingId.value = id
  deleteError.value = null
  try {
    await users.remove(id)
    confirmDeleteId.value = null
    if ((users.result.value?.items.length ?? 0) === 0 && users.page.value > 1) {
      await users.goToPage(users.page.value - 1)
    }
  } catch (err) {
    deleteError.value = mapUserDeleteError(err)
  } finally {
    deletingId.value = null
  }
}

function isSelf(row: AdminUser): boolean {
  return auth.user?.email?.toLowerCase() === row.email.toLowerCase()
}

// --- Logs ---
const logs = ref<LogEntry[]>([])
const logsLoading = ref(false)
const logsError = ref<string | null>(null)
const logsPage = ref(1)
const logsPerPage = ref(20)
const logsTotal = ref(0)
const logsTotalPages = ref(0)
const expandedLogs = ref<Set<number>>(new Set())

async function loadLogs(): Promise<void> {
  logsLoading.value = true
  logsError.value = null
  try {
    const res = await listErrorLogs({ page: logsPage.value, per_page: logsPerPage.value })
    logs.value = res.items
    logsTotal.value = res.total
    logsTotalPages.value = res.total_pages
    expandedLogs.value = new Set()
  } catch (err) {
    logsError.value = err instanceof Error ? err.message : String(err)
  } finally {
    logsLoading.value = false
  }
}

function toggleLogFields(index: number): void {
  const next = new Set(expandedLogs.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  expandedLogs.value = next
}

function onLogsPage(page: number): void {
  logsPage.value = page
  void loadLogs()
}

function onLogsPerPage(perPage: number): void {
  logsPerPage.value = perPage
  logsPage.value = 1
  void loadLogs()
}

// --- Stats ---
const stats = ref<AdminStats | null>(null)
const statsLoading = ref(false)
const statsError = ref<string | null>(null)

async function loadStats(): Promise<void> {
  statsLoading.value = true
  statsError.value = null
  try {
    stats.value = await getStats()
  } catch (err) {
    statsError.value = err instanceof Error ? err.message : String(err)
  } finally {
    statsLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'logs' && logs.value.length === 0 && !logsLoading.value) {
    void loadLogs()
  }
  if (tab === 'stats' && !stats.value && !statsLoading.value) {
    void loadStats()
  }
})

onMounted(() => {
  // Los usuarios ya se cargan vía useDataList init; los demás se cargan al abrir pestaña.
})

const formTitle = computed(() => (formMode.value === 'create' ? t('admin.users.form.createTitle') : t('admin.users.form.editTitle')))
</script>

<template>
  <div class="space-y-6" data-test="admin-view">
    <header class="space-y-1">
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('nav.admin.title') }}</h1>
      <p class="text-muted-foreground">{{ t('nav.admin.description') }}</p>
    </header>

    <div class="flex flex-wrap gap-2" role="tablist" :aria-label="t('nav.admin.title')">
      <Button
        :variant="activeTab === 'users' ? 'default' : 'outline'"
        size="sm"
        role="tab"
        :aria-selected="activeTab === 'users'"
        data-test="admin-tab-users"
        @click="activeTab = 'users'"
      >
        {{ t('admin.tabs.users') }}
      </Button>
      <Button
        :variant="activeTab === 'logs' ? 'default' : 'outline'"
        size="sm"
        role="tab"
        :aria-selected="activeTab === 'logs'"
        data-test="admin-tab-logs"
        @click="activeTab = 'logs'"
      >
        {{ t('admin.tabs.logs') }}
      </Button>
      <Button
        :variant="activeTab === 'stats' ? 'default' : 'outline'"
        size="sm"
        role="tab"
        :aria-selected="activeTab === 'stats'"
        data-test="admin-tab-stats"
        @click="activeTab = 'stats'"
      >
        {{ t('admin.tabs.stats') }}
      </Button>
    </div>

    <!-- Usuarios -->
    <section v-if="activeTab === 'users'" class="space-y-4" data-test="admin-panel-users">
      <Card>
        <CardHeader class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-1">
            <CardTitle>{{ t('admin.users.title') }}</CardTitle>
            <CardDescription>{{ t('admin.users.subtitle') }}</CardDescription>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" :disabled="users.loading.value" data-test="admin-users-refresh" @click="users.refresh()">
              <RefreshCw class="mr-2 h-4 w-4" />
              {{ t('admin.users.refresh') }}
            </Button>
            <Button size="sm" data-test="admin-users-new" @click="openCreate">
              <Plus class="mr-2 h-4 w-4" />
              {{ t('admin.users.new') }}
            </Button>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Input
              :model-value="users.filters.value.q ?? ''"
              :placeholder="t('table.search')"
              data-test="admin-users-filter-q"
              @update:model-value="users.setFilters({ ...users.filters.value, q: String($event ?? '') })"
            />
            <select
              class="border-input bg-background h-9 rounded-lg border px-3 text-sm"
              :value="users.filters.value.role ?? ''"
              data-test="admin-users-filter-role"
              @change="users.setFilters({ ...users.filters.value, role: ( $event.target as HTMLSelectElement ).value })"
            >
              <option v-for="opt in roleOptions" :key="opt || 'all'" :value="opt">{{ opt || t('table.filters') }}</option>
            </select>
            <select
              class="border-input bg-background h-9 rounded-lg border px-3 text-sm"
              :value="users.filters.value.active ?? ''"
              data-test="admin-users-filter-active"
              @change="users.setFilters({ ...users.filters.value, active: ( $event.target as HTMLSelectElement ).value })"
            >
              <option value="">{{ t('table.filters') }}</option>
              <option value="true">{{ t('admin.users.status.active') }}</option>
              <option value="false">{{ t('admin.users.status.inactive') }}</option>
            </select>
          </div>

          <p v-if="users.error.value" class="text-destructive flex items-center gap-2 text-sm" role="alert">
            <AlertTriangle class="h-4 w-4" />
            {{ t('admin.users.loadError') }}
            <Button variant="link" size="sm" class="px-1" @click="users.refresh()">{{ t('admin.users.retry') }}</Button>
          </p>
          <p v-else-if="users.loading.value && (users.result.value?.items.length ?? 0) === 0" class="text-muted-foreground text-sm">{{ t('admin.users.loading') }}</p>
          <p v-else-if="(users.result.value?.items.length ?? 0) === 0" class="text-muted-foreground text-sm">{{ t('admin.users.empty') }}</p>

          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm" data-test="admin-users-table">
              <thead>
                <tr class="text-muted-foreground border-b text-left">
                  <th class="py-2 pr-4 font-medium">{{ t('admin.users.columns.id') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.users.columns.email') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.users.columns.role') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.users.columns.active') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.users.columns.createdAt') }}</th>
                  <th class="py-2 font-medium">{{ t('admin.users.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, index) in users.result.value?.items" :key="row.id" class="border-b" :data-test="`admin-user-row-${index}`">
                  <td class="py-2 pr-4"><IdChip :id="row.id" /></td>
                  <td class="py-2 pr-4">{{ row.email }}</td>
                  <td class="py-2 pr-4"><StatusBadge :status="row.role" :label="row.role" :variant="roleBadgeVariant(row.role)" /></td>
                  <td class="py-2 pr-4">
                    <span class="text-sm">{{ row.active ? t('admin.users.status.active') : t('admin.users.status.inactive') }}</span>
                  </td>
                  <td class="py-2 pr-4">{{ formatDateTime(row.created_at) }}</td>
                  <td class="py-2 flex flex-wrap items-center gap-2">
                    <Button variant="outline" size="sm" :data-test="`admin-user-edit-${index}`" @click="openEdit(row, index)">
                      <Pencil class="mr-1 h-3.5 w-3.5" />
                      {{ t('admin.users.actions.edit') }}
                    </Button>
                    <template v-if="confirmDeleteId === row.id">
                      <Button variant="destructive" size="sm" :disabled="deletingId === row.id || isSelf(row)" :data-test="`admin-user-delete-confirm-${index}`" @click="doDelete(row.id)">
                        {{ t('admin.users.actions.confirmDelete') }}
                      </Button>
                      <Button variant="ghost" size="sm" @click="cancelDelete">{{ t('admin.users.actions.cancel') }}</Button>
                    </template>
                    <Button v-else variant="outline" size="sm" :disabled="isSelf(row)" :data-test="`admin-user-delete-${index}`" @click="askDelete(row.id)">
                      <Trash2 class="mr-1 h-3.5 w-3.5" />
                      {{ t('admin.users.actions.delete') }}
                    </Button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <TablePager
            v-if="(users.result.value?.total_pages ?? 0) > 0"
            :page="users.page.value"
            :per-page="users.perPage.value"
            :total="users.result.value?.total ?? 0"
            :total-pages="users.result.value?.total_pages ?? 0"
            test-prefix="admin-users-pager"
            @update:page="users.goToPage"
            @update:per-page="users.setPerPage"
          />
          <p v-if="deleteError" class="text-destructive text-sm" role="alert">{{ t(deleteError) }}</p>
        </CardContent>
      </Card>

      <Card v-if="formOpen" data-test="admin-user-form">
        <CardHeader>
          <CardTitle class="text-base">{{ formTitle }}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1">
              <label class="text-sm font-medium" for="admin-user-email">{{ t('admin.users.form.email') }}</label>
              <Input id="admin-user-email" v-model="form.email" :placeholder="t('admin.users.form.emailPlaceholder')" data-test="admin-user-email" />
              <p v-if="fieldErrors.email" class="text-destructive text-xs">{{ t(fieldErrors.email) }}</p>
            </div>
            <div class="space-y-1">
              <label class="text-sm font-medium" for="admin-user-password">{{ t('admin.users.form.password') }}</label>
              <Input id="admin-user-password" v-model="form.password" type="password" data-test="admin-user-password" />
              <p class="text-muted-foreground text-xs">{{ formMode === 'create' ? t('admin.users.form.passwordCreateHint') : t('admin.users.form.passwordEditHint') }}</p>
              <p v-if="fieldErrors.password" class="text-destructive text-xs">{{ t(fieldErrors.password) }}</p>
            </div>
            <div class="space-y-1">
              <label class="text-sm font-medium" for="admin-user-role">{{ t('admin.users.form.role') }}</label>
              <select id="admin-user-role" v-model="form.role" class="border-input bg-background h-9 rounded-lg border px-3 text-sm" data-test="admin-user-role">
                <option value="admin">admin</option>
                <option value="operator">operator</option>
                <option value="viewer">viewer</option>
              </select>
              <p v-if="fieldErrors.role" class="text-destructive text-xs">{{ t(fieldErrors.role) }}</p>
            </div>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.active" type="checkbox" data-test="admin-user-active" />
              {{ t('admin.users.form.active') }}
            </label>
          </div>
          <p v-if="formError" class="text-destructive text-sm" role="alert">{{ t(formError) }}</p>
          <div class="flex flex-wrap gap-2">
            <Button :disabled="formSaving" data-test="admin-user-save" @click="submitForm">
              {{ formSaving ? t('admin.users.form.saving') : t('admin.users.form.save') }}
            </Button>
            <Button variant="outline" :disabled="formSaving" data-test="admin-user-cancel" @click="closeForm">
              {{ t('admin.users.form.cancel') }}
            </Button>
          </div>
        </CardContent>
      </Card>
    </section>

    <!-- Logs -->
    <section v-if="activeTab === 'logs'" class="space-y-4" data-test="admin-panel-logs">
      <Card>
        <CardHeader class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-1">
            <CardTitle>{{ t('admin.logs.title') }}</CardTitle>
            <CardDescription>{{ t('admin.logs.subtitle') }}</CardDescription>
          </div>
          <Button variant="outline" size="sm" :disabled="logsLoading" data-test="admin-logs-refresh" @click="loadLogs">
            <RefreshCw class="mr-2 h-4 w-4" />
            {{ t('admin.logs.refresh') }}
          </Button>
        </CardHeader>
        <CardContent class="space-y-4">
          <p v-if="logsError" class="text-destructive flex items-center gap-2 text-sm" role="alert">
            <AlertTriangle class="h-4 w-4" />
            {{ t('admin.logs.loadError') }}
            <Button variant="link" size="sm" class="px-1" @click="loadLogs">{{ t('admin.logs.retry') }}</Button>
          </p>
          <p v-else-if="logsLoading && logs.length === 0" class="text-muted-foreground text-sm">{{ t('admin.logs.loading') }}</p>
          <p v-else-if="logs.length === 0" class="text-muted-foreground text-sm">{{ t('admin.logs.empty') }}</p>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm" data-test="admin-logs-table">
              <thead>
                <tr class="text-muted-foreground border-b text-left">
                  <th class="py-2 pr-4 font-medium">{{ t('admin.logs.columns.time') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.logs.columns.level') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.logs.columns.message') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('admin.logs.columns.caller') }}</th>
                  <th class="py-2 font-medium">{{ t('admin.logs.columns.fields') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(entry, index) in logs" :key="index" class="border-b align-top" :data-test="`admin-log-row-${index}`">
                  <td class="py-2 pr-4 whitespace-nowrap">{{ logTime(entry.time) }}</td>
                  <td class="py-2 pr-4"><StatusBadge :status="entry.level" :label="entry.level" :variant="logLevelVariant(entry.level)" /></td>
                  <td class="py-2 pr-4 max-w-[28rem]"><TruncatedCell :value="entry.message" /></td>
                  <td class="py-2 pr-4 max-w-[16rem]"><TruncatedCell :value="entry.caller ?? ''" /></td>
                  <td class="py-2">
                    <Button v-if="fieldsJson(entry.fields)" variant="ghost" size="sm" :data-test="`admin-log-fields-${index}`" @click="toggleLogFields(index)">
                      {{ expandedLogs.has(index) ? t('admin.logs.hideFields') : t('admin.logs.showFields') }}
                    </Button>
                    <pre v-if="expandedLogs.has(index) && fieldsJson(entry.fields)" class="bg-muted mt-2 max-w-[32rem] overflow-x-auto rounded-md p-2 text-xs">{{ fieldsJson(entry.fields) }}</pre>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <TablePager
            v-if="logsTotalPages > 0"
            :page="logsPage"
            :per-page="logsPerPage"
            :total="logsTotal"
            :total-pages="logsTotalPages"
            test-prefix="admin-logs-pager"
            @update:page="onLogsPage"
            @update:per-page="onLogsPerPage"
          />
        </CardContent>
      </Card>
    </section>

    <!-- Stats -->
    <section v-if="activeTab === 'stats'" class="space-y-4" data-test="admin-panel-stats">
      <Card>
        <CardHeader class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-1">
            <CardTitle>{{ t('admin.stats.title') }}</CardTitle>
            <CardDescription>{{ t('admin.stats.subtitle') }}</CardDescription>
          </div>
          <Button variant="outline" size="sm" :disabled="statsLoading" data-test="admin-stats-refresh" @click="loadStats">
            <RefreshCw class="mr-2 h-4 w-4" />
            {{ t('admin.stats.refresh') }}
          </Button>
        </CardHeader>
        <CardContent class="space-y-6">
          <p v-if="statsError" class="text-destructive flex items-center gap-2 text-sm" role="alert">
            <AlertTriangle class="h-4 w-4" />
            {{ t('admin.stats.loadError') }}
            <Button variant="link" size="sm" class="px-1" @click="loadStats">{{ t('admin.stats.retry') }}</Button>
          </p>
          <p v-else-if="statsLoading && !stats" class="text-muted-foreground text-sm">{{ t('admin.stats.loading') }}</p>
          <div v-else-if="stats" class="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle class="text-base">{{ t('admin.stats.users') }}</CardTitle>
              </CardHeader>
              <CardContent class="space-y-2 text-sm">
                <div class="flex justify-between"><span>{{ t('admin.stats.usersTotal') }}</span><span>{{ stats.users.total }}</span></div>
                <div class="flex justify-between"><span>{{ t('admin.stats.usersActive') }}</span><span>{{ stats.users.active }}</span></div>
                <div class="space-y-1">
                  <p class="text-muted-foreground">{{ t('admin.stats.usersByRole') }}</p>
                  <div class="flex justify-between"><span>admin</span><span>{{ stats.users.by_role.admin ?? 0 }}</span></div>
                  <div class="flex justify-between"><span>operator</span><span>{{ stats.users.by_role.operator ?? 0 }}</span></div>
                  <div class="flex justify-between"><span>viewer</span><span>{{ stats.users.by_role.viewer ?? 0 }}</span></div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle class="text-base">{{ t('admin.stats.entities') }}</CardTitle>
              </CardHeader>
              <CardContent class="space-y-3 text-sm">
                <div>
                  <p class="text-muted-foreground mb-1">{{ t('admin.stats.source') }}</p>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.customers') }}</span><span>{{ stats.source.customers }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.products') }}</span><span>{{ stats.source.products }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.orders') }}</span><span>{{ stats.source.orders }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.orderItems') }}</span><span>{{ stats.source.order_items }}</span></div>
                </div>
                <div>
                  <p class="text-muted-foreground mb-1">{{ t('admin.stats.target') }}</p>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.customers') }}</span><span>{{ stats.target.customers }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.products') }}</span><span>{{ stats.target.products }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.orders') }}</span><span>{{ stats.target.orders }}</span></div>
                  <div class="flex justify-between"><span>{{ t('admin.stats.entity.orderItems') }}</span><span>{{ stats.target.order_items }}</span></div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle class="text-base">{{ t('admin.stats.jobs') }}</CardTitle>
              </CardHeader>
              <CardContent class="space-y-2 text-sm">
                <div class="flex justify-between"><span>{{ t('admin.stats.jobsTotal') }}</span><span>{{ stats.jobs.total }}</span></div>
                <div class="space-y-1">
                  <p class="text-muted-foreground">{{ t('admin.stats.jobsByStatus') }}</p>
                  <div v-for="(count, status) in stats.jobs.by_status" :key="status" class="flex justify-between">
                    <span>{{ status }}</span><span>{{ count }}</span>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle class="text-base">{{ t('admin.stats.schedules') }}</CardTitle>
              </CardHeader>
              <CardContent class="space-y-2 text-sm">
                <div class="flex justify-between"><span>{{ t('admin.stats.schedulesTotal') }}</span><span>{{ stats.schedules.total }}</span></div>
                <div class="flex justify-between"><span>{{ t('admin.stats.schedulesEnabled') }}</span><span>{{ stats.schedules.enabled }}</span></div>
              </CardContent>
            </Card>

            <Card class="lg:col-span-2">
              <CardHeader>
                <CardTitle class="text-base">{{ t('admin.stats.server') }}</CardTitle>
              </CardHeader>
              <CardContent class="grid gap-2 text-sm sm:grid-cols-2">
                <div class="flex justify-between"><span>{{ t('admin.stats.uptime') }}</span><span>{{ formatUptime(stats.server.uptime_seconds) }}</span></div>
                <div class="flex justify-between"><span>{{ t('admin.stats.goVersion') }}</span><span>{{ stats.server.go_version }}</span></div>
                <div class="flex justify-between sm:col-span-2"><span>{{ t('admin.stats.startedAt') }}</span><span>{{ formatDateTime(stats.server.started_at) }}</span></div>
              </CardContent>
            </Card>
          </div>
        </CardContent>
      </Card>
    </section>
  </div>
</template>
