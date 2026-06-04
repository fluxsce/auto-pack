<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatFileSize } from './format'

const props = withDefaults(
  defineProps<{
    path: string
    content?: string
    size?: number | null
    truncated?: boolean
    readable?: boolean
    loading?: boolean
    showHead?: boolean
  }>(),
  {
    content: '',
    size: null,
    truncated: false,
    readable: true,
    loading: false,
    showHead: true,
  },
)

const { t } = useI18n()

const sizeLabel = computed(() => {
  if (props.size == null) return ''
  return formatFileSize(props.size)
})
</script>

<template>
  <div v-loading="loading" class="file-text-viewer">
    <header v-if="showHead && path" class="file-text-viewer__head">
      <span class="file-text-viewer__path" :title="path">{{ path }}</span>
      <span v-if="sizeLabel" class="file-text-viewer__meta">{{ $t('fileViewerSize', { size: sizeLabel }) }}</span>
    </header>

    <el-alert
      v-if="truncated"
      type="warning"
      :closable="false"
      show-icon
      class="file-text-viewer__alert"
      :title="$t('fileViewerTruncated')"
    />

    <div v-if="!readable" class="file-text-viewer__placeholder">
      <p>{{ $t('fileViewerBinaryHint') }}</p>
    </div>
    <div v-else-if="content" class="file-text-viewer__body">
      <pre class="file-text-viewer__pre">{{ content }}</pre>
    </div>
    <div v-else-if="!loading" class="file-text-viewer__placeholder">
      <p>{{ $t('fileViewerEmpty') }}</p>
    </div>
  </div>
</template>

<style scoped lang="scss" src="./FileTextViewer.scss"></style>
