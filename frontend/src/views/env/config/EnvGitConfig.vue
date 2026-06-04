<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GitAuthType } from '../env-git'

const config = defineModel<Record<string, unknown>>('config', { required: true })
const { t } = useI18n()

const authType = computed({
  get: (): GitAuthType => {
    const v = String(config.value.auth_type || 'none').toLowerCase()
    if (v === 'https' || v === 'ssh') return v
    return 'none'
  },
  set: (v: GitAuthType) => {
    config.value.auth_type = v
  },
})
</script>

<template>
  <h4 class="env-git-config__subtitle">{{ $t('envGitSectionAuth') }}</h4>
  <p class="env-git-config__hint">{{ $t('envGitAuthDesc') }}</p>

  <el-form-item :label="$t('envGitAuthType')">
    <el-select v-model="authType" class="env-git-config__full">
      <el-option :label="$t('envGitAuthType_none')" value="none" />
      <el-option :label="$t('envGitAuthType_https')" value="https" />
      <el-option :label="$t('envGitAuthType_ssh')" value="ssh" />
    </el-select>
  </el-form-item>

  <template v-if="authType === 'https'">
    <el-form-item :label="$t('envGitAuthToken')">
      <el-input
        v-model="config.auth_token"
        type="password"
        show-password
        autocomplete="new-password"
        :placeholder="$t('envGitAuthTokenPlaceholder')"
      />
    </el-form-item>
    <el-form-item :label="$t('envGitAuthUsername')">
      <el-input v-model="config.auth_username" autocomplete="off" />
    </el-form-item>
    <el-form-item :label="$t('envGitAuthPassword')">
      <el-input
        v-model="config.auth_password"
        type="password"
        show-password
        autocomplete="new-password"
        :placeholder="$t('envSecretPlaceholder')"
      />
    </el-form-item>
    <p class="env-git-config__hint">{{ $t('envGitAuthHttpsHint') }}</p>
  </template>

  <template v-else-if="authType === 'ssh'">
    <el-form-item :label="$t('envGitPrivateKeyPath')">
      <el-input v-model="config.private_key_path" :placeholder="$t('envGitPrivateKeyPlaceholder')" />
    </el-form-item>
    <el-form-item :label="$t('envGitPrivateKeyPassphrase')">
      <el-input
        v-model="config.private_key_passphrase"
        type="password"
        show-password
        autocomplete="new-password"
        :placeholder="$t('envSecretPlaceholder')"
      />
    </el-form-item>
  </template>

  <h4 class="env-git-config__subtitle env-git-config__subtitle--spaced">{{ $t('envGitSectionRuntime') }}</h4>
  <el-form-item :label="$t('envGitPath')">
    <el-input v-model="config.git_path" :placeholder="$t('envGitPathPlaceholder')" />
  </el-form-item>

  <h4 class="env-git-config__subtitle env-git-config__subtitle--spaced">{{ $t('envGitSectionCommit') }}</h4>
  <p class="env-git-config__hint">{{ $t('envGitCommitDesc') }}</p>
  <el-form-item :label="$t('envGitCommitName')">
    <el-input v-model="config.username" />
  </el-form-item>
  <el-form-item :label="$t('envGitCommitEmail')">
    <el-input v-model="config.email" />
  </el-form-item>
</template>

<style scoped lang="scss">
.env-git-config {
  &__subtitle {
    margin: 0 0 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));

    &--spaced {
      margin-top: 20px;
    }
  }

  &__hint {
    margin: 0 0 12px;
    font-size: 12px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__full {
    width: 100%;
  }
}
</style>
