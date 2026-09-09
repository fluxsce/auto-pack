<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import { fetchNpmrcPaths } from '@/request/api/env'

const config = defineModel<Record<string, unknown>>('config', { required: true })
const { t } = useI18n()
const message = useMessage()

const candidates = ref<string[]>([])
const loadingCandidates = ref(false)

const npmrcPath = computed({
  get: () => String(config.value.npmrc_path ?? ''),
  set: (v: string) => {
    config.value.npmrc_path = v
  },
})

const allowAllBuilds = computed({
  get: () => config.value.pnpm_allow_all_builds !== false,
  set: (v: boolean) => {
    config.value.pnpm_allow_all_builds = v
  },
})

async function loadCandidates(silent = false): Promise<void> {
  loadingCandidates.value = true
  try {
    const res = await fetchNpmrcPaths()
    candidates.value = res.items ?? []
  } catch (e) {
    candidates.value = []
    if (!silent) {
      message.error((e as Error).message)
    }
  } finally {
    loadingCandidates.value = false
  }
}

watch(
  () => config.value.npmrc_path,
  () => {
    void loadCandidates(true)
  },
  { immediate: true },
)
</script>

<template>
  <el-form-item label="node path">
    <el-input v-model="config.node_path" placeholder="留空用 PATH；或 node.exe / Node 安装目录" />
  </el-form-item>
  <el-form-item label="npm path">
    <el-input v-model="config.npm_path" placeholder="npm" />
  </el-form-item>
  <el-form-item label="npx path">
    <el-input v-model="config.npx_path" placeholder="npx" />
  </el-form-item>

  <el-form-item :label="$t('envNodejsPnpmAllowBuilds')">
    <el-switch v-model="allowAllBuilds" />
    <p class="env-nodejs-config__hint">{{ $t('envNodejsPnpmAllowBuildsHint') }}</p>
  </el-form-item>

  <h4 class="env-nodejs-config__subtitle">{{ $t('envNodejsRegistrySection') }}</h4>
  <p class="env-nodejs-config__hint">{{ $t('envNodejsRegistryDesc') }}</p>

  <el-form-item :label="$t('envNodejsNpmrcPath')">
    <div class="env-nodejs-config__npmrc">
      <el-select
        v-model="npmrcPath"
        filterable
        allow-create
        clearable
        default-first-option
        :loading="loadingCandidates"
        :placeholder="$t('envNodejsNpmrcPathPlaceholder')"
        class="env-nodejs-config__npmrc-select"
      >
        <el-option v-for="p in candidates" :key="p" :label="p" :value="p" />
      </el-select>
      <el-button
        :loading="loadingCandidates"
        :title="$t('envNodejsNpmrcScan')"
        @click="loadCandidates(false)"
      >
        <el-icon><Refresh /></el-icon>
      </el-button>
    </div>
    <p class="env-nodejs-config__hint">{{ $t('envNodejsNpmrcPathHint') }}</p>
  </el-form-item>

  <el-form-item :label="$t('envNodejsRegistry')">
    <el-input
      v-model="config.registry"
      :placeholder="$t('envNodejsRegistryPlaceholder')"
    />
  </el-form-item>
  <el-form-item :label="$t('envGitAuthToken')">
    <el-input
      v-model="config.auth_token"
      type="password"
      show-password
      autocomplete="new-password"
      :placeholder="$t('envNodejsAuthTokenPlaceholder')"
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
  <p class="env-nodejs-config__hint">{{ $t('envNodejsRegistryAuthHint') }}</p>
</template>

<style scoped lang="scss">
.env-nodejs-config {
  &__subtitle {
    margin: 16px 0 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--shell-content-text, var(--el-text-color-primary));
  }

  &__hint {
    margin: 6px 0 0;
    width: 100%;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__npmrc {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  &__npmrc-select {
    flex: 1;
    min-width: 0;
  }
}
</style>
