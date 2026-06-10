<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh } from '@element-plus/icons-vue'
import { useMessage } from '@/components/feedback/message'
import { fetchMavenSettingsPaths } from '@/request/api/env'

const config = defineModel<Record<string, unknown>>('config', { required: true })

const { t } = useI18n()
const message = useMessage()

const candidates = ref<string[]>([])
const loadingCandidates = ref(false)

const settingsPath = computed({
  get: () => String(config.value.settings_xml ?? ''),
  set: (v: string) => {
    config.value.settings_xml = v
  },
})

const mavenHome = computed(() => String(config.value.maven_home ?? ''))

async function loadCandidates(silent = false): Promise<void> {
  loadingCandidates.value = true
  try {
    const res = await fetchMavenSettingsPaths(mavenHome.value.trim())
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
  () => config.value.maven_home,
  () => {
    void loadCandidates(true)
  },
  { immediate: true },
)
</script>

<template>
  <el-form-item label="Maven home">
    <el-input v-model="config.maven_home" placeholder="可选" />
  </el-form-item>
  <el-form-item label="mvn path">
    <el-input v-model="config.mvn_path" placeholder="mvn 或绝对路径" />
  </el-form-item>
  <el-form-item :label="$t('envMavenSettingsXml')">
    <div class="env-maven-config__settings">
      <el-select
        v-model="settingsPath"
        filterable
        allow-create
        clearable
        default-first-option
        :loading="loadingCandidates"
        :placeholder="$t('envMavenSettingsXmlPlaceholder')"
        class="env-maven-config__settings-select"
      >
        <el-option v-for="p in candidates" :key="p" :label="p" :value="p" />
      </el-select>
      <el-button
        :loading="loadingCandidates"
        :title="$t('envMavenSettingsScan')"
        @click="loadCandidates(false)"
      >
        <el-icon><Refresh /></el-icon>
      </el-button>
    </div>
    <p class="env-maven-config__hint">{{ $t('envMavenSettingsXmlHint') }}</p>
  </el-form-item>
  <el-form-item label="JAVA_HOME">
    <el-input v-model="config.java_home" placeholder="Maven 使用的 JDK" />
  </el-form-item>
</template>

<style scoped lang="scss">
.env-maven-config {
  &__settings {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  &__settings-select {
    flex: 1;
    min-width: 0;
  }

  &__hint {
    margin: 6px 0 0;
    width: 100%;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }
}
</style>
