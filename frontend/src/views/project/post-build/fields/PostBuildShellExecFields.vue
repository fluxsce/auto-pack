<script setup lang="ts">
import type { PostBuildAction } from '@/types/post-build'

const action = defineModel<PostBuildAction>({ required: true })
</script>

<template>
  <div class="post-build-fields">
    <el-form-item :label="$t('projectPostBuildExecCommand')" required>
      <el-input
        v-model="action.command"
        type="textarea"
        :rows="4"
        :placeholder="$t('projectPostBuildShellCommandPlaceholder')"
      />
      <p class="post-build-fields__hint">{{ $t('projectPostBuildShellWorkDirHint') }}</p>
    </el-form-item>
  </div>
</template>

<style scoped lang="scss" src="./post-build-fields.scss"></style>
