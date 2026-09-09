<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { RsButton, RsCard, RsIcon, RsTag, useRsToast, type RsSelectModelValue } from '@/ui'
import type { EnvRow } from '@/pages/envs/kinds'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  asSelectString,
  errText,
  getProject,
  isBuildJob,
  listJobs,
  listProjects,
  listWorkspaceEnvs,
  saveProject,
  type JobRow,
  type Project,
} from '../api'
import { cronValidity } from '../lib/cron'
import { isGitURL, isLocalSourcePath, isPathSlug, uniqueCopyName } from '../lib/path-slug'
import { envKindForAction } from '../lib/post-actions'
import {
  defaultBuild,
  emptyRecipe,
  envKindsForType,
  parseRecipe,
  projectTypes,
  serializeRecipe,
  workRel,
  type ProjectRecipe,
} from '../lib/project-defaults'
import { asProjectSection, projectSectionIcons, projectSections, type ProjectSection } from '../lib/project-sections'
import ProjectBuildPanel from './editor/ProjectBuildPanel.vue'
import ProjectDeployPanel from './editor/ProjectDeployPanel.vue'
import ProjectEnvPanel from './editor/ProjectEnvPanel.vue'
import ProjectGeneralPanel from './editor/ProjectGeneralPanel.vue'
import ProjectScmPanel from './editor/ProjectScmPanel.vue'
import ProjectTriggerPanel from './editor/ProjectTriggerPanel.vue'
import ProjectHistoryPane from './detail/ProjectHistoryPane.vue'

const { t } = useI18n()
const toast = useRsToast()
const route = useRoute()
const router = useRouter()
const ws = useWorkspaceStore()
const saving = ref(false)
const loading = ref(false)
const advancedOpen = ref(false)
const envs = ref<EnvRow[]>([])
const peers = ref<Project[]>([])
const jobs = ref<JobRow[]>([])
const form = ref({
  rec_id: '',
  project_name: '',
  project_type: 'maven',
  repo_url: '',
  repo_branch: 'main',
  cron_expr: '',
  is_enabled: 1,
  remark: '',
})
const recipe = ref<ProjectRecipe>(emptyRecipe('maven'))

const isNew = computed(() => route.name === 'workspace-project-new')
const cloneFromId = computed(() => String(route.query.cloneFrom ?? '').trim())
const isClone = computed(() => isNew.value && Boolean(cloneFromId.value))
const current = computed(() => ws.list.find((item) => item.rec_id === ws.currentId))
const typeOptions = computed(() => projectTypes.map((item) => ({ value: item.value, label: item.label })))
const title = computed(() => {
  const name = form.value.project_name.trim()
  if (name) return name
  if (isClone.value) return t('workspaces.cloneProject')
  return isNew.value ? t('workspaces.newProject') : t('workspaces.configureProject')
})
const section = computed(() => asProjectSection(route.query.section))
const sections = computed(() =>
  projectSections.map((id) => ({
    id,
    title: t(`workspaces.nav.${id}`),
    icon: projectSectionIcons[id],
  })),
)
const checkoutPath = computed(() => {
  const root = current.value?.root_dir?.replace(/[/\\]+$/, '') ?? ''
  const name = form.value.project_name.trim() || t('workspaces.projectName')
  const rel = workRel(name, recipe.value)
  return root ? `${root}/${rel}` : rel
})
const toolchainKinds = computed(() => envKindsForType(form.value.project_type))
const branchTags = computed(() => ['main', 'develop', 'release'])
const cronState = computed(() => cronValidity(form.value.cron_expr))
const gitOptions = computed(() => envOptions('git'))
const peerOptions = computed(() =>
  peers.value
    .filter((item) => item.rec_id !== form.value.rec_id)
    .map((item) => ({ value: item.rec_id, label: item.project_name })),
)

function envOptions(kind: string): { value: string; label: string }[] {
  return envs.value
    .filter((item) => item.env_kind === kind && item.is_enabled !== 0)
    .map((item) => ({ value: item.rec_id, label: item.env_name }))
}

