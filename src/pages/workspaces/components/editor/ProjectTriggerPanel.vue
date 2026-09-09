<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsFormItem, RsInput, RsSelect, type RsSelectModelValue } from '@/ui'
import { asSelectStrings, previewCron, type CronPreview } from '../../api'
import type { CronValidity } from '../../lib/cron'
import type { ProjectRecipe } from '../../lib/project-defaults'

const recipe = defineModel<ProjectRecipe>('recipe', { required: true })
const form = defineModel<{ cron_expr: string; is_enabled: number }>('form', { required: true })

const props = defineProps<{
  cronState: CronValidity
  projectOptions: { value: string; label: string }[]
}>()

const { t } = useI18n()
const preview = ref<CronPreview | null>(null)
const previewing = ref(false)
let timer = 0

const presets = [
  { expr: '0 * * * *', key: 'hourly' },
  { expr: '0 2 * * *', key: 'daily' },
  { expr: '0 9 * * 1', key: 'weekly' },
  { expr: '0 3 1 * *', key: 'monthly' },
  { expr: '0 18 * * 1-5', key: 'weekdays' },
]

watch(
  () => form.value.cron_expr,
  (expr) => {
    window.clearTimeout(timer)
    const value = expr.trim()
    if (!value) {
      preview.value = null
      previewing.value = false
      return
    }
    timer = window.setTimeout(() => {
      void loadPreview(value)
    }, 350)
  },
  { immediate: true },
)

onUnmounted(() => {
  window.clearTimeout(timer)
})

async function loadPreview(expr: string): Promise<void> {
  previewing.value = true
  try {
    preview.value = await previewCron(expr)
  } catch (e) {
    preview.value = { expr, ok: false, error: e instanceof Error ? e.message : String(e) }
  } finally {
    previewing.value = false
  }
}

function applyPreset(expr: string): void {
  form.value.cron_expr = expr
}

function cronParts(): string[] {
  const parts = form.value.cron_expr.trim().split(/\s+/)
  return parts[0]?.startsWith('@') ? [] : parts
}

function formatTime(iso: string): string {
  const ms = Date.parse(iso)
  if (Number.isNaN(ms)) return iso
  return new Date(ms).toLocaleString()
}

function onDepends(value: RsSelectModelValue): void {
  recipe.value.depends_on = asSelectStrings(value)
}
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ t('workspaces.nav.trigger') }}</h3>
      <p>{{ t('workspaces.sectionTriggerDesc') }}</p>
    </header>

    <RsInput v-model="form.cron_expr" :label="t('workspaces.cron')" :placeholder="t('workspaces.cronHint')" />
    <div class="tags">
      <button v-for="item in presets" :key="item.expr" type="button" class="tag" @click="applyPreset(item.expr)">
        {{ t(`workspaces.cronPresets.${item.key}`) }}
      </button>
    </div>
    <p class="hint">{{ t('workspaces.cronFieldsHint') }}</p>
    <div v-if="cronParts().length === 5" class="cron-fields">
      <span v-for="(part, i) in cronParts()" :key="i">
        <em>{{ t(`workspaces.cronFields.${i}`) }}</em>
        <code>{{ part }}</code>
      </span>
    </div>

    <p v-if="!form.cron_expr.trim()" class="hint">{{ t('workspaces.cronEmptyHint') }}</p>
    <p v-else-if="previewing" class="hint">{{ t('workspaces.cronPreviewing') }}</p>
    <p v-else-if="preview && !preview.ok" class="hint hint--warn">{{ preview.error || t('workspaces.cronInvalid') }}</p>
    <p v-else-if="props.cronState === 'bad' && !preview" class="hint hint--warn">{{ t('workspaces.cronInvalid') }}</p>
    <p v-if="form.is_enabled === 0 && form.cron_expr.trim()" class="hint hint--warn">{{ t('workspaces.cronDisabledHint') }}</p>

    <div v-if="preview?.ok && preview.times?.length" class="path cron-next">
      <span>{{ t('workspaces.cronNext', { tz: preview.tz || '' }) }}</span>
      <ol>
        <li v-for="(item, i) in preview.times" :key="item">
          <em>{{ t('workspaces.cronNextN', { n: i + 1 }) }}</em>
          <code>{{ formatTime(item) }}</code>
        </li>
      </ol>
    </div>

    <RsFormItem :label="t('workspaces.dependsOn')">
      <RsSelect
        multiple
        searchable
        :model-value="recipe.depends_on ?? []"
        :options="projectOptions"
        clearable
        :placeholder="t('workspaces.dependsOnPh')"
        :search-placeholder="t('workspaces.dependsOnSearch')"
        @update:model-value="onDepends"
      />
    </RsFormItem>
    <p class="hint">{{ t('workspaces.dependsOnHint') }}</p>
  </section>
</template>

<style scoped src="./panel.css"></style>
<style scoped>
.cron-fields {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.45rem;
}

.cron-fields span {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.4rem 0.45rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.55rem;
  background: var(--shell-inset-bg);
}

.cron-fields em {
  font-size: 0.7rem;
  font-style: normal;
  color: var(--shell-muted);
}

.cron-fields code {
  font-family: var(--rs-font-mono);
  font-size: 0.82rem;
  color: var(--shell-title);
}

.cron-next ol {
  margin: 0.45rem 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cron-next li {
  display: flex;
  align-items: baseline;
  gap: 0.55rem;
}

.cron-next em {
  flex-shrink: 0;
  width: 3.4rem;
  font-size: 0.72rem;
  font-style: normal;
  color: var(--shell-muted);
}

@media (max-width: 720px) {
  .cron-fields {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
