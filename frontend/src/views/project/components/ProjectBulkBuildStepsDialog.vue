<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProjectType } from '@/types'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { useMessage } from '@/components/feedback/message'
import { batchUpdateBuildSteps } from '@/request/api/project'
import {
  DEFAULT_BUILD_CONFIG,
  DEFAULT_LOG_RETENTION_DAYS,
} from '../project-defaults'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  projectCount: number
  projectTypes: ProjectType[]
  typeLabel: (type: unknown) => string
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

const projectTypeFilter = ref('')
const updateBuildCmd = ref(false)
const updateOutputDir = ref(false)
const updateArtifactPattern = ref(false)
const updateLogRetention = ref(false)
const buildCmd = ref('')
const outputDir = ref('')
const artifactPattern = ref('')
const logRetentionDays = ref(DEFAULT_LOG_RETENTION_DAYS)
const saving = ref(false)

const filteredHintCount = computed(() => {
  if (!projectTypeFilter.value?.trim()) return props.projectCount
  return null
})

function applyTypeDefaults(type: ProjectType): void {
  const d = DEFAULT_BUILD_CONFIG[type]
  buildCmd.value = d.build_cmd
  outputDir.value = d.output_dir
  artifactPattern.value = d.artifact_pattern
  logRetentionDays.value = d.log_retention_days
}

function resetForm(): void {
  projectTypeFilter.value = ''
  updateBuildCmd.value = false
  updateOutputDir.value = false
  updateArtifactPattern.value = false
  updateLogRetention.value = false
  applyTypeDefaults('maven')
}

watch(open, (v) => {
  if (v) resetForm()
})

watch(projectTypeFilter, (type) => {
  if (type && props.projectTypes.includes(type as ProjectType)) {
    applyTypeDefaults(type as ProjectType)
  }
})

async function onSubmit(): Promise<void> {
  if (
    !updateBuildCmd.value &&
    !updateOutputDir.value &&
    !updateArtifactPattern.value &&
    !updateLogRetention.value
  ) {
    message.warning(t('projectBulkBuildStepsFieldRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  saving.value = true
  try {
    const res = await batchUpdateBuildSteps({
      workspace_rec_id: props.workspaceRecId,
      project_type: projectTypeFilter.value?.trim() ?? '',
      update_build_cmd: updateBuildCmd.value,
      build_cmd: buildCmd.value,
      update_output_dir: updateOutputDir.value,
      output_dir: outputDir.value,
      update_artifact_pattern: updateArtifactPattern.value,
      artifact_pattern: artifactPattern.value,
      update_log_retention: updateLogRetention.value,
      log_retention_days: logRetentionDays.value,
    })
    message.success(t('projectBulkBuildStepsSuccess', { count: res.updated }))
    open.value = false
    emit('success')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkBuildStepsTitle')"
    width="560px"
    initial-height="auto"
    :resizable="false"
    :show-footer="true"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <p class="project-bulk-build-steps__hint">
      {{
        filteredHintCount != null
          ? $t('projectBulkBuildStepsHint', { name: workspaceName, count: filteredHintCount })
          : $t('projectBulkBuildStepsHintTyped', {
              name: workspaceName,
              type: typeLabel(projectTypeFilter),
            })
      }}
    </p>
    <el-form label-width="132px" class="project-bulk-build-steps__form" @submit.prevent="onSubmit">
      <el-form-item :label="$t('projectColType')">
        <el-select
          v-model="projectTypeFilter"
          clearable
          teleported
          :popper-class="APP_DIALOG_POPPER_CLASS"
          :placeholder="$t('projectTabAll')"
          class="project-bulk-build-steps__full"
        >
          <el-option
            v-for="tp in projectTypes"
            :key="tp"
            :label="typeLabel(tp)"
            :value="tp"
          />
        </el-select>
      </el-form-item>

      <el-form-item :label="$t('projectColBuildCmd')">
        <div class="project-bulk-build-steps__field">
          <el-checkbox v-model="updateBuildCmd" class="project-bulk-build-steps__apply">
            {{ $t('projectBulkBuildStepsApplyField') }}
          </el-checkbox>
          <el-input
            v-model="buildCmd"
            type="textarea"
            :rows="3"
            :disabled="!updateBuildCmd"
          />
        </div>
      </el-form-item>

      <el-form-item :label="$t('projectColOutputDir')">
        <div class="project-bulk-build-steps__field">
          <el-checkbox v-model="updateOutputDir" class="project-bulk-build-steps__apply">
            {{ $t('projectBulkBuildStepsApplyField') }}
          </el-checkbox>
          <el-input v-model="outputDir" :disabled="!updateOutputDir" />
        </div>
      </el-form-item>

      <el-form-item :label="$t('projectColArtifact')">
        <div class="project-bulk-build-steps__field">
          <el-checkbox v-model="updateArtifactPattern" class="project-bulk-build-steps__apply">
            {{ $t('projectBulkBuildStepsApplyField') }}
          </el-checkbox>
          <el-input v-model="artifactPattern" :disabled="!updateArtifactPattern" />
        </div>
      </el-form-item>

      <el-form-item :label="$t('projectColLogRetention')">
        <div class="project-bulk-build-steps__field">
          <el-checkbox v-model="updateLogRetention" class="project-bulk-build-steps__apply">
            {{ $t('projectBulkBuildStepsApplyField') }}
          </el-checkbox>
          <el-input-number
            v-model="logRetentionDays"
            :min="1"
            :max="365"
            :step="1"
            :disabled="!updateLogRetention"
          />
          <p class="project-bulk-build-steps__field-hint">{{ $t('projectLogRetentionHint') }}</p>
        </div>
      </el-form-item>

      <p class="project-bulk-build-steps__note">{{ $t('projectBulkBuildStepsNote') }}</p>
    </el-form>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-build-steps {
  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__field {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }

  &__apply {
    align-self: flex-start;
  }

  &__full {
    width: 100%;
  }

  &__field-hint {
    margin: 6px 0 0;
    width: 100%;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__note {
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }
}
</style>
