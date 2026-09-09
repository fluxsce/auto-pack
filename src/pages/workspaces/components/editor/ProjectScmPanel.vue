<script setup lang="ts">
import { RsFormItem, RsInput, RsRadio, RsRadioItem, RsSelect, RsSwitch, type RsSelectModelValue } from '@/ui'
import type { ProjectRecipe } from '../../lib/project-defaults'

const recipe = defineModel<ProjectRecipe>('recipe', { required: true })
const form = defineModel<{ repo_url: string; repo_branch: string }>('form', { required: true })

defineProps<{
  gitOptions: { value: string; label: string }[]
  branchTags: string[]
}>()

const emit = defineEmits<{
  envChange: [key: keyof ProjectRecipe, value: RsSelectModelValue]
}>()
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ $t('workspaces.nav.scm') }}</h3>
      <p>{{ $t('workspaces.sectionScmDesc') }}</p>
    </header>
    <RsFormItem :label="$t('workspaces.nav.scm')">
      <RsRadio v-model="recipe.scm_mode">
        <RsRadioItem value="local">{{ $t('workspaces.scmLocal') }}</RsRadioItem>
        <RsRadioItem value="git">Git</RsRadioItem>
      </RsRadio>
    </RsFormItem>
    <template v-if="recipe.scm_mode === 'local'">
      <RsInput v-model="recipe.local_source_path" :label="$t('workspaces.localPath')" :placeholder="$t('workspaces.localPathPh')" />
      <p class="hint">{{ $t('workspaces.localPathHint') }}</p>
    </template>
    <template v-else>
      <RsInput v-model="form.repo_url" :label="$t('workspaces.gitUrl')" :placeholder="$t('workspaces.gitUrlPh')" required />
      <RsInput v-model="form.repo_branch" :label="$t('workspaces.branch')" :placeholder="$t('workspaces.branchPh')" />
      <div class="tags">
        <button v-for="tag in branchTags" :key="tag" type="button" class="tag" @click="form.repo_branch = tag">{{ tag }}</button>
      </div>
      <RsFormItem :label="$t('workspaces.gitAuth')">
        <RsSelect
          :model-value="recipe.git_env_rec_id"
          :options="gitOptions"
          clearable
          :placeholder="$t('workspaces.gitAuthPh')"
          @update:model-value="(v: RsSelectModelValue) => emit('envChange', 'git_env_rec_id', v)"
        />
      </RsFormItem>
      <RsFormItem :label="$t('workspaces.autoSync')">
        <RsSwitch v-model="recipe.auto_sync" />
      </RsFormItem>
      <p class="hint">{{ $t('workspaces.autoSyncHint') }}</p>
    </template>
  </section>
</template>

<style scoped src="./panel.css"></style>
