<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { Upload } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import {
  exportWorkspaceBundle,
  importWorkspaceBundle,
  type WorkspaceBundle,
  type WorkspaceImportMode,
} from '@/request/api/workspace'
import { isValidWorkspaceCode, suggestCloneWorkspaceCode } from '../workspace/workspace-code'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  workspaceCode: string
  projectCount: number
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

const activeTab = ref<'export' | 'import'>('export')
const exporting = ref(false)
const importing = ref(false)
const importMode = ref<WorkspaceImportMode>('new')
const bundle = ref<WorkspaceBundle | null>(null)
const fileName = ref('')
const workspaceCode = ref('')
const workspaceName = ref('')
const rootDir = ref('')

const bundleProjectCount = computed(() => bundle.value?.projects?.length ?? 0)
const bundleWorkspaceName = computed(() => bundle.value?.workspace?.workspace_name?.trim() ?? '')

watch(open, (v) => {
  if (!v) return
  activeTab.value = 'export'
  importMode.value = 'new'
  bundle.value = null
  fileName.value = ''
  workspaceCode.value = ''
  workspaceName.value = ''
  rootDir.value = ''
})

function suggestImportCode(): void {
  const code = bundle.value?.workspace?.workspace_code?.trim() ?? props.workspaceCode
  workspaceCode.value = suggestCloneWorkspaceCode(code)
}

async function onExport(): Promise<void> {
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  exporting.value = true
  try {
    const code = props.workspaceCode.trim() || 'workspace'
    const ts = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '').replace(' ', '-')
    await exportWorkspaceBundle(props.workspaceRecId, `${code}-autopack-${ts}.json`)
    message.success(t('projectBulkWorkspaceBundleExportSuccess', { count: props.projectCount }))
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    exporting.value = false
  }
}

function onPickFile(): void {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.json,application/json'
  input.onchange = () => {
    const file = input.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result ?? '')) as WorkspaceBundle
        if (!parsed?.workspace || !Array.isArray(parsed.projects)) {
          throw new Error(t('projectBulkWorkspaceBundleInvalidFile'))
        }
        bundle.value = parsed
        fileName.value = file.name
        workspaceCode.value = parsed.workspace.workspace_code?.trim() ?? ''
        workspaceName.value = parsed.workspace.workspace_name?.trim() ?? ''
        rootDir.value = parsed.workspace.root_dir?.trim() ?? ''
        suggestImportCode()
      } catch (e) {
        bundle.value = null
        fileName.value = ''
        message.error((e as Error).message)
      }
    }
    reader.readAsText(file, 'utf-8')
  }
  input.click()
}

async function onImport(): Promise<void> {
  if (!bundle.value) {
    message.warning(t('projectBulkWorkspaceBundleFileRequired'))
    return
  }
  if (importMode.value === 'replace') {
    if (!props.workspaceRecId) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
  } else {
    const code = workspaceCode.value.trim()
    const name = workspaceName.value.trim()
    if (!code || !isValidWorkspaceCode(code)) {
      message.warning(t('workspaceCodeInvalid'))
      return
    }
    if (!name) {
      message.warning(t('workspaceNameRequired'))
      return
    }
  }
  importing.value = true
  try {
    if (importMode.value === 'replace') {
      await importWorkspaceBundle({
        mode: 'replace',
        target_workspace_rec_id: props.workspaceRecId,
        bundle: bundle.value,
      })
      message.success(
        t('projectBulkWorkspaceBundleImportReplaceSuccess', { count: bundleProjectCount.value }),
      )
    } else {
      await importWorkspaceBundle({
        mode: 'new',
        workspace_code: workspaceCode.value.trim(),
        workspace_name: workspaceName.value.trim(),
        root_dir: rootDir.value.trim(),
        bundle: bundle.value,
      })
      message.success(
        t('projectBulkWorkspaceBundleImportNewSuccess', {
          name: workspaceName.value.trim(),
          count: bundleProjectCount.value,
        }),
      )
    }
    open.value = false
    emit('success')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    importing.value = false
  }
}

