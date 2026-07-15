<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { ArrowDown, Plus, Refresh, Search, Setting } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import { purgeWorkspace } from '@/request/api/workspace'
import ProjectBulkAutoBuildDialog from './components/ProjectBulkAutoBuildDialog.vue'
import ProjectBulkAutoReleaseDialog from './components/ProjectBulkAutoReleaseDialog.vue'
import ProjectBulkBuildStepsDialog from './components/ProjectBulkBuildStepsDialog.vue'
import ProjectBulkGitBranchDialog from './components/ProjectBulkGitBranchDialog.vue'
import ProjectBulkBranchCloneDialog from './components/ProjectBulkBranchCloneDialog.vue'
import ProjectBulkWorkspaceBundleDialog from './components/ProjectBulkWorkspaceBundleDialog.vue'
import ProjectBulkVersionDialog from './components/ProjectBulkVersionDialog.vue'
import ProjectBulkPomVersionDialog from './components/ProjectBulkPomVersionDialog.vue'
import ProjectBulkPostBuildEmailDialog from './components/ProjectBulkPostBuildEmailDialog.vue'
import ProjectMavenDepsDialog from './components/ProjectMavenDepsDialog.vue'
import type { ProjectBuildStatusFilter } from './useProjects'
import ProjectDetailView from './components/ProjectDetailView.vue'
import ProjectEditor from './components/ProjectEditor.vue'
import ProjectList from './components/ProjectList.vue'
import WorkspaceManageDialog from './workspace/WorkspaceManageDialog.vue'
import WorkspacePickerDialog from './workspace/WorkspacePickerDialog.vue'
import { useProjectDetail } from './useProjectDetail'
import { useProjects, type WorkspaceManageDialogExpose } from './useProjects'
import { hasRunningAutoBuildSession } from '@/utils/auto-build-session'

const { t } = useI18n()
const message = useMessage()

const workspacePickerOpen = ref(false)
const bulkGitBranchOpen = ref(false)
const bulkBranchCloneOpen = ref(false)
const bulkWorkspaceBundleOpen = ref(false)
const bulkVersionOpen = ref(false)
const bulkPomVersionOpen = ref(false)
const bulkPostBuildEmailOpen = ref(false)
const bulkBuildStepsOpen = ref(false)
const bulkAutoReleaseOpen = ref(false)
const bulkAutoBuildOpen = ref(false)
const mavenDepsOpen = ref(false)
const mavenDepsMode = ref<'workspace' | 'project'>('workspace')
const mavenDepsTargetId = ref<string | undefined>(undefined)
const purgingWorkspace = ref(false)
const autoBuildResumePending = ref(false)

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

const workspaceMavenProjects = computed(() =>
  items.value.filter((p) => String(p.project_type).toLowerCase() === 'maven'),
)

const workspaceMavenProjectCount = computed(() => workspaceMavenProjects.value.length)

async function onWorkspaceBundleSuccess(): Promise<void> {
  await handleWorkspacesChanged()
  await loadList()
}

async function syncAutoBuildResumeFlag(): Promise<void> {
  autoBuildResumePending.value = await hasRunningAutoBuildSession(activeWorkspaceRecId.value)
}

watch(activeWorkspaceRecId, () => void syncAutoBuildResumeFlag(), { immediate: true })

function openAutoBuildResume(): void {
  openAutoBuild()
}

function openMavenDepsAnalysis(projectRecId?: string): void {
  if (!activeWorkspaceRecId.value) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (workspaceMavenProjects.value.length === 0) {
    message.warning(t('projectMavenDepsNoMaven'))
    return
  }
  if (projectRecId) {
    mavenDepsMode.value = 'project'
    mavenDepsTargetId.value = projectRecId
  } else {
    mavenDepsMode.value = 'workspace'
    mavenDepsTargetId.value = undefined
  }
  mavenDepsOpen.value = true
}

function openAutoBuild(): void {
  if (!activeWorkspaceRecId.value) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  bulkAutoBuildOpen.value = true
}

function openAutoRelease(): void {
  if (!activeWorkspaceRecId.value) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  bulkAutoReleaseOpen.value = true
}

function onBulkCommand(command: string): void {
  if (command === 'git-branch') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkGitBranchOpen.value = true
    return
  }
  if (command === 'branch-clone') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkBranchCloneOpen.value = true
    return
  }
  if (command === 'workspace-bundle') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkWorkspaceBundleOpen.value = true
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
  if (command === 'pom-version') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    if (workspaceMavenProjects.value.length === 0) {
      message.warning(t('projectMavenDepsNoMaven'))
      return
    }
    bulkPomVersionOpen.value = true
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
  if (command === 'build-steps') {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    bulkBuildStepsOpen.value = true
    return
  }
  if (command === 'maven-deps') {
    openMavenDepsAnalysis()
    return
  }
  if (command === 'delete-workspace') {
    void confirmPurgeCurrentWorkspace()
  }
}