function sectionMark(id: ProjectSection): string {
  if (id === 'scm') {
    if (recipe.value.scm_mode === 'git' && !form.value.repo_url.trim()) return t('workspaces.sectionUnset')
    return ''
  }
  if (id === 'environment') {
    const bound = toolchainKinds.value.some((kind) => {
      if (kind === 'jdk') return !!recipe.value.jdk_env_rec_id
      if (kind === 'maven') return !!recipe.value.maven_env_rec_id
      if (kind === 'node') return !!recipe.value.node_env_rec_id
      return false
    })
    return toolchainKinds.value.length && !bound ? t('workspaces.envFallback') : ''
  }
  if (id === 'build') return recipe.value.command.trim() ? '' : t('workspaces.sectionUnset')
  if (id === 'trigger') {
    const hasCron = cronState.value === 'ok'
    const hasDep = (recipe.value.depends_on ?? []).length > 0
    if (cronState.value === 'bad') return t('workspaces.sectionUnset')
    if (!hasCron && !hasDep) return t('workspaces.sectionOff')
    if (form.value.is_enabled === 0 && hasCron) return t('workspaces.sectionOff')
    return ''
  }
  if (id === 'deploy') return (recipe.value.post_actions ?? []).length ? '' : t('workspaces.sectionOff')
  return ''
}

function goSection(id: ProjectSection): void {
  void router.replace({ query: { ...route.query, section: id } })
}

function blank(): void {
  form.value = {
    rec_id: '',
    project_name: '',
    project_type: 'maven',
    repo_url: '',
    repo_branch: 'main',
    cron_expr: '',
    is_enabled: 1,
    remark: '',
  }
  recipe.value = emptyRecipe('maven')
  advancedOpen.value = false
}

async function loadEnvs(): Promise<void> {
  if (!ws.currentId) {
    envs.value = []
    return
  }
  try {
    envs.value = await listWorkspaceEnvs(ws.currentId)
  } catch {
    envs.value = []
  }
}

async function loadPeers(): Promise<void> {
  if (!ws.currentId) {
    peers.value = []
    return
  }
  try {
    peers.value = await listProjects(ws.currentId)
  } catch {
    peers.value = []
  }
}

async function loadJobs(): Promise<void> {
  const projectId = form.value.rec_id
  if (!ws.currentId || !projectId) {
    jobs.value = []
    return
  }
  try {
    const rows = await listJobs(ws.currentId, { projectId, jobType: 'build' })
    jobs.value = rows.filter((item) => isBuildJob(item))
  } catch {
    jobs.value = []
  }
}

async function applyClone(row: Project): Promise<void> {
  if (row.workspace_rec_id && row.workspace_rec_id !== ws.currentId) {
    ws.select(row.workspace_rec_id)
  }
  const workspaceId = row.workspace_rec_id || ws.currentId
  const rows = workspaceId ? await listProjects(workspaceId) : []
  form.value = {
    rec_id: '',
    project_name: uniqueCopyName(row.project_name, rows.map((item) => item.project_name)),
    project_type: row.project_type,
    repo_url: row.repo_url || '',
    repo_branch: row.repo_branch || '',
    cron_expr: row.cron_expr || '',
    is_enabled: row.is_enabled ?? 1,
    remark: row.remark || '',
  }
  recipe.value = parseRecipe(row.recipe_json, row.project_type)
  advancedOpen.value = !!form.value.remark
  jobs.value = []
}

async function load(): Promise<void> {
  if (isNew.value) {
    if (!cloneFromId.value) {
      blank()
      jobs.value = []
      return
    }
    loading.value = true
    try {
      await applyClone(await getProject(cloneFromId.value))
    } catch (e) {
      toast.error(errText(e))
      await back()
    } finally {
      loading.value = false
    }
    return
  }
  const id = String(route.params.id ?? '')
  if (!id) {
    await back()
    return
  }
  loading.value = true
  try {
    const row = await getProject(id)
    form.value = {
      rec_id: row.rec_id,
      project_name: row.project_name,
      project_type: row.project_type,
      repo_url: row.repo_url || '',
      repo_branch: row.repo_branch || '',
      cron_expr: row.cron_expr || '',
      is_enabled: row.is_enabled ?? 1,
      remark: row.remark || '',
    }
    recipe.value = parseRecipe(row.recipe_json, row.project_type)
    advancedOpen.value = !!form.value.remark
    if (row.workspace_rec_id && row.workspace_rec_id !== ws.currentId) {
      ws.select(row.workspace_rec_id)
    }
    await loadJobs()
  } catch (e) {
    toast.error(errText(e))
    await back()
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadEnvs()
  void loadPeers()
})
watch(() => ws.currentId, () => {
  void loadEnvs()
  void loadPeers()
})
watch(section, (id) => {
  if (id === 'scm' || id === 'environment' || id === 'deploy' || id === 'trigger') {
    void loadEnvs()
    void loadPeers()
  }
})
watch(
  [() => String(route.name ?? ''), () => String(route.params.id ?? ''), () => cloneFromId.value],
  () => {
    void load()
  },
  { immediate: true },
)