async function onConfirm(): Promise<void> {
  if (activeTab.value === 'export') {
    await onExport()
    return
  }
  if (importMode.value === 'replace' && bundle.value) {
    try {
      await ElMessageBox.confirm(
        t('projectBulkWorkspaceBundleImportReplaceConfirm', {
          name: props.workspaceName,
          count: props.projectCount,
        }),
        t('projectBulkWorkspaceBundleImportReplaceTitle'),
        { type: 'warning', confirmButtonText: t('projectBulkWorkspaceBundleImport'), cancelButtonText: t('cancel') },
      )
    } catch {
      return
    }
  }
  await onImport()
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkWorkspaceBundleTitle')"
    width="640px"
    initial-height="auto"
    :show-footer="true"
    :confirm-loading="exporting || importing"
    :confirm-text="activeTab === 'export' ? $t('projectBulkWorkspaceBundleExport') : $t('projectBulkWorkspaceBundleImport')"
    @confirm="onConfirm"
  >
    <el-tabs v-model="activeTab" class="project-bulk-bundle__tabs">
      <el-tab-pane :label="$t('projectBulkWorkspaceBundleExportTab')" name="export">
        <p class="project-bulk-bundle__hint">
          {{
            $t('projectBulkWorkspaceBundleExportHint', {
              name: workspaceName,
              count: projectCount,
            })
          }}
        </p>
        <ul class="project-bulk-bundle__list">
          <li>{{ $t('projectBulkWorkspaceBundleExportItemWorkspace') }}</li>
          <li>{{ $t('projectBulkWorkspaceBundleExportItemProjects') }}</li>
        </ul>
      </el-tab-pane>

      <el-tab-pane :label="$t('projectBulkWorkspaceBundleImportTab')" name="import">
        <p class="project-bulk-bundle__hint">{{ $t('projectBulkWorkspaceBundleImportHint') }}</p>

        <el-form label-width="108px" class="project-bulk-bundle__form" @submit.prevent="onConfirm">
          <el-form-item :label="$t('projectBulkWorkspaceBundleImportMode')">
            <el-radio-group v-model="importMode">
              <el-radio value="new">{{ $t('projectBulkWorkspaceBundleImportModeNew') }}</el-radio>
              <el-radio value="replace">
                {{ $t('projectBulkWorkspaceBundleImportModeReplace') }}
              </el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item :label="$t('projectBulkWorkspaceBundleImportFile')">
            <div class="project-bulk-bundle__file-row">
              <el-button :icon="Upload" @click="onPickFile">
                {{ $t('projectBulkWorkspaceBundleChooseFile') }}
              </el-button>
              <span v-if="fileName" class="project-bulk-bundle__file-name">{{ fileName }}</span>
              <span v-else class="project-bulk-bundle__file-placeholder">
                {{ $t('projectBulkWorkspaceBundleNoFile') }}
              </span>
            </div>
            <p v-if="bundle" class="project-bulk-bundle__file-meta">
              {{
                $t('projectBulkWorkspaceBundleFileMeta', {
                  name: bundleWorkspaceName,
                  count: bundleProjectCount,
                })
              }}
            </p>
          </el-form-item>

          <template v-if="importMode === 'new'">
            <el-form-item :label="$t('workspaceColCode')" required>
              <el-input v-model="workspaceCode" autocomplete="off" />
            </el-form-item>
            <el-form-item :label="$t('workspaceColName')" required>
              <el-input v-model="workspaceName" autocomplete="off" />
            </el-form-item>
            <el-form-item :label="$t('workspaceColRoot')">
              <el-input v-model="rootDir" autocomplete="off" />
            </el-form-item>
          </template>

          <p v-else class="project-bulk-bundle__warning">
            {{
              $t('projectBulkWorkspaceBundleImportReplaceNote', {
                name: workspaceName,
                count: projectCount,
              })
            }}
          </p>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-bundle {
  &__tabs {
    margin-top: -4px;
  }

  &__hint {
    margin: 0 0 12px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__list {
    margin: 0;
    padding-left: 18px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__form {
    margin-top: 4px;
  }

  &__file-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
    width: 100%;
  }

  &__file-name,
  &__file-placeholder {
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__file-meta {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  &__warning {
    margin: 0;
    font-size: 12px;
    line-height: 1.55;
    color: var(--el-color-warning);
  }
}
</style>
