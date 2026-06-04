<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EnvSetting, EnvType } from '@/types'
import { envTypeTagClass } from './env-meta'

const props = defineProps<{
  loading: boolean
  items: EnvSetting[]
  envTypes: EnvType[]
  activeType: string
  verifyingId: string | null
  typeLabel: (type: unknown) => string
}>()

const emit = defineEmits<{
  edit: [row: EnvSetting]
  verify: [row: EnvSetting]
  delete: [row: EnvSetting]
}>()

const activeType = defineModel<string>('activeType', { required: true })
const { t } = useI18n()

const isFiltered = computed(() => activeType.value !== 'all')
const showEmpty = computed(() => !props.loading && props.items.length === 0)
const emptyTitle = computed(() =>
  isFiltered.value ? t('envEmptyFilter') : t('envEmptyList'),
)
const emptyHint = computed(() => (isFiltered.value ? '' : t('envEmptyListHint')))
</script>

<template>
  <el-card class="env-list env-panel" shadow="never">
    <el-tabs v-model="activeType" class="env-list__tabs">
      <el-tab-pane :label="$t('envTabAll')" name="all" />
      <el-tab-pane v-for="tp in envTypes" :key="tp" :label="typeLabel(tp)" :name="tp" />
    </el-tabs>

    <div v-loading="loading" class="env-list__body">
      <el-table
        v-if="!showEmpty"
        :data="items"
        row-key="rec_id"
        class="env-list__table"
        :header-cell-style="{ fontWeight: 600 }"
      >
        <el-table-column prop="env_name" :label="$t('envColName')" min-width="140">
          <template #default="{ row }">
            <span class="env-list__name">{{ row.env_name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="env_type" :label="$t('envColType')" width="108">
          <template #default="{ row }">
            <span :class="envTypeTagClass(row.env_type)">
              {{ typeLabel(row.env_type) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('envColDefault')" width="72" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.is_default === 1" type="warning" size="small" effect="plain">
              {{ $t('yes') }}
            </el-tag>
            <span v-else class="env-list__muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('envColEnabled')" width="88" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.is_enabled === 1 ? 'success' : 'info'"
              size="small"
              effect="plain"
            >
              {{ row.is_enabled === 1 ? $t('enabled') : $t('disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('envColActions')" min-width="200" fixed="right" align="right">
          <template #default="{ row }">
            <div class="env-list__actions">
              <el-button link type="primary" @click.stop="emit('edit', row)">{{ $t('edit') }}</el-button>
              <el-button
                link
                type="success"
                :loading="verifyingId === row.rec_id"
                @click.stop="emit('verify', row)"
              >
                {{ $t('envVerify') }}
              </el-button>
              <AppPopconfirm :title="$t('envDeleteConfirm')" @confirm="emit('delete', row)">
                <template #reference>
                  <el-button link type="danger" @click.stop>{{ $t('delete') }}</el-button>
                </template>
              </AppPopconfirm>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-else class="env-list__empty" :description="emptyTitle">
        <p v-if="emptyHint" class="env-list__empty-hint">{{ emptyHint }}</p>
      </el-empty>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
.env-list {
  overflow: hidden;
  border-radius: 16px;

  :deep(.el-card__body) {
    padding: 0 20px 20px;
  }

  &__tabs {
    margin: 0 -4px;

    :deep(.el-tabs__header) {
      margin: 0 0 4px;
    }

    :deep(.el-tabs__nav-wrap::after) {
      background-color: var(--shell-card-border, var(--el-border-color-lighter));
    }

    :deep(.el-tabs__item) {
      height: 44px;
      font-size: 14px;
      color: var(--shell-content-text-secondary, var(--el-text-color-secondary));

      &.is-active {
        font-weight: 600;
        color: var(--shell-accent, var(--el-color-primary));
      }
    }

    :deep(.el-tabs__active-bar) {
      background-color: var(--shell-accent, var(--el-color-primary));
    }
  }

  &__body {
    min-height: 200px;
  }

  &__table {
    width: 100%;

    :deep(.el-table__inner-wrapper::before) {
      display: none;
    }

    :deep(th.el-table__cell) {
      background: var(--shell-table-header-bg, var(--el-fill-color-light));
      color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
      font-size: 13px;
    }

    :deep(.el-table__row) {
      transition: background-color 0.15s ease;
    }
  }

  &__name {
    font-weight: 500;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__muted {
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
    user-select: none;
  }

  &__actions {
    display: inline-flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    align-items: center;
    gap: 2px 4px;
  }

  &__empty {
    padding: 32px 16px 40px;

    :deep(.el-empty__description) {
      margin-top: 12px;
      font-size: 15px;
      font-weight: 500;
      color: var(--shell-content-text, var(--el-text-color-primary));
    }
  }

  &__empty-hint {
    margin: 8px 0 0;
    max-width: 320px;
    font-size: 13px;
    line-height: 1.6;
    text-align: center;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }
}

</style>
