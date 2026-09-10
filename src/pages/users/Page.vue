<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsAvatar,
  RsButton,
  RsCard,
  RsCheckbox,
  RsTable,
  RsTabs,
  rsConfirm,
  useRsToast,
  type RsTabItem,
  type RsTableColumn,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import PageFrame from '@/components/PageFrame.vue'
import UserDialogs from './UserDialogs.vue'

const MEMBER_SCOPE = '00000000-0000-4000-8000-000000000010'

type User = { rec_id: string; login_name: string; display_name: string; is_enabled: number }
type Member = { user_rec_id: string; login_name: string; display_name: string; member_role: string }
type Role = (typeof roles)[number]

const roles = ['owner', 'operator', 'viewer'] as const
const allActions = ['job.submit', 'job.cancel', 'project.write', 'env.write', 'user.admin', 'settings.write', 'audit.read']
const actionI18n: Record<string, string> = {
  'job.submit': 'users.actions.jobSubmit',
  'job.cancel': 'users.actions.jobCancel',
  'project.write': 'users.actions.projectWrite',
  'env.write': 'users.actions.envWrite',
  'user.admin': 'users.actions.userAdmin',
  'settings.write': 'users.actions.settingsWrite',
  'audit.read': 'users.actions.auditRead',
}

const { t } = useI18n()
const toast = useRsToast()
const users = ref<User[]>([])
const members = ref<Member[]>([])
const actions = ref<Record<string, string[]>>({ owner: [], operator: [], viewer: [] })
const tab = ref('members')
const userOpen = ref(false)
const memOpen = ref(false)
const editOpen = ref(false)
const editing = ref<Member | null>(null)
const saving = ref(false)
const busyId = ref('')

const tabItems = computed<RsTabItem[]>(() => [
  {
    value: 'members',
    label: members.value.length ? `${t('users.members')} (${members.value.length})` : t('users.members'),
  },
  { value: 'matrix', label: t('users.matrix') },
])

const roleOptions = computed(() => roles.map((role) => ({ value: role, label: t(`users.roles.${role}`) })))

const memberColumns = computed<RsTableColumn<Member>[]>(() => [
  { key: 'identity', title: t('users.member'), minWidth: 240 },
  { key: 'member_role', title: t('users.role'), width: 140 },
  { key: 'actions', title: t('common.actions'), width: 148, align: 'right' },
])

const userOptions = computed(() => {
  const taken = new Set(members.value.map((item) => item.user_rec_id))
  return users.value
    .filter((item) => !taken.has(item.rec_id))
    .map((item) => ({
      value: item.rec_id,
      label:
        item.display_name && item.display_name !== item.login_name
          ? `${item.display_name} · ${item.login_name}`
          : item.login_name,
    }))
})

function memberName(row: Member): string {
  return row.display_name || row.login_name
}

function showLogin(row: Member): boolean {
  return Boolean(row.login_name && row.login_name !== row.display_name)
}

function avatarTone(role: string): 'warning' | 'primary' | 'default' {
  if (role === 'owner') return 'warning'
  if (role === 'operator') return 'primary'
  return 'default'
}

