<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsTable, copyTextToClipboard, useRsToast, type RsTableColumn } from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

type OSSItem = {
  name: string
  key: string
  dir: boolean
  size?: number
  modified?: string
}

type OSSList = {
  ok: boolean
  message?: string
  bucket?: string
  prefix: string
  next_marker?: string
  truncated?: boolean
  items?: OSSItem[]
}

type OSSSign = {
  ok: boolean
  message?: string
  url?: string
  expires_at?: string
}

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ recId: string; envName: string }>()

const { t } = useI18n()
const toast = useRsToast()
const prefix = ref('')
const items = ref<OSSItem[]>([])
const marker = ref('')
const truncated = ref(false)
const bucket = ref('')
const loading = ref(false)
const failed = ref('')
const signing = ref('')
const signedURL = ref('')
const signedUntil = ref('')

const title = computed(() =>
  props.envName ? t('envs.ossBrowseTitleNamed', { name: props.envName }) : t('envs.ossBrowseTitle'),
)

const crumbs = computed(() => {
  const parts = prefix.value.split('/').filter(Boolean)
  const out: { label: string; prefix: string }[] = [{ label: bucket.value || t('envs.ossRoot'), prefix: '' }]
  let acc = ''
  for (const part of parts) {
    acc += part + '/'
    out.push({ label: part, prefix: acc })
  }
  return out
})

const columns = computed<RsTableColumn<OSSItem>[]>(() => [
  { key: 'name', title: t('envs.ossColName'), minWidth: 220 },
  { key: 'size', title: t('envs.ossColSize'), width: 110 },
  { key: 'modified', title: t('envs.ossColModified'), width: 200 },
  { key: 'actions', title: t('common.actions'), width: 240, align: 'right' },
])