function onType(value: RsSelectModelValue): void {
  const next = asSelectString(value)
  const prev = defaultBuild(form.value.project_type)
  const nextBuild = defaultBuild(next)
  if (!recipe.value.command.trim() || recipe.value.command === prev.command) {
    recipe.value.command = nextBuild.command ?? ''
  }
  if (!recipe.value.output_dir?.trim() || recipe.value.output_dir === prev.output_dir) {
    recipe.value.output_dir = nextBuild.output_dir
  }
  if (!recipe.value.artifact_pattern?.trim() || recipe.value.artifact_pattern === prev.artifact_pattern) {
    recipe.value.artifact_pattern = nextBuild.artifact_pattern
  }
  form.value.project_type = next
}

function onEnv(key: keyof ProjectRecipe, value: RsSelectModelValue): void {
  recipe.value[key] = asSelectString(value) as never
}

function slugValidator(value: string): true | string {
  return isPathSlug(value) ? true : t('workspaces.slugRule')
}

function openJob(id: string): void {
  if (!form.value.rec_id) return
  void router.push({
    name: 'workspace-project',
    params: { id: form.value.rec_id },
    query: { tab: 'console', job: id },
  })
}

async function back(): Promise<void> {
  if (!isNew.value && form.value.rec_id) {
    await router.push({ name: 'workspace-project', params: { id: form.value.rec_id } })
    return
  }
  await router.push({ name: 'workspaces' })
}

function failSection(id: ProjectSection, message: string): boolean {
  goSection(id)
  toast.error(message)
  return false
}

function validateForm(): boolean {
  if (!form.value.project_name.trim() || !isPathSlug(form.value.project_name)) {
    return failSection('general', t('workspaces.slugRule'))
  }
  if (!form.value.project_type.trim()) {
    return failSection('general', t('common.required'))
  }
  if (recipe.value.scm_mode === 'local' && !isLocalSourcePath(recipe.value.local_source_path || '')) {
    return failSection('scm', t('workspaces.localPathRule'))
  }
  if (recipe.value.scm_mode === 'git') {
    if (!form.value.repo_url.trim()) return failSection('scm', t('workspaces.gitUrlRequired'))
    if (!isGitURL(form.value.repo_url)) return failSection('scm', t('workspaces.gitUrlRule'))
  }
  if (!recipe.value.command.trim()) {
    return failSection('build', t('workspaces.commandRequired'))
  }
  if (cronState.value === 'bad') {
    return failSection('trigger', t('workspaces.cronInvalid'))
  }
  recipe.value.depends_on = (recipe.value.depends_on ?? []).filter((id) => id && id !== form.value.rec_id)
  const badPost = (recipe.value.post_actions ?? []).some((item) => {
    if (item.enabled === false) return false
    const need = envKindForAction(item.kind)
    return !!need && !item.env_rec_id
  })
  if (badPost) {
    return failSection('deploy', t('workspaces.postEnvRequired'))
  }
  const badShell = (recipe.value.post_actions ?? []).some((item) => item.kind === 'shell' && item.enabled !== false && !item.command?.trim())
  if (badShell) {
    return failSection('deploy', t('workspaces.shellRequired'))
  }
  return true
}

