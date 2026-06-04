<script setup lang="ts">
import type { EnvProbeResult, EnvSettingPayload, EnvType } from '@/types'
import EnvConfigFields from './config/EnvConfigFields.vue'
import { envTypeTagClass, supportsEnvVerify } from './env-meta'

defineProps<{
  title: string
  editingId: string | null
  saving: boolean
  probing: boolean
  envTypes: EnvType[]
  form: EnvSettingPayload
  probeResult: EnvProbeResult | null
  typeLabel: (type: unknown) => string
}>()

const emit = defineEmits<{
  save: []
  back: []
  probe: []
  typeChange: [type: EnvType]
}>()

const envType = defineModel<EnvType>('envType', { required: true })
const config = defineModel<Record<string, unknown>>('config', { required: true })
</script>

<template>
  <el-card class="env-editor env-panel" shadow="never">
    <template #header>
      <div class="env-editor__head">
        <h2 class="env-editor__title">{{ title }}</h2>
        <span :class="envTypeTagClass(envType)">
          {{ typeLabel(envType) }}
        </span>
      </div>
    </template>

    <div v-if="probeResult" class="env-editor__probe-wrap">
      <el-alert
        class="env-editor__probe"
        :type="probeResult.ok ? 'success' : 'error'"
        :title="probeResult.ok ? $t('envVerifyOk') : $t('envVerifyFail')"
        :description="probeResult.message"
        show-icon
        :closable="true"
      />
    </div>

    <el-form
      label-width="112px"
      label-position="right"
      class="env-editor__form"
      @submit.prevent="emit('save')"
    >
      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ $t('envSectionBasic') }}</h3>
        <div class="env-editor__fields">
          <el-form-item :label="$t('envColName')" required>
            <el-input v-model="form.env_name" />
          </el-form-item>
          <el-form-item :label="$t('envColType')" required>
            <el-select
              v-model="envType"
              :disabled="!!editingId"
              class="env-editor__full"
              @change="emit('typeChange', envType)"
            >
              <el-option v-for="tp in envTypes" :key="tp" :label="typeLabel(tp)" :value="tp" />
            </el-select>
          </el-form-item>
        </div>
      </section>

      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ $t('envSectionConfig') }}</h3>
        <div class="env-editor__config-box">
          <EnvConfigFields v-model:env-type="envType" v-model:config="config" />
        </div>
      </section>

      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ $t('envSectionMeta') }}</h3>
        <div class="env-editor__meta-grid">
          <el-form-item :label="$t('envColDefault')">
            <el-switch v-model="form.is_default" :active-value="1" :inactive-value="0" />
          </el-form-item>
          <el-form-item :label="$t('envColEnabled')">
            <el-switch v-model="form.is_enabled" :active-value="1" :inactive-value="0" />
          </el-form-item>
          <el-form-item :label="$t('envColSort')">
            <el-input-number v-model="form.sort_num" :min="0" class="env-editor__full" />
          </el-form-item>
        </div>
        <el-form-item :label="$t('envColRemark')" class="env-editor__remark">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </section>
    </el-form>

    <footer class="env-editor__footer">
      <el-button
        v-if="supportsEnvVerify(envType)"
        :loading="probing"
        @click="emit('probe')"
      >
        {{ $t('envTestConnection') }}
      </el-button>
      <el-button type="primary" :loading="saving" @click="emit('save')">
        {{ $t('save') }}
      </el-button>
    </footer>
  </el-card>
</template>

<style scoped lang="scss">
.env-editor {
  width: 100%;
  overflow: hidden;

  :deep(.el-card__header) {
    padding: 16px 20px;
  }

  :deep(.el-card__body) {
    padding: 0;
  }

  &__head {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px 12px;
  }

  &__title {
    margin: 0;
    font-size: 17px;
    font-weight: 600;
    line-height: 1.4;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__probe-wrap {
    padding: 16px 20px 0;
    box-sizing: border-box;
  }

  &__probe {
    width: 100%;
    box-sizing: border-box;
    align-items: flex-start;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;

    :deep(.el-alert__content) {
      flex: 1;
      min-width: 0;
    }

    :deep(.el-alert__description) {
      white-space: pre-wrap;
      word-break: break-word;
      overflow-wrap: anywhere;
    }
  }

  &__form {
    width: 100%;
    padding: 8px 20px 4px;
    box-sizing: border-box;
  }

  &__section {
    padding: 16px 0;

    & + & {
      border-top: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
    }
  }

  &__section-title {
    margin: 0 0 14px;
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.02em;
    text-transform: none;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__fields {
    width: 100%;
  }

  &__config-box {
    width: 100%;
    box-sizing: border-box;
    padding: 16px 16px 4px;
    border-radius: 12px;
    border: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
    background: var(--env-panel-inset-bg, var(--el-fill-color-lighter));
  }

  &__meta-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0 24px;
    width: 100%;
  }

  &__remark {
    width: 100%;
    margin-top: 4px;
  }

  &__full {
    width: 100%;
  }

  &__footer {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
    padding: 14px 20px;
    border-top: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
    background: var(--env-panel-footer-bg, var(--el-fill-color-lighter));
  }

  :deep(.el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  :deep(.el-form-item__label) {
    font-weight: 500;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));
  }

  :deep(.el-form-item__content) {
    flex: 1;
    min-width: 0;
    max-width: none;
  }

  :deep(.el-input),
  :deep(.el-select),
  :deep(.el-textarea) {
    width: 100%;
  }

  :deep(.el-input__wrapper),
  :deep(.el-textarea__inner),
  :deep(.el-select__wrapper) {
    background-color: var(--env-panel-input-bg, var(--el-fill-color-blank));
    box-shadow: 0 0 0 1px var(--env-panel-input-border, var(--el-border-color)) inset;
  }

  :deep(.el-input__wrapper:hover),
  :deep(.el-textarea__inner:hover),
  :deep(.el-select__wrapper:hover) {
    box-shadow: 0 0 0 1px var(--shell-accent, var(--el-color-primary)) inset;
  }
}

@media (max-width: 720px) {
  .env-editor__meta-grid {
    grid-template-columns: 1fr;
  }
}
</style>
