<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import EnvSettingsEditor from './EnvSettingsEditor.vue'
import EnvSettingsList from './EnvSettingsList.vue'
import { useEnvSettings } from './useEnvSettings'

const {
  envTypes,
  activeType,
  loading,
  editorOpen,
  saving,
  probing,
  verifyingId,
  editingId,
  probeResult,
  form,
  configObj,
  filteredItems,
  editorTitle,
  typeLabel,
  openCreate,
  openEdit,
  closeEditor,
  onTypeChange,
  onSave,
  onDelete,
  onVerifyRow,
  onProbeDraft,
} = useEnvSettings()
</script>

<template>
  <div class="env-settings">
    <header class="env-settings__head">
      <div class="env-settings__intro">
        <h1 class="env-settings__title">{{ $t('envPageTitle') }}</h1>
        <p class="env-settings__desc">{{ $t('envPageDesc') }}</p>
      </div>
      <div class="env-settings__actions">
        <el-button v-if="!editorOpen" type="primary" @click="openCreate">
          <el-icon class="env-settings__btn-icon"><Plus /></el-icon>
          {{ $t('envAdd') }}
        </el-button>
        <el-button v-else text type="primary" @click="closeEditor">{{ $t('back') }}</el-button>
      </div>
    </header>

    <EnvSettingsList
      v-if="!editorOpen"
      v-model:active-type="activeType"
      :loading="loading"
      :items="filteredItems"
      :env-types="envTypes"
      :verifying-id="verifyingId"
      :type-label="typeLabel"
      @edit="openEdit"
      @verify="onVerifyRow"
      @delete="onDelete"
    />

    <div v-else class="env-settings__editor-wrap">
      <EnvSettingsEditor
        v-model:env-type="form.env_type"
        v-model:config="configObj"
        :title="editorTitle"
        :editing-id="editingId"
        :saving="saving"
        :probing="probing"
        :env-types="envTypes"
        :form="form"
        :probe-result="probeResult"
        :type-label="typeLabel"
        @save="onSave"
        @back="closeEditor"
        @probe="onProbeDraft"
        @type-change="onTypeChange"
      />
    </div>
  </div>
</template>

<style scoped lang="scss">
.env-settings {
  &__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 20px;
  }

  &__intro {
    flex: 1;
    min-width: 0;
  }

  &__title {
    margin: 0 0 6px;
    font-size: 22px;
    font-weight: 600;
    line-height: 1.35;
    letter-spacing: -0.02em;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__desc {
    margin: 0;
    max-width: 52em;
    font-size: 14px;
    line-height: 1.65;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__actions {
    display: flex;
    flex-shrink: 0;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 2px;
  }

  &__btn-icon {
    margin-right: 4px;
  }

  &__editor-wrap {
    width: 100%;
    max-width: var(--app-content-max-width, 1200px);
    margin-inline: auto;
  }
}

@media (max-width: 640px) {
  .env-settings__head {
    flex-direction: column;
    align-items: stretch;
  }

  .env-settings__actions {
    padding-top: 0;
    width: 100%;

    .el-button {
      flex: 1;
      min-width: 0;
    }
  }
}
</style>
