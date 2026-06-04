<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  FileContentDialog,
  FileEntryActions,
  downloadAuthenticated,
  basenameFromPath,
  downloadFileName,
  formatFileSize,
  projectWorkspaceDownloadUrl,
} from '@/components/files'
import { useMessage } from '@/components/feedback/message'
import { deleteProjectWorkspace, fetchProjectWorkspaceFile } from '@/request/api/build'
import type { ProjectWorkspaceEntry, ProjectWorkspaceFile } from '@/types'
import ProjectWorkspaceFtpPushDialog from '../ProjectWorkspaceFtpPushDialog.vue'

const props = defineProps<{
  projectRecId: string
  path: string
  absPath: string
  exists: boolean
  entries: ProjectWorkspaceEntry[]
  loading: boolean
}>()

const emit = defineEmits<{
  up: []
  refresh: []
  open: [entry: ProjectWorkspaceEntry]
}>()

const { t } = useI18n()
const message = useMessage()

const viewerOpen = ref(false)
const viewerLoading = ref(false)
const dialogDownloadLoading = ref(false)
const downloadingPath = ref<string | null>(null)
const deletingPath = ref<string | null>(null)
const clearingDir = ref(false)
const ftpPushOpen = ref(false)
const ftpPushEntry = ref<ProjectWorkspaceEntry | null>(null)
const viewerFile = ref<ProjectWorkspaceFile | null>(null)

const isRoot = computed(() => props.path === '.' || !props.path)

const breadcrumb = computed(() => {
  if (isRoot.value) return [t('projectDetailProjectDirRoot')]
  return [t('projectDetailProjectDirRoot'), ...props.path.split('/')]
})

async function viewEntry(entry: ProjectWorkspaceEntry, event?: Event): Promise<void> {
  event?.stopPropagation()
  if (entry.is_dir || !props.projectRecId) return
  viewerOpen.value = true
  viewerLoading.value = true
  viewerFile.value = null
  try {
    viewerFile.value = await fetchProjectWorkspaceFile(props.projectRecId, entry.path)
  } catch (e) {
    viewerOpen.value = false
    message.error((e as Error).message)
  } finally {
    viewerLoading.value = false
  }
}

async function downloadEntry(entry: ProjectWorkspaceEntry, event?: Event): Promise<void> {
  event?.stopPropagation()
  if (!props.projectRecId) return
  downloadingPath.value = entry.path
  try {
    const url = projectWorkspaceDownloadUrl(props.projectRecId, entry.path)
    const filename = downloadFileName(entry.path, entry.is_dir)
    await downloadAuthenticated(url, filename)
    message.success(t('fileDownloadSuccess', { name: entry.name }))
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    downloadingPath.value = null
  }
}

async function downloadViewedFile(): Promise<void> {
  if (!viewerFile.value) return
  dialogDownloadLoading.value = true
  try {
    const entry: ProjectWorkspaceEntry = {
      name: basenameFromPath(viewerFile.value.path),
      path: viewerFile.value.path,
      is_dir: false,
      size: viewerFile.value.size,
    }
    await downloadEntry(entry)
  } finally {
    dialogDownloadLoading.value = false
  }
}

async function deleteAtPath(targetPath: string): Promise<void> {
  if (!props.projectRecId) return
  const isClearRoot = targetPath === '.' || targetPath === ''
  if (isClearRoot) {
    clearingDir.value = true
  } else {
    deletingPath.value = targetPath
  }
  try {
    await deleteProjectWorkspace(props.projectRecId, isClearRoot ? '.' : targetPath)
    message.success(t('projectWorkspaceDeleteSuccess'))
    if (isClearRoot) {
      emit('refresh')
      return
    }
    if (targetPath === props.path) {
      emit('up')
      return
    }
    emit('refresh')
  } catch (e) {
    const msg = (e as Error).message
    if (msg.includes('build is running')) {
      message.error(t('projectWorkspaceDeleteRunning'))
    } else {
      message.error(msg)
    }
  } finally {
    clearingDir.value = false
    deletingPath.value = null
  }
}

