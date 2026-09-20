<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsAlert,
  RsButton,
  RsCodeBlock,
  RsDialog,
  RsEmpty,
  RsIcon,
  RsInput,
  RsTable,
  rsConfirm,
  useRsToast,
  type RsContextMenuItem,
  type RsTableColumn,
} from '@/ui'
import {
  clearProjectFiles,
  deleteProjectFile,
  downloadProjectFile,
  errText,
  listProjectFiles,
  readProjectFile,
  type ProjectFileBlob,
  type ProjectFileEntry,
  type ProjectTree,
} from '../../api'
import { formatStamp } from '../../composables/use-project-list'
import { fileBaseName, fileExt, formatBytes, keepDirsFirst, matchProjectFile, previewLang } from '../../lib/project-files'
import type { ProjectRecipe } from '../../lib/project-defaults'
import RemoteDialog from '../remote/Dialog.vue'

const props = defineProps<{
  projectId: string
  workspaceId: string
  projectName?: string
  projectType?: string
  checkoutPath: string
  recipe?: ProjectRecipe
}>()
const { t } = useI18n()
const toast = useRsToast()
const tree = ref<ProjectTree | null>(null)
const dir = ref('')
const preview = ref<{
  name: string
  path: string
  blob: ProjectFileBlob | null
  binary: boolean
} | null>(null)
const loading = ref(false)
const clearing = ref(false)
const downloading = ref(false)
const busyKey = ref('')
const selectedKeys = ref<string[]>([])
const pushOpen = ref(false)
const query = ref('')

/** 列宽合计 + 选择列 40px；与 scroll-x 配合使 fixed 左/右列 sticky（见 niuma-ui playground TablePage）。 */
const TABLE_SCROLL_X = 786

const rows = computed(() => tree.value?.entries ?? [])
const filtered = computed(() => {
  const q = query.value
  if (!q.trim()) return rows.value
  return rows.value.filter((row) => matchProjectFile(row, q, kindLabel(row)))
})
const selectedRows = computed(() => {
  const keys = new Set(selectedKeys.value)
  return rows.value.filter((row) => keys.has(row.path))
})

const crumbs = computed(() => {
  const parts = dir.value.split('/').filter(Boolean)
  const items = [{ label: t('workspaces.detail.root'), path: '' }]
  let acc = ''
  for (const part of parts) {
    acc = acc ? `${acc}/${part}` : part
    items.push({ label: part, path: acc })
  }
  return items
})

const columns = computed<RsTableColumn<ProjectFileEntry>[]>(() => [
  {
    key: 'name',
    title: t('workspaces.detail.fileName'),
    minWidth: 180,
    ellipsis: true,
    sortable: true,
    filterable: true,
    sorter: keepDirsFirst,
    fixed: 'left',
  },
  {
    key: 'kind',
    title: t('workspaces.detail.kind'),
    width: 120,
    sortable: true,
    filterable: true,
    filter: (_value, row, q) => kindLabel(row).toLowerCase().includes(q.trim().toLowerCase()),
    sorter: (left, right) => keepDirsFirst(left, right) || kindLabel(left).localeCompare(kindLabel(right)),
  },
  {
    key: 'size',
    title: t('workspaces.detail.size'),
    width: 110,
    align: 'right',
    sortable: true,
    sorter: (left, right) => keepDirsFirst(left, right) || (left.size ?? 0) - (right.size ?? 0),
  },
  {
    key: 'mod_time',
    title: t('workspaces.detail.modified'),
    minWidth: 168,
    sortable: true,
    sorter: (left, right) => keepDirsFirst(left, right) || (left.mod_time || '').localeCompare(right.mod_time || ''),
  },
  {
    key: 'actions',
    title: t('common.actions'),
    width: 168,
    minWidth: 168,
    align: 'right',
    ellipsis: false,
    fixed: 'right',
  },
])

const previewTitle = computed(() => preview.value?.name || t('workspaces.detail.preview'))

/** 文件名仅在发生省略时设置 title，供悬浮查看完整名称。 */
function bindNameTitle(event: MouseEvent): void {
  const el = event.currentTarget as HTMLElement
  const text = el.textContent?.trim() ?? ''
  el.title = el.scrollWidth > el.clientWidth ? text : ''
}

function kindLabel(row: ProjectFileEntry): string {
  if (row.is_dir) return t('workspaces.detail.folder')
  const ext = fileExt(row.name)
  return ext ? t('workspaces.detail.fileExt', { ext: ext.toUpperCase() }) : t('workspaces.detail.file')
}