async function confirmPurgeCurrentWorkspace(): Promise<void> {
  const ws = activeWorkspace.value
  if (!activeWorkspaceRecId.value || !ws) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (ws.is_default === 1) {
    message.warning(t('workspaceDeleteDefaultHint'))
    return
  }
  try {
    await ElMessageBox.confirm(
      t('projectBulkDeleteWorkspaceConfirm', {
        name: ws.workspace_name,
        count: workspaceProjectCount.value,
      }),
      t('projectBulkDeleteWorkspaceTitle'),
      {
        type: 'warning',
        confirmButtonText: t('delete'),
        cancelButtonText: t('cancel'),
        confirmButtonClass: 'el-button--danger',
      },
    )
  } catch {
    return
  }
  purgingWorkspace.value = true
  try {
    const res = await purgeWorkspace(ws.rec_id)
    message.success(
      t('projectBulkDeleteWorkspaceSuccess', {
        name: ws.workspace_name,
        count: res.deleted_projects,
      }),
    )
    if (detailProjectId.value) detailProjectId.value = null
    await onWorkspacesChanged(workspaceManageRef.value)
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    purgingWorkspace.value = false
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

    <el-alert
      v-if="!editorOpen && !detailOpen && autoBuildResumePending"
      :title="$t('projectBulkAutoBuildResumeBanner')"
      type="warning"
      :closable="false"
      show-icon
      class="project-manage__auto-build-resume"
    >
      <el-button type="primary" link @click="openAutoBuildResume">
        {{ $t('projectBulkAutoBuildResumeOpen') }}
      </el-button>
    </el-alert>

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
        <div class="project-manage__quick-ops">
          <el-dropdown trigger="click" class="project-manage__bulk-ops" @command="onBulkCommand">
            <el-button :loading="purgingWorkspace">
              {{ $t('projectBulkOps') }}
              <el-icon class="project-manage__bulk-ops-icon"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <li class="el-dropdown-menu__item project-manage__bulk-group" aria-disabled="true">
                  {{ $t('projectBulkGroupConfig') }}
                </li>
                <el-dropdown-item command="git-branch">
                  {{ $t('projectBulkUpdateGitBranch') }}
                </el-dropdown-item>
                <el-dropdown-item command="branch-clone">
                  {{ $t('projectBulkBranchClone') }}
                </el-dropdown-item>
                <el-dropdown-item command="project-version">
                  {{ $t('projectBulkUpdateVersion') }}
                </el-dropdown-item>
                <el-dropdown-item command="pom-version">
                  {{ $t('projectBulkPomVersion') }}
                </el-dropdown-item>
                <el-dropdown-item command="build-steps">
                  {{ $t('projectBulkBuildSteps') }}
                </el-dropdown-item>
                <el-dropdown-item command="post-build-email">
                  {{ $t('projectBulkPostBuildEmail') }}
                </el-dropdown-item>
                <el-dropdown-item command="workspace-bundle">
                  {{ $t('projectBulkWorkspaceBundle') }}
                </el-dropdown-item>
                <li
                  class="el-dropdown-menu__item project-manage__bulk-group project-manage__bulk-group--spaced"
                  aria-disabled="true"
                >
                  {{ $t('projectBulkGroupBuild') }}
                </li>
                <el-dropdown-item command="maven-deps">
                  {{ $t('projectBulkMavenDeps') }}
                </el-dropdown-item>
                <el-dropdown-item command="delete-workspace" divided>
                  <span class="project-manage__bulk-danger">{{ $t('projectBulkDeleteWorkspace') }}</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button
            :type="autoBuildResumePending ? 'warning' : 'primary'"
            class="project-manage__quick-op"
            @click="openAutoBuild()"
          >
            {{ $t('projectBulkAutoBuild') }}
          </el-button>
          <el-button type="primary" plain class="project-manage__quick-op" @click="openAutoRelease()">
            {{ $t('projectBulkAutoRelease') }}
          </el-button>
        </div>
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
      @analyze-deps="openMavenDepsAnalysis($event.rec_id)"
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
        @analyze-deps="openMavenDepsAnalysis(detailProject?.rec_id)"
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
    <ProjectBulkBranchCloneDialog
      v-model:open="bulkBranchCloneOpen"
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
    <ProjectBulkPomVersionDialog
      v-model:open="bulkPomVersionOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceMavenProjectCount"
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
    <ProjectBulkWorkspaceBundleDialog
      v-model:open="bulkWorkspaceBundleOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :workspace-code="activeWorkspace?.workspace_code ?? ''"
      :project-count="workspaceProjectCount"
      @success="onWorkspaceBundleSuccess()"
    />
    <ProjectBulkBuildStepsDialog
      v-model:open="bulkBuildStepsOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      :project-types="projectTypes"
      :type-label="typeLabel"
      @success="loadList()"
    />
    <ProjectMavenDepsDialog
      v-model:open="mavenDepsOpen"
      :mode="mavenDepsMode"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name"
      :projects="items"
      :initial-project-rec-id="mavenDepsTargetId"
    />
    <ProjectBulkAutoBuildDialog
      v-model:open="bulkAutoBuildOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :project-count="workspaceProjectCount"
      @success="loadList()"
      @session-change="syncAutoBuildResumeFlag()"
    />
    <ProjectBulkAutoReleaseDialog
      v-model:open="bulkAutoReleaseOpen"
      :workspace-rec-id="activeWorkspaceRecId"
      :workspace-name="activeWorkspace?.workspace_name ?? ''"
      :workspace-code="activeWorkspace?.workspace_code ?? ''"
      :project-count="workspaceProjectCount"
      @success="loadList()"
    />
  </div>
</template>

<style scoped lang="scss" src="./ProjectManageView.scss"></style>
