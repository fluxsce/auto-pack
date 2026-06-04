<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, CopyDocument } from '@element-plus/icons-vue'
import { BuildLogViewer } from '@/components/log'
import { useMessage } from '@/components/feedback/message'
import type { WorkspaceReleaseStatus } from '../../release-status'

const props = defineProps<{
  releaseNum?: number | null
  releaseStatus?: WorkspaceReleaseStatus | string | null
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
  if (!props.complete) return t('projectAutoReleaseRunningTitle')
  const st = props.releaseStatus
  if (st) return t(`projectReleaseStatus_${st}`)
  return t('projectReleaseStatus_success')
})

const statusType = computed((): 'success' | 'warning' | 'danger' | 'info' => {
  if (!props.complete) return 'warning'
  switch (props.releaseStatus) {
    case 'failed':
      return 'danger'
    case 'success':
      return 'success'
    case 'partial':
      return 'warning'
    case 'running':
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
  try {
    await navigator.clipboard.writeText(props.content)
    message.success(t('projectLogCopied'))
  } catch {
    message.error(t('projectLogCopyFailed'))
  }
}

watch(
  () => props.content,
  () => {
    if (!stickToBottom.value) return
    requestAnimationFrame(() => scrollToBottom())
  },
)

watch(
  () => props.releaseNum,
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
  <section class="project-build-log-panel project-release-log-panel">
    <header class="project-build-log-panel__toolbar">
      <div class="project-build-log-panel__toolbar-left">
        <span class="project-build-log-panel__console-label">{{ $t('projectAutoReleaseLogTitle') }}</span>
        <span v-if="releaseNum != null" class="project-build-log-panel__build-badge">#{{ releaseNum }}</span>
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
        <BuildLogViewer :content="content" variant="release" />
      </div>
      <div v-else class="project-build-log-panel__empty">
        <el-empty :description="$t('projectAutoReleaseLogEmpty')" :image-size="72" />
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
