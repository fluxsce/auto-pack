<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsButton,
  RsCard,
  RsIcon,
  RsInput,
  RsPagination,
  RsSelect,
  RsTable,
  RsTag,
  useRsToast,
  type RsSelectModelValue,
  type RsTableColumn,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import { useWorkspaceStore } from '@/stores/workspace'
import PageFrame from '@/components/PageFrame.vue'

type Entry = {
  rec_id: string
  actor_user_id: string
  actor_login_name?: string
  actor_display_name?: string
  action_code: string
  target_type?: string
  target_id?: string
  summary: string
  success_flag: number
  created_at?: string
}

type ListPage = {
  items: Entry[]
  total: number
  page: number
  page_size: number
}

const actionI18n: Record<string, string> = {
  'job.submit': 'audit.actions.jobSubmit',
  'job.cancel': 'audit.actions.jobCancel',
  'project.save': 'audit.actions.projectSave',
  'project.clone': 'audit.actions.projectClone',
  'project.delete': 'audit.actions.projectDelete',
  'project.branch': 'audit.actions.projectBranch',
  'project.post': 'audit.actions.projectPost',
  'project.clear_files': 'audit.actions.projectClearFiles',
  'env.save': 'audit.actions.envSave',
  'env.delete': 'audit.actions.envDelete',
  'env.mail': 'audit.actions.envMail',
  'user.create': 'audit.actions.userCreate',
  'role.replace': 'audit.actions.roleReplace',
  'workspace.create': 'audit.actions.workspaceCreate',
  'workspace.save': 'audit.actions.workspaceSave',
  'workspace.clone': 'audit.actions.workspaceClone',
  'workspace.export': 'audit.actions.workspaceExport',
  'workspace.import': 'audit.actions.workspaceImport',
  'workspace.delete': 'audit.actions.workspaceDelete',
  'member.add': 'audit.actions.memberAdd',
  'member.remove': 'audit.actions.memberRemove',
  'settings.save': 'audit.actions.settingsSave',
  'settings.var.save': 'audit.actions.varSave',
  'settings.var.delete': 'audit.actions.varDelete',
}

const { t } = useI18n()
const toast = useRsToast()
const ws = useWorkspaceStore()
const rows = ref<Entry[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const ALL = 'all'

const loading = ref(false)
const action = ref(ALL)
const result = ref(ALL)
const draftQ = ref('')
const keyword = ref('')

const actionOptions = computed(() => [
  { value: ALL, label: t('audit.allActions') },
  ...Object.keys(actionI18n).map((code) => ({ value: code, label: actionLabel(code) })),
])

const resultOptions = computed(() => [
  { value: ALL, label: t('audit.allResults') },
  { value: 'ok', label: t('audit.ok') },
  { value: 'fail', label: t('audit.fail') },
])

const columns = computed<RsTableColumn<Entry>[]>(() => [
  { key: 'action_code', title: t('audit.action'), minWidth: 140 },
  { key: 'actor_user_id', title: t('audit.actor'), minWidth: 140 },
  { key: 'summary', title: t('audit.summary'), minWidth: 220 },
  { key: 'target_type', title: t('audit.target'), minWidth: 140 },
  { key: 'success_flag', title: t('audit.result'), width: 100 },
  { key: 'created_at', title: t('audit.time'), minWidth: 180 },
])

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null) return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}

function actionLabel(code: string): string {
  const key = actionI18n[code]
  if (!key) return code
  const label = t(key)
  return label === key ? code : label
}

function targetLabel(row: Entry): string {
  if (!row.target_type && !row.target_id) return t('common.dash')
  const typeKey = row.target_type ? `audit.targets.${row.target_type}` : ''
  const typeLabel = typeKey && t(typeKey) !== typeKey ? t(typeKey) : row.target_type
  if (!row.target_id) return typeLabel || t('common.dash')
  if (!typeLabel) return row.target_id
  return `${typeLabel} · ${shortID(row.target_id)}`
}

function actorName(row: Entry): string {
  return row.actor_display_name || row.actor_login_name || row.actor_user_id || t('common.dash')
}

function actorLogin(row: Entry): string {
  if (!row.actor_login_name) return ''
  if (row.actor_display_name && row.actor_display_name !== row.actor_login_name) {
    return row.actor_login_name
  }
  return ''
}

function shortID(id: string): string {
  return id.length > 12 ? `${id.slice(0, 8)}…` : id
}