function deleteEntry(entry: ProjectWorkspaceEntry, event?: Event): void {
  event?.stopPropagation()
  void deleteAtPath(entry.path)
}

function openFtpPush(entry: ProjectWorkspaceEntry, event?: Event): void {
  event?.stopPropagation()
  ftpPushEntry.value = entry
  ftpPushOpen.value = true
}

function onRowClick(row: ProjectWorkspaceEntry): void {
  if (row.is_dir) {
    emit('open', row)
    return
  }
  void viewEntry(row)
}
</script>

<template>
  <section class="project-workspace-panel">
    <header class="project-workspace-panel__head">
      <div>
        <h3 class="project-workspace-panel__title">{{ $t('projectDetailProjectDir') }}</h3>
        <p v-if="absPath" class="project-workspace-panel__path-hint">{{ absPath }}</p>
      </div>
      <div class="project-workspace-panel__actions">
        <el-button size="small" :disabled="isRoot" @click="emit('up')">
          {{ $t('projectWorkspaceUp') }}
        </el-button>
        <el-button size="small" @click="emit('refresh')">{{ $t('refresh') }}</el-button>
        <AppPopconfirm
          v-if="isRoot && exists"
          :title="$t('projectWorkspaceClearDirConfirm')"
          @confirm="deleteAtPath('.')"
        >
          <template #reference>
            <el-button size="small" type="danger" plain :loading="clearingDir">
              {{ $t('projectWorkspaceClearDir') }}
            </el-button>
          </template>
        </AppPopconfirm>
      </div>
    </header>

    <nav class="project-workspace-panel__breadcrumb" aria-label="breadcrumb">
      <span v-for="(part, i) in breadcrumb" :key="i" class="project-workspace-panel__crumb">
        {{ part }}
        <span v-if="i < breadcrumb.length - 1" class="project-workspace-panel__crumb-sep">/</span>
      </span>
    </nav>

    <div v-loading="loading" class="project-workspace-panel__body">
      <p v-if="!exists" class="project-workspace-panel__hint">{{ $t('projectWorkspaceNotCreated') }}</p>
      <el-table
        v-else-if="entries.length"
        :data="entries"
        row-key="path"
        size="small"
        class="project-workspace-panel__table"
        @row-click="onRowClick"
      >
        <el-table-column prop="name" :label="$t('projectWorkspaceName')" min-width="160">
          <template #default="{ row }">
            <span :class="{ 'is-dir': row.is_dir }">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="size" :label="$t('projectWorkspaceSize')" width="100" align="right">
          <template #default="{ row }">
            <span v-if="!row.is_dir">{{ formatFileSize(row.size) }}</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('projectWorkspaceActions')" width="320" align="center" fixed="right">
          <template #default="{ row }">
            <FileEntryActions
              :is-dir="row.is_dir"
              show-delete
              show-remote-push
              :delete-confirm-title="$t('projectWorkspaceDeleteEntryConfirm', { name: row.name })"
              :download-loading="downloadingPath === row.path"
              :delete-loading="deletingPath === row.path"
              @view="viewEntry(row)"
              @download="downloadEntry(row)"
              @delete="deleteEntry(row)"
              @remote-push="openFtpPush(row)"
            />
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="$t('projectWorkspaceEmpty')" />
    </div>

    <ProjectWorkspaceFtpPushDialog
      v-model:open="ftpPushOpen"
      :project-rec-id="projectRecId"
      :entry="ftpPushEntry"
    />

    <FileContentDialog
      v-model="viewerOpen"
      :path="viewerFile?.path ?? ''"
      :content="viewerFile?.content ?? ''"
      :size="viewerFile?.size ?? null"
      :truncated="viewerFile?.truncated ?? false"
      :readable="viewerFile?.readable ?? true"
      :loading="viewerLoading"
      :download-loading="dialogDownloadLoading"
      @download="downloadViewedFile"
    />
  </section>
</template>

<style scoped lang="scss" src="./ProjectWorkspacePanel.scss"></style>
