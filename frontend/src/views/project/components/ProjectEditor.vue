<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ArrowDown, ArrowRight, Plus } from '@element-plus/icons-vue'
import type {
  EnvSetting,
  PackProject,
  PackProjectPayload,
  PackWorkspace,
  ProjectBuildConfig,
  ProjectType,
} from '@/types'
import { gitEnvOptionLabel } from '@/views/env/env-git'
import { projectTypeTagClass, suggestedEnvTypes } from '../project-meta'
import { derivedProjectWorkDir, resolvedWorkPath } from '../project-paths'
import { normalizeProjectFormFields } from '../project-form-trim'
import { effectiveWorkspaceRoot, workspaceOptionLabel } from '../workspace/workspace-paths'
import {
  PROJECT_EDITOR_NAV,
  type ProjectEditorSection,
  type ScmMode,
} from './project-editor-nav'
import ProjectBuildTriggersPanel from './panels/ProjectBuildTriggersPanel.vue'
import ProjectPostBuildPanel from './panels/ProjectPostBuildPanel.vue'
import WorkspacePickerDialog from '../workspace/WorkspacePickerDialog.vue'

const props = defineProps<{
  title: string
  editingId: string | null
  saving: boolean
  projectTypes: ProjectType[]
  workspaces: PackWorkspace[]
  form: PackProjectPayload
  buildConfig: ProjectBuildConfig
  typeLabel: (type: unknown) => string
  envOptions: (envType: string) => EnvSetting[]
  activeWorkspace?: PackWorkspace | null
  workspaceProjects?: PackProject[]
}>()

const emit = defineEmits<{
  save: []
  back: []
  typeChange: [type: ProjectType]
  reloadEnvs: []
}>()

const { t } = useI18n()
const router = useRouter()

const projectType = defineModel<ProjectType>('projectType', { required: true })
const buildConfigModel = defineModel<ProjectBuildConfig>('buildConfig', { required: true })

const activeSection = ref<ProjectEditorSection>('general')
const workspacePickerOpen = ref(false)
const scmMode = ref<ScmMode>('none')
const generalAdvancedOpen = ref(false)
const scmAdvancedOpen = ref(false)

const repoRefSuggestions = computed(() => [
  t('projectBranchTagMain'),
  t('projectBranchTagDevelop'),
  t('projectBranchTagRelease'),
])

const editorWorkspace = computed(() =>
  props.workspaces.find((w) => w.rec_id === props.form.workspace_rec_id) ?? props.activeWorkspace ?? null,
)

const editorWorkspaceLabel = computed(() => {
  const ws = editorWorkspace.value
  return ws ? workspaceOptionLabel(ws) : ''
})

const workPathPreview = computed(() => {
  const rel = derivedProjectWorkDir(props.form.project_name)
  return resolvedWorkPath(editorWorkspace.value, rel)
})

const visibleEnvTypes = computed(() => suggestedEnvTypes(projectType.value))

const showJdk = computed(() => visibleEnvTypes.value.includes('jdk'))
const showMaven = computed(() => visibleEnvTypes.value.includes('maven'))
const showNodejs = computed(() => visibleEnvTypes.value.includes('nodejs'))

function inferScmMode(): void {
  scmMode.value =
    props.form.repo_url.trim() || props.form.git_env_rec_id.trim() ? 'git' : 'none'
  generalAdvancedOpen.value = props.form.remark.trim() !== '' || props.form.sort_num > 0
  scmAdvancedOpen.value = !!props.form.git_env_rec_id.trim()
}

watch(
  () => [
    props.editingId,
    props.form.repo_url,
    props.form.git_env_rec_id,
    props.form.remark,
    props.form.sort_num,
  ],
  () => inferScmMode(),
  { immediate: true },
)

watch(scmMode, (mode, prev) => {
  if (mode === 'none' && prev === 'git') {
    props.form.repo_url = ''
    props.form.repo_branch = 'main'
    props.form.git_env_rec_id = ''
  }
})

watch(
  () => props.editingId,
  (id) => {
    if (!id) {
      activeSection.value = 'general'
      scmMode.value = 'none'
      generalAdvancedOpen.value = false
      scmAdvancedOpen.value = false
    }
  },
)

