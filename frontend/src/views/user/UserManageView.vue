<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import { useUsers } from './useUsers'
import { formatBuildTime } from '../project/project-format'

const {
  items,
  loading,
  saving,
  editorOpen,
  editorTitle,
  isCreateMode,
  canDeleteEditing,
  resetPassword,
  form,
  openCreate,
  openEdit,
  closeEditor,
  onSave,
  onResetPassword,
  onDelete,
  onDeleteEditing,
} = useUsers()
</script>

<template>
  <div class="user-manage">
    <header class="user-manage__head">
      <div class="user-manage__intro">
        <h1 class="user-manage__title">{{ $t('userPageTitle') }}</h1>
        <p class="user-manage__desc">{{ $t('userPageDesc') }}</p>
      </div>
      <div class="user-manage__actions">
        <el-button v-if="!editorOpen" type="primary" @click="openCreate">
          <el-icon class="user-manage__btn-icon"><Plus /></el-icon>
          {{ $t('userAdd') }}
        </el-button>
        <el-button v-else text type="primary" @click="closeEditor">{{ $t('back') }}</el-button>
      </div>
    </header>

    <el-card v-if="!editorOpen" class="user-manage__card env-panel" shadow="never" v-loading="loading">
      <el-table :data="items" row-key="rec_id" class="user-manage__table">
        <el-table-column prop="login_name" :label="$t('userColLoginName')" min-width="120" />
        <el-table-column :label="$t('userColEnabled')" width="96" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_enabled === 1 ? 'success' : 'info'" size="small" effect="plain">
              {{ row.is_enabled === 1 ? $t('enabled') : $t('disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="$t('envColRemark')" min-width="140" show-overflow-tooltip />
        <el-table-column :label="$t('userColUpdated')" width="168">
          <template #default="{ row }">
            {{ formatBuildTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('projectColActions')" width="160" align="right" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">{{ $t('edit') }}</el-button>
            <AppPopconfirm :title="$t('userDeleteConfirm')" @confirm="onDelete(row)">
              <template #reference>
                <el-button link type="danger" @click.stop>{{ $t('delete') }}</el-button>
              </template>
            </AppPopconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && items.length === 0" :description="$t('userListEmpty')" />
    </el-card>

    <el-card v-else class="user-manage__card env-panel" shadow="never">
      <h3 class="user-manage__form-title">{{ editorTitle }}</h3>
      <el-form label-width="108px" class="user-manage__form" @submit.prevent="onSave">
        <el-form-item :label="$t('userColLoginName')" required>
          <el-input v-model="form.login_name" autocomplete="off" />
        </el-form-item>
        <el-form-item v-if="isCreateMode" :label="$t('userColPassword')" required>
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item :label="$t('envColEnabled')">
          <el-switch v-model="form.is_enabled" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-form-item :label="$t('envColRemark')">
          <el-input v-model="form.remark" type="textarea" :rows="3" />
        </el-form-item>
        <template v-if="!isCreateMode">
          <el-divider />
          <el-form-item :label="$t('userResetPassword')">
            <el-input
              v-model="resetPassword"
              type="password"
              show-password
              autocomplete="new-password"
              :placeholder="$t('userResetPasswordPlaceholder')"
            />
          </el-form-item>
          <el-form-item>
            <el-button :loading="saving" @click="onResetPassword">{{ $t('userResetPasswordConfirm') }}</el-button>
          </el-form-item>
        </template>
        <div class="user-manage__form-actions">
          <AppPopconfirm
            v-if="!isCreateMode && canDeleteEditing"
            :title="$t('userDeleteConfirm')"
            @confirm="onDeleteEditing"
          >
            <template #reference>
              <el-button type="danger" native-type="button">{{ $t('delete') }}</el-button>
            </template>
          </AppPopconfirm>
          <el-button type="primary" :loading="saving" native-type="submit">{{ $t('save') }}</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
.user-manage {
  &__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
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

  &__btn-icon {
    margin-right: 4px;
  }

  &__card {
    border-radius: 16px;
  }

  &__form-title {
    margin: 0 0 16px;
    font-size: 16px;
    font-weight: 600;
  }

  &__form {
    max-width: 480px;
  }

  &__form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
  }
}
</style>
