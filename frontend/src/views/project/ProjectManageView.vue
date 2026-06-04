<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, Plus, Refresh, Search, Setting } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import ProjectBulkAutoReleaseDialog from './components/ProjectBulkAutoReleaseDialog.vue'
import ProjectBulkGitBranchDialog from './components/ProjectBulkGitBranchDialog.vue'
import ProjectBulkVersionDialog from './components/ProjectBulkVersionDialog.vue'
import ProjectBulkPostBuildEmailDialog from './components/ProjectBulkPostBuildEmailDialog.vue'
import type { ProjectBuildStatusFilter } from './useProjects'
import ProjectDetailView from './components/ProjectDetailView.vue'
import ProjectEditor from './components/ProjectEditor.vue'
import ProjectList from './components/ProjectList.vue'
import WorkspaceManageDialog from './workspace/WorkspaceManageDialog.vue'
import WorkspacePickerDialog from './workspace/WorkspacePickerDialog.vue'
import { useProjectDetail } from './useProjectDetail'
import { useProjects, type WorkspaceManageDialogExpose } from './useProjects'

const { t } = useI18n()
const message = useMessage()

const workspacePickerOpen = ref(false)
const bulkGitBranchOpen = ref(false)
const bulkVersionOpen = ref(false)
const bulkPostBuildEmailOpen = ref(false)
const bulkAutoReleaseOpen = ref(false)

const buildStatusFilterOptions: { value: ProjectBuildStatusFilter; labelKey: string }[] = [
  { value: 'all', labelKey: 'projectFilterBuildAll' },
  { value: 'none', labelKey: 'projectBuildStatus_none' },
  { value: 'success', labelKey: 'projectBuildStatus_success' },
  { value: 'failed', labelKey: 'projectBuildStatus_failed' },
]

const workspaceManageRef = ref<WorkspaceManageDialogExpose | null>(null)
const detailProjectId = ref<string | null>(null)

async function handleWorkspacesChanged(): Promise<void> {
  await onWorkspacesChanged(workspaceManageRef.value)
}

const {
  projectTypes,
  workspaces,
  activeWorkspaceRecId,
  activeWorkspace,
  activeWorkspaceLabel,
  activeType,
  loading,
  editorOpen,
  saving,
  editingId,
  workspaceDialogOpen,
  form,
  buildConfig,
  items,
  filteredItems,
  searchQuery,
  buildStatusFilter,
  hasListFilters,
  loadList,
  editorTitle,
  typeLabel,
  envOptions,
  openCreate,
  openEdit,
  openClone,
  closeEditor,
  onTypeChange,
  onSave,
  onDelete,
  onWorkspaceFilterChange,
  onWorkspacesChanged,
  loadEnvs,
} = useProjects()

const detail = useProjectDetail(() => activeWorkspace.value)

const {
  project: detailProject,
  builds: detailBuilds,
  loading: detailLoading,
  building: detailBuilding,
  activeSection: detailSection,
  selectedBuildId: detailSelectedBuildId,
  lastBuild: detailLastBuild,
  logContent: detailLogContent,
  logLoading: detailLogLoading,
  logComplete: detailLogComplete,
  workPathPreview: detailWorkPath,
  workspacePath: detailWorkspacePath,
  workspaceEntries: detailWorkspaceEntries,
  workspaceExists: detailWorkspaceExists,
  workspaceAbsPath: detailWorkspaceAbsPath,
  workspaceLoading: detailWorkspaceLoading,
  refreshAll: detailRefreshAll,
  onBuildNow: detailBuildNow,
  selectBuild: detailSelectBuild,
  openSection: detailOpenSection,
  workspaceUp: detailWorkspaceUp,
  loadWorkspace: detailLoadWorkspace,
  openWorkspaceEntry: detailOpenWorkspaceEntry,
  reset: detailReset,
} = detail

const detailOpen = ref(false)