function formatSize(row: OSSItem): string {
  if (row.dir) return t('envs.ossDir')
  const n = row.size ?? 0
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatTime(value?: string): string {
  if (!value) return t('common.dash')
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function load(nextPrefix: string, append: boolean): Promise<void> {
  if (!props.recId) return
  loading.value = true
  try {
    const out = await request<OSSList>({
      method: 'GET',
      url: apiPath('envs', props.recId, 'oss-objects'),
      params: { prefix: nextPrefix, marker: append ? marker.value : '' },
    })
    if (!out.ok) {
      failed.value = out.message || t('envs.ossListFail')
      if (!append) items.value = []
      toast.error(failed.value)
      return
    }
    failed.value = ''
    prefix.value = out.prefix || ''
    bucket.value = out.bucket || bucket.value
    marker.value = out.next_marker || ''
    truncated.value = !!out.truncated
    items.value = append ? items.value.concat(out.items ?? []) : (out.items ?? [])
    if (!append) {
      signedURL.value = ''
      signedUntil.value = ''
    }
  } catch (e) {
    failed.value = e instanceof Error ? e.message : String(e)
    if (!append) items.value = []
    toast.error(failed.value)
  } finally {
    loading.value = false
  }
}

async function sign(row: OSSItem, mode: 'view' | 'download' | 'link'): Promise<void> {
  if (!props.recId || row.dir) return
  signing.value = row.key + mode
  try {
    const out = await request<OSSSign>({
      method: 'POST',
      url: apiPath('envs', props.recId, 'oss-sign'),
      data: { key: row.key, mode },
    })
    if (!out.ok || !out.url) {
      toast.error(out.message || t('envs.ossSignFail'))
      return
    }
    signedURL.value = out.url
    signedUntil.value = out.expires_at || ''
    if (mode === 'link') {
      toast.success(t('envs.ossSigned'))
      return
    }
    window.open(out.url, '_blank', 'noopener')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    signing.value = ''
  }
}

async function copyURL(): Promise<void> {
  const text = signedURL.value
  if (!text) return
  const ok = await copyTextToClipboard(text)
  if (ok) toast.success(t('envs.ossCopied'))
  else toast.error(t('envs.ossCopyFail'))
}

watch(
  () => [open.value, props.recId] as const,
  ([shown]) => {
    if (!shown) return
    prefix.value = ''
    items.value = []
    void load('', false)
  },
)
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="window"
    width="82%"
    height="76%"
    draggable
    resizable
    fullscreenable
    teleport-to="body"
    :title="title"
  >
    <template #body>
      <div class="oss-browse">
        <p class="oss-browse__hint">{{ t('envs.ossBrowseHint') }}</p>
        <div class="oss-browse__crumbs">
          <button
            v-for="(crumb, index) in crumbs"
            :key="crumb.prefix || 'root'"
            type="button"
            class="oss-browse__crumb"
            :disabled="index === crumbs.length - 1"
            @click="load(crumb.prefix, false)"
          >
            {{ crumb.label }}
          </button>
        </div>
        <RsTable
          class="oss-browse__table"
          :columns="columns"
          :data="items"
          :loading="loading"
          row-key="key"
          :bordered="false"
          :cell-tooltip="true"
        >
          <template #name="{ row }">
            <button v-if="row.dir" type="button" class="oss-browse__dir" @click="load(row.key, false)">
              {{ row.name }}
            </button>
            <span v-else>{{ row.name }}</span>
          </template>
          <template #size="{ row }">{{ formatSize(row) }}</template>
          <template #modified="{ row }">{{ row.dir ? t('common.dash') : formatTime(row.modified) }}</template>
          <template #actions="{ row }">
            <div v-if="!row.dir" class="oss-browse__actions">
              <RsButton size="sm" variant="text" tone="primary" :loading="signing === row.key + 'view'" @click="sign(row, 'view')">
                {{ t('envs.ossView') }}
              </RsButton>
              <RsButton size="sm" variant="text" tone="primary" :loading="signing === row.key + 'download'" @click="sign(row, 'download')">
                {{ t('envs.ossDownload') }}
              </RsButton>
              <RsButton size="sm" variant="text" tone="success" :loading="signing === row.key + 'link'" @click="sign(row, 'link')">
                {{ t('envs.ossSign') }}
              </RsButton>
            </div>
          </template>
        </RsTable>
        <p v-if="!loading && failed" class="oss-browse__empty">{{ failed }}</p>
        <p v-else-if="!loading && !items.length" class="oss-browse__empty">{{ t('envs.ossEmpty') }}</p>
        <div v-if="truncated" class="oss-browse__more">
          <RsButton size="sm" variant="ghost" :loading="loading" @click="load(prefix, true)">{{ t('envs.ossMore') }}</RsButton>
        </div>
        <section v-if="signedURL" class="oss-browse__link">
          <div class="oss-browse__link-head">
            <span>{{ t('envs.ossSignTitle') }}</span>
            <span v-if="signedUntil">{{ t('envs.ossSignUntil', { time: signedUntil }) }}</span>
          </div>
          <p class="oss-browse__url">{{ signedURL }}</p>
          <RsButton size="sm" variant="ghost" @click="copyURL">{{ t('envs.ossCopy') }}</RsButton>
        </section>
      </div>
    </template>
  </RsDialog>
</template>

<style scoped>
.oss-browse {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
  min-height: 0;
}

.oss-browse__hint,
.oss-browse__empty {
  margin: 0;
  font-size: 0.8rem;
  color: var(--shell-muted);
}

.oss-browse__crumbs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.oss-browse__crumb {
  margin: 0;
  padding: 0.15rem 0.4rem;
  border: 0;
  border-radius: 0.4rem;
  background: transparent;
  color: #0369a1;
  font: inherit;
  cursor: pointer;
}

.oss-browse__crumb:disabled {
  color: var(--shell-title);
  font-weight: 650;
  cursor: default;
}

.oss-browse__crumb:not(:last-child)::after {
  content: '/';
  margin-left: 0.25rem;
  color: var(--shell-muted);
}

.oss-browse__dir {
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: #0369a1;
  font: inherit;
  font-weight: 650;
  cursor: pointer;
}

.oss-browse__actions {
  display: inline-flex;
  justify-content: flex-end;
  gap: 0.15rem;
}

.oss-browse__more {
  display: flex;
  justify-content: center;
}

.oss-browse__link {
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.65rem;
  background: var(--shell-inset-bg);
}

.oss-browse__link-head {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.78rem;
  font-weight: 650;
}

.oss-browse__url {
  margin: 0.4rem 0;
  font-family: var(--rs-font-mono);
  font-size: 0.75rem;
  word-break: break-all;
}
</style>
