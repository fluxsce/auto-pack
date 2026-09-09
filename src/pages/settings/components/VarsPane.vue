<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsEmpty, RsIcon, RsInput, RsSwitch, RsTag, rsConfirm, useRsToast } from '@/ui'
import { useWorkspaceStore } from '@/stores/workspace'
import { deleteVar, listVars, saveVar, type GlobalVar } from '../api'

const KEY_PAT = /^[A-Za-z_][A-Za-z0-9_]*$/

const { t } = useI18n()
const toast = useRsToast()
const ws = useWorkspaceStore()
const items = ref<GlobalVar[]>([])
const loading = ref(false)
const saving = ref('')
const adding = ref(false)
const draftKey = ref('')
const draftValue = ref('')
const edits = reactive<Record<string, string>>({})
const enabled = reactive<Record<string, boolean>>({})

function isVar(row: unknown): row is GlobalVar {
  if (!row || typeof row !== 'object') return false
  const key = String((row as GlobalVar).key ?? '').trim()
  return KEY_PAT.test(key)
}

function accept(rows: unknown): GlobalVar[] {
  if (!Array.isArray(rows)) return []
  return rows.filter(isVar)
}

async function load(): Promise<void> {
  if (!ws.currentId) return
  loading.value = true
  try {
    const rows = accept(await listVars(ws.currentId))
    items.value = rows
    for (const item of rows) {
      edits[item.key] = item.value
      enabled[item.key] = item.is_enabled !== 0
    }
  } catch (e) {
    items.value = []
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

async function persist(key: string, value: string, on: boolean): Promise<boolean> {
  if (!ws.currentId || saving.value) return false
  const name = key.trim()
  if (!KEY_PAT.test(name)) {
    toast.error(t('settings.vars.badKey'))
    return false
  }
  saving.value = name
  try {
    const saved = await saveVar(ws.currentId, { key: name, value, is_enabled: on ? 1 : 0 })
    edits[name] = saved.value
    enabled[name] = saved.is_enabled !== 0
    const idx = items.value.findIndex((row) => row.key === name)
    if (idx >= 0) items.value[idx] = saved
    else items.value = [...items.value, saved]
    toast.success(t('common.saved'))
    return true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
    return false
  } finally {
    saving.value = ''
  }
}

async function add(): Promise<void> {
  const name = draftKey.value.trim()
  if (!name || adding.value) return
  if (!KEY_PAT.test(name)) {
    toast.error(t('settings.vars.badKey'))
    return
  }
  adding.value = true
  try {
    if (await persist(name, draftValue.value, true)) {
      draftKey.value = ''
      draftValue.value = ''
    }
  } finally {
    adding.value = false
  }
}

function onEnabled(item: GlobalVar, value: unknown): void {
  const on = value === true || value === 1
  if (on === (enabled[item.key] !== false)) return
  const prev = enabled[item.key] !== false
  enabled[item.key] = on
  void persist(item.key, edits[item.key] ?? item.value, on).then((ok) => {
    if (!ok) enabled[item.key] = prev
  })
}

async function remove(key: string): Promise<void> {
  if (!ws.currentId || saving.value) return
  const ok = await rsConfirm.danger({
    title: t('common.delete'),
    description: t('settings.vars.deleteConfirm', { key }),
    confirmText: t('common.delete'),
  })
  if (!ok) return
  saving.value = key
  try {
    await deleteVar(ws.currentId, key)
    items.value = items.value.filter((row) => row.key !== key)
    delete edits[key]
    delete enabled[key]
    toast.success(t('common.deleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = ''
  }
}

onMounted(() => {
  void load()
})
watch(
  () => ws.currentId,
  () => {
    void load()
  },
)

defineExpose({ reload: load, loading })
</script>

<template>
  <div class="vars">
    <div class="vars__bar">
      <div>
        <h2 class="vars__title">{{ t('settings.vars.title') }}</h2>
        <p class="vars__desc">{{ t('settings.vars.desc') }}</p>
      </div>
    </div>

    <section class="vars__card">
      <form class="vars__form" @submit.prevent="add">
        <label class="vars__field">
          <span>{{ t('settings.vars.keyCol') }}</span>
          <RsInput v-model="draftKey" size="sm" :placeholder="t('settings.vars.keyPlaceholder')" :disabled="adding" />
        </label>
        <label class="vars__field vars__field--grow">
          <span>{{ t('settings.vars.valueCol') }}</span>
          <RsInput v-model="draftValue" size="sm" :placeholder="t('settings.vars.valuePlaceholder')" :disabled="adding" />
        </label>
        <RsButton size="sm" type="submit" :loading="adding" :disabled="adding || !draftKey.trim()">
          {{ t('common.add') }}
        </RsButton>
      </form>
    </section>

    <RsEmpty
      v-if="loading && !items.length"
      class="vars__state"
      :description="t('settings.vars.loading')"
    >
      <template #icon>
        <RsIcon name="loader-circle" :size="22" />
      </template>
    </RsEmpty>
    <RsEmpty v-else-if="!items.length" class="vars__state" :description="t('settings.vars.empty')">
      <template #icon>
        <RsIcon name="list" :size="22" />
      </template>
      <p class="vars__empty-hint">{{ t('settings.vars.emptyHint') }}</p>
    </RsEmpty>
    <section v-else class="vars__card vars__table">
      <div class="vars__head" aria-hidden="true">
        <span>{{ t('settings.vars.keyCol') }}</span>
        <span>{{ t('settings.vars.valueCol') }}</span>
        <span>{{ t('settings.vars.enabledCol') }}</span>
        <span>{{ t('common.actions') }}</span>
      </div>
      <ul class="vars__list">
        <li v-for="item in items" :key="item.rec_id || item.key" class="vars__row">
          <code class="vars__key" :title="item.key">{{ item.key }}</code>
          <RsInput
            :model-value="edits[item.key] ?? item.value"
            size="sm"
            :disabled="saving === item.key"
            @update:model-value="edits[item.key] = $event"
          />
          <div class="vars__on">
            <RsSwitch
              :model-value="enabled[item.key] !== false"
              size="sm"
              :disabled="!!saving || adding"
              @change="onEnabled(item, $event)"
            />
            <RsTag :variant="enabled[item.key] !== false ? 'success' : 'default'" round>
              {{ enabled[item.key] !== false ? t('envs.enabled') : t('envs.disabled') }}
            </RsTag>
          </div>
          <div class="vars__ops">
            <RsButton
              size="sm"
              variant="text"
              tone="primary"
              type="button"
              :loading="saving === item.key && !adding"
              :disabled="adding"
              @click="persist(item.key, edits[item.key] ?? item.value, enabled[item.key] !== false)"
            >
              {{ t('common.save') }}
            </RsButton>
            <RsButton
              size="sm"
              variant="text"
              tone="danger"
              type="button"
              :disabled="!!saving || adding"
              @click="remove(item.key)"
            >
              {{ t('common.delete') }}
            </RsButton>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.vars {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.vars__bar {
  flex-shrink: 0;
}

.vars__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 650;
  color: var(--shell-title);
}

.vars__desc {
  margin: 0.35rem 0 0;
  max-width: 42rem;
  font-size: 0.8rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.vars__card {
  border: 1px solid var(--shell-border);
  border-radius: 0.75rem;
  background: var(--shell-inset-bg);
}

.vars__form {
  display: grid;
  grid-template-columns: minmax(12rem, 16rem) minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: end;
  padding: 0.9rem 1rem;
}

.vars__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
}

.vars__field span {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--shell-muted);
}

.vars__state {
  min-height: 9rem;
}

.vars__empty-hint {
  margin: 0.4rem 0 0;
  font-size: 0.78rem;
  color: var(--shell-muted);
}

.vars__table {
  overflow: hidden;
}

.vars__head,
.vars__row {
  display: grid;
  grid-template-columns: minmax(10rem, 16rem) minmax(0, 1fr) 7.5rem 7.5rem;
  gap: 0.75rem;
  align-items: center;
}

.vars__head {
  padding: 0.55rem 1rem;
  border-bottom: 1px solid var(--shell-border);
  color: var(--shell-muted);
  font-size: 0.72rem;
  font-weight: 650;
  letter-spacing: 0.02em;
}

.vars__list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.vars__row {
  padding: 0.65rem 1rem;
  border-bottom: 1px solid color-mix(in srgb, var(--shell-border) 70%, transparent);
}

.vars__row:last-child {
  border-bottom: none;
}

.vars__key {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 0.28rem 0.5rem;
  border-radius: 0.4rem;
  background: color-mix(in srgb, #0ea5e9 10%, transparent);
  color: #0369a1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.78rem;
  font-weight: 600;
}

.vars__on,
.vars__ops {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.vars__ops {
  justify-content: flex-end;
}

@media (max-width: 860px) {
  .vars__form,
  .vars__head,
  .vars__row {
    grid-template-columns: 1fr;
  }

  .vars__head {
    display: none;
  }
}
</style>
