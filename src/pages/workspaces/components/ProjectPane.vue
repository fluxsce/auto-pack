<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RsButton, RsCard, RsIcon, RsInput, RsSelect, RsTable, RsTag } from '@/ui'
import AutoBuildDialog from '../auto-build/Dialog.vue'
import AutoPushDialog from '../auto-push/Dialog.vue'
import AutoReleaseDialog from '../auto-release/Dialog.vue'
import type { Project } from '../api'
import {
  formatStamp,
  projectSource,
  statusTone,
  statusVariant,
  typeLabel,
  typeVariant,
  useProjectList,
} from '../composables/use-project-list'

const { t } = useI18n()
const {
  current,
  projects,
  filtered,
  query,
  typeFilter,
  statusFilter,
  typeOptions,
  statusOptions,
  loading,
  busyId,
  columns,
  onTypeFilter,
  onStatusFilter,
  load,
  create,
  open,
  edit,
  clone,
  remove,
  menuItems,
  onMenu,
  autoBuildOpen,
  autoReleaseOpen,
  autoPushOpen,
  openAutoBuild,
  openAutoRelease,
  openAutoPush,
} = useProjectList()

function statusLabel(code?: string): string {
  if (!code) return t('workspaces.neverBuilt')
  const key = `jobs.statuses.${code}`
  const label = t(key)
  return label === key ? code : label
}

function rowTime(row: { last_job_at?: string; updated_at?: string; created_at?: string }): string {
  return formatStamp(row.last_job_at || row.updated_at || row.created_at, t('common.dash'))
}

function repoCell(row: Project) {
  return projectSource(row)
}
</script>

<template>
  <div class="ws-pane">
    <RsCard
      class="ws-pane__card"
      fill
      variant="outlined"
      elevated
      radius="lg"
      :title="current ? t('workspaces.projectsTitleNamed', { name: current.workspace_name }) : t('workspaces.projectsTitle')"
      :padding="false"
    >
      <template #actions>
        <RsButton size="sm" variant="ghost" tone="info" :disabled="!current" @click="openAutoBuild">{{ t('workspaces.autoBuild') }}</RsButton>
        <RsButton size="sm" variant="ghost" tone="success" :disabled="!current" @click="openAutoRelease">{{ t('workspaces.autoRelease') }}</RsButton>
        <RsButton size="sm" variant="ghost" tone="warning" :disabled="!current" @click="openAutoPush">{{ t('workspaces.autoPush') }}</RsButton>
        <RsButton size="sm" variant="ghost" tone="primary" :loading="loading" @click="load">{{ t('common.refresh') }}</RsButton>
        <RsButton size="sm" :disabled="!current" @click="create">{{ t('workspaces.newProject') }}</RsButton>
      </template>
      <div class="ws-toolbar">
        <RsSelect
          class="ws-toolbar__type"
          :model-value="typeFilter"
          :options="typeOptions"
          clearable
          :placeholder="t('workspaces.allTypes')"
          :aria-label="t('workspaces.type')"
          @update:model-value="onTypeFilter"
        />
        <RsSelect
          class="ws-toolbar__status"
          :model-value="statusFilter"
          :options="statusOptions"
          clearable
          :placeholder="t('workspaces.allStatuses')"
          :aria-label="t('workspaces.buildStatus')"
          @update:model-value="onStatusFilter"
        />
        <div class="ws-toolbar__search">
          <RsInput
            v-model="query"
            clearable
            :placeholder="t('workspaces.searchProjectPlaceholder')"
            :aria-label="t('workspaces.searchProjects')"
          >
            <template #prefix>
              <RsIcon name="search" />
            </template>
          </RsInput>
        </div>
      </div>
      <RsTable
        v-if="filtered.length || loading"
        fill
        cell-focus="none"
        :columns="columns"
        :data="filtered"
        :loading="loading"
        row-key="rec_id"
        :bordered="false"
        :cell-tooltip="true"
        :context-menu-items="menuItems"
        @row-dblclick="open"
        @context-menu-select="onMenu"
      >
        <template #project_name="{ row }">
          <div class="ws-proj-name">
            <button type="button" class="ws-proj-link" @click="open(row)">{{ row.project_name }}</button>
            <RsTag v-if="row.is_enabled === 0" variant="warning" round>{{ t('envs.disabled') }}</RsTag>
          </div>
        </template>
        <template #project_type="{ row }">
          <RsTag :variant="typeVariant(row.project_type)" round>{{ typeLabel(row.project_type) }}</RsTag>
        </template>
        <template #repo_branch="{ row }">
          <span v-if="repoCell(row).branch" class="ws-branch">{{ repoCell(row).branch }}</span>
          <span v-else class="ws-muted">{{ t('common.dash') }}</span>
        </template>
        <template #last_job_status="{ row }">
          <RsTag :variant="statusVariant(row.last_job_status)" round>{{ statusLabel(row.last_job_status) }}</RsTag>
        </template>
        <template #last_job_at="{ row }">
          <span class="ws-time" :class="`is-${statusTone(row.last_job_status)}`">{{ rowTime(row) }}</span>
        </template>
        <template #cron_expr="{ row }">
          <span v-if="row.cron_expr" class="ws-cron">{{ row.cron_expr }}</span>
          <span v-else class="ws-muted">{{ t('common.dash') }}</span>
        </template>
        <template #actions="{ row }">
          <RsButton size="sm" variant="text" tone="primary" @click="edit(row)">{{ t('common.configure') }}</RsButton>
          <RsButton size="sm" variant="text" tone="success" :loading="busyId === row.rec_id" @click="clone(row)">
            {{ t('workspaces.cloneProject') }}
          </RsButton>
          <RsButton size="sm" variant="text" tone="danger" :loading="busyId === row.rec_id" @click="remove(row)">
            {{ t('common.delete') }}
          </RsButton>
        </template>
      </RsTable>
      <p v-else class="page__quiet">{{ projects.length ? t('workspaces.emptyProjectSearch') : t('workspaces.empty') }}</p>
    </RsCard>
    <AutoBuildDialog v-model:open="autoBuildOpen" :workspace="current" @success="load" />
    <AutoReleaseDialog v-model:open="autoReleaseOpen" :workspace="current" @success="load" />
    <AutoPushDialog v-model:open="autoPushOpen" :workspace="current" @success="load" />
  </div>
