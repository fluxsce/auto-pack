<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { ArrowDown, CircleCheckFilled, CircleCloseFilled } from '@element-plus/icons-vue'
import type { PackProject, ProjectType } from '@/types'
import { buildStatusLabel, buildStatusTagType, listBuildOutcome } from '../project-build-status'
import { formatBuildTime, lastBuildAtSortKey } from '../project-format'
import { projectTypeTagClass } from '../project-meta'

const props = defineProps<{
  loading: boolean
  items: PackProject[]
  projectTypes: ProjectType[]
  activeType: string
  hasListFilters: boolean
  typeLabel: (type: unknown) => string
  workspaceName?: string
}>()

const emit = defineEmits<{
  open: [row: PackProject]
  build: [row: PackProject]
  analyzeDeps: [row: PackProject]
  clone: [row: PackProject]
  edit: [row: PackProject]
  delete: [row: PackProject]
}>()

function isMavenProject(row: PackProject): boolean {
  return String(row.project_type).toLowerCase() === 'maven'
}

const activeType = defineModel<string>('activeType', { required: true })

const { t } = useI18n()

const listItems = computed(() => props.items ?? [])

const showEmpty = computed(() => !props.loading && listItems.value.length === 0)
const emptyTitle = computed(() =>
  props.hasListFilters ? t('projectEmptyFilter') : t('projectEmptyList'),
)
const emptyHint = computed(() => (props.hasListFilters ? '' : t('projectEmptyListHint')))

function extractBuildAt(v: unknown): unknown {
  if (v && typeof v === 'object' && 'last_build_at' in v) {
    return (v as { last_build_at?: unknown }).last_build_at
  }
  return v
}

/** Element Plus passes row objects to sort-method; be defensive anyway. */
function compareLastBuildAt(a: unknown, b: unknown): number {
  return lastBuildAtSortKey(extractBuildAt(a)) - lastBuildAtSortKey(extractBuildAt(b))
}

type RowActionCommand = 'open' | 'analyze-deps' | 'clone' | 'edit' | 'delete'

async function onRowAction(command: RowActionCommand, row: PackProject): Promise<void> {
  if (command === 'delete') {
    try {
      await ElMessageBox.confirm(t('projectDeleteConfirm'), t('delete'), {
        type: 'warning',
        confirmButtonText: t('delete'),
        cancelButtonText: t('cancel'),
        confirmButtonClass: 'el-button--danger',
      })
    } catch {
      return
    }
    emit('delete', row)
    return
  }
  switch (command) {
    case 'open':
      emit('open', row)
      break
    case 'analyze-deps':
      emit('analyzeDeps', row)
      break
    case 'clone':
      emit('clone', row)
      break
    case 'edit':
      emit('edit', row)
      break
    default:
      break
  }
}
</script>