function menuItems(row: ProjectFileEntry | null, selected: ProjectFileEntry[] = []): RsContextMenuItem[] {
  const targets = menuTargets(row, selected)
  const target = targets[0] ?? row
  if (!target) return []
  const downloadLabel = targets.length > 1
    ? t('workspaces.detail.downloadSelected', { n: targets.length })
    : t('common.download')
  return [
    { key: 'open', label: target.is_dir ? t('workspaces.detail.openEntry') : t('workspaces.detail.preview'), icon: target.is_dir ? 'folder' : 'file' },
    { key: 'download', label: downloadLabel, icon: 'download' },
    { key: 'push', label: t('workspaces.detail.pushRemote'), icon: 'upload' },
    { key: 'delete', label: t('common.delete'), icon: 'trash-2', danger: true },
  ]
}

async function load(path = dir.value): Promise<void> {
  if (!props.projectId) return
  loading.value = true
  preview.value = null
  selectedKeys.value = []
  try {
    tree.value = await listProjectFiles(props.projectId, path)
    dir.value = tree.value.path || path
  } catch (e) {
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

async function open(entry: ProjectFileEntry): Promise<void> {
  if (entry.is_dir) {
    await load(entry.path)
    return
  }
  try {
    const blob = await readProjectFile(props.projectId, entry.path)
    preview.value = { name: entry.name, path: entry.path, blob, binary: false }
  } catch (e) {
    const msg = errText(e)
    if (msg.includes('not a text file')) {
      preview.value = { name: entry.name, path: entry.path, blob: null, binary: true }
      return
    }
    toast.error(msg)
  }
}

function downloadZipName(): string {
  const folder = fileBaseName(dir.value)
  const base = folder || props.projectName?.trim() || 'files'
  return `${base}-files.zip`
}

async function download(entry: ProjectFileEntry): Promise<void> {
  const name = entry.is_dir ? `${entry.name}.zip` : entry.name
  busyKey.value = entry.path
  try {
    await downloadProjectFile(props.projectId, entry.path, name)
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busyKey.value = ''
  }
}

async function downloadMany(entries: ProjectFileEntry[]): Promise<void> {
  if (!entries.length) {
    toast.warning(t('workspaces.detail.downloadNeedSelect'))
    return
  }
  if (entries.length === 1) {
    await download(entries[0])
    return
  }
  downloading.value = true
  try {
    await downloadProjectFile(
      props.projectId,
      entries.map((item) => item.path),
      downloadZipName(),
    )
  } catch (e) {
    toast.error(errText(e))
  } finally {
    downloading.value = false
  }
}

async function remove(entry: ProjectFileEntry): Promise<void> {
  const ok = await rsConfirm.danger({
    title: t('common.delete'),
    description: t('workspaces.detail.deleteEntryConfirm', { name: entry.name }),
    confirmText: t('common.delete'),
  })
  if (!ok) return
  busyKey.value = entry.path
  try {
    await deleteProjectFile(props.projectId, entry.path)
    if (preview.value?.path === entry.path || preview.value?.path.startsWith(`${entry.path}/`)) {
      preview.value = null
    }
    selectedKeys.value = selectedKeys.value.filter((key) => key !== entry.path && !key.startsWith(`${entry.path}/`))
    toast.success(t('workspaces.detail.deleteEntryDone', { name: entry.name }))
    await load(dir.value)
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busyKey.value = ''
  }
}

async function clearCheckout(): Promise<void> {
  const ok = await rsConfirm.danger({
    title: t('workspaces.detail.clearDir'),
    description: t('workspaces.detail.clearDirConfirm', { path: props.checkoutPath || t('workspaces.detail.root') }),
    confirmText: t('workspaces.detail.clearDir'),
  })
  if (!ok) return
  clearing.value = true
  try {
    await clearProjectFiles(props.projectId)
    toast.success(t('workspaces.detail.clearDirDone'))
    dir.value = ''
    await load('')
  } catch (e) {
    toast.error(errText(e))
  } finally {
    clearing.value = false
  }
}

function openPush(entries = selectedRows.value): void {
  if (!entries.length) {
    toast.warning(t('workspaces.detail.pushNeedSelect'))
    return
  }
  selectedKeys.value = entries.map((item) => item.path)
  pushOpen.value = true
}

function menuTargets(row: ProjectFileEntry | null, selected: ProjectFileEntry[]): ProjectFileEntry[] {
  if (selected.length) return selected
  return row ? [row] : []
}

function onMenu(key: string, row: ProjectFileEntry | null, selected: ProjectFileEntry[]): void {
  const target = row ?? selected[0]
  if (key === 'open' && target) {
    void open(target)
    return
  }
  if (key === 'download') {
    void downloadMany(menuTargets(row, selected))
    return
  }
  if (key === 'delete' && target) {
    void remove(target)
    return
  }
  if (key === 'push') {
    openPush(menuTargets(row, selected))
  }
}

function onPreviewOpen(next: boolean): void {
  if (!next) preview.value = null
}

async function downloadPreview(): Promise<void> {
  if (!preview.value) return
  await download({
    name: preview.value.name,
    path: preview.value.path,
    is_dir: false,
  })
}

watch(() => props.projectId, () => {
  dir.value = ''
  query.value = ''
  void load('')
}, { immediate: true })
</script>

<template>
  <section class="pane">
    <header class="pane-head">
      <div class="pane-bar">
        <h3>{{ t('workspaces.detail.nav.files') }}</h3>
        <span v-if="selectedRows.length" class="pane-meta">{{ t('workspaces.detail.selectedCount', { n: selectedRows.length }) }}</span>
        <div class="pane-bar__ops">
          <RsButton
            size="sm"
            variant="default"
            :disabled="!selectedRows.length || downloading"
            :loading="downloading"
            @click="downloadMany(selectedRows)"
          >
            {{ selectedRows.length > 1 ? t('workspaces.detail.downloadSelected', { n: selectedRows.length }) : t('common.download') }}
          </RsButton>
          <RsButton size="sm" variant="default" tone="warning" :disabled="!selectedRows.length" @click="openPush()">
            {{ t('workspaces.detail.pushRemote') }}
          </RsButton>
          <RsButton size="sm" variant="default" tone="primary" :loading="loading" @click="load()">{{ t('common.refresh') }}</RsButton>
          <RsButton
            size="sm"
            variant="default"
            tone="danger"
            :loading="clearing"
            :disabled="!tree?.exists || loading"
            @click="clearCheckout"
          >
            {{ t('workspaces.detail.clearDir') }}
          </RsButton>
        </div>
      </div>
      <p class="pane-desc mono" :title="props.checkoutPath">{{ props.checkoutPath || t('common.dash') }}</p>
    </header>
    <nav class="crumbs" :aria-label="t('workspaces.detail.nav.files')">
      <div class="crumbs__path">
        <button
          v-for="(item, index) in crumbs"
          :key="item.path || 'root'"
          type="button"
          class="crumbs__item"
          @click="load(item.path)"
        >
          <span v-if="index">/</span>
          {{ item.label }}
        </button>
      </div>
      <RsInput
        v-model="query"
        class="crumbs__filter"
        size="sm"
        clearable
        :placeholder="t('workspaces.detail.searchFilesPlaceholder')"
        :aria-label="t('workspaces.detail.searchFiles')"
      >
        <template #prefix>
          <RsIcon name="search" />
        </template>
      </RsInput>
    </nav>
    <div class="explorer">
      <div v-if="rows.length || loading" class="grow">
        <RsTable
          fill
          selectable
          cell-focus="none"
          :cell-tooltip="true"
          :scroll-x="TABLE_SCROLL_X"
          :columns="columns"
          :data="filtered"
          :loading="loading"
          row-key="path"
          :view-key="dir"
          :selected-row-keys="selectedKeys"
          :context-menu-items="menuItems"
          @update:selected-row-keys="selectedKeys = $event"
          @row-dblclick="open"
          @context-menu-select="onMenu"
        >
          <template #empty>
            {{ query.trim() ? t('workspaces.detail.emptyFileSearch') : t('workspaces.detail.emptyDir') }}
          </template>
          <template #name="{ row }">
            <button type="button" class="name-link" @click="open(row)">
              <RsIcon class="name-link__icon" :name="row.is_dir ? 'folder' : 'file'" :size="16" />
              <span class="name-link__text" @mouseenter="bindNameTitle">{{ row.name }}</span>
            </button>
          </template>
          <template #kind="{ row }">{{ kindLabel(row) }}</template>
          <template #size="{ row }">{{ row.is_dir ? t('common.dash') : formatBytes(row.size) }}</template>
          <template #mod_time="{ row }">{{ formatStamp(row.mod_time, t('common.dash')) }}</template>
          <template #actions="{ row }">
            <div class="row-ops">
              <RsButton
                size="sm"
                variant="text"
                tone="primary"
                :loading="busyKey === row.path"
                :disabled="downloading"
                @click.stop="download(row)"
              >
                {{ t('common.download') }}
              </RsButton>
              <RsButton
                size="sm"
                variant="text"
                tone="danger"
                :disabled="busyKey === row.path"
                @click.stop="remove(row)"
              >
                {{ t('common.delete') }}
              </RsButton>
            </div>
          </template>
        </RsTable>
      </div>
      <RsEmpty
        v-else
        :description="tree?.exists ? t('workspaces.detail.emptyDir') : t('workspaces.detail.noCheckout')"
      />
    </div>
    <RsDialog
      class="project-files-preview"
      :open="!!preview"
      layout="window"
      width="64rem"
      height="75%"
      draggable
      resizable
      fullscreenable
      teleport-to="body"
      :title="previewTitle"
      :style="{
        '--rs-dialog-body-padding-y': '0px',
        '--rs-dialog-body-padding-x': '0px',
      }"
      @update:open="onPreviewOpen"
    >
      <template #body>
        <div v-if="preview" class="preview-body" :class="{ 'preview-body--empty': preview.binary }">
          <RsAlert v-if="preview.blob?.truncated" type="warning" class="preview-alert">{{ t('workspaces.detail.previewTruncated') }}</RsAlert>
          <RsEmpty v-if="preview.binary" :description="t('workspaces.detail.binaryFile')">
            <RsButton size="sm" variant="default" tone="primary" :loading="busyKey === preview.path" @click="downloadPreview">
              {{ t('common.download') }}
            </RsButton>
          </RsEmpty>
          <RsCodeBlock
            v-else-if="preview.blob"
            class="preview-code"
            :code="preview.blob.content"
            :lang="previewLang(preview.name || fileBaseName(preview.path))"
            :download-filename="preview.name"
          />
        </div>
      </template>
    </RsDialog>
    <RemoteDialog
      v-model:open="pushOpen"
      :workspace-id="props.workspaceId"
      :project-id="props.projectId"
      :project-name="props.projectName"
      :project-type="props.projectType"
      :recipe="props.recipe"
      :entries="selectedRows"
    />
  </section>
</template>

<style scoped src="./pane.css"></style>
<style scoped>
.pane {
  min-width: 0;
}

.pane-bar {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  min-width: 0;
}

.pane-bar__ops {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-left: auto;
}

.crumbs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem 0.65rem;
  padding: 0.45rem 0.7rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.65rem;
  background: var(--shell-inset-bg);
}

