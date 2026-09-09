<script setup lang="ts">
import type { PostBuildAction } from '@/types/post-build'
import type { PostBuildFieldsProps } from '../post-build-fields-props'

const props = defineProps<PostBuildFieldsProps>()

const action = defineModel<PostBuildAction>({ required: true })

const emit = defineEmits<{
  reloadEnvs: []
}>()

function onEnvSelectVisible(visible: boolean): void {
  if (visible) emit('reloadEnvs')
}
</script>

<template>
  <div class="post-build-fields">
    <el-form-item :label="$t('projectPostBuildRemoteEnv')" required>
      <el-select
        v-model="action.env_rec_id"
        clearable
        filterable
        class="post-build-fields__full"
        :placeholder="$t('projectPostBuildSshEnvPlaceholder')"
        @visible-change="onEnvSelectVisible"
      >
        <el-option
          v-for="e in props.envOptions('ssh')"
          :key="e.rec_id"
          :label="e.env_name"
          :value="e.rec_id"
        />
      </el-select>
    </el-form-item>
    <el-form-item :label="$t('projectPostBuildExecCommand')" required>
      <el-input
        v-model="action.command"
        type="textarea"
        :rows="4"
        :placeholder="$t('projectPostBuildExecCommandPlaceholder')"
      />
    </el-form-item>
  </div>
</template>

<style scoped lang="scss" src="./post-build-fields.scss"></style>
