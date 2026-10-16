<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsAlert, RsButton, RsFormItem, RsInput, useRsToast } from '@/ui'
import { getUpgradeStatus, upgradeServer, type UpgradeStatus } from '../api'

const { t } = useI18n()
const toast = useRsToast()

const status = ref<UpgradeStatus | null>(null)
const loading = ref(true)
const busy = ref(false)
const file = ref<File | null>(null)
const sha256 = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const ready = computed(
  () =>
    !!status.value?.upgradable &&
    !!file.value &&
    /^[0-9a-fA-F]{64}$/.test(sha256.value.trim()) &&
    !busy.value,
)

async function load(): Promise<void> {
  loading.value = true
  try {
    status.value = await getUpgradeStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function onPick(): void {
  fileInput.value?.click()
}

function onFile(ev: Event): void {
  const input = ev.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

async function onSubmit(): Promise<void> {
  if (!file.value) {
    toast.error(t('settings.upgrade.needFile'))
    return
  }
  if (!/^[0-9a-fA-F]{64}$/.test(sha256.value.trim())) {
    toast.error(t('settings.upgrade.needSha'))
    return
  }
  busy.value = true
  try {
    const res = await upgradeServer(file.value, sha256.value.trim())
    toast.success(res.message || t('settings.upgrade.ok'))
    file.value = null
    sha256.value = ''
    if (fileInput.value) fileInput.value.value = ''
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
    busy.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="upgrade">
    <header class="upgrade__head">
      <h2 class="upgrade__title">{{ t('settings.upgrade.title') }}</h2>
      <p class="upgrade__desc">{{ t('settings.upgrade.desc') }}</p>
    </header>

    <RsAlert v-if="loading" type="info" :title="t('settings.upgrade.loading')" />
    <template v-else-if="status">
      <div class="upgrade__meta">
        <div>
          <span class="upgrade__label">{{ t('settings.upgrade.current') }}</span>
          <strong>{{ status.version || t('common.dash') }}</strong>
        </div>
        <div>
          <span class="upgrade__label">{{ t('settings.upgrade.platform') }}</span>
          <strong>{{ status.goos }}/{{ status.goarch }}</strong>
        </div>
      </div>

      <RsAlert
        class="upgrade__hint"
        :type="status.upgradable ? 'info' : 'warning'"
        :title="status.upgradable ? t('settings.upgrade.ready') : t('settings.upgrade.blocked')"
      >
        {{ status.message }}
      </RsAlert>

      <div v-if="status.upgradable" class="upgrade__form">
        <RsFormItem :label="t('settings.upgrade.file')" required>
          <div class="upgrade__file">
            <RsButton variant="ghost" size="sm" :disabled="busy" @click="onPick">
              {{ file ? file.name : t('settings.upgrade.pickFile') }}
            </RsButton>
            <span class="upgrade__file-hint">{{ t('settings.upgrade.fileHint') }}</span>
          </div>
          <input
            ref="fileInput"
            type="file"
            class="upgrade__input"
            accept=".tar.gz,.tgz,.zip,application/gzip,application/zip"
            @change="onFile"
          />
        </RsFormItem>
        <RsFormItem :label="t('settings.upgrade.sha')" required>
          <RsInput
            v-model="sha256"
            :placeholder="t('settings.upgrade.shaHint')"
            :disabled="busy"
            spellcheck="false"
          />
        </RsFormItem>
        <div class="upgrade__actions">
          <RsButton :disabled="!ready" :loading="busy" @click="onSubmit">
            {{ t('settings.upgrade.submit') }}
          </RsButton>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.upgrade {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 40rem;
}

.upgrade__head {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.upgrade__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 650;
  color: var(--shell-title);
}

.upgrade__desc {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.upgrade__meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem 1.25rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.75rem;
  background: var(--shell-inset-bg);
}

.upgrade__label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.75rem;
  color: var(--shell-muted);
}

.upgrade__hint {
  margin: 0;
}

.upgrade__form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.upgrade__file {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.65rem;
}

.upgrade__file-hint {
  font-size: 0.78rem;
  color: var(--shell-muted);
}

.upgrade__input {
  display: none;
}

.upgrade__actions {
  display: flex;
  justify-content: flex-start;
}

@media (max-width: 640px) {
  .upgrade__meta {
    grid-template-columns: 1fr;
  }
}
</style>
