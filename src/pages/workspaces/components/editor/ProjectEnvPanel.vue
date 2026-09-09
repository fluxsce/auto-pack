<script setup lang="ts">
import { RsFormItem, RsSelect, type RsSelectModelValue } from '@/ui'
import type { ProjectRecipe } from '../../lib/project-defaults'

const recipe = defineModel<ProjectRecipe>('recipe', { required: true })

defineProps<{
  kinds: string[]
  optionsOf: (kind: string) => { value: string; label: string }[]
}>()

const emit = defineEmits<{
  envChange: [key: keyof ProjectRecipe, value: RsSelectModelValue]
}>()

function bindKey(kind: string): keyof ProjectRecipe {
  if (kind === 'jdk') return 'jdk_env_rec_id'
  if (kind === 'maven') return 'maven_env_rec_id'
  return 'node_env_rec_id'
}

function boundValue(kind: string): string {
  const key = bindKey(kind)
  return String(recipe.value[key] ?? '')
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ $t('workspaces.nav.environment') }}</h3>
      <p>{{ $t('workspaces.sectionEnvDesc') }}</p>
    </header>
    <div v-for="kind in kinds" :key="kind" class="card">
      <div class="card-head">
        <h4>{{ $t(`envs.kinds.${kind}`) }}</h4>
      </div>
      <RsFormItem :label="$t('workspaces.envPick')">
        <RsSelect
          :model-value="boundValue(kind)"
          :options="optionsOf(kind)"
          clearable
          :placeholder="$t('workspaces.envPick')"
          @update:model-value="(v: RsSelectModelValue) => emit('envChange', bindKey(kind), v)"
        />
      </RsFormItem>
    </div>
  </section>
</template>

<style scoped src="./panel.css"></style>
