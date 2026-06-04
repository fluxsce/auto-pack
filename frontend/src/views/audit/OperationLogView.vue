<script setup lang="ts">
import { Search } from '@element-plus/icons-vue'
import { formatBuildTime } from '../project/project-format'
import { useOperationLogs } from './useOperationLogs'

const {
  items,
  total,
  loading,
  filters,
  modules,
  actions,
  page,
  totalPages,
  moduleLabel,
  actionLabel,
  applyFilters,
  onPageChange,
} = useOperationLogs()
</script>

<template>
  <div class="op-log">
    <header class="op-log__head">
      <div class="op-log__intro">
        <h1 class="op-log__title">{{ $t('opLogPageTitle') }}</h1>
        <p class="op-log__desc">{{ $t('opLogPageDesc') }}</p>
      </div>
    </header>

    <el-card class="op-log__card env-panel" shadow="never">
      <div class="op-log__toolbar">
        <el-input
          v-model="filters.keyword"
          clearable
          class="op-log__search"
          :placeholder="$t('opLogSearchPlaceholder')"
          @keyup.enter="applyFilters"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="filters.module"
          clearable
          class="op-log__filter"
          :placeholder="$t('opLogFilterModule')"
          @change="applyFilters"
        >
          <el-option :label="$t('workspaceFilterAll')" value="" />
          <el-option
            v-for="m in modules"
            :key="m"
            :label="moduleLabel(m)"
            :value="m"
          />
        </el-select>
        <el-select
          v-model="filters.action"
          clearable
          class="op-log__filter"
          :placeholder="$t('opLogFilterAction')"
          @change="applyFilters"
        >
          <el-option :label="$t('workspaceFilterAll')" value="" />
          <el-option
            v-for="a in actions"
            :key="a"
            :label="actionLabel(a)"
            :value="a"
          />
        </el-select>
        <el-button type="primary" @click="applyFilters">{{ $t('opLogSearch') }}</el-button>
      </div>

      <el-table v-loading="loading" :data="items" row-key="rec_id" class="op-log__table">
        <el-table-column :label="$t('opLogColTime')" width="168">
          <template #default="{ row }">
            {{ formatBuildTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('opLogColUser')" width="108" prop="login_name" />
        <el-table-column :label="$t('opLogColModule')" width="100">
          <template #default="{ row }">{{ moduleLabel(row.module) }}</template>
        </el-table-column>
        <el-table-column :label="$t('opLogColAction')" width="108">
          <template #default="{ row }">{{ actionLabel(row.action) }}</template>
        </el-table-column>
        <el-table-column :label="$t('opLogColResult')" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.success === 1 ? 'success' : 'danger'" size="small" effect="plain">
              {{ row.success === 1 ? $t('opLogSuccess') : $t('opLogFailed') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('opLogColSummary')" min-width="200" prop="summary" show-overflow-tooltip />
        <el-table-column :label="$t('opLogColTarget')" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.target_name">{{ row.target_name }}</span>
            <span v-else class="op-log__muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('opLogColIP')" width="120" prop="client_ip" />
      </el-table>

      <div v-if="total > 0" class="op-log__pager">
        <el-pagination
          :current-page="page"
          :page-size="50"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="onPageChange"
        />
      </div>
      <el-empty v-if="!loading && items.length === 0" :description="$t('opLogEmpty')" />
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.op-log {
  &__head {
    margin-bottom: 20px;
  }

  &__title {
    margin: 0 0 6px;
    font-size: 22px;
    font-weight: 600;
  }

  &__desc {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__card {
    border-radius: 16px;
  }

  &__toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 14px;
  }

  &__search {
    flex: 1;
    min-width: 200px;
    max-width: 320px;
  }

  &__filter {
    width: 140px;
  }

  &__pager {
    display: flex;
    justify-content: flex-end;
    margin-top: 14px;
  }

  &__muted {
    color: var(--el-text-color-placeholder);
  }
}
</style>