const workspaceProjectCount = computed(() => items.value.length)

function onBulkCommand(command: string): void {
  if (command === 'git-branch') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkGitBranchOpen.value = true
    return
  }
  if (command === 'project-version') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkVersionOpen.value = true
    return
  }
  if (command === 'post-build-email') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkPostBuildEmailOpen.value = true
    return
  }
  if (command === 'auto-release') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkAutoReleaseOpen.value = true
  }
}

watch(detailProjectId, (id) => {
  detailOpen.value = !!id
  if (id) {
    detailReset()
    void detailRefreshAll(id)
  } else {
    detailReset()
  }
})

function openDetail(row: { rec_id: string }): void {
  detailProjectId.value = row.rec_id
}

function closeDetail(): void {
  detailProjectId.value = null
}

function openConfigureFromDetail(): void {
  const p = detailProject.value
  if (!p) return
  detailProjectId.value = null
  void openEdit(p)
}

async function quickBuild(row: { rec_id: string }): Promise<void> {
  if (detailProjectId.value === row.rec_id) {
    await detailBuildNow()
    void loadList(true)
    return
  }
  detailProjectId.value = row.rec_id
  await detailRefreshAll(row.rec_id)
  await detailBuildNow()
  void loadList(true)
}
</script>

<template>
  <div class="project-manage">
    <header class="project-manage__head">
      <div class="project-manage__intro">
        <h1 class="project-manage__title">{{ $t('projectPageTitle') }}</h1>
        <p class="project-manage__desc">{{ $t('projectPageDesc') }}</p>
      </div>
      <div class="project-manage__actions">
        <template v-if="!editorOpen && !detailOpen">
          <el-button @click="workspaceDialogOpen = true">
            <el-icon class="project-manage__btn-icon"><Setting /></el-icon>
            {{ $t('workspaceManage') }}
          </el-button>
          <el-button type="primary" @click="openCreate">
            <el-icon class="project-manage__btn-icon"><Plus /></el-icon>
            {{ $t('projectAdd') }}
          </el-button>
        </template>
        <el-button v-else text type="primary" @click="detailOpen ? closeDetail() : closeEditor()">
          {{ $t('back') }}
        </el-button>
      </div>
    </header>

    <div v-if="!editorOpen && !detailOpen" class="project-manage__toolbar">
      <div class="project-manage__toolbar-workspace">
        <span class="project-manage__toolbar-label">{{ $t('workspaceFilterLabel') }}</span>
        <button
          type="button"
          class="project-manage__workspace-trigger"
          :aria-label="$t('workspacePickerOpen')"
          @click="workspacePickerOpen = true"
        >
          <span class="project-manage__workspace-value">
            {{ activeWorkspaceLabel || $t('workspaceSelectPlaceholder') }}
          </span>
          <el-icon class="project-manage__workspace-chevron" :size="14">
            <ArrowDown />
          </el-icon>
        </button>
      </div>
      <div class="project-manage__toolbar-filters">
        <el-input
          v-model="searchQuery"
          class="project-manage__filter-search"
          clearable
          :prefix-icon="Search"
          :placeholder="$t('projectFilterSearch')"
        />
        <el-select
          v-model="buildStatusFilter"
          class="project-manage__filter-select"
          :placeholder="$t('projectFilterBuildStatus')"
        >
          <el-option
            v-for="opt in buildStatusFilterOptions"
            :key="opt.value"
            :label="t(opt.labelKey)"
            :value="opt.value"
          />
        </el-select>
        <el-button
          class="project-manage__filter-refresh"
          :loading="loading"
          :aria-label="$t('refresh')"
          @click="loadList()"
        >
          <el-icon><Refresh /></el-icon>
          {{ $t('refresh') }}
        </el-button>
        <el-dropdown trigger="click" class="project-manage__bulk-ops" @command="onBulkCommand">
          <el-button>
            {{ $t('projectBulkOps') }}
            <el-icon class="project-manage__bulk-ops-icon"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="git-branch">
                {{ $t('projectBulkUpdateGitBranch') }}
              </el-dropdown-item>
              <el-dropdown-item command="project-version">
                {{ $t('projectBulkUpdateVersion') }}
              </el-dropdown-item>
              <el-dropdown-item command="post-build-email">
                {{ $t('projectBulkPostBuildEmail') }}
              </el-dropdown-item>
              <el-dropdown-item command="auto-release">
                {{ $t('projectBulkAutoRelease') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <ProjectList
      v-if="!editorOpen && !detailOpen"
      v-model:active-type="activeType"
      :loading="loading"
      :items="filteredItems"
      :has-list-filters="hasListFilters"
      :project-types="projectTypes"
      :type-label="typeLabel"
      :workspace-name="activeWorkspace?.workspace_name"
      @open="openDetail"
      @build="quickBuild"
      @clone="openClone"
      @edit="openEdit"
      @delete="onDelete"
    />

    <div v-else-if="detailOpen" class="project-manage__editor-wrap">
      <ProjectDetailView
        :project="detailProject"
        :loading="detailLoading"
        :building="detailBuilding"
        :active-section="detailSection"
        :builds="detailBuilds"
        :selected-build-id="detailSelectedBuildId"
        :last-build="detailLastBuild"
        :log-content="detailLogContent"
        :log-loading="detailLogLoading"
        :log-complete="detailLogComplete"
        :work-path-preview="detailWorkPath"
        :workspace-path="detailWorkspacePath"
        :workspace-entries="detailWorkspaceEntries"
        :workspace-exists="detailWorkspaceExists"
        :workspace-abs-path="detailWorkspaceAbsPath"
        :workspace-loading="detailWorkspaceLoading"
        :type-label="typeLabel"
        @back="closeDetail"
        @configure="openConfigureFromDetail"
        @build-now="detailBuildNow()"
        @select-section="detailOpenSection($event)"
        @select-build="detailSelectBuild($event)"
        @workspace-up="detailWorkspaceUp()"
        @workspace-refresh="detailLoadWorkspace(detailWorkspacePath)"
        @workspace-open="detailOpenWorkspaceEntry($event)"
      />
    </div>

    <div v-else class="project-manage__editor-wrap">
      <ProjectEditor
        v-model:project-type="form.project_type"
        v-model:build-config="buildConfig"
        :title="editorTitle"
        :editing-id="editingId"
        :saving="saving"
        :project-types="projectTypes"
        :workspaces="workspaces"
        :form="form"
        :type-label="typeLabel"
        :env-options="envOptions"
        :active-workspace="activeWorkspace"
        :workspace-projects="items"
        @save="onSave"
        @back="closeEditor"
        @type-change="onTypeChange"
        @reload-envs="loadEnvs"
      />
    </div>

    <WorkspacePickerDialog
      v-model:open="workspacePickerOpen"
      v-model="activeWorkspaceRecId"
      :workspaces="workspaces"
      @select="onWorkspaceFilterChange"
    />
    <WorkspaceManageDialog
      ref="workspaceManageRef"
      v-model:open="workspaceDialogOpen"
      @changed="handleWorkspacesChanged"
    />
    <ProjectBulkGitBranchDialog
      v-model:open="bulkGitBranchOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      @success="loadList()"
    />
    <ProjectBulkVersionDialog
      v-model:open="bulkVersionOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      @success="loadList()"
    />
    <ProjectBulkPostBuildEmailDialog
      v-model:open="bulkPostBuildEmailOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      :env-options="envOptions"
      @success="loadList()"
      @reload-envs="loadEnvs()"
    />
    <ProjectBulkAutoReleaseDialog
      v-model:open="bulkAutoReleaseOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      @success="loadList()"
    />
  </div>
</template>

<style scoped lang="scss" src="./ProjectManageView.scss"></style>
