<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CopyDocument, Download } from '@element-plus/icons-vue'
import { AppDialog } from '@/components/feedback/dialog'
import { useMessage } from '@/components/feedback/message'
import { copyTextToClipboard } from '@/utils/clipboard'
import { basenameFromPath, formatFileSize } from './format'
import FileTextViewer from './FileTextViewer.vue'

const open = defineModel<boolean>({ default: false })

const props = withDefaults(
  defineProps<{
    path?: string
    content?: string
    size?: number | null
    truncated?: boolean
    readable?: boolean
    loading?: boolean
    downloadLoading?: boolean
  }>(),
  {
    path: '',
    content: '',
    size: null,
    truncated: false,
    readable: true,
    loading: false,
    downloadLoading: false,
  },
)

const emit = defineEmits<{
  download: []
}>()

const { t } = useI18n()
const message = useMessage()

const title = computed(() => {
  const name = basenameFromPath(props.path)
  return name ? `${t('fileViewerTitle')}: ${name}` : t('fileViewerTitle')
})

const canCopy = computed(() => props.readable && !!props.content)

async function copyContent(): Promise<void> {
  if (!props.content) return
  const ok = await copyTextToClipboard(props.content)
  if (ok) message.success(t('fileViewerCopied'))
  else message.error(t('fileViewerCopyFailed'))
}

function onDownload(): void {
  emit('download')
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="title"
    width="860px"
    initial-height="72vh"
    :show-footer="false"
    modal
    destroy-on-close
  >
    <div class="file-content-dialog">
      <div class="file-content-dialog__toolbar">
        <div class="file-content-dialog__meta">
          <span v-if="path" class="file-content-dialog__path" :title="path">{{ path }}</span>
          <span v-if="size != null" class="file-content-dialog__size">
            {{ $t('fileViewerSize', { size: formatFileSize(size) }) }}
          </span>
        </div>
        <div class="file-content-dialog__actions">
          <el-tooltip v-if="canCopy" :content="$t('fileViewerCopy')" placement="bottom">
            <el-button size="small" text :aria-label="$t('fileViewerCopy')" @click="copyContent">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </el-tooltip>
          <el-button size="small" type="primary" :loading="downloadLoading" @click="onDownload">
            <el-icon><Download /></el-icon>
            {{ $t('fileDownload') }}
          </el-button>
        </div>
      </div>
      <FileTextViewer
        :path="path"
        :content="content"
        :size="size"
        :truncated="truncated"
        :readable="readable"
        :loading="loading"
        :show-head="false"
      />
    </div>
  </AppDialog>
</template>

<style scoped lang="scss" src="./FileContentDialog.scss"></style>

<style lang="scss">
.app-dialog--windowed.app-dialog--initial-height .app-dialog__content:has(.file-content-dialog) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-dialog.is-fullscreen .app-dialog__content:has(.file-content-dialog),
.app-dialog--fullscreen .app-dialog__content:has(.file-content-dialog) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  max-width: none;
  width: 100%;
  margin: 0;
}

.app-dialog.is-fullscreen .file-content-dialog,
.app-dialog--fullscreen .file-content-dialog {
  min-height: 0;
  height: 100%;
}
</style>
