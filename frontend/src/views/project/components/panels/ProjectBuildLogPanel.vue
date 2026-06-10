<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, CopyDocument } from '@element-plus/icons-vue'
import { BuildLogViewer } from '@/components/log'
import { useMessage } from '@/components/feedback/message'
import { copyTextToClipboard } from '@/utils/clipboard'

import type { PackBuildStatus } from '@/types'

const props = defineProps<{
  buildNum?: number | null
  buildStatus?: PackBuildStatus | null
  content: string
  loading: boolean
  complete: boolean
}>()

const { t } = useI18n()
const message = useMessage()

const viewportRef = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)

const lineCount = computed(() => {
  if (!props.content) return 0
  return props.content.split('\n').length
})

const statusLabel = computed(() => {
  if (!props.complete) return t('projectBuildRunning')
  const st = props.buildStatus
  if (st) return t(`projectBuildStatus_${st}`)
  return t('projectBuildStatus_success')
})

const statusType = computed((): 'success' | 'warning' | 'danger' | 'info' => {
  if (!props.complete) return 'warning'
  switch (props.buildStatus) {
    case 'failed':
      return 'danger'
    case 'success':
      return 'success'
    case 'cancelled':
      return 'info'
    default:
      return 'success'
  }
})

function scrollToBottom(behavior: ScrollBehavior = 'auto'): void {
  const el = viewportRef.value
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior })
}

function onScroll(): void {
  const el = viewportRef.value
  if (!el) return
  const distance = el.scrollHeight - el.scrollTop - el.clientHeight
  stickToBottom.value = distance < 48
}

async function copyLog(): Promise<void> {
  if (!props.content) return
  const ok = await copyTextToClipboard(props.content)
  if (ok) message.success(t('projectLogCopied'))
  else message.error(t('projectLogCopyFailed'))
}

watch(
  () => props.content,
  () => {
    if (!stickToBottom.value) return
    requestAnimationFrame(() => scrollToBottom())
  },
)

watch(
  () => props.buildNum,
  () => {
    stickToBottom.value = true
    requestAnimationFrame(() => scrollToBottom())
  },
)

function onScrollToEndClick(): void {
  stickToBottom.value = true
  scrollToBottom('smooth')
}
</script>

<template>
  <section class="project-build-log-panel">
    <header class="project-build-log-panel__toolbar">
      <div class="project-build-log-panel__toolbar-left">
        <span class="project-build-log-panel__console-label">{{ $t('projectLogConsole') }}</span>
        <span v-if="buildNum != null" class="project-build-log-panel__build-badge">#{{ buildNum }}</span>
        <span v-if="lineCount > 0" class="project-build-log-panel__meta">
          {{ $t('projectLogLines', { count: lineCount }) }}
        </span>
      </div>
      <div class="project-build-log-panel__toolbar-right">
        <el-tag :type="statusType" size="small" effect="dark" class="project-build-log-panel__status">
          {{ statusLabel }}
        </el-tag>
        <el-button
          size="small"
          text
          :disabled="!content"
          :aria-label="$t('projectLogCopy')"
          @click="copyLog"
        >
          <el-icon><CopyDocument /></el-icon>
        </el-button>
      </div>
    </header>

    <div v-loading="loading && !content" class="project-build-log-panel__console">
      <div
        v-if="content"
        ref="viewportRef"
        class="project-build-log-panel__viewport"
        role="log"
        aria-live="polite"
        @scroll="onScroll"
      >
        <BuildLogViewer v-if="content" :content="content" />
      </div>
      <div v-else class="project-build-log-panel__empty">
        <el-empty :description="$t('projectLogEmpty')" :image-size="72" />
      </div>

      <Transition name="project-build-log-panel-fade">
        <button
          v-if="content && !stickToBottom"
          type="button"
          class="project-build-log-panel__scroll-end"
          @click="onScrollToEndClick"
        >
          <el-icon><ArrowDown /></el-icon>
          {{ $t('projectLogScrollEnd') }}
        </button>
      </Transition>
    </div>
  </section>
</template>

<style scoped lang="scss" src="./ProjectBuildLogPanel.scss"></style>
