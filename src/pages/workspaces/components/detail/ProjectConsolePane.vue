<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RsLog, RsTag } from '@/ui'
import { formatStamp, statusVariant } from '../../composables/use-project-list'
import type { JobRow, JobStep } from '../../api'

const props = defineProps<{
  job: JobRow | null
  steps: JobStep[]
  log: string
  live: boolean
}>()

const { t } = useI18n()

function statusLabel(code?: string): string {
  if (!code) return t('workspaces.neverBuilt')
  const key = `jobs.statuses.${code}`
  const label = t(key)
  return label === key ? code : label
}
</script>

<template>
  <section class="pane">
    <header class="pane-head">
      <h3>{{ t('workspaces.detail.nav.console') }}</h3>
      <template v-if="props.job">
        <RsTag :variant="statusVariant(props.job.job_status)" round>{{ statusLabel(props.job.job_status) }}</RsTag>
        <RsTag v-if="props.live" variant="info" round>{{ t('workspaces.detail.live') }}</RsTag>
        <span class="pane-meta">{{ formatStamp(props.job.started_at || props.job.created_at) }}</span>
      </template>
    </header>
    <RsLog
      class="console-log"
      :class="{ 'is-live': props.live }"
      :lines="props.log"
      height="100%"
      :follow="props.live"
      :empty-text="t('workspaces.detail.emptyLog')"
      :aria-label="t('workspaces.detail.nav.console')"
    />
  </section>
</template>

<style scoped src="./pane.css"></style>
<style scoped>
.console-log {
  flex: 1;
  min-height: 16rem;
  height: 0;
}

.console-log.is-live {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--rs-info) 35%, transparent);
}
</style>
