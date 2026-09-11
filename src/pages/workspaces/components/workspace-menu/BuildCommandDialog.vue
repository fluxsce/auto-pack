<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsRadio, RsRadioItem, RsSwitch, useRsToast } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { errText, listProjects, updateWorkspaceCommands, type Project } from '../../api'
import {
  type CommandFamily,
  type CommandPatchDraft,
  commandWithGoal,
  familyProjectType,
  lastPackGoal,
  matchCommandFamily,
  previewCommand,
  projectsForPatch,
  recipeOf,
} from '../../lib/command-patch'
import { emptyRecipe } from '../../lib/project-defaults'
import ProjectBuildPanel from '../editor/ProjectBuildPanel.vue'

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{
  success: []
}>()

const props = defineProps<{
  workspace?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const saving = ref(false)
const loading = ref(false)
const rows = ref<Project[]>([])
const family = ref<CommandFamily>('maven')
const keepPhases = ref(true)
const applyMeta = ref(false)
const recipe = ref(emptyRecipe('maven'))

const projectType = computed(() => familyProjectType(family.value))

const draft = computed<CommandPatchDraft>(() => ({
  family: family.value,
  keepPhases: keepPhases.value,
  applyMeta: applyMeta.value,
  recipe: recipe.value,
}))

const familyRows = computed(() => rows.value.filter((row) => matchCommandFamily(row.project_type, family.value)))
const matches = computed(() => projectsForPatch(rows.value, draft.value))
const samples = computed(() =>
  matches.value.slice(0, 6).map((row) => ({
    name: row.project_name,
    from: recipeOf(row).command,
    to: previewCommand(row, draft.value) || recipeOf(row).command,
  })),
)

const canSubmit = computed(() => !saving.value && !loading.value && matches.value.length > 0 && !!recipe.value.command.trim())

const mavenGoals = ['package', 'install', 'deploy'] as const

watch(open, (visible) => {
  if (!visible) return
  family.value = 'maven'
  keepPhases.value = true
  applyMeta.value = false
  recipe.value = emptyRecipe('maven')
  recipe.value.command = commandWithGoal('maven', 'install')
  void loadRows()
})

watch(family, (next) => {
  const type = familyProjectType(next)
  recipe.value = emptyRecipe(type)
  keepPhases.value = next === 'maven'
  if (next === 'maven') recipe.value.command = commandWithGoal('maven', 'install')
})

async function loadRows(): Promise<void> {
  const id = props.workspace?.rec_id || ''
  if (!id) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    rows.value = await listProjects(id)
  } catch (e) {
    rows.value = []
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

function setGoal(goal: string): void {
  recipe.value.command = commandWithGoal(projectType.value, goal)
  keepPhases.value = true
}

function setNodeCommand(kind: 'npm' | 'pnpm'): void {
  keepPhases.value = false
  recipe.value.command = kind === 'pnpm' ? 'pnpm run build' : 'npm run build'
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id || !canSubmit.value) return
  saving.value = true
  try {
    const goal = family.value === 'maven' && keepPhases.value ? lastPackGoal(recipe.value.command) : ''
    const out = await updateWorkspaceCommands(id, {
      family: family.value,
      goal: goal || undefined,
      set_command: goal ? undefined : recipe.value.command.trim(),
      workdir: applyMeta.value ? recipe.value.workdir || '' : undefined,
      output_dir: applyMeta.value ? recipe.value.output_dir || '' : undefined,
      artifact_pattern: applyMeta.value ? recipe.value.artifact_pattern || '' : undefined,
      log_retention_days: applyMeta.value ? recipe.value.log_retention_days ?? 0 : undefined,
    })
    if (out.updated === 0) {
      toast.warning(t('workspaces.batchCommandEmpty'))
      return
    }
    toast.success(t('workspaces.batchCommandDone', { n: out.updated }))
    open.value = false
    emit('success')
  } catch (e) {
    toast.error(errText(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="form"
    width="md"
    teleport-to="body"
    :title="t('workspaces.batchCommand')"
    :description="t('workspaces.batchCommandHint')"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.batchCommandFamily')">
          <RsRadio v-model="family">
            <RsRadioItem value="maven">Maven</RsRadioItem>
            <RsRadioItem value="node">Node / Vue</RsRadioItem>
          </RsRadio>
        </RsFormItem>
        <RsFormItem v-if="family === 'maven'" :label="t('workspaces.batchCommandGoal')">
          <div class="tags">
            <button
              v-for="goal in mavenGoals"
              :key="goal"
              type="button"
              class="tag"
              :class="{ 'is-on': lastPackGoal(recipe.command) === goal }"
              @click="setGoal(goal)"
            >
              {{ goal }}
            </button>
          </div>
        </RsFormItem>
        <RsFormItem v-else :label="t('workspaces.batchCommandPresets')">
          <div class="tags">
            <button type="button" class="tag" @click="setNodeCommand('npm')">npm run build</button>
            <button type="button" class="tag" @click="setNodeCommand('pnpm')">pnpm run build</button>
          </div>
        </RsFormItem>
        <RsSwitch v-if="family === 'maven'" v-model="keepPhases">
          {{ t('workspaces.batchCommandKeepPhases') }}
        </RsSwitch>
        <RsSwitch v-model="applyMeta">
          {{ t('workspaces.batchCommandApplyMeta') }}
        </RsSwitch>
        <ProjectBuildPanel v-model:recipe="recipe" :project-type="projectType" />
        <p class="hint">
          {{
            loading
              ? t('workspaces.batchCommandLoading')
              : matches.length
                ? t('workspaces.batchCommandCount', { n: matches.length, name: workspace?.workspace_name || '' })
                : familyRows.length
                  ? t('workspaces.batchCommandEmpty')
                  : t('workspaces.batchCommandNoFamily')
          }}
        </p>
        <ul v-if="samples.length" class="samples">
          <li v-for="item in samples" :key="item.name">
            <span class="samples__name">{{ item.name }}</span>
            <code>{{ item.from }}</code>
            <span class="samples__arrow">→</span>
            <code>{{ item.to }}</code>
          </li>
        </ul>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="saving" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" :disabled="!canSubmit" @click="submit">{{ t('common.save') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.tag.is-on {
  border-color: #0ea5e9;
  color: #0ea5e9;
}

.samples {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.samples li {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.35rem 0.5rem;
  font-size: 0.75rem;
  color: var(--shell-muted);
}

.samples__name {
  min-width: 7rem;
  font-weight: 650;
  color: var(--shell-title);
}

.samples code {
  font-family: var(--rs-font-mono);
  color: var(--shell-title);
}

.samples__arrow {
  color: #0ea5e9;
}
</style>
