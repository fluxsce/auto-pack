<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh } from '@element-plus/icons-vue'
import type { PackProject } from '@/types'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { useMessage } from '@/components/feedback/message'
import {
  fetchMavenDependencyAnalysis,
  type MavenDependencyAnalysis,
  type MavenInternalEdge,
  type MavenWorkspaceAnalysis,
  type MavenWorkspaceModule,
} from '@/request/api/project'
import { fetchWorkspaceMavenDependencyAnalysis } from '@/request/api/workspace'

const open = defineModel<boolean>('open', { required: true })

const props = withDefaults(
  defineProps<{
    mode?: 'workspace' | 'project'
    workspaceRecId?: string
    workspaceName?: string
    projects: PackProject[]
    initialProjectRecId?: string
  }>(),
  { mode: 'project' },
)

const { t } = useI18n()
const message = useMessage()

const targetRecId = ref('')
const loading = ref(false)
const projectAnalysis = ref<MavenDependencyAnalysis | null>(null)
const workspaceAnalysis = ref<MavenWorkspaceAnalysis | null>(null)
const modules = ref<MavenWorkspaceModule[]>([])
const installCommand = ref('')

const isWorkspaceMode = computed(() => props.mode === 'workspace')

const mavenProjects = computed(() =>
  (props.projects ?? []).filter((p) => String(p.project_type).toLowerCase() === 'maven'),
)

const targetProject = computed(() =>
  mavenProjects.value.find((p) => p.rec_id === targetRecId.value) ?? null,
)

const hasMavenProjects = computed(() => mavenProjects.value.length > 0)

const dialogTitle = computed(() =>
  isWorkspaceMode.value ? t('projectMavenDepsWorkspaceTitle') : t('projectMavenDepsTitle'),
)

const missingPom = computed(() =>
  isWorkspaceMode.value
    ? (workspaceAnalysis.value?.missing_pom ?? [])
    : (projectAnalysis.value?.missing_pom ?? []),
)

const workspaceSkippedIds = computed(() => {
  const ids = new Set<string>()
  for (const m of workspaceAnalysis.value?.skipped_installed ?? []) {
    ids.add(m.project_rec_id)
  }
  return ids
})

function workspaceModuleNeedsInstall(recId: string): boolean {
  return !workspaceSkippedIds.value.has(recId)
}

function formatGav(m: { coordinates: { group_id: string; artifact_id: string; version?: string } }): string {
  const c = m.coordinates
  const ver = c.version?.trim()
  return ver ? `${c.group_id}:${c.artifact_id}:${ver}` : `${c.group_id}:${c.artifact_id}`
}

function formatDepGav(dep: MavenInternalEdge['dependency']): string {
  const ver = dep.version?.trim()
  return ver ? `${dep.group_id}:${dep.artifact_id}:${ver}` : `${dep.group_id}:${dep.artifact_id}`
}

function resetState(): void {
  projectAnalysis.value = null
  workspaceAnalysis.value = null
  modules.value = []
  installCommand.value = ''
}

function pickDefaultTarget(): void {
  const initial = props.initialProjectRecId?.trim()
  if (initial && mavenProjects.value.some((p) => p.rec_id === initial)) {
    targetRecId.value = initial
    return
  }
  targetRecId.value = mavenProjects.value[0]?.rec_id ?? ''
}