async function load(): Promise<void> {
  try {
    users.value =
      (await request<User[]>({
        method: 'GET',
        url: apiPath('users'),
      })) ?? []
    members.value =
      (await request<Member[]>({
        method: 'GET',
        url: apiPath('workspaces', MEMBER_SCOPE, 'members'),
      })) ?? []
    for (const role of roles) {
      actions.value[role] =
        (await request<string[]>({
          method: 'GET',
          url: apiPath('roles', role, 'actions'),
        })) ?? []
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(load)

function openEdit(row: Member): void {
  editing.value = row
  editOpen.value = true
}

async function removeMember(row: Member): Promise<void> {
  const ok = await rsConfirm.danger({
    title: t('common.delete'),
    description: t('users.removeConfirm', { name: memberName(row) }),
    confirmText: t('common.delete'),
  })
  if (!ok) return
  busyId.value = row.user_rec_id
  try {
    await request({
      method: 'DELETE',
      url: apiPath('workspaces', MEMBER_SCOPE, 'members', row.user_rec_id),
    })
    toast.success(t('common.deleted'))
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    busyId.value = ''
  }
}

async function saveMatrix(): Promise<void> {
  saving.value = true
  try {
    for (const role of roles) {
      await request({
        method: 'PUT',
        url: apiPath('roles', role, 'actions'),
        data: { actions: actions.value[role] },
      })
    }
    toast.success(t('common.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function toggleAction(role: Role, action: string, on: boolean): void {
  const cur = new Set(actions.value[role])
  if (on) cur.add(action)
  else cur.delete(action)
  actions.value[role] = [...cur]
}
</script>

<template>
  <PageFrame :title="t('users.title')" :desc="t('users.desc')">
    <template #actions>
      <RsButton @click="userOpen = true">{{ t('users.newUser') }}</RsButton>
      <RsButton variant="ghost" :disabled="!userOptions.length" @click="memOpen = true">{{ t('users.join') }}</RsButton>
      <RsButton variant="ghost" @click="load">{{ t('common.refresh') }}</RsButton>
    </template>

    <RsCard variant="outlined" elevated radius="lg" :padding="false" class="user-panel">
      <div class="user-panel__tabs">
        <RsTabs v-model="tab" :items="tabItems" panelless variant="line" />
      </div>

      <template v-if="tab === 'members'">
        <RsTable
          v-if="members.length"
          :columns="memberColumns"
          :data="members"
          row-key="user_rec_id"
          :bordered="false"
          :column-bordered="false"
          :context-menu="false"
          cell-focus="none"
          highlight-row
        >
          <template #identity="{ row }">
            <div class="user-id">
              <RsAvatar size="sm" :name="memberName(row)" :tone="avatarTone(row.member_role)" />
              <div class="user-id__meta">
                <span class="user-id__name">{{ memberName(row) }}</span>
                <span v-if="showLogin(row)" class="user-id__login">{{ row.login_name }}</span>
              </div>
            </div>
          </template>
          <template #member_role="{ row }">
            <span class="user-role">
              <i :class="['user-role__dot', `user-role__dot--${row.member_role}`]" />
              {{ t(`users.roles.${row.member_role}`) }}
            </span>
          </template>
          <template #actions="{ row }">
            <div class="user-panel__actions">
              <RsButton size="sm" variant="text" tone="primary" @click="openEdit(row)">
                {{ t('common.edit') }}
              </RsButton>
              <RsButton
                size="sm"
                variant="text"
                tone="danger"
                :disabled="busyId === row.user_rec_id"
                @click="removeMember(row)"
              >
                {{ t('users.remove') }}
              </RsButton>
            </div>
          </template>
        </RsTable>

        <div v-else class="user-panel__empty">
          <p class="user-panel__empty-title">{{ t('users.empty') }}</p>
          <p class="user-panel__empty-hint">{{ t('users.emptyHint') }}</p>
        </div>
      </template>

      <template v-else>
        <table class="matrix">
          <thead>
            <tr>
              <th>{{ t('users.action') }}</th>
              <th v-for="role in roles" :key="role">
                <span class="user-role user-role--static">
                  <i :class="['user-role__dot', `user-role__dot--${role}`]" />
                  {{ t(`users.roles.${role}`) }}
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="action in allActions" :key="action">
              <td>
                <div class="matrix__action">
                  <span class="matrix__name">{{ t(actionI18n[action]) }}</span>
                  <span class="matrix__code">{{ action }}</span>
                </div>
              </td>
              <td v-for="role in roles" :key="role">
                <RsCheckbox
                  :model-value="actions[role]?.includes(action)"
                  :aria-label="`${t(`users.roles.${role}`)} ${t(actionI18n[action])}`"
                  @update:model-value="(on: boolean) => toggleAction(role, action, on)"
                />
              </td>
            </tr>
          </tbody>
        </table>
        <div class="user-panel__footer">
          <p class="user-panel__hint">{{ t('users.matrixDesc') }}</p>
          <RsButton :loading="saving" @click="saveMatrix">{{ t('users.saveMatrix') }}</RsButton>
        </div>
      </template>
    </RsCard>

    <UserDialogs
      v-model:user-open="userOpen"
      v-model:mem-open="memOpen"
      v-model:edit-open="editOpen"
      :workspace-id="MEMBER_SCOPE"
      :user-options="userOptions"
      :role-options="roleOptions"
      :editing="editing"
      @saved="load"
    />
  </PageFrame>
</template>

<style scoped>
.user-panel {
  overflow: hidden;
}

.user-panel :deep(.rs-table__row:last-child .rs-table__td) {
  border-bottom: none;
}

.user-panel__tabs {
  padding: 0 1.15rem;
  border-bottom: 1px solid var(--shell-border);
}

.user-panel__tabs :deep(.rs-tabs--line.rs-tabs--panelless .rs-tabs__nav) {
  background: transparent;
  border-bottom: none;
}

.user-panel__tabs :deep(.rs-tabs--line .rs-tabs__trigger) {
  height: 2.75rem;
  font-size: 0.875rem;
}

.user-panel__tabs :deep(.rs-tabs--line .rs-tabs__trigger[data-state='active']::after) {
  inset-inline: 0.35rem;
}

.user-id {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.user-id__meta {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.user-id__name {
  font-weight: 550;
  color: var(--shell-title);
}

.user-id__login {
  font-size: 0.78rem;
  color: var(--shell-muted);
}

.user-role {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  font-weight: 650;
  color: var(--shell-title);
  white-space: nowrap;
}

.user-role--static {
  cursor: default;
}

.user-role__dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  background: var(--shell-muted);
}

.user-role__dot--owner {
  background: #d97706;
}

.user-role__dot--operator {
  background: #0284c7;
}

.user-role__dot--viewer {
  background: #94a3b8;
}

.user-panel__actions {
  display: inline-flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.15rem;
}

.user-panel__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 2.4rem 1.25rem 2.75rem;
  text-align: center;
}

.user-panel__empty-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 550;
  color: var(--shell-title);
}

.user-panel__empty-hint {
  margin: 0.45rem 0 0;
  max-width: 22rem;
  font-size: 0.82rem;
  line-height: 1.6;
  color: var(--shell-muted);
}

.user-panel__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  padding: 0.85rem 1.15rem 1rem;
  border-top: 1px solid var(--shell-border);
}

.user-panel__hint {
  margin: 0;
  max-width: 36rem;
  font-size: 0.82rem;
  line-height: 1.55;
  color: var(--shell-muted);
}
</style>