</template>

<style scoped>
.ws-pane {
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.ws-pane__card {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.ws-pane__card :deep(.rs-card__body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.ws-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  flex-shrink: 0;
  gap: 0.55rem;
  padding: 0.75rem 0.85rem 0.5rem;
}

.ws-toolbar__type,
.ws-toolbar__status {
  flex: 0 0 8.5rem;
  width: 8.5rem;
}

.ws-toolbar__search {
  flex: 0 0 16rem;
  width: 16rem;
  max-width: 16rem;
}

.ws-pane__card :deep(.rs-table-shell--fill) {
  flex: 1 1 auto;
  min-height: 0;
}

.page__quiet {
  flex: 1;
}

.ws-proj-name {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
}

.ws-proj-link {
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 650;
  text-align: left;
  cursor: pointer;
}

.ws-proj-link:hover {
  color: #0ea5e9;
  text-decoration: underline;
}

.ws-branch,
.ws-cron {
  font-family: var(--rs-font-mono);
  font-size: 0.8rem;
  font-weight: 650;
  color: #0369a1;
}

.ws-time {
  font-variant-numeric: tabular-nums;
  font-weight: 650;
}

.ws-time.is-ok {
  color: #15803d;
}

.ws-time.is-fail {
  color: #b91c1c;
}

.ws-time.is-run {
  color: #0284c7;
}

.ws-time.is-warn {
  color: #b45309;
}

.ws-time.is-muted,
.ws-muted {
  color: var(--shell-muted);
  font-weight: 500;
}

:deep(.rs-tag--info),
:deep(.rs-tag--primary),
:deep(.rs-tag--success),
:deep(.rs-tag--warning) {
  font-weight: 700;
}
</style>