async function loadAnalysis(): Promise<void> {
  loading.value = true
  resetState()
  try {
    if (isWorkspaceMode.value) {
      const wsId = props.workspaceRecId?.trim()
      if (!wsId) {
        message.warning(t('workspaceSelectRequired'))
        return
      }
      const res = await fetchWorkspaceMavenDependencyAnalysis(wsId)
      workspaceAnalysis.value = res.analysis
      modules.value = res.modules ?? []
      installCommand.value = res.install_command ?? ''
      return
    }
    if (!targetRecId.value) return
    const res = await fetchMavenDependencyAnalysis(targetRecId.value)
    projectAnalysis.value = res.analysis
    modules.value = res.modules ?? []
    installCommand.value = res.install_command ?? ''
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

watch(open, (v) => {
  if (!v) {
    resetState()
    return
  }
  if (!isWorkspaceMode.value) pickDefaultTarget()
  void loadAnalysis()
})

watch(targetRecId, (id, prev) => {
  if (!open.value || isWorkspaceMode.value || !id || id === prev) return
  void loadAnalysis()
})
</script>

<template>
  <AppDialog
    v-model="open"
    :title="dialogTitle"
    width="760px"
    initial-height="76vh"
    :show-footer="false"
  >
    <p class="project-maven-deps-dialog__hint">
      {{
        isWorkspaceMode
          ? $t('projectMavenDepsWorkspaceHint', { name: workspaceName || '—' })
          : $t('projectMavenDepsHint')
      }}
    </p>

    <el-empty v-if="!hasMavenProjects" :description="$t('projectMavenDepsNoMaven')" />

    <template v-else>
      <div v-if="!isWorkspaceMode" class="project-maven-deps-dialog__picker">
        <span class="project-maven-deps-dialog__label">{{ $t('projectMavenDepsTarget') }}</span>
        <el-select
          v-model="targetRecId"
          class="project-maven-deps-dialog__select"
          filterable
          :placeholder="$t('projectMavenDepsTargetPlaceholder')"
          :teleported="true"
          :popper-class="APP_DIALOG_POPPER_CLASS"
        >
          <el-option
            v-for="p in mavenProjects"
            :key="p.rec_id"
            :label="`${p.project_name} (${p.project_version})`"
            :value="p.rec_id"
          />
        </el-select>
        <el-button :loading="loading" :icon="Refresh" @click="loadAnalysis()">
          {{ $t('projectMavenDepsRefresh') }}
        </el-button>
      </div>
      <div v-else class="project-maven-deps-dialog__picker">
        <el-button :loading="loading" :icon="Refresh" @click="loadAnalysis()">
          {{ $t('projectMavenDepsRefresh') }}
        </el-button>
      </div>

      <div v-loading="loading" class="project-maven-deps-dialog__body">
        <!-- Workspace-wide analysis -->
        <template v-if="isWorkspaceMode && workspaceAnalysis">
          <p class="project-maven-deps-dialog__summary">
            {{
              $t('projectMavenDepsWorkspaceSummary', {
                modules: workspaceAnalysis.module_count,
                edges: workspaceAnalysis.internal_edges?.length ?? 0,
                install: workspaceAnalysis.build_order?.length ?? 0,
              })
            }}
          </p>
          <p v-if="installCommand" class="project-maven-deps-dialog__cmd">
            {{ $t('projectMavenDepsInstallCmd') }}:
            <code>{{ installCommand }}</code>
          </p>

          <el-alert
            v-for="(msg, i) in workspaceAnalysis.warnings ?? []"
            :key="'warn-' + i"
            type="error"
            :title="msg"
            show-icon
            :closable="false"
            class="project-maven-deps-dialog__alert"
          />
          <el-alert
            v-for="(msg, i) in missingPom"
            :key="'miss-' + i"
            type="warning"
            :title="msg"
            show-icon
            :closable="false"
            class="project-maven-deps-dialog__alert"
          />

          <section class="project-maven-deps-dialog__section">
            <h4 class="project-maven-deps-dialog__section-title">
              {{ $t('projectMavenDepsWorkspaceBuildOrder') }}
              <span class="project-maven-deps-dialog__count">
                ({{ workspaceAnalysis.build_order?.length ?? 0 }})
              </span>
            </h4>
            <p class="project-maven-deps-dialog__section-desc">
              {{ $t('projectMavenDepsWorkspaceBuildOrderDesc') }}
            </p>
            <el-table
              v-if="(workspaceAnalysis.full_build_order ?? []).length"
              :data="workspaceAnalysis.full_build_order"
              size="small"
              border
              stripe
              class="project-maven-deps-dialog__table"
            >
              <el-table-column type="index" width="48" :label="'#'" />
              <el-table-column prop="project_name" :label="$t('projectColName')" min-width="140" />
              <el-table-column :label="$t('projectMavenDepsGav')" min-width="200">
                <template #default="{ row }">
                  <code class="project-maven-deps-dialog__gav">{{ formatGav(row) }}</code>
                </template>
              </el-table-column>
              <el-table-column :label="$t('projectMavenDepsStatus')" width="120">
                <template #default="{ row }">
                  <el-tag
                    v-if="workspaceModuleNeedsInstall(row.project_rec_id)"
                    type="warning"
                    size="small"
                    effect="plain"
                  >
                    {{ $t('projectMavenDepsStatusInstall') }}
                  </el-tag>
                  <el-tag v-else type="success" size="small" effect="plain">
                    {{ $t('projectMavenDepsStatusLocal') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
            <el-empty
              v-else
              :description="$t('projectMavenDepsWorkspaceBuildOrderEmpty')"
              :image-size="64"
            />
          </section>

          <section
            v-if="(workspaceAnalysis.skipped_installed ?? []).length"
            class="project-maven-deps-dialog__section"
          >
            <h4 class="project-maven-deps-dialog__section-title">
              {{ $t('projectMavenDepsSkipped') }}
              <span class="project-maven-deps-dialog__count">
                ({{ workspaceAnalysis.skipped_installed?.length ?? 0 }})
              </span>
            </h4>
            <el-table
              :data="workspaceAnalysis.skipped_installed"
              size="small"
              border
              class="project-maven-deps-dialog__table"
            >
              <el-table-column prop="project_name" :label="$t('projectColName')" min-width="140" />
              <el-table-column :label="$t('projectMavenDepsGav')" min-width="200">
                <template #default="{ row }">
                  <code class="project-maven-deps-dialog__gav">{{ formatGav(row) }}</code>
                </template>
              </el-table-column>
            </el-table>
          </section>

          <section
            v-if="(workspaceAnalysis.internal_edges ?? []).length"
            class="project-maven-deps-dialog__section"
          >
            <h4 class="project-maven-deps-dialog__section-title">
              {{ $t('projectMavenDepsInternalEdges') }}
              <span class="project-maven-deps-dialog__count">
                ({{ workspaceAnalysis.internal_edges?.length ?? 0 }})
              </span>
            </h4>
            <p class="project-maven-deps-dialog__section-desc">
              {{ $t('projectMavenDepsInternalEdgesDesc') }}
            </p>
            <el-table
              :data="workspaceAnalysis.internal_edges"
              size="small"
              border
              max-height="280"
              class="project-maven-deps-dialog__table"
            >
              <el-table-column prop="consumer_name" :label="$t('projectMavenDepsConsumer')" min-width="120" />
              <el-table-column prop="provider_name" :label="$t('projectMavenDepsProvider')" min-width="120" />
              <el-table-column :label="$t('projectMavenDepsGav')" min-width="180">
                <template #default="{ row }">
                  <code class="project-maven-deps-dialog__gav">{{ formatDepGav(row.dependency) }}</code>
                </template>
              </el-table-column>
            </el-table>
          </section>
        </template>

        <!-- Single-project analysis -->
        <template v-else-if="!isWorkspaceMode && projectAnalysis && targetProject">
          <p class="project-maven-deps-dialog__target">
            {{ $t('projectMavenDepsTargetSummary', { name: targetProject.project_name }) }}
          </p>
          <p v-if="installCommand" class="project-maven-deps-dialog__cmd">
            {{ $t('projectMavenDepsInstallCmd') }}:
            <code>{{ installCommand }}</code>
          </p>

          <el-alert
            v-for="(msg, i) in projectAnalysis.warnings ?? []"
            :key="'warn-p-' + i"
            type="error"
            :title="msg"
            show-icon
            :closable="false"
            class="project-maven-deps-dialog__alert"
          />
          <el-alert
            v-for="(msg, i) in missingPom"
            :key="'miss-p-' + i"
            type="warning"
            :title="msg"
            show-icon
            :closable="false"
            class="project-maven-deps-dialog__alert"
          />

          <section class="project-maven-deps-dialog__section">
            <h4 class="project-maven-deps-dialog__section-title">
              {{ $t('projectMavenDepsBuildOrder') }}
              <span class="project-maven-deps-dialog__count">
                ({{ (projectAnalysis.build_order ?? []).length }})
              </span>
            </h4>
            <p class="project-maven-deps-dialog__section-desc">{{ $t('projectMavenDepsBuildOrderDesc') }}</p>
            <el-table
              v-if="(projectAnalysis.build_order ?? []).length"
              :data="projectAnalysis.build_order"
              size="small"
              border
              stripe
              class="project-maven-deps-dialog__table"
            >
              <el-table-column type="index" width="48" :label="'#'" />
              <el-table-column prop="project_name" :label="$t('projectColName')" min-width="140" />
              <el-table-column :label="$t('projectMavenDepsGav')" min-width="200">
                <template #default="{ row }">
                  <code class="project-maven-deps-dialog__gav">{{ formatGav(row) }}</code>
                </template>
              </el-table-column>
              <el-table-column :label="$t('projectMavenDepsStatus')" width="120">
                <template #default>
                  <el-tag type="warning" size="small" effect="plain">
                    {{ $t('projectMavenDepsStatusInstall') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else :description="$t('projectMavenDepsBuildOrderEmpty')" :image-size="64" />
          </section>

          <section
            v-if="(projectAnalysis.skipped_installed ?? []).length"
            class="project-maven-deps-dialog__section"
          >
            <h4 class="project-maven-deps-dialog__section-title">
              {{ $t('projectMavenDepsSkipped') }}
              <span class="project-maven-deps-dialog__count">
                ({{ projectAnalysis.skipped_installed?.length ?? 0 }})
              </span>
            </h4>
            <el-table
              :data="projectAnalysis.skipped_installed"
              size="small"
              border
              class="project-maven-deps-dialog__table"
            >
              <el-table-column prop="project_name" :label="$t('projectColName')" min-width="140" />
              <el-table-column :label="$t('projectMavenDepsGav')" min-width="200">
                <template #default="{ row }">
                  <code class="project-maven-deps-dialog__gav">{{ formatGav(row) }}</code>
                </template>
              </el-table-column>
            </el-table>
          </section>

          <el-collapse class="project-maven-deps-dialog__collapse">
            <el-collapse-item :title="$t('projectMavenDepsAllModules', { count: modules.length })">
              <el-table :data="modules" size="small" border max-height="240">
                <el-table-column prop="project_name" :label="$t('projectColName')" min-width="120" />
                <el-table-column :label="$t('projectMavenDepsGav')" min-width="200">
                  <template #default="{ row }">
                    <code class="project-maven-deps-dialog__gav">{{ formatGav(row) }}</code>
                  </template>
                </el-table-column>
                <el-table-column :label="$t('projectMavenDepsDepCount')" width="88" align="center">
                  <template #default="{ row }">
                    {{ row.dependencies?.length ?? 0 }}
                  </template>
                </el-table-column>
              </el-table>
            </el-collapse-item>
          </el-collapse>
        </template>
      </div>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-maven-deps-dialog {
  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.5;
    color: var(--el-text-color-secondary);
  }

  &__picker {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
    margin-bottom: 16px;
  }

  &__label {
    font-size: 13px;
    font-weight: 500;
  }

  &__select {
    flex: 1;
    min-width: 200px;
  }

  &__body {
    min-height: 120px;
  }

  &__summary,
  &__target {
    margin: 0 0 8px;
    font-size: 14px;
    font-weight: 500;
  }

  &__cmd {
    margin: 0 0 12px;
    font-size: 12px;
    color: var(--el-text-color-secondary);

    code {
      font-family: ui-monospace, monospace;
      font-size: 12px;
    }
  }

  &__alert {
    margin-bottom: 12px;
  }

  &__section {
    margin-bottom: 20px;
  }

  &__section-title {
    margin: 0 0 4px;
    font-size: 14px;
    font-weight: 600;
  }

  &__section-desc {
    margin: 0 0 10px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  &__count {
    font-weight: 400;
    color: var(--el-text-color-secondary);
  }

  &__gav {
    font-size: 12px;
    word-break: break-all;
  }

  &__collapse {
    margin-top: 8px;
    border: none;

    :deep(.el-collapse-item__header) {
      font-size: 13px;
      font-weight: 500;
    }
  }
}
</style>
