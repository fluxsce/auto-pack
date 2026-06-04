<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EnvType } from '@/types'
import { envCategoryOf, isRemoteEnvType, supportsEnvVerify } from '../env-meta'
import EnvGitConfig from './EnvGitConfig.vue'
import EnvJdkConfig from './EnvJdkConfig.vue'
import EnvMavenConfig from './EnvMavenConfig.vue'
import EnvNodejsConfig from './EnvNodejsConfig.vue'
import EnvRemoteConfig from './EnvRemoteConfig.vue'
import EnvSmtpConfig from './EnvSmtpConfig.vue'

const envType = defineModel<EnvType>('envType', { required: true })
const config = defineModel<Record<string, unknown>>('config', { required: true })

const { t } = useI18n()

const categoryLabel = computed(() => {
  const key = envCategoryOf(envType.value)
  if (key === 'remote') return t('envCategoryRemote')
  if (key === 'credential') return t('envCategoryCredential')
  if (key === 'notification') return t('envCategoryNotification')
  return t('envCategoryToolchain')
})

const showVerifyHint = computed(() => supportsEnvVerify(envType.value))
</script>

<template>
  <div class="env-config-fields">
    <div class="env-config-fields__head">
      <span class="env-config-fields__category">{{ categoryLabel }}</span>
      <el-tag v-if="showVerifyHint" type="info" size="small">{{ t('envSupportsVerify') }}</el-tag>
    </div>

    <EnvRemoteConfig v-if="isRemoteEnvType(envType)" v-model:env-type="envType" v-model:config="config" />
    <EnvGitConfig v-else-if="envType === 'git'" v-model:config="config" />
    <EnvMavenConfig v-else-if="envType === 'maven'" v-model:config="config" />
    <EnvNodejsConfig v-else-if="envType === 'nodejs'" v-model:config="config" />
    <EnvJdkConfig v-else-if="envType === 'jdk'" v-model:config="config" />
    <EnvSmtpConfig v-else-if="envType === 'smtp'" v-model:config="config" />
  </div>
</template>

<style scoped lang="scss">
.env-config-fields {
  &__head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 14px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--env-panel-border, var(--el-border-color-lighter));
  }

  &__category {
    font-size: 14px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  :deep(.el-form-item) {
    margin-bottom: 16px;
  }

  :deep(.el-form-item__label) {
    font-weight: 500;
  }
}
</style>