.crumbs__path {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.15rem 0.2rem;
  flex: 1 1 8rem;
  min-width: 0;
}

.crumbs__filter {
  flex: 0 1 14rem;
  width: 14rem;
  max-width: 100%;
  margin-left: auto;
}

.crumbs__item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0;
  border: 0;
  background: transparent;
  color: #0ea5e9;
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
}

.crumbs__item span {
  color: var(--shell-muted);
}

.explorer {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.explorer .grow {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.explorer :deep(.rs-table-shell--fill) {
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.explorer :deep(.rs-table__td[data-col-key='name'] .rs-table__cell-body) {
  display: flex;
  width: 100%;
  min-width: 0;
  overflow: visible;
  text-overflow: clip;
  white-space: normal;
}

.name-link {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.name-link__icon {
  flex-shrink: 0;
}

.name-link__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-ops {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: nowrap;
  gap: 0.1rem;
  white-space: nowrap;
}

.explorer :deep(.rs-table__th[data-col-key='actions']),
.explorer :deep(.rs-table__td[data-col-key='actions']) {
  width: 10.5rem;
  min-width: 10.5rem;
  max-width: 10.5rem;
  box-shadow: -8px 0 10px -8px color-mix(in srgb, var(--shell-title) 22%, transparent);
}

.explorer :deep(.rs-table__td[data-col-key='actions']),
.explorer :deep(.rs-table__td[data-col-key='actions'] .rs-table__cell-body) {
  overflow: visible;
}

.preview-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 0;
  min-height: 0;
  height: 100%;
}

.preview-body--empty {
  align-items: center;
  justify-content: center;
}

.preview-alert {
  flex-shrink: 0;
  margin: 0.7rem 0.8rem 0;
}

.preview-code {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 0;
  border-radius: 0;
}

.preview-code :deep(.rs-code-block__editor) {
  flex: 1;
  min-height: 0;
}

.preview-code :deep(.cm-editor) {
  height: 100%;
  max-height: none;
}
</style>
<style>
.project-files-preview.rs-dialog__content .rs-dialog__body {
  padding: 0;
  overflow: hidden;
}
</style>
