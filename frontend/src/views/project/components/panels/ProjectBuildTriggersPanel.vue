<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewProjectCron } from '@/request/api/project'
import type { BuildTriggers } from '@/types/build-triggers'
import type { PackProject } from '@/types'

const props = defineProps<{
  editingId: string | null
  workspaceProjects: PackProject[]
}>()

const triggers = defineModel<BuildTriggers>('triggers', { required: true })

const { t } = useI18n()

const cronPreviewRuns = ref<string[]>([])
const cronPreviewTimezone = ref('')
const cronPreviewError = ref('')
const cronPreviewOk = ref(false)
const cronPreviewLoading = ref(false)

const cronInputId = 'project-build-cron-expr'

const dependencyOptions = computed(() =>
  props.workspaceProjects
    .filter((p) => p.rec_id !== props.editingId)
    .map((p) => ({
      value: p.rec_id,
      label: `${p.project_name} @ ${p.project_version}`,
    })),
)

async function runCronPreview(): Promise<void> {
  if (!triggers.value.schedule.enabled) {
    cronPreviewRuns.value = []
    cronPreviewTimezone.value = ''
    cronPreviewError.value = ''
    cronPreviewOk.value = false
    return
  }
  const cron = triggers.value.schedule.cron.trim()
  if (!cron) {
    cronPreviewRuns.value = []
    cronPreviewTimezone.value = ''
    cronPreviewError.value = t('projectBuildCronPreviewEmpty')
    cronPreviewOk.value = false
    return
  }

  cronPreviewLoading.value = true
  cronPreviewError.value = ''
  try {
    const res = await previewProjectCron({ cron, count: 3 })
    cronPreviewRuns.value = res.next_runs ?? []
    cronPreviewTimezone.value = res.timezone?.trim() ?? ''
    cronPreviewOk.value = cronPreviewRuns.value.length > 0
  } catch (e) {
    cronPreviewRuns.value = []
    cronPreviewTimezone.value = ''
    cronPreviewOk.value = false
    cronPreviewError.value = t('projectBuildCronPreviewInvalid', { msg: (e as Error).message })
  } finally {
    cronPreviewLoading.value = false
  }
}

let cronDebounceTimer: ReturnType<typeof setTimeout> | undefined

watch(
  () => [triggers.value.schedule.enabled, triggers.value.schedule.cron] as const,
  ([enabled]) => {
    if (!enabled) {
      cronPreviewRuns.value = []
      cronPreviewTimezone.value = ''
      cronPreviewError.value = ''
      cronPreviewOk.value = false
      return
    }
    clearTimeout(cronDebounceTimer)
    cronDebounceTimer = setTimeout(() => {
      void runCronPreview()
    }, 500)
  },
  { immediate: true },
)
</script>

<template>
  <div class="build-triggers-panel">
    <p class="build-triggers-panel__desc">{{ $t('projectBuildTriggersDesc') }}</p>

    <section class="build-triggers-panel__block">
      <h4 class="build-triggers-panel__block-title">{{ $t('projectBuildDependsOn') }}</h4>
      <p class="build-triggers-panel__hint">{{ $t('projectBuildDependsOnHint') }}</p>
      <el-empty
        v-if="dependencyOptions.length === 0"
        :description="$t('projectBuildDependsOnEmpty')"
        :image-size="64"
        class="build-triggers-panel__dep-empty"
      />
      <el-checkbox-group v-else v-model="triggers.depends_on" class="build-triggers-panel__dep-group">
        <ul class="build-triggers-panel__dep-list">
          <li
            v-for="opt in dependencyOptions"
            :key="opt.value"
            class="build-triggers-panel__dep-row"
          >
            <el-checkbox :value="opt.value" class="build-triggers-panel__dep-checkbox">
              {{ opt.label }}
            </el-checkbox>
          </li>
        </ul>
      </el-checkbox-group>
    </section>

    <section class="build-triggers-panel__block">
      <div class="build-triggers-panel__schedule-head">
        <h4 class="build-triggers-panel__block-title">{{ $t('projectBuildSchedule') }}</h4>
        <el-switch v-model="triggers.schedule.enabled" />
      </div>
      <p class="build-triggers-panel__hint">{{ $t('projectBuildScheduleHint') }}</p>
      <el-form-item v-show="triggers.schedule.enabled" :label="$t('projectBuildCron')" label-width="120px">
        <div class="build-triggers-panel__cron-field">
          <el-input
            :id="cronInputId"
            v-model="triggers.schedule.cron"
            :placeholder="$t('projectBuildCronPlaceholder')"
            @blur="runCronPreview"
          />
          <el-button
            size="small"
            :loading="cronPreviewLoading"
            @click="runCronPreview"
          >
            {{ $t('projectBuildCronTest') }}
          </el-button>
        </div>
        <p class="build-triggers-panel__hint">{{ $t('projectBuildCronHelp') }}</p>
        <output
          v-if="triggers.schedule.enabled && cronPreviewOk"
          :for="cronInputId"
          class="build-triggers-panel__cron-preview"
        >
          <p class="build-triggers-panel__cron-preview-title">
            {{
              cronPreviewTimezone
                ? $t('projectBuildCronPreviewTitleTz', { tz: cronPreviewTimezone })
                : $t('projectBuildCronPreviewTitle')
            }}
          </p>
          <ul class="build-triggers-panel__cron-preview-list">
            <li v-for="(run, i) in cronPreviewRuns" :key="i">
              {{ $t('projectBuildCronNextRun', { n: i + 1, time: run }) }}
            </li>
          </ul>
          <p class="build-triggers-panel__cron-preview-note">{{ $t('projectBuildCronPreviewNote') }}</p>
        </output>
        <p
          v-else-if="triggers.schedule.enabled && cronPreviewError"
          class="build-triggers-panel__cron-preview-error"
          role="alert"
        >
          {{ cronPreviewError }}
        </p>
      </el-form-item>
    </section>

    <el-alert type="info" :closable="false" show-icon class="build-triggers-panel__note">
      {{ $t('projectBuildManualBypassHint') }}
    </el-alert>
  </div>
</template>

<style scoped lang="scss" src="./ProjectBuildTriggersPanel.scss"></style>