function applyRepoRef(ref: string): void {
  props.form.repo_branch = ref
}

function onEnvSelectVisible(visible: boolean): void {
  if (visible) emit('reloadEnvs')
}

function goAddGitAuth(): void {
  router.push({ name: 'env', query: { type: 'git', create: '1' } }).catch(() => {})
}

function selectSection(id: ProjectEditorSection): void {
  activeSection.value = id
  if (id === 'environment' || id === 'scm' || id === 'deploy') emit('reloadEnvs')
}

function trimFormFields(): void {
  normalizeProjectFormFields(props.form)
}
</script>

<template>
  <el-card class="project-editor env-panel" shadow="never">
    <template #header>
      <div class="project-editor__head">
        <h2 class="project-editor__title">{{ title }}</h2>
        <span :class="projectTypeTagClass(projectType)">
          {{ typeLabel(projectType) }}
        </span>
      </div>
    </template>

    <div class="project-editor__layout">
      <nav class="project-editor__nav" :aria-label="$t('projectConfigureNav')">
        <button
          v-for="item in PROJECT_EDITOR_NAV"
          :key="item.id"
          type="button"
          class="project-editor__nav-item"
          :class="{ 'project-editor__nav-item--active': activeSection === item.id }"
          @click="selectSection(item.id)"
        >
          <el-icon class="project-editor__nav-icon"><component :is="item.icon" /></el-icon>
          <span>{{ $t(item.labelKey) }}</span>
        </button>
      </nav>

      <div class="project-editor__main">
        <el-form
          label-width="148px"
          label-position="right"
          class="project-editor__form"
          @submit.prevent="emit('save')"
        >
          <!-- General -->
          <section v-show="activeSection === 'general'" class="project-editor__panel">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavGeneral') }}</h3>
            </header>
            <el-form-item :label="$t('workspaceColName')" required>
              <button
                type="button"
                class="project-editor__workspace-trigger"
                :disabled="!!editingId"
                @click="workspacePickerOpen = true"
              >
                <span class="project-editor__workspace-value">
                  {{ editorWorkspaceLabel || $t('workspaceSelectPlaceholder') }}
                </span>
                <el-icon :size="14"><ArrowDown /></el-icon>
              </button>
              <WorkspacePickerDialog
                v-if="!editingId"
                v-model:open="workspacePickerOpen"
                v-model="form.workspace_rec_id"
                :workspaces="workspaces"
              />
            </el-form-item>
            <el-form-item :label="$t('projectColName')" required>
              <el-input v-model="form.project_name" @blur="trimFormFields" />
            </el-form-item>
            <el-form-item :label="$t('projectColVersion')" required>
              <el-input
                v-model="form.project_version"
                :disabled="!!editingId"
                :placeholder="$t('projectVersionPlaceholder')"
              />
            </el-form-item>
            <el-form-item :label="$t('projectColType')" required>
              <el-select
                v-model="projectType"
                class="project-editor__full"
                @change="emit('typeChange', projectType)"
              >
                <el-option
                  v-for="tp in projectTypes"
                  :key="tp"
                  :label="typeLabel(tp)"
                  :value="tp"
                />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('envColEnabled')">
              <el-switch v-model="form.is_enabled" :active-value="1" :inactive-value="0" />
            </el-form-item>

            <button
              type="button"
              class="project-editor__advanced-toggle"
              @click="generalAdvancedOpen = !generalAdvancedOpen"
            >
              <el-icon :class="{ 'project-editor__advanced-chevron--open': generalAdvancedOpen }">
                <ArrowRight />
              </el-icon>
              {{ $t('projectAdvancedToggle') }}
            </button>
            <div v-show="generalAdvancedOpen" class="project-editor__advanced-body">
              <el-form-item :label="$t('envColSort')">
                <el-input-number v-model="form.sort_num" :min="0" :max="9999" />
              </el-form-item>
              <el-form-item :label="$t('envColRemark')">
                <el-input v-model="form.remark" type="textarea" :rows="2" @blur="trimFormFields" />
              </el-form-item>
            </div>
          </section>

          <!-- Source Code Management -->
          <section v-show="activeSection === 'scm'" class="project-editor__panel">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavScm') }}</h3>
              <p class="project-editor__panel-desc">{{ $t('projectScmDesc') }}</p>
            </header>

            <el-form-item :label="$t('projectNavScm')" class="project-editor__scm-radio">
              <el-radio-group v-model="scmMode">
                <el-radio label="none">{{ $t('projectScmNone') }}</el-radio>
                <el-radio label="git">Git</el-radio>
              </el-radio-group>
            </el-form-item>

            <template v-if="scmMode === 'git'">
              <el-form-item :label="$t('projectColRepoUrl')">
                <el-input
                  v-model="form.repo_url"
                  :placeholder="$t('projectRepoUrlPlaceholder')"
                  @blur="trimFormFields"
                />
              </el-form-item>
              <el-form-item :label="$t('projectColBranch')">
                <el-input
                  v-model="form.repo_branch"
                  :placeholder="$t('projectBranchPlaceholder')"
                  @blur="trimFormFields"
                />
                <p class="project-editor__hint">{{ $t('projectBranchHint') }}</p>
                <div class="project-editor__ref-tags">
                  <el-tag
                    v-for="ref in repoRefSuggestions"
                    :key="ref"
                    size="small"
                    effect="plain"
                    class="project-editor__ref-tag"
                    @click="applyRepoRef(ref)"
                  >
                    {{ ref }}
                  </el-tag>
                </div>
              </el-form-item>

              <button
                type="button"
                class="project-editor__advanced-toggle"
                @click="scmAdvancedOpen = !scmAdvancedOpen"
              >
                <el-icon :class="{ 'project-editor__advanced-chevron--open': scmAdvancedOpen }">
                  <ArrowRight />
                </el-icon>
                {{ $t('projectAdvancedToggle') }}
              </button>
              <div v-show="scmAdvancedOpen" class="project-editor__advanced-body">
                <el-form-item :label="$t('projectColGitAuth')">
                  <el-select
                    v-model="form.git_env_rec_id"
                    clearable
                    filterable
                    class="project-editor__full"
                    :placeholder="$t('projectGitAuthPlaceholder')"
                    @visible-change="onEnvSelectVisible"
                  >
                    <el-option
                      v-for="e in envOptions('git')"
                      :key="e.rec_id"
                      :label="gitEnvOptionLabel(t, e.env_name, e.config_json)"
                      :value="e.rec_id"
                    />
                    <template #footer>
                      <button type="button" class="project-editor__select-add" @click="goAddGitAuth">
                        <el-icon><Plus /></el-icon>
                        {{ $t('projectGitAuthAdd') }}
                      </button>
                    </template>
                  </el-select>
                  <p class="project-editor__hint">{{ $t('projectGitAuthHint') }}</p>
                </el-form-item>
              </div>

              <p v-if="workPathPreview" class="project-editor__path-hint">
                {{ $t('projectRepoLocationHint', { path: workPathPreview }) }}
              </p>
              <p v-else-if="editorWorkspace" class="project-editor__path-hint">
                {{ $t('projectRepoLocationPending', { root: effectiveWorkspaceRoot(editorWorkspace) }) }}
              </p>
            </template>
          </section>

          <!-- Environment -->
          <section v-show="activeSection === 'environment'" class="project-editor__panel">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavEnvironment') }}</h3>
              <p class="project-editor__panel-desc">{{ $t('projectEnvironmentDesc') }}</p>
            </header>

            <el-form-item v-if="showJdk" :label="$t('envType_jdk')">
              <template #label>
                <span>{{ $t('envType_jdk') }}</span>
              </template>
              <div class="project-editor__field-stack">
                <p class="project-editor__field-desc">{{ $t('projectEnvJdkDesc') }}</p>
                <el-select
                  v-model="form.jdk_env_rec_id"
                  clearable
                  filterable
                  class="project-editor__full"
                  @visible-change="onEnvSelectVisible"
                >
                  <el-option
                    v-for="e in envOptions('jdk')"
                    :key="e.rec_id"
                    :label="e.env_name"
                    :value="e.rec_id"
                  />
                </el-select>
              </div>
            </el-form-item>

            <el-form-item v-if="showMaven" :label="$t('envType_maven')">
              <div class="project-editor__field-stack">
                <p class="project-editor__field-desc">{{ $t('projectEnvMavenDesc') }}</p>
                <el-select
                  v-model="form.maven_env_rec_id"
                  clearable
                  filterable
                  class="project-editor__full"
                  @visible-change="onEnvSelectVisible"
                >
                  <el-option
                    v-for="e in envOptions('maven')"
                    :key="e.rec_id"
                    :label="e.env_name"
                    :value="e.rec_id"
                  />
                </el-select>
              </div>
            </el-form-item>

            <el-form-item v-if="showNodejs" :label="$t('envType_nodejs')">
              <div class="project-editor__field-stack">
                <p class="project-editor__field-desc">{{ $t('projectEnvNodejsDesc') }}</p>
                <el-select
                  v-model="form.nodejs_env_rec_id"
                  clearable
                  filterable
                  class="project-editor__full"
                  @visible-change="onEnvSelectVisible"
                >
                  <el-option
                    v-for="e in envOptions('nodejs')"
                    :key="e.rec_id"
                    :label="e.env_name"
                    :value="e.rec_id"
                  />
                </el-select>
              </div>
            </el-form-item>

            <p v-if="!showJdk && !showMaven && !showNodejs" class="project-editor__empty-hint">
              {{ $t('projectEnvironmentEmpty') }}
            </p>
          </section>

          <!-- Build Steps -->
          <section v-show="activeSection === 'build'" class="project-editor__panel">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavBuild') }}</h3>
              <p class="project-editor__panel-desc">{{ $t('projectBuildDesc') }}</p>
            </header>

            <div class="project-editor__build-block">
              <div class="project-editor__build-block-head">{{ $t('projectBlockBuild') }}</div>
              <div class="project-editor__build-block-body">
                <el-form-item :label="$t('projectColBuildCmd')" label-width="148px">
                  <el-input v-model="buildConfigModel.build_cmd" type="textarea" :rows="3" />
                </el-form-item>
                <el-form-item :label="$t('projectColOutputDir')" label-width="148px">
                  <el-input v-model="buildConfigModel.output_dir" />
                </el-form-item>
                <el-form-item :label="$t('projectColArtifact')" label-width="148px">
                  <el-input v-model="buildConfigModel.artifact_pattern" />
                </el-form-item>
                <el-form-item :label="$t('projectColLogRetention')" label-width="148px">
                  <el-input-number
                    v-model="buildConfigModel.log_retention_days"
                    :min="1"
                    :max="365"
                    :step="1"
                  />
                  <p class="project-editor__hint">{{ $t('projectLogRetentionHint') }}</p>
                </el-form-item>
              </div>
            </div>
          </section>

          <!-- Build triggers (dependencies & schedule) -->
          <section v-show="activeSection === 'triggers'" class="project-editor__panel">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavTriggers') }}</h3>
              <p class="project-editor__panel-desc">{{ $t('projectBuildTriggersDesc') }}</p>
            </header>

            <ProjectBuildTriggersPanel
              v-model:triggers="buildConfigModel.build_triggers"
              :editing-id="editingId"
              :workspace-projects="workspaceProjects ?? []"
            />
          </section>

          <!-- Post-build actions -->
          <section v-show="activeSection === 'deploy'" class="project-editor__panel project-editor__panel--wide">
            <header class="project-editor__panel-head">
              <h3 class="project-editor__panel-title">{{ $t('projectNavDeploy') }}</h3>
              <p class="project-editor__panel-desc">{{ $t('projectPostBuildDesc') }}</p>
            </header>

            <ProjectPostBuildPanel
              v-model:actions="buildConfigModel.post_build_actions"
              :env-options="envOptions"
              @reload-envs="emit('reloadEnvs')"
            />
          </section>

          <footer class="project-editor__footer">
            <el-button @click="emit('back')">{{ $t('cancel') }}</el-button>
            <el-button type="primary" :loading="saving" native-type="submit">{{ $t('save') }}</el-button>
          </footer>
        </el-form>
      </div>
    </div>
  </el-card>
</template>

<style scoped lang="scss" src="./ProjectEditor.scss"></style>
