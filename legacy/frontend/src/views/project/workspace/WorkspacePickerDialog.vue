<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Search } from '@element-plus/icons-vue'
import type { PackWorkspace } from '@/types'
import { effectiveWorkspaceRoot, workspaceOptionLabel } from './workspace-paths'

const open = defineModel<boolean>('open', { required: true })
const modelValue = defineModel<string>('modelValue', { required: true })

const props = defineProps<{
  workspaces: PackWorkspace[]
}>()

const emit = defineEmits<{
  select: [recId: string]
}>()

const keyword = ref('')

const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return props.workspaces
  return props.workspaces.filter((ws) => {
    const hay = `${ws.workspace_name} ${ws.workspace_code} ${ws.root_dir} ${ws.effective_root_dir ?? ''}`.toLowerCase()
    return hay.includes(q)
  })
})

const selectedRow = computed(() => props.workspaces.find((w) => w.rec_id === modelValue.value))

watch(open, (v) => {
  if (v) keyword.value = ''
})

function onPick(row: PackWorkspace): void {
  modelValue.value = row.rec_id
  emit('select', row.rec_id)
  open.value = false
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('workspacePickerTitle')"
    width="640px"
    initial-height="auto"
    :show-footer="false"
  >
    <div class="workspace-picker-dialog">
      <el-input
        v-model="keyword"
        clearable
        :placeholder="$t('workspacePickerSearch')"
        class="workspace-picker-dialog__search"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <p v-if="selectedRow" class="workspace-picker-dialog__current">
        {{ $t('workspacePickerCurrent') }}：
        <strong>{{ workspaceOptionLabel(selectedRow) }}</strong>
      </p>

      <el-table
        :data="filtered"
        row-key="rec_id"
        max-height="360"
        highlight-current-row
        class="workspace-picker-dialog__table"
        :empty-text="$t('workspacePickerEmpty')"
        @row-dblclick="onPick"
      >
        <el-table-column prop="workspace_name" :label="$t('workspaceColName')" min-width="120" />
        <el-table-column prop="workspace_code" :label="$t('workspaceColCode')" width="110" />
        <el-table-column :label="$t('workspaceColRoot')" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="workspace-picker-dialog__root">{{ effectiveWorkspaceRoot(row) }}</span>
            <el-tag
              v-if="!row.root_dir?.trim()"
              size="small"
              type="info"
              effect="plain"
              class="workspace-picker-dialog__tag"
            >
              {{ $t('workspaceRootDefaultTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('projectColActions')" width="88" align="right" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :class="{ 'is-current': row.rec_id === modelValue }"
              @click="onPick(row)"
            >
              {{ row.rec_id === modelValue ? $t('workspacePickerSelected') : $t('workspacePickerChoose') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.workspace-picker-dialog {
  &__search {
    margin-bottom: 12px;
  }

  &__current {
    margin: 0 0 12px;
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__table {
    width: 100%;
  }

  &__root {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }

  &__tag {
    margin-left: 6px;
    vertical-align: middle;
  }

  .is-current {
    font-weight: 600;
  }
}
</style>
