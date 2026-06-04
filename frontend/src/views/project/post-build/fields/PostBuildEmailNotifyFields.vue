<script setup lang="ts">
import { onMounted } from 'vue'
import type { PostBuildAction } from '@/types/post-build'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import type { PostBuildFieldsProps } from '../post-build-fields-props'
import { useSmtpEnvOptions } from '../use-smtp-env-options'

const props = defineProps<PostBuildFieldsProps>()

const action = defineModel<PostBuildAction>({ required: true })

const emit = defineEmits<{
  reloadEnvs: []
}>()

const { loading, options, hasOptions, load, optionLabel } = useSmtpEnvOptions(props.envOptions)

onMounted(() => {
  void load()
})

function onEnvSelectVisible(visible: boolean): void {
  if (!visible) return
  void load()
  emit('reloadEnvs')
}
</script>

<template>
  <div class="post-build-fields">
    <el-form-item :label="$t('projectPostBuildSmtpEnv')">
      <el-select
        v-model="action.env_rec_id"
        clearable
        filterable
        teleported
        :loading="loading"
        :popper-class="APP_DIALOG_POPPER_CLASS"
        class="post-build-fields__full"
        :placeholder="$t('projectPostBuildSmtpEnvPlaceholder')"
        @visible-change="onEnvSelectVisible"
      >
        <el-option
          v-for="e in options"
          :key="e.rec_id"
          :label="optionLabel(e)"
          :value="e.rec_id"
          :disabled="e.is_enabled !== 1"
        />
      </el-select>
      <p v-if="!hasOptions && !loading" class="post-build-fields__hint">
        {{ $t('projectPostBuildSmtpEnvHint') }}
      </p>
    </el-form-item>
    <el-form-item :label="$t('projectPostBuildRecipients')" required>
      <el-input
        v-model="action.recipients"
        :placeholder="$t('projectPostBuildRecipientsPlaceholder')"
      />
    </el-form-item>
    <el-form-item :label="$t('projectPostBuildSubject')">
      <el-input v-model="action.subject" :placeholder="$t('projectPostBuildSubjectPlaceholder')" />
    </el-form-item>
    <el-form-item :label="$t('projectPostBuildBody')">
      <el-input
        v-model="action.body"
        type="textarea"
        :rows="4"
        :placeholder="$t('projectPostBuildBodyPlaceholder')"
      />
    </el-form-item>
  </div>
</template>

<style scoped lang="scss" src="./post-build-fields.scss"></style>
