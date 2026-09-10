<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsIcon, RsInputNumber, RsTooltip, useRsToast } from '@/ui'
import { setRequestTimeout } from '@/api/http'
import { getProps, saveProps, type SystemProps } from '../api'

const { t } = useI18n()
const toast = useRsToast()
const loading = ref(false)
const saving = ref(false)
const form = ref<SystemProps>({
  job_concurrency: 6,
  log_retention_days: 0,
  web_timeout_sec: 60,
})

const fields = [
  {
    key: 'job_concurrency' as const,
    icon: 'layers',
    label: 'settings.props.concurrency',
    tip: 'settings.props.concurrencyHint',
    unit: 'settings.props.concurrencyUnit',
    min: 1,
    max: 16,
    step: 1,
  },
  {
    key: 'log_retention_days' as const,
    icon: 'calendar-clock',
    label: 'settings.props.retention',
    tip: 'settings.props.retentionHint',
    unit: 'settings.props.retentionUnit',
    min: 0,
    max: 365,
    step: 1,
  },
  {
    key: 'web_timeout_sec' as const,
    icon: 'timer',
    label: 'settings.props.timeout',
    tip: 'settings.props.timeoutHint',
    unit: 'settings.props.timeoutUnit',
    min: 5,
    max: 600,
    step: 5,
  },
]

async function load(): Promise<void> {
  loading.value = true
  try {
    const got = await getProps()
    form.value = {
      job_concurrency: got.job_concurrency,
      log_retention_days: got.log_retention_days,
      web_timeout_sec: got.web_timeout_sec,
    }
    setRequestTimeout(got.web_timeout_sec)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (saving.value) return
  saving.value = true
  try {
    const got = await saveProps(form.value)
    form.value = got
    setRequestTimeout(got.web_timeout_sec)
    toast.success(t('common.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})

defineExpose({ reload: load, loading })
</script>

<template>
  <div class="props">
    <div class="props__bar">
      <div>
        <h2 class="props__title">{{ t('settings.props.title') }}</h2>
        <p class="props__desc">{{ t('settings.props.desc') }}</p>
      </div>
      <RsButton size="sm" :loading="saving" :disabled="loading || saving" @click="save">{{ t('common.save') }}</RsButton>
    </div>

    <div class="props__grid">
      <article v-for="field in fields" :key="field.key" class="prop">
        <div class="prop__label">
          <span class="prop__icon" aria-hidden="true">
            <RsIcon :name="field.icon" :size="15" />
          </span>
          <span>{{ t(field.label) }}</span>
          <RsTooltip :content="t(field.tip)" side="top">
            <button type="button" class="prop__tip" :aria-label="t(field.tip)">
              <RsIcon name="circle-help" :size="14" />
            </button>
          </RsTooltip>
        </div>
        <div class="prop__ctrl">
          <RsInputNumber
            v-model="form[field.key]"
            :min="field.min"
            :max="field.max"
            :step="field.step"
            :disabled="loading || saving"
          />
          <span class="prop__unit">{{ t(field.unit) }}</span>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.props {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.props__bar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.props__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 650;
  color: var(--shell-title);
}

.props__desc {
  margin: 0.35rem 0 0;
  max-width: 42rem;
  font-size: 0.8rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.props__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.prop {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  min-width: 0;
  padding: 0.95rem 1rem 1.05rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.75rem;
  background: var(--shell-inset-bg);
}

.prop__label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--shell-title);
  font-size: 0.82rem;
  font-weight: 600;
}

.prop__icon {
  display: inline-flex;
  color: #0284c7;
}

.prop__tip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--shell-muted);
  cursor: help;
}

.prop__ctrl {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.prop__ctrl :deep(.rs-input-number) {
  flex: 1;
  min-width: 0;
  max-width: 10rem;
}

.prop__unit {
  flex-shrink: 0;
  color: var(--shell-muted);
  font-size: 0.78rem;
}

@media (max-width: 860px) {
  .props__grid {
    grid-template-columns: 1fr;
  }
}
</style>
