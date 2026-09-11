<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsFormItem, RsInput, RsInputNumber, RsTextarea } from '@/ui'
import { defaultBuild, isCustomProjectType, type ProjectRecipe } from '../../lib/project-defaults'

const recipe = defineModel<ProjectRecipe>('recipe', { required: true })

const props = defineProps<{
  projectType: string
}>()

const { t } = useI18n()
const preview = computed(() => {
  const dir = recipe.value.output_dir?.trim() || t('workspaces.outputDir')
  const pattern = recipe.value.artifact_pattern?.trim() || '*.jar / *.war'
  return `${dir} / ${pattern}`
})

function onRetention(value: number | string | null): void {
  const n = typeof value === 'number' ? value : Number(value)
  recipe.value.log_retention_days = Number.isFinite(n) ? Math.max(0, Math.min(365, n)) : 0
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ t('workspaces.nav.build') }}</h3>
      <p>{{ t('workspaces.sectionBuildDesc') }}</p>
    </header>
    <RsTextarea
      v-model="recipe.command"
      :label="t('workspaces.command')"
      :required="!isCustomProjectType(projectType)"
      :rows="4"
      :placeholder="defaultBuild(projectType).command"
    />
    <p v-if="isCustomProjectType(projectType)" class="hint">{{ t('workspaces.commandSkipHint') }}</p>
    <RsInput
      v-model="recipe.workdir"
      :label="t('workspaces.workdir')"
      :placeholder="t('workspaces.workdirPh')"
    />
    <p class="hint">{{ t('workspaces.workdirHint') }}</p>
    <div class="grid2">
      <RsInput v-model="recipe.output_dir" :label="t('workspaces.outputDir')" :placeholder="t('workspaces.outputDir')" />
      <RsInput
        v-model="recipe.artifact_pattern"
        :label="t('workspaces.artifact')"
        :placeholder="t('workspaces.artifactPh')"
      />
    </div>
    <div class="path">
      <span>{{ t('workspaces.artifactPreview') }}</span>
      <code>{{ preview }}</code>
    </div>
    <RsFormItem :label="t('workspaces.logRetention')">
      <RsInputNumber
        :model-value="recipe.log_retention_days ?? 0"
        :min="0"
        :max="365"
        :step="1"
        @update:model-value="onRetention"
      />
    </RsFormItem>
    <p class="hint">{{ t('workspaces.logRetentionHint') }}</p>
  </section>
</template>

<style scoped src="./panel.css"></style>
