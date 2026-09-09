<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsFieldset, RsFormItem, RsInput, RsSelect, RsTextarea } from '@/ui'
import type { ProjectFileEntry } from '../../api'
import {
  backupFilePath,
  buildManifestPreviewPairs,
  mergeUpdateNotePreview,
  resolvePushLayout,
  uploadFileName,
} from '../../lib/workspace-ftp'

const form = defineModel<{
  envRecId: string
  pushPath: string
  backupPath: string
  pusher: string
  taskRef: string
  detail: string
}>('form', { required: true })

const props = withDefaults(
  defineProps<{
    entries: ProjectFileEntry[]
    envOptions: { value: string; label: string }[]
    disabled?: boolean
    remoteNote?: string
    remoteNoteState?: 'idle' | 'loading' | 'exists' | 'missing' | 'error'
    /** 为 false 时只渲染推送配置（自动推送卡片已展示工程与相对路径） */
    showTargets?: boolean
  }>(),
  { showTargets: true },
)

const emit = defineEmits<{
  'backup-edit': []
  'env-change': [id: string]
}>()

const { t } = useI18n()

const previewBlock = computed(() => {
  const pushPath = form.value.pushPath.trim()
  const names = props.entries.map((item) => uploadFileName(item.name, item.is_dir))
  const pairs = pushPath
    ? names.map((name) => ({
        backupPath: backupFilePath(pushPath, name, form.value.backupPath),
        updatePath: resolvePushLayout(pushPath, name).updatePath,
      }))
    : []
  return buildManifestPreviewPairs(new Date(), form.value.pusher, form.value.taskRef, pairs, form.value.detail)
})

const preview = computed(() => mergeUpdateNotePreview(props.remoteNote ?? '', previewBlock.value))

const previewHint = computed(() => {
  switch (props.remoteNoteState) {
    case 'loading':
      return t('workspaces.remote.previewLoading')
    case 'exists':
      return t('workspaces.remote.previewExists')
    case 'missing':
      return t('workspaces.remote.previewMissing')
    case 'error':
      return t('workspaces.remote.previewError')
    default:
      return t('workspaces.remote.previewHint')
  }
})

function entryLabel(row: ProjectFileEntry): string {
  return row.is_dir ? `${row.name}/` : row.name
}
</script>

<template>
  <RsFieldset
    v-if="showTargets"
    :legend="t('workspaces.remote.targets')"
    :tooltip="t('workspaces.remote.targetsHint', { n: entries.length })"
    border-style="dashed"
    title-tone="muted"
    size="sm"
  >
    <ul class="file-list" :aria-label="t('workspaces.remote.targets')">
      <li v-for="item in entries" :key="item.path" class="row">{{ entryLabel(item) }}</li>
    </ul>
  </RsFieldset>

  <RsFieldset
    :legend="t('workspaces.remote.sectionPush')"
    :tooltip="t('workspaces.remote.sectionPushDesc')"
    :disabled="disabled"
    border-style="dashed"
    title-tone="muted"
    size="sm"
  >
    <RsFormItem :label="t('workspaces.remote.env')" :tooltip="t('workspaces.remote.envTip')" required>
      <RsSelect
        v-model="form.envRecId"
        :options="envOptions"
        :disabled="disabled"
        :placeholder="t('workspaces.remote.envPh')"
        @update:model-value="emit('env-change', String($event || ''))"
      />
    </RsFormItem>
    <RsFormItem :label="t('workspaces.remote.path')" :tooltip="t('workspaces.remote.pathTip')" required>
      <RsInput
        v-model="form.pushPath"
        :disabled="disabled"
        :placeholder="t('workspaces.remote.pathPh')"
      />
    </RsFormItem>
    <RsFormItem :label="t('workspaces.remote.backup')" :tooltip="t('workspaces.remote.backupTip')">
      <RsInput
        v-model="form.backupPath"
        :disabled="disabled"
        :placeholder="t('workspaces.remote.backupPh')"
        @update:model-value="emit('backup-edit')"
      />
    </RsFormItem>
    <div class="pair">
      <RsFormItem :label="t('workspaces.remote.pusher')" :tooltip="t('workspaces.remote.pusherTip')" required>
        <RsInput
          v-model="form.pusher"
          :disabled="disabled"
          :placeholder="t('workspaces.remote.pusherPh')"
        />
      </RsFormItem>
      <RsFormItem :label="t('workspaces.remote.taskRef')" :tooltip="t('workspaces.remote.taskRefTip')" required>
        <RsInput
          v-model="form.taskRef"
          :disabled="disabled"
          :placeholder="t('workspaces.remote.taskRefPh')"
        />
      </RsFormItem>
    </div>
    <RsFormItem :label="t('workspaces.remote.detail')" :tooltip="t('workspaces.remote.detailTip')">
      <RsTextarea
        v-model="form.detail"
        :disabled="disabled"
        :rows="2"
        :placeholder="t('workspaces.remote.detailPh')"
      />
    </RsFormItem>
    <RsFormItem :label="t('workspaces.remote.preview')" :tooltip="t('workspaces.remote.previewTip')">
      <pre class="preview-note">{{ preview }}</pre>
    </RsFormItem>
    <p class="hint">{{ previewHint }}</p>
  </RsFieldset>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.preview-note {
  box-sizing: border-box;
  width: 100%;
  min-height: 7.5rem;
  max-height: 16rem;
  margin: 0;
  padding: 0.55rem 0.7rem;
  overflow: auto;
  border: 1px solid var(--shell-border);
  border-radius: 0.45rem;
  background: var(--shell-inset-bg);
  color: var(--shell-title);
  font-family: var(--rs-font-mono);
  font-size: 0.75rem;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
