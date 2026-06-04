<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CopyDocument, Filter, Plus, Search, Delete } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import {
  cloneWorkspace,
  createWorkspace,
  deleteWorkspace,
  fetchWorkspaces,
  updateWorkspace,
} from '@/request/api/workspace'
import type { PackWorkspace, PackWorkspacePayload } from '@/types'
import { effectiveWorkspaceRoot } from './workspace-paths'
import { isValidWorkspaceCode, suggestCloneWorkspaceCode } from './workspace-code'

type FormMode = 'create' | 'edit' | 'clone'

const open = defineModel<boolean>('open', { required: true })

const emit = defineEmits<{
  changed: []
}>()

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const saving = ref(false)
const items = ref<PackWorkspace[]>([])
const editingId = ref<string | null>(null)
const formMode = ref<FormMode>('create')
const cloneSourceId = ref<string | null>(null)
const keyword = ref('')
const filterEnabled = ref<'all' | 'enabled' | 'disabled'>('all')
const filterPopoverOpen = ref(false)

const form = reactive<PackWorkspacePayload>({
  workspace_code: '',
  workspace_name: '',
  root_dir: '',
  is_default: 0,
  is_enabled: 1,
  sort_num: 0,
  remark: '',
})

const isCreateMode = computed(() => formMode.value === 'create')
const isEditMode = computed(() => formMode.value === 'edit')
const isCloneMode = computed(() => formMode.value === 'clone')

const formTitle = computed(() => {
  if (isCloneMode.value) return t('workspaceCloneTitle')
  if (isEditMode.value) return t('workspaceEditTitle')
  return t('workspaceCreateTitle')
})

const confirmLabel = computed(() => {
  if (isCloneMode.value) return t('workspaceClone')
  if (isEditMode.value) return t('save')
  return t('workspaceAdd')
})

const cloneSource = computed(() =>
  cloneSourceId.value ? items.value.find((w) => w.rec_id === cloneSourceId.value) ?? null : null,
)

const filterEnabledLabel = computed(() => {
  if (filterEnabled.value === 'enabled') return t('workspaceFilterEnabled')
  if (filterEnabled.value === 'disabled') return t('workspaceFilterDisabled')
  return t('workspaceFilterAll')
})

const editingWorkspace = computed(() =>
  editingId.value ? items.value.find((w) => w.rec_id === editingId.value) ?? null : null,
)

const canDeleteEditing = computed(() => {
  const row = editingWorkspace.value
  return !!row && row.is_default !== 1
})

function canDeleteWorkspace(row: PackWorkspace): boolean {
  return row.is_default !== 1
}

const filteredList = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return items.value.filter((row) => {
    if (filterEnabled.value === 'enabled' && row.is_enabled !== 1) return false
    if (filterEnabled.value === 'disabled' && row.is_enabled !== 0) return false
    if (!q) return true
    const hay = `${row.workspace_name} ${row.workspace_code} ${row.root_dir} ${row.effective_root_dir ?? ''}`.toLowerCase()
    return hay.includes(q)
  })
})

function asItemList<T>(raw: T[] | null | undefined): T[] {
  return Array.isArray(raw) ? raw : []
}

function resetForm(): void {
  formMode.value = 'create'
  editingId.value = null
  cloneSourceId.value = null
  form.workspace_code = ''
  form.workspace_name = ''
  form.root_dir = ''
  form.is_default = 0
  form.is_enabled = 1
  form.sort_num = 0
  form.remark = ''
}

function openCreate(): void {
  resetForm()
}

function openClone(row: PackWorkspace): void {
  formMode.value = 'clone'
  editingId.value = null
  cloneSourceId.value = row.rec_id
  const nameSuffix = t('projectCloneNameSuffix')
  form.workspace_code = suggestCloneWorkspaceCode(row.workspace_code)
  form.workspace_name = `${row.workspace_name}${nameSuffix}`
  form.root_dir = ''
  form.is_default = 0
  form.is_enabled = row.is_enabled
  form.sort_num = row.sort_num
  form.remark = row.remark
}

function openEdit(row: PackWorkspace): void {
  formMode.value = 'edit'
  cloneSourceId.value = null
  editingId.value = row.rec_id
  form.workspace_code = row.workspace_code
  form.workspace_name = row.workspace_name
  form.root_dir = row.root_dir
  form.is_default = row.is_default
  form.is_enabled = row.is_enabled
  form.sort_num = row.sort_num
  form.remark = row.remark
}

function applyFilter(enabled: 'all' | 'enabled' | 'disabled'): void {
  filterEnabled.value = enabled
  filterPopoverOpen.value = false
}

async function loadList(): Promise<void> {
  loading.value = true
  try {
    const res = await fetchWorkspaces()
    items.value = asItemList(res.items)
  } catch (e) {
    message.error((e as Error).message)
    items.value = []
  } finally {
    loading.value = false
  }
}