<template>
  <el-card class="project-list env-panel" shadow="never">
    <p v-if="workspaceName" class="project-list__scope">
      {{ $t('projectListScope', { name: workspaceName }) }}
    </p>
    <el-tabs v-model="activeType" class="project-list__tabs">
      <el-tab-pane :label="$t('projectTabAll')" name="all" />
      <el-tab-pane v-for="tp in projectTypes" :key="tp" :label="typeLabel(tp)" :name="tp" />
    </el-tabs>

    <div v-loading="loading" class="project-list__body">
      <el-table
        v-if="!showEmpty"
        :data="listItems"
        row-key="rec_id"
        class="project-list__table"
        :header-cell-style="{ fontWeight: 600 }"
      >
        <el-table-column prop="project_name" :label="$t('projectColName')" min-width="140">
          <template #default="{ row }">
            <button type="button" class="project-list__name-link" @click.stop="emit('open', row)">
              {{ row.project_name }}
            </button>
          </template>
        </el-table-column>
        <el-table-column prop="project_type" :label="$t('projectColType')" width="100">
          <template #default="{ row }">
            <span :class="projectTypeTagClass(row.project_type)">
              {{ typeLabel(row.project_type) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="project_version" :label="$t('projectColVersion')" width="96">
          <template #default="{ row }">
            <span class="project-list__version">{{ row.project_version }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="repo_branch" :label="$t('projectColBranch')" width="88" />
        <el-table-column :label="$t('projectColBuildStatus')" width="108" align="center">
          <template #default="{ row }">
            <div class="project-list__build-status">
              <el-icon
                v-if="listBuildOutcome(row.last_build_status) === 'success'"
                class="project-list__build-icon project-list__build-icon--success"
                :size="16"
                :title="buildStatusLabel(t, 'success')"
              >
                <CircleCheckFilled />
              </el-icon>
              <el-icon
                v-else-if="listBuildOutcome(row.last_build_status) === 'failed'"
                class="project-list__build-icon project-list__build-icon--failed"
                :size="16"
                :title="buildStatusLabel(t, 'failed')"
              >
                <CircleCloseFilled />
              </el-icon>
              <el-tag
                v-else-if="row.last_build_status"
                :type="buildStatusTagType(row.last_build_status)"
                size="small"
                effect="plain"
              >
                {{ buildStatusLabel(t, row.last_build_status) }}
              </el-tag>
              <span v-else class="project-list__build-status-none">—</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          prop="last_build_at"
          :label="$t('projectColBuildTime')"
          width="156"
          align="left"
          sortable
          :sort-method="compareLastBuildAt"
        >
          <template #default="{ row }">
            <span
              class="project-list__build-time"
              :title="row.last_build_at ? formatBuildTime(row.last_build_at) : undefined"
            >
              {{ formatBuildTime(row.last_build_at) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('projectColEnabled')" width="88" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.is_enabled === 1 ? 'success' : 'info'"
              size="small"
              effect="plain"
            >
              {{ row.is_enabled === 1 ? $t('enabled') : $t('disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('projectColActions')" width="168" fixed="right" align="right">
          <template #default="{ row }">
            <div class="project-list__actions">
              <el-button
                class="project-list__action-primary"
                size="small"
                type="primary"
                plain
                @click.stop="emit('build', row)"
              >
                {{ $t('projectBuildNow') }}
              </el-button>
              <el-dropdown
                trigger="click"
                @command="(cmd: RowActionCommand) => onRowAction(cmd, row)"
              >
                <el-button class="project-list__action-more" size="small" @click.stop>
                  {{ $t('projectListMore') }}
                  <el-icon class="project-list__action-more-icon"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="open">
                      {{ $t('projectDetailOpen') }}
                    </el-dropdown-item>
                    <el-dropdown-item v-if="isMavenProject(row)" command="analyze-deps">
                      {{ $t('projectMavenDepsAnalyze') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="clone" divided>
                      {{ $t('projectClone') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="edit">
                      {{ $t('edit') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="delete">
                      <span class="project-list__action-danger">{{ $t('delete') }}</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-else class="project-list__empty" :description="emptyTitle">
        <p v-if="emptyHint" class="project-list__empty-hint">{{ emptyHint }}</p>
      </el-empty>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
.project-list {
  overflow: hidden;
  border-radius: 16px;

  :deep(.el-card__body) {
    padding: 0 20px 20px;
  }

  &__scope {
    margin: 12px 0 0;
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__version {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }

  &__build-status {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 22px;
  }

  &__build-status-none {
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__build-time {
    font-size: 13px;
    line-height: 1.35;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
    white-space: nowrap;
  }

  &__build-icon {
    vertical-align: middle;

    &--success {
      color: var(--el-color-success);
    }

    &--failed {
      color: var(--el-color-danger);
    }
  }

  &__name-link {
    padding: 0;
    border: none;
    background: none;
    font-weight: 500;
    color: var(--shell-accent, var(--el-color-primary));
    cursor: pointer;
    text-align: left;

    &:hover {
      text-decoration: underline;
    }
  }

  &__tabs {
    margin: 0 -4px;

    :deep(.el-tabs__header) {
      margin: 0 0 4px;
    }

    :deep(.el-tabs__nav-wrap::after) {
      background-color: var(--shell-card-border, var(--el-border-color-lighter));
    }

    :deep(.el-tabs__item) {
      height: 44px;
      font-size: 14px;
      color: var(--shell-content-text-secondary, var(--el-text-color-secondary));

      &.is-active {
        font-weight: 600;
        color: var(--shell-accent, var(--el-color-primary));
      }
    }

    :deep(.el-tabs__active-bar) {
      background-color: var(--shell-accent, var(--el-color-primary));
    }
  }

  &__body {
    min-height: 200px;
  }

  &__table {
    width: 100%;

    :deep(.el-table__inner-wrapper::before) {
      display: none;
    }

    :deep(th.el-table__cell) {
      background: var(--shell-table-header-bg, var(--el-fill-color-light));
      color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
      font-size: 13px;
    }
  }

  &__actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    white-space: nowrap;
  }

  &__action-primary {
    padding-left: 10px;
    padding-right: 10px;
  }

  &__action-more {
    padding-left: 8px;
    padding-right: 8px;
  }

  &__action-more-icon {
    margin-left: 2px;
    font-size: 12px;
  }

  &__action-danger {
    color: var(--el-color-danger);
  }

  &__empty {
    padding: 32px 16px 40px;
  }

  &__empty-hint {
    margin: 8px 0 0;
    max-width: 360px;
    font-size: 13px;
    line-height: 1.6;
    text-align: center;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }
}
</style>