function formatTime(value?: string): string {
  if (!value) return t('common.dash')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

async function load(): Promise<void> {
  if (!ws.currentId) return
  loading.value = true
  try {
    const out =
      (await request<ListPage>({
        method: 'GET',
        url: apiPath('audit'),
        params: {
          workspace_rec_id: ws.currentId,
          action_code: action.value === ALL ? undefined : action.value,
          result: result.value === ALL ? undefined : result.value,
          q: keyword.value || undefined,
          page: page.value,
          page_size: pageSize.value,
        },
      })) ?? { items: [], total: 0, page: 1, page_size: pageSize.value }
    rows.value = out.items ?? []
    total.value = out.total ?? 0
    if (out.page) page.value = out.page
    if (out.page_size) pageSize.value = out.page_size
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function applyKeyword(): void {
  const next = draftQ.value.trim()
  if (next === keyword.value && page.value === 1) {
    void load()
    return
  }
  keyword.value = next
  page.value = 1
  void load()
}

function refresh(): void {
  keyword.value = draftQ.value.trim()
  void load()
}

function onAction(value: RsSelectModelValue): void {
  action.value = asString(value)
  page.value = 1
  void load()
}

function onResult(value: RsSelectModelValue): void {
  result.value = asString(value)
  page.value = 1
  void load()
}

function onPage(value: number): void {
  page.value = value
  void load()
}

function onPageSize(value: number): void {
  pageSize.value = value
  page.value = 1
  void load()
}

onMounted(load)
watch(draftQ, (value) => {
  if (value.trim() === '' && keyword.value) {
    keyword.value = ''
    page.value = 1
    void load()
  }
})
watch(() => ws.currentId, () => {
  action.value = ALL
  result.value = ALL
  draftQ.value = ''
  keyword.value = ''
  page.value = 1
  void load()
})
</script>

<template>
  <PageFrame :title="t('audit.title')" :desc="t('audit.desc')">
    <RsCard variant="outlined" elevated radius="lg" :padding="false" class="audit-panel">
      <div class="audit-toolbar">
        <RsSelect
          class="audit-toolbar__action"
          :model-value="action"
          :options="actionOptions"
          :placeholder="t('audit.allActions')"
          @update:model-value="onAction"
        />
        <RsSelect
          class="audit-toolbar__result"
          :model-value="result"
          :options="resultOptions"
          :placeholder="t('audit.allResults')"
          @update:model-value="onResult"
        />
        <RsInput
          v-model="draftQ"
          class="audit-toolbar__search"
          clearable
          :placeholder="t('audit.keywordPlaceholder')"
          :aria-label="t('audit.keyword')"
          @keydown.enter.prevent="applyKeyword"
        >
          <template #prefix>
            <RsIcon name="search" />
          </template>
        </RsInput>
        <RsButton variant="ghost" :loading="loading" @click="refresh">
          {{ t('common.refresh') }}
        </RsButton>
      </div>
      <RsTable
        v-if="rows.length || loading"
        :columns="columns"
        :data="rows"
        :loading="loading"
        row-key="rec_id"
        :bordered="false"
      >
        <template #action_code="{ row }">{{ actionLabel(row.action_code) }}</template>
        <template #actor_user_id="{ row }">
          <span class="audit-actor">
            <span>{{ actorName(row) }}</span>
            <span v-if="actorLogin(row)" class="audit-actor__login">{{ actorLogin(row) }}</span>
          </span>
        </template>
        <template #target_type="{ row }">{{ targetLabel(row) }}</template>
        <template #success_flag="{ row }">
          <RsTag :variant="row.success_flag ? 'success' : 'danger'" round>
            {{ row.success_flag ? t('audit.ok') : t('audit.fail') }}
          </RsTag>
        </template>
        <template #created_at="{ row }">{{ formatTime(row.created_at) }}</template>
      </RsTable>
      <div v-else class="page__quiet">
        <p>{{ t('audit.empty') }}</p>
        <p class="audit-empty__hint">{{ t('audit.emptyHint') }}</p>
      </div>
      <div v-if="total > 0" class="page__pager">
        <RsPagination
          :page="page"
          :page-size="pageSize"
          :total="total"
          show-page-size
          @update:page="onPage"
          @update:page-size="onPageSize"
        />
      </div>
    </RsCard>
  </PageFrame>
</template>

<style scoped>
.audit-panel {
  overflow: hidden;
}

.audit-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: var(--rs-space-sm);
  padding: var(--rs-space-md) var(--rs-space-lg);
  border-bottom: 1px solid var(--shell-border);
}

.audit-toolbar__action {
  flex: 0 0 calc(var(--rs-control-height-md) * 7);
  width: calc(var(--rs-control-height-md) * 7);
}

.audit-toolbar__result {
  flex: 0 0 calc(var(--rs-control-height-md) * 5);
  width: calc(var(--rs-control-height-md) * 5);
}

.audit-toolbar__search {
  flex: 0 1 calc(var(--rs-control-height-md) * 12);
  width: calc(var(--rs-control-height-md) * 12);
  min-width: calc(var(--rs-control-height-md) * 8);
  margin-inline-start: auto;
}

.audit-actor {
  display: flex;
  flex-direction: column;
  gap: var(--rs-space-xs);
  line-height: var(--rs-line-height-tight);
}

.audit-actor__login,
.audit-empty__hint {
  font-size: var(--rs-font-size-xs);
  color: var(--shell-muted);
}

.audit-empty__hint {
  margin: var(--rs-space-xs) 0 0;
}

.page__quiet p {
  margin: 0;
}
</style>