async function save(): Promise<void> {
  if (!ws.currentId) {
    toast.error(t('workspaces.emptyWs'))
    return
  }
  if (!validateForm()) return
  saving.value = true
  try {
    const out = await saveProject({
      rec_id: form.value.rec_id,
      workspace_rec_id: ws.currentId,
      project_name: form.value.project_name,
      project_type: form.value.project_type,
      repo_url: form.value.repo_url,
      repo_branch: form.value.repo_branch,
      work_rel_path: workRel(form.value.project_name.trim(), recipe.value),
      recipe_json: serializeRecipe(recipe.value),
      cron_expr: form.value.cron_expr,
      is_enabled: form.value.is_enabled,
      remark: form.value.remark,
    })
    toast.success(t('common.saved'))
    await ws.load()
    if (out.rec_id) {
      form.value.rec_id = out.rec_id
    }
    if (isNew.value && out.rec_id) {
      const { cloneFrom: _cloneFrom, ...query } = route.query
      await router.replace({
        name: 'workspace-project-edit',
        params: { id: out.rec_id },
        query,
      })
    } else {
      await loadJobs()
    }
  } catch (e) {
    toast.error(errText(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="ws-pane">
    <RsCard class="editor" fill variant="outlined" elevated radius="lg" :padding="false">
      <template #header>
        <div class="editor__head">
          <h3 class="rs-card__title">{{ title }}</h3>
          <RsTag round>{{ form.project_type }}</RsTag>
          <RsTag v-if="isClone" variant="info" round>{{ t('workspaces.cloneFrom') }}</RsTag>
          <RsButton class="editor__back" size="sm" variant="ghost" :disabled="saving" @click="back">
            {{ t('workspaces.backToProjects') }}
          </RsButton>
        </div>
      </template>

      <div class="editor__layout">
        <nav class="editor__nav" :aria-label="t('workspaces.configureProject')">
          <button
            v-for="item in sections"
            :key="item.id"
            type="button"
            class="editor__nav-item"
            :class="{ 'is-active': item.id === section }"
            @click="goSection(item.id)"
          >
            <RsIcon :name="item.icon" :size="16" />
            <span>{{ item.title }}</span>
            <em v-if="sectionMark(item.id)" class="editor__nav-mark">{{ sectionMark(item.id) }}</em>
          </button>
          <div v-if="form.rec_id" class="editor__history">
            <ProjectHistoryPane aside :jobs="jobs" selected-id="" @select="openJob" />
          </div>
        </nav>

        <div class="editor__main">
        <form class="editor__form" @submit.prevent="save">
          <ProjectGeneralPanel
            v-show="section === 'general'"
            v-model:form="form"
            v-model:advanced="advancedOpen"
            :type-options="typeOptions"
            :checkout-path="checkoutPath"
            :name-validator="slugValidator"
            @type-change="onType"
          />
          <ProjectScmPanel
            v-show="section === 'scm'"
            v-model:recipe="recipe"
            v-model:form="form"
            :git-options="gitOptions"
            :branch-tags="branchTags"
            @env-change="onEnv"
          />
          <ProjectEnvPanel
            v-show="section === 'environment'"
            v-model:recipe="recipe"
            :kinds="toolchainKinds"
            :options-of="envOptions"
            @env-change="onEnv"
          />
          <ProjectBuildPanel v-show="section === 'build'" v-model:recipe="recipe" :project-type="form.project_type" />
          <ProjectTriggerPanel
            v-show="section === 'trigger'"
            v-model:recipe="recipe"
            v-model:form="form"
            :cron-state="cronState"
            :project-options="peerOptions"
          />
          <ProjectDeployPanel v-show="section === 'deploy'" v-model:recipe="recipe" :envs="envs" />

          <footer class="editor__footer">
            <RsButton variant="ghost" :disabled="saving" @click="back">{{ t('common.cancel') }}</RsButton>
            <RsButton type="submit" :loading="saving" :disabled="loading || !current">{{ t('common.save') }}</RsButton>
          </footer>
        </form>
        </div>
      </div>
    </RsCard>
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

.editor {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.editor :deep(.rs-card__body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.editor__head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.editor__back {
  margin-left: auto;
}

.editor__layout {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  align-items: stretch;
}

.editor__nav {
  display: flex;
  flex-direction: column;
  align-self: stretch;
  flex-shrink: 0;
  width: 15.5rem;
  min-height: 0;
  padding: 0.55rem 0 0;
  border-right: 1px solid var(--shell-border);
  background: var(--shell-inset-bg);
}

.editor__nav-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  padding: 0.65rem 0.9rem;
  border: 0;
  border-left: 3px solid transparent;
  background: transparent;
  color: var(--shell-title);
  font-size: 0.86rem;
  text-align: left;
  cursor: pointer;
}

.editor__nav-item:hover {
  background: var(--shell-nav-hover);
}

.editor__nav-item.is-active {
  border-left-color: #0ea5e9;
  background: color-mix(in srgb, #0ea5e9 10%, transparent);
  font-weight: 650;
  color: #0ea5e9;
}

.editor__nav-mark {
  margin-left: auto;
  font-size: 0.7rem;
  font-style: normal;
  font-weight: 500;
  color: var(--shell-muted);
}

.editor__main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: auto;
}

.editor__form {
  display: flex;
  flex-direction: column;
  gap: 0.95rem;
  padding: 1.1rem 1.2rem 1rem;
}

.editor__history {
  flex: 1;
  min-height: 0;
  margin-top: 0.35rem;
  padding: 0.7rem 0.65rem 0.75rem;
  border-top: 1px solid var(--shell-border);
  overflow: hidden;
}

.editor__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.55rem;
  margin-top: auto;
  padding-top: 0.85rem;
  border-top: 1px solid var(--shell-border);
}

@media (max-width: 720px) {
  .editor__history {
    display: none;
  }

  .editor__layout {
    flex-direction: column;
    min-height: 0;
  }

  .editor__nav {
    width: auto;
    flex-direction: row;
    overflow: auto;
    border-right: 0;
    border-bottom: 1px solid var(--shell-border);
  }

  .editor__nav-item {
    flex-shrink: 0;
    border-left: 0;
    border-bottom: 3px solid transparent;
  }

  .editor__nav-item.is-active {
    border-left-color: transparent;
    border-bottom-color: #0ea5e9;
  }
}
</style>
