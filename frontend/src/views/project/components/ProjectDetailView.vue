<script setup lang="ts">
import { computed } from 'vue'
import { VideoPlay } from '@element-plus/icons-vue'
import type { PackBuild, PackProject } from '@/types'
import { projectTypeTagClass } from '../project-meta'
import { formatBuildTime } from '../project-format'
import { PROJECT_DETAIL_NAV, buildStatusTagType } from './project-detail-nav'
import { ProjectBuildLogPanel, ProjectWorkspacePanel } from './panels'
import type { ProjectDetailSection } from '../useProjectDetail'

const props = defineProps<{
  project: PackProject | null
  loading: boolean
  building: boolean
  activeSection: ProjectDetailSection
  builds: PackBuild[]
  selectedBuildId: string | null
  lastBuild: PackBuild | null
  logContent: string
  logLoading: boolean
  logComplete: boolean
  workPathPreview: string
  workspacePath: string
  workspaceEntries: import('@/types').ProjectWorkspaceEntry[]
  workspaceExists: boolean
  workspaceAbsPath: string
  workspaceLoading: boolean
  typeLabel: (type: unknown) => string
}>()

const isMaven = computed(
  () => String(props.project?.project_type ?? '').toLowerCase() === 'maven',
)

const emit = defineEmits<{
  back: []
  configure: []
  buildNow: []
  analyzeDeps: []
  selectSection: [section: ProjectDetailSection]
  selectBuild: [buildId: string]
  workspaceUp: []
  workspaceRefresh: []
  workspaceOpen: [entry: import('@/types').ProjectWorkspaceEntry]
}>()

const title = computed(() => props.project?.project_name?.trim() ?? '')

const displayRepoBranch = computed(() => props.project?.repo_branch?.trim() || '—')
const displayRepoUrl = computed(() => props.project?.repo_url?.trim() || '—')

const selectedBuildNum = computed(() => {
  if (!props.selectedBuildId) return null
  return props.builds.find((b) => b.rec_id === props.selectedBuildId)?.build_num ?? null
})

const selectedBuildStatus = computed(() => {
  if (!props.selectedBuildId) return null
  return props.builds.find((b) => b.rec_id === props.selectedBuildId)?.status ?? null
})

function onNavClick(id: ProjectDetailSection | 'configure'): void {
  if (id === 'configure') {
    emit('configure')
    return
  }
  emit('selectSection', id)
}

</script>

<template>
  <el-card v-loading="loading" class="project-detail env-panel" shadow="never">
    <template #header>
      <div class="project-detail__head">
        <div class="project-detail__head-main">
          <h2 class="project-detail__title">{{ title }}</h2>
          <span v-if="project" :class="projectTypeTagClass(project.project_type)">
            {{ typeLabel(project.project_type) }}
          </span>
          <el-tag v-if="project" size="small" effect="plain">{{ project.project_version }}</el-tag>
        </div>
        <div class="project-detail__head-actions">
          <el-button type="primary" :loading="building" :disabled="!project" @click="emit('buildNow')">
            <el-icon class="project-detail__btn-icon"><VideoPlay /></el-icon>
            {{ $t('projectBuildNow') }}
          </el-button>
          <el-button v-if="isMaven" :disabled="!project" @click="emit('analyzeDeps')">
            {{ $t('projectMavenDepsAnalyze') }}
          </el-button>
          <el-button @click="emit('back')">{{ $t('back') }}</el-button>
        </div>
      </div>
    </template>

    <div class="project-detail__layout">
      <aside class="project-detail__sidebar">
        <nav class="project-detail__nav" :aria-label="$t('projectDetailNav')">
          <button
            v-for="item in PROJECT_DETAIL_NAV"
            :key="item.id"
            type="button"
            class="project-detail__nav-item"
            :class="{ 'project-detail__nav-item--active': activeSection === item.id }"
            @click="onNavClick(item.id)"
          >
            <el-icon class="project-detail__nav-icon"><component :is="item.icon" /></el-icon>
            <span>{{ $t(item.labelKey) }}</span>
          </button>
        </nav>

        <section class="project-detail__builds">
          <header class="project-detail__builds-head">
            <h3 class="project-detail__builds-title">{{ $t('projectBuildHistory') }}</h3>
          </header>
          <ul v-if="builds.length" class="project-detail__build-list">
            <li v-for="b in builds" :key="b.rec_id">
              <button
                type="button"
                class="project-detail__build-item"
                :class="{ 'is-active': selectedBuildId === b.rec_id }"
                @click="emit('selectBuild', b.rec_id)"
              >
                <el-tag :type="buildStatusTagType(b.status)" size="small" effect="plain">
                  #{{ b.build_num }}
                </el-tag>
                <span class="project-detail__build-time">{{ formatBuildTime(b.started_at ?? b.created_at) }}</span>
              </button>
            </li>
          </ul>
          <p v-else class="project-detail__build-empty">{{ $t('projectBuildEmpty') }}</p>
        </section>
      </aside>

      <div class="project-detail__main">
        <section v-show="activeSection === 'status'" class="project-detail__panel project-detail__panel--status">
          <header class="project-detail__panel-head">
            <h3 class="project-detail__panel-title">{{ $t('projectDetailStatus') }}</h3>
          </header>
          <dl v-if="project" class="project-detail__meta">
            <div class="project-detail__meta-row">
              <dt>{{ $t('projectColBranch') }}</dt>
              <dd>{{ displayRepoBranch }}</dd>
            </div>
            <div class="project-detail__meta-row">
              <dt>{{ $t('projectColRepoUrl') }}</dt>
              <dd class="project-detail__mono">{{ displayRepoUrl }}</dd>
            </div>
            <div class="project-detail__meta-row">
              <dt>{{ $t('projectColWorkDir') }}</dt>
              <dd class="project-detail__mono">{{ workPathPreview || '—' }}</dd>
            </div>
          </dl>

          <div v-if="lastBuild" class="project-detail__last-build">
            <h4 class="project-detail__sub-title">{{ $t('projectLastBuild') }}</h4>
            <p class="project-detail__last-build-line">
              <el-tag :type="buildStatusTagType(lastBuild.status)" size="small">
                #{{ lastBuild.build_num }} · {{ $t(`projectBuildStatus_${lastBuild.status}`) }}
              </el-tag>
              <span class="project-detail__muted">{{ lastBuild.message }}</span>
            </p>
            <el-button link type="primary" @click="emit('selectBuild', lastBuild.rec_id)">
              {{ $t('projectViewLog') }}
            </el-button>
          </div>
          <p v-else class="project-detail__hint">{{ $t('projectNoBuildYet') }}</p>
        </section>

        <ProjectBuildLogPanel
          v-show="activeSection === 'log'"
          class="project-detail__log-panel"
          :build-num="selectedBuildNum"
          :build-status="selectedBuildStatus"
          :content="logContent"
          :loading="logLoading"
          :complete="logComplete"
        />

        <ProjectWorkspacePanel
          v-show="activeSection === 'workspace'"
          class="project-detail__workspace-panel"
          :project-rec-id="project?.rec_id ?? ''"
          :path="workspacePath"
          :abs-path="workspaceAbsPath"
          :exists="workspaceExists"
          :entries="workspaceEntries"
          :loading="workspaceLoading"
          @up="emit('workspaceUp')"
          @refresh="emit('workspaceRefresh')"
          @open="emit('workspaceOpen', $event)"
        />
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss" src="./ProjectDetailView.scss"></style>
