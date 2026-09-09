<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RsEmpty } from '@/ui'
import type { JobRow } from '../../api'
import { statusTone } from '../../composables/use-project-list'

const props = defineProps<{
  jobs: JobRow[]
  selectedId: string
  aside?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
}>()

const { t } = useI18n()

function buildNo(index: number): number {
  return props.jobs.length - index
}

function noTone(row: JobRow): string {
  return statusTone(row.job_status)
}

function historyTime(row: JobRow): string {
  const value = row.started_at || row.created_at
  if (!value) return ''
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  const d = new Date(zoned ? iso : `${iso}Z`)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<template>
  <section class="pane" :class="{ 'is-aside': props.aside }">
    <header class="pane-head">
      <h3>{{ t('workspaces.detail.nav.history') }}</h3>
    </header>
    <ol v-if="props.jobs.length" class="history">
      <li v-for="(row, i) in props.jobs" :key="row.rec_id">
        <button
          type="button"
          class="history__row"
          :class="{ 'is-on': row.rec_id === props.selectedId }"
          @click="emit('select', row.rec_id)"
        >
          <span class="history__no" :class="`is-${noTone(row)}`">#{{ buildNo(i) }}</span>
          <span class="history__time">{{ historyTime(row) }}</span>
        </button>
      </li>
    </ol>
    <RsEmpty v-else :description="t('workspaces.detail.emptyHistory')" />
  </section>
</template>

<style scoped src="./pane.css"></style>
<style scoped>
.pane.is-aside {
  flex: 1;
  min-height: 0;
  gap: 0.5rem;
}

.pane.is-aside .pane-head h3 {
  font-size: 0.78rem;
  font-weight: 650;
  color: #64748b;
}

.history {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.22rem;
  max-width: 22rem;
  max-height: 22rem;
  overflow: auto;
}

.pane.is-aside .history {
  flex: 1;
  max-width: none;
  max-height: none;
}

.history__row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  padding: 0.28rem 0.4rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: transparent;
  text-align: left;
  cursor: pointer;
  outline: none;
  box-shadow: none;
}

.history__row:hover {
  background: color-mix(in srgb, #0ea5e9 7%, transparent);
}

.history__row:focus-visible {
  box-shadow: 0 0 0 2px color-mix(in srgb, #0ea5e9 22%, transparent);
}

.history__row.is-on {
  border-color: color-mix(in srgb, #0ea5e9 38%, transparent);
  background: color-mix(in srgb, #0ea5e9 10%, #fff);
}

.history__no {
  flex-shrink: 0;
  min-width: 2.35rem;
  padding: 0.08rem 0.42rem;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--rs-success, #34c759) 32%, transparent);
  background: var(--rs-success-container, #e4f8e9);
  color: var(--rs-on-success-container, #1a7f37);
  font-size: 0.78rem;
  font-weight: 650;
  text-align: center;
}

.history__no.is-fail {
  border-color: color-mix(in srgb, var(--rs-danger, #ff3b30) 28%, transparent);
  background: var(--rs-danger-container, #ffe5e3);
  color: var(--rs-on-danger-container, #c41e16);
}

.history__no.is-run {
  border-color: color-mix(in srgb, #0ea5e9 32%, transparent);
  background: #e0f2fe;
  color: #0369a1;
}

.history__no.is-warn {
  border-color: color-mix(in srgb, var(--rs-warning, #ff9500) 28%, transparent);
  background: var(--rs-warning-container, #fff3e0);
  color: var(--rs-on-warning-container, #b26500);
}

.history__no.is-muted {
  border-color: var(--shell-border);
  background: var(--shell-inset-bg);
  color: var(--shell-muted);
}

.history__time {
  color: #94a3b8;
  font-size: 0.82rem;
}

.pane.is-aside .history__row {
  padding: 0.22rem 0.28rem;
  gap: 0.4rem;
}

.pane.is-aside .history__no {
  min-width: 2.05rem;
  font-size: 0.72rem;
}

.pane.is-aside .history__time {
  font-size: 0.7rem;
}
</style>
