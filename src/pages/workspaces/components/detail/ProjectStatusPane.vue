<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RsButton, RsIcon, RsTag } from '@/ui'
import { formatStamp, statusVariant } from '../../composables/use-project-list'
import type { JobRow } from '../../api'

const props = defineProps<{
  scmMode: string
  checkoutPath: string
  branch: string
  repoUrl: string
  command: string
  cron: string
  latest: JobRow | null
}>()

const emit = defineEmits<{
  console: []
}>()

const { t } = useI18n()

function statusLabel(code?: string): string {
  if (!code) return t('workspaces.neverBuilt')
  const key = `jobs.statuses.${code}`
  const label = t(key)
  return label === key ? code : label
}

function scmLabel(mode: string): string {
  return mode === 'local' ? t('workspaces.scmLocal') : t('workspaces.scmGit')
}
</script>

<template>
  <section class="pane">
    <header class="pane-head">
      <h3>{{ t('workspaces.detail.nav.status') }}</h3>
      <p class="pane-desc">{{ t('workspaces.detail.statusDesc') }}</p>
    </header>

    <dl class="facts">
      <div class="fact">
        <dt class="fact__label">
          <RsIcon name="share-2" :size="14" />
          {{ t('workspaces.nav.scm') }}
        </dt>
        <dd class="fact__value">{{ scmLabel(props.scmMode) }}</dd>
      </div>
      <div class="fact">
        <dt class="fact__label">
          <RsIcon name="git-branch" :size="14" />
          {{ t('workspaces.branch') }}
        </dt>
        <dd class="fact__value">{{ props.branch || t('common.dash') }}</dd>
      </div>
      <div class="fact fact--wide">
        <dt class="fact__label">
          <RsIcon name="folder" :size="14" />
          {{ t('workspaces.saveDir') }}
        </dt>
        <dd class="fact__value"><code>{{ props.checkoutPath || t('common.dash') }}</code></dd>
      </div>
      <div class="fact fact--wide">
        <dt class="fact__label">
          <RsIcon name="link" :size="14" />
          {{ t('workspaces.repo') }}
        </dt>
        <dd class="fact__value mono">{{ props.repoUrl || t('common.dash') }}</dd>
      </div>
      <div class="fact">
        <dt class="fact__label">
          <RsIcon name="terminal" :size="14" />
          {{ t('workspaces.command') }}
        </dt>
        <dd class="fact__value"><code>{{ props.command || t('common.dash') }}</code></dd>
      </div>
      <div class="fact">
        <dt class="fact__label">
          <RsIcon name="history" :size="14" />
          {{ t('workspaces.cron') }}
        </dt>
        <dd class="fact__value">{{ props.cron || t('common.dash') }}</dd>
      </div>
    </dl>

    <div class="block">
      <div class="block-head">
        <h4>{{ t('workspaces.detail.lastBuild') }}</h4>
        <template v-if="props.latest">
          <RsTag :variant="statusVariant(props.latest.job_status)" round>{{ statusLabel(props.latest.job_status) }}</RsTag>
          <span class="pane-meta">{{ formatStamp(props.latest.finished_at || props.latest.started_at || props.latest.created_at) }}</span>
          <RsButton size="sm" variant="ghost" class="block-action" @click="emit('console')">
            {{ t('workspaces.detail.viewLog') }}
          </RsButton>
        </template>
      </div>
      <p v-if="!props.latest" class="quiet">{{ t('workspaces.detail.emptyBuild') }}</p>
      <p v-else-if="props.latest.error_text" class="quiet">{{ props.latest.error_text }}</p>
    </div>
  </section>
</template>

<style scoped src="./pane.css"></style>
<style scoped>
.block-action {
  margin-left: auto;
}
</style>
