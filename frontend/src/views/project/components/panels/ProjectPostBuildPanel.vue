<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Delete, Plus, Rank } from '@element-plus/icons-vue'
import type { EnvSetting } from '@/types'
import {
  applyPostBuildTypeChange,
  defaultPostBuildAction,
  newPostBuildActionId,
} from '../../post-build/post-build-defaults'
import { POST_BUILD_TYPE_REGISTRY, postBuildFieldComponent } from '../../post-build/post-build-registry'
import { POST_BUILD_WHEN_OPTIONS, type PostBuildAction, type PostBuildActionType } from '@/types/post-build'

const props = defineProps<{
  envOptions: (envType: string) => EnvSetting[]
}>()

const emit = defineEmits<{
  reloadEnvs: []
}>()

const actions = defineModel<PostBuildAction[]>('actions', { required: true })

const { t } = useI18n()

const addActionType = ref<PostBuildActionType | ''>('')

const actionTypeOptions = computed(() =>
  POST_BUILD_TYPE_REGISTRY.map((d) => ({
    value: d.type,
    label: t(d.labelKey),
  })),
)

function addAction(type: PostBuildActionType): void {
  actions.value = [...actions.value, defaultPostBuildAction(type)]
}

function confirmAddAction(): void {
  const type = addActionType.value
  if (!type) return
  addAction(type)
  addActionType.value = ''
}

function removeAction(index: number): void {
  actions.value = actions.value.filter((_, i) => i !== index)
}

function duplicateAction(index: number): void {
  const src = actions.value[index]
  if (!src) return
  const copy: PostBuildAction = {
    ...structuredClone(src),
    id: newPostBuildActionId(),
    name: src.name ? `${src.name} (${t('projectPostBuildCopySuffix')})` : '',
  }
  actions.value = [...actions.value.slice(0, index + 1), copy, ...actions.value.slice(index + 1)]
}

function onTypeChange(action: PostBuildAction, type: PostBuildActionType): void {
  applyPostBuildTypeChange(action, type)
}

function whenLabel(when: string): string {
  return t(`projectPostBuildWhen_${when}`)
}
</script>

<template>
  <div class="post-build-panel">
    <p class="post-build-panel__desc">{{ $t('projectPostBuildDesc') }}</p>

    <el-empty
      v-if="actions.length === 0"
      :description="$t('projectPostBuildEmpty')"
      class="post-build-panel__empty"
    />

    <div v-for="(action, index) in actions" :key="action.id" class="post-build-panel__action">
      <div class="post-build-panel__action-toolbar">
        <el-icon class="post-build-panel__drag" aria-hidden="true"><Rank /></el-icon>
        <el-switch v-model="action.enabled" />
        <el-input
          v-model="action.name"
          class="post-build-panel__name"
          :placeholder="$t('projectPostBuildNamePlaceholder')"
        />
        <el-select
          v-model="action.type"
          class="post-build-panel__type-select"
          @change="onTypeChange(action, action.type)"
        >
          <el-option
            v-for="d in POST_BUILD_TYPE_REGISTRY"
            :key="d.type"
            :label="$t(d.labelKey)"
            :value="d.type"
          />
        </el-select>
        <el-button text type="primary" @click="duplicateAction(index)">
          {{ $t('projectPostBuildDuplicate') }}
        </el-button>
        <el-button text type="danger" :icon="Delete" @click="removeAction(index)" />
      </div>

      <div class="post-build-panel__action-body">
        <el-form-item :label="$t('projectPostBuildWhen')">
          <el-select v-model="action.when" class="post-build-panel__full">
            <el-option
              v-for="w in POST_BUILD_WHEN_OPTIONS"
              :key="w"
              :label="whenLabel(w)"
              :value="w"
            />
          </el-select>
        </el-form-item>

        <component
          :is="postBuildFieldComponent(action.type)"
          v-model="actions[index]"
          :env-options="props.envOptions"
          @reload-envs="emit('reloadEnvs')"
        />
      </div>
    </div>

    <section class="post-build-panel__add">
      <h4 class="post-build-panel__add-title">{{ $t('projectPostBuildAddAction') }}</h4>
      <div class="post-build-panel__add-row">
        <el-select
          v-model="addActionType"
          clearable
          filterable
          class="post-build-panel__add-select"
          :placeholder="$t('projectPostBuildAddActionPlaceholder')"
        >
          <el-option
            v-for="opt in actionTypeOptions"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
        <el-button type="primary" :icon="Plus" :disabled="!addActionType" @click="confirmAddAction">
          {{ $t('projectPostBuildAddConfirm') }}
        </el-button>
      </div>
    </section>
  </div>
</template>

<style scoped lang="scss" src="./ProjectPostBuildPanel.scss"></style>