function validateWorkspaceForm(): boolean {
  if (!form.workspace_name.trim()) {
    message.warning(t('workspaceNameRequired'))
    return false
  }
  const code = form.workspace_code.trim()
  if (!code) {
    message.warning(t('workspaceCodeRequired'))
    return false
  }
  if (!isValidWorkspaceCode(code)) {
    message.warning(t('workspaceCodeInvalid'))
    return false
  }
  form.workspace_code = code
  return true
}

async function onClone(): Promise<void> {
  if (!cloneSourceId.value) {
    message.warning(t('workspaceCloneSelectSource'))
    return
  }
  if (!validateWorkspaceForm()) return
  saving.value = true
  try {
    const result = await cloneWorkspace({
      source_rec_id: cloneSourceId.value,
      workspace_code: form.workspace_code,
      workspace_name: form.workspace_name.trim(),
      root_dir: form.root_dir,
      is_default: 0,
      is_enabled: form.is_enabled,
      sort_num: form.sort_num,
      remark: form.remark,
    })
    message.success(t('workspaceCloneSuccess', { count: result.projects_cloned }))
    await loadList()
    openEdit(result)
    emit('changed')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function onSave(): Promise<void> {
  if (isCloneMode.value) {
    await onClone()
    return
  }
  if (!validateWorkspaceForm()) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateWorkspace(editingId.value, { ...form })
      message.success(t('workspaceSaveSuccess'))
    } else {
      const created = await createWorkspace({ ...form })
      message.success(t('workspaceCreateSuccess'))
      editingId.value = created.rec_id
      form.workspace_code = created.workspace_code
    }
    emit('changed')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function deleteWorkspaceById(recId: string): Promise<void> {
  try {
    await deleteWorkspace(recId)
    message.success(t('workspaceDeleteSuccess'))
    if (editingId.value === recId) resetForm()
    await loadList()
    emit('changed')
  } catch (e) {
    message.error((e as Error).message)
  }
}

async function onDelete(): Promise<void> {
  if (!editingId.value) return
  await deleteWorkspaceById(editingId.value)
}

function setItems(list: PackWorkspace[]): void {
  items.value = list
}

defineExpose({ setItems })

watch(open, (v) => {
  if (v) {
    keyword.value = ''
    filterEnabled.value = 'all'
    resetForm()
    void loadList()
  }
})
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('workspaceManageTitle')"
    width="920px"
    :min-height="280"
    :show-footer="false"
  >
    <div v-loading="loading" class="workspace-manage">
      <aside class="workspace-manage__list-panel">
        <div class="workspace-manage__list-toolbar">
          <el-input
            v-model="keyword"
            clearable
            :placeholder="$t('workspaceSearchPlaceholder')"
            class="workspace-manage__search"
            @keyup.enter.prevent
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <div class="workspace-manage__list-actions">
            <AppPopover
              v-model:visible="filterPopoverOpen"
              placement="bottom-start"
              :width="168"
              trigger="click"
            >
              <template #reference>
                <el-button :icon="Filter">{{ $t('workspaceFilter') }}</el-button>
              </template>
              <div class="workspace-manage__filter-menu">
                <button
                  type="button"
                  class="workspace-manage__filter-item"
                  :class="{ 'is-active': filterEnabled === 'all' }"
                  @click="applyFilter('all')"
                >
                  {{ $t('workspaceFilterAll') }}
                </button>
                <button
                  type="button"
                  class="workspace-manage__filter-item"
                  :class="{ 'is-active': filterEnabled === 'enabled' }"
                  @click="applyFilter('enabled')"
                >
                  {{ $t('workspaceFilterEnabled') }}
                </button>
                <button
                  type="button"
                  class="workspace-manage__filter-item"
                  :class="{ 'is-active': filterEnabled === 'disabled' }"
                  @click="applyFilter('disabled')"
                >
                  {{ $t('workspaceFilterDisabled') }}
                </button>
              </div>
            </AppPopover>
            <el-button type="primary" :icon="Plus" @click="openCreate">{{ $t('workspaceAdd') }}</el-button>
          </div>
        </div>

        <p class="workspace-manage__filter-hint">
          {{ $t('workspaceFilterActive', { label: filterEnabledLabel }) }}
          <span v-if="keyword.trim()"> · {{ $t('workspaceSearchActive', { q: keyword.trim() }) }}</span>
        </p>

        <div class="workspace-manage__list-scroll">
          <div
            v-for="row in filteredList"
            :key="row.rec_id"
            class="workspace-manage__list-row"
            :class="{ 'is-active': editingId === row.rec_id && !isCloneMode }"
          >
            <div class="workspace-manage__list-item">
              <div class="workspace-manage__list-item-top">
                <button type="button" class="workspace-manage__list-item-head" @click="openEdit(row)">
                  <span class="workspace-manage__list-name">{{ row.workspace_name }}</span>
                  <span class="workspace-manage__list-code">{{ row.workspace_code }}</span>
                </button>
                <div class="workspace-manage__list-item-actions">
                  <el-tooltip :content="$t('workspaceClone')" placement="top">
                    <el-button
                      text
                      type="primary"
                      size="small"
                      :aria-label="$t('workspaceClone')"
                      @click.stop="openClone(row)"
                    >
                      <el-icon><CopyDocument /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <el-tooltip
                    v-if="!canDeleteWorkspace(row)"
                    :content="$t('workspaceDeleteDefaultHint')"
                    placement="top"
                  >
                    <el-button text disabled size="small" :aria-label="$t('delete')">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <AppPopconfirm
                    v-else
                    :title="$t('workspaceDeleteConfirm')"
                    @confirm="deleteWorkspaceById(row.rec_id)"
                  >
                    <template #reference>
                      <el-button
                        text
                        type="danger"
                        size="small"
                        :aria-label="$t('delete')"
                        @click.stop
                      >
                        <el-icon><Delete /></el-icon>
                      </el-button>
                    </template>
                  </AppPopconfirm>
                </div>
              </div>
              <button type="button" class="workspace-manage__list-item-body" @click="openEdit(row)">
                <span class="workspace-manage__list-root">{{ effectiveWorkspaceRoot(row) }}</span>
                <span class="workspace-manage__list-tags">
                  <el-tag v-if="row.is_default === 1" size="small" type="warning" effect="plain">
                    {{ $t('envColDefault') }}
                  </el-tag>
                  <el-tag
                    size="small"
                    effect="plain"
                    :type="row.is_enabled === 1 ? 'success' : 'info'"
                  >
                    {{ row.is_enabled === 1 ? $t('enabled') : $t('disabled') }}
                  </el-tag>
                </span>
              </button>
            </div>
          </div>
          <el-empty
            v-if="filteredList.length === 0"
            class="workspace-manage__list-empty"
            :description="$t('workspaceListEmpty')"
          />
        </div>
      </aside>

      <section class="workspace-manage__form-panel">
        <header class="workspace-manage__form-head">
          <h3 class="workspace-manage__form-title">{{ formTitle }}</h3>
          <p v-if="isCreateMode" class="workspace-manage__form-hint">{{ $t('workspaceCreateHint') }}</p>
          <p v-else-if="isCloneMode" class="workspace-manage__form-hint">{{ $t('workspaceCloneHint') }}</p>
        </header>

        <el-form
          label-width="96px"
          label-position="right"
          class="workspace-manage__form"
          @submit.prevent="onSave"
        >
          <el-form-item v-if="isCloneMode" :label="$t('workspaceCloneSource')">
            <el-input :model-value="cloneSource?.workspace_name ?? ''" disabled />
          </el-form-item>
          <el-form-item :label="$t('workspaceColName')" required>
            <el-input v-model="form.workspace_name" />
          </el-form-item>
          <el-form-item :label="$t('workspaceColCode')" required>
            <el-input
              v-model="form.workspace_code"
              :disabled="isEditMode"
              :placeholder="$t('workspaceCodePlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="$t('workspaceColRoot')">
            <el-input v-model="form.root_dir" :placeholder="$t('workspaceRootPlaceholder')" />
          </el-form-item>
          <el-form-item :label="$t('envColEnabled')">
            <el-switch v-model="form.is_enabled" :active-value="1" :inactive-value="0" />
          </el-form-item>
          <el-form-item v-if="!isCloneMode" :label="$t('envColDefault')">
            <el-switch v-model="form.is_default" :active-value="1" :inactive-value="0" />
          </el-form-item>
          <el-form-item :label="$t('envColSort')">
            <el-input-number v-model="form.sort_num" :min="0" :max="9999" />
          </el-form-item>
          <el-form-item :label="$t('envColRemark')">
            <el-input v-model="form.remark" type="textarea" :rows="3" />
          </el-form-item>

          <div class="workspace-manage__form-actions">
            <el-tooltip
              v-if="editingId && !canDeleteEditing"
              :content="$t('workspaceDeleteDefaultHint')"
              placement="top"
            >
              <el-button type="danger" disabled native-type="button">{{ $t('delete') }}</el-button>
            </el-tooltip>
            <el-button v-if="isCloneMode" native-type="button" @click="openCreate">
              {{ $t('cancel') }}
            </el-button>
            <AppPopconfirm
              v-else-if="isEditMode"
              :title="$t('workspaceDeleteConfirm')"
              @confirm="onDelete"
            >
              <template #reference>
                <el-button type="danger" native-type="button">{{ $t('delete') }}</el-button>
              </template>
            </AppPopconfirm>
            <el-button type="primary" :loading="saving" native-type="submit">
              {{ confirmLabel }}
            </el-button>
          </div>
        </el-form>
      </section>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.workspace-manage {
  display: flex;
  gap: 0;
  flex: 1;
  min-height: 0;
  height: 100%;
  margin: -4px 0;
  box-sizing: border-box;

  &__list-panel {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    width: 42%;
    min-width: 280px;
    max-width: 360px;
    min-height: 0;
    padding-right: 16px;
    border-right: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
  }

  &__list-toolbar {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 8px;
  }

  &__search {
    width: 100%;
  }

  &__list-actions {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: 8px;
  }

  &__filter-hint {
    margin: 0 0 8px;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__filter-menu {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  &__filter-item {
    padding: 8px 10px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--shell-content-text, var(--el-text-color-primary));
    font-size: 13px;
    text-align: left;
    cursor: pointer;

    &:hover {
      background: var(--el-fill-color-light);
    }

    &.is-active {
      font-weight: 600;
      color: var(--shell-accent, var(--el-color-primary));
      background: rgba(14, 165, 233, 0.1);
    }
  }

  &__list-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-right: 4px;
  }

  &__list-row {
    margin-bottom: 6px;

    &.is-active .workspace-manage__list-item {
      border-color: var(--shell-accent, var(--el-color-primary));
      background: rgba(14, 165, 233, 0.08);
      box-shadow: 0 0 0 1px var(--shell-accent, var(--el-color-primary));
    }
  }

  &__list-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px 10px 10px;
    border: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
    border-radius: 10px;
    background: var(--env-panel-inset-bg, var(--el-fill-color-lighter));
    transition:
      border-color 0.15s ease,
      background-color 0.15s ease;

    &:hover {
      border-color: var(--shell-accent, var(--el-color-primary));
    }
  }

  &__list-item-top {
    display: flex;
    align-items: flex-start;
    gap: 4px;
    min-width: 0;
  }

  &__list-item-head,
  &__list-item-body {
    margin: 0;
    padding: 0;
    border: none;
    background: transparent;
    font: inherit;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  &__list-item-head {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 6px;
    flex: 1;
    min-width: 0;
    padding: 2px 0 0;
  }

  &__list-item-body {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    width: 100%;
    padding: 0 0 2px;
  }

  &__list-item-actions {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    margin: -4px -6px 0 0;

    .el-button {
      padding: 4px;
    }
  }

  &__list-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__list-code {
    font-family: ui-monospace, monospace;
    font-size: 12px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__list-root {
    font-size: 11px;
    line-height: 1.4;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
    word-break: break-all;
  }

  &__list-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 2px;
  }

  &__list-empty {
    padding: 24px 8px;
  }

  &__form-panel {
    flex: 1;
    min-width: 0;
    min-height: 0;
    padding-left: 20px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  &__form-head {
    margin-bottom: 16px;
  }

  &__form-title {
    margin: 0 0 4px;
    font-size: 16px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__form-hint {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__form {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-right: 4px;
  }

  &__form-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
    padding-top: 16px;
    border-top: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
  }
}

@media (max-width: 720px) {
  .workspace-manage {
    flex-direction: column;

    &__list-panel {
      width: 100%;
      max-width: none;
      padding-right: 0;
      padding-bottom: 16px;
      border-right: none;
      border-bottom: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
    }

    &__list-scroll {
      flex: 0 1 auto;
      max-height: min(240px, 35vh);
    }

    &__form-panel {
      flex: 1 1 auto;
      min-height: 200px;
    }

    &__form-panel {
      padding-left: 0;
      padding-top: 16px;
    }
  }
}
</style>

<style lang="scss">
/* 窗口模式定高：内容区 flex 填满，避免先被内容撑高再收缩 */
.app-dialog--windowed.app-dialog--initial-height .app-dialog__content:has(.workspace-manage) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* 全屏：内容区占满对话框，左右栏各自滚动 */
.app-dialog.is-fullscreen .app-dialog__body,
.app-dialog--fullscreen .app-dialog__body {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-dialog.is-fullscreen .app-dialog__content:has(.workspace-manage),
.app-dialog--fullscreen .app-dialog__content:has(.workspace-manage) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  max-width: none;
  width: 100%;
  margin: 0;
}

.app-dialog.is-fullscreen .workspace-manage,
.app-dialog--fullscreen .workspace-manage {
  min-height: 0;
  height: 100%;
}

.app-dialog.is-fullscreen .workspace-manage__list-panel,
.app-dialog--fullscreen .workspace-manage__list-panel {
  max-width: min(420px, 36vw);
}
</style>
