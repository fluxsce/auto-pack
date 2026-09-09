<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsInput, RsSelect, useRsToast, type RsSelectModelValue } from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

type Option = { value: string; label: string }
type CreatedUser = { rec_id: string }
type EditingMember = { user_rec_id: string; login_name: string; display_name: string; member_role: string }

const userOpen = defineModel<boolean>('userOpen', { default: false })
const memOpen = defineModel<boolean>('memOpen', { default: false })
const editOpen = defineModel<boolean>('editOpen', { default: false })

const props = defineProps<{
  workspaceId: string
  userOptions: Option[]
  roleOptions: Option[]
  editing?: EditingMember | null
}>()

const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()
const toast = useRsToast()
const saving = ref(false)
const userForm = ref({ login_name: '', display_name: '', password: '', member_role: 'viewer' })
const memForm = ref({ user_rec_id: '', member_role: 'viewer' })
const editForm = ref({ user_rec_id: '', member_role: 'viewer' })
const editingName = computed(() => {
  const row = props.editing
  if (!row) return ''
  if (row.display_name && row.display_name !== row.login_name) {
    return `${row.display_name} · ${row.login_name}`
  }
  return row.display_name || row.login_name
})

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}

watch(userOpen, (open) => {
  if (open) userForm.value = { login_name: '', display_name: '', password: '', member_role: 'viewer' }
})

watch(memOpen, (open) => {
  if (open) {
    memForm.value = {
      user_rec_id: props.userOptions[0]?.value ?? '',
      member_role: 'viewer',
    }
  }
})

watch(editOpen, (open) => {
  if (open && props.editing) {
    editForm.value = {
      user_rec_id: props.editing.user_rec_id,
      member_role: props.editing.member_role,
    }
  }
})

async function createUser(): Promise<void> {
  saving.value = true
  try {
    const created = await request<CreatedUser>({
      method: 'POST',
      url: apiPath('users'),
      data: {
        login_name: userForm.value.login_name,
        display_name: userForm.value.display_name,
        password: userForm.value.password,
        workspace_rec_id: props.workspaceId,
      },
    })
    if (created?.rec_id) {
      await request({
        method: 'POST',
        url: apiPath('workspaces', props.workspaceId, 'members'),
        data: { user_rec_id: created.rec_id, member_role: userForm.value.member_role },
      })
    }
    userOpen.value = false
    toast.success(t('common.saved'))
    emit('saved')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function saveMember(): Promise<void> {
  if (!editForm.value.user_rec_id) return
  saving.value = true
  try {
    await request({
      method: 'POST',
      url: apiPath('workspaces', props.workspaceId, 'members'),
      data: editForm.value,
    })
    editOpen.value = false
    toast.success(t('common.saved'))
    emit('saved')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function addMember(): Promise<void> {
  if (!memForm.value.user_rec_id) return
  saving.value = true
  try {
    await request({
      method: 'POST',
      url: apiPath('workspaces', props.workspaceId, 'members'),
      data: memForm.value,
    })
    memOpen.value = false
    toast.success(t('common.saved'))
    emit('saved')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RsDialog v-model:open="userOpen" layout="form" width="sm" :title="t('users.newUser')">
    <template #body>
      <div class="form-grid">
        <RsInput v-model="userForm.login_name" :label="t('users.loginName')" :placeholder="t('users.loginName')" />
        <RsInput v-model="userForm.display_name" :label="t('users.displayName')" :placeholder="t('users.displayName')" />
        <RsInput
          v-model="userForm.password"
          type="password"
          :label="t('users.password')"
          :placeholder="t('users.password')"
        />
        <div>
          <div class="dlg-label">{{ t('users.role') }}</div>
          <RsSelect
            :model-value="userForm.member_role"
            :options="roleOptions"
            :placeholder="t('users.rolePlaceholder')"
            @update:model-value="(v: RsSelectModelValue) => (userForm.member_role = asString(v))"
          />
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="userOpen = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" @click="createUser">{{ t('common.create') }}</RsButton>
    </template>
  </RsDialog>

  <RsDialog v-model:open="memOpen" layout="form" width="sm" :title="t('users.join')">
    <template #body>
      <div class="form-grid">
        <div>
          <div class="dlg-label">{{ t('users.userId') }}</div>
          <RsSelect
            :model-value="memForm.user_rec_id"
            :options="userOptions"
            :placeholder="t('users.userId')"
            @update:model-value="(v: RsSelectModelValue) => (memForm.user_rec_id = asString(v))"
          />
        </div>
        <div>
          <div class="dlg-label">{{ t('users.role') }}</div>
          <RsSelect
            :model-value="memForm.member_role"
            :options="roleOptions"
            :placeholder="t('users.rolePlaceholder')"
            @update:model-value="(v: RsSelectModelValue) => (memForm.member_role = asString(v))"
          />
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="memOpen = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" :disabled="!memForm.user_rec_id" @click="addMember">{{ t('users.join') }}</RsButton>
    </template>
  </RsDialog>

  <RsDialog v-model:open="editOpen" layout="form" width="sm" :title="t('users.editMember')">
    <template #body>
      <div class="form-grid">
        <div>
          <div class="dlg-label">{{ t('users.member') }}</div>
          <p class="dlg-name">{{ editingName }}</p>
        </div>
        <div>
          <div class="dlg-label">{{ t('users.role') }}</div>
          <RsSelect
            :model-value="editForm.member_role"
            :options="roleOptions"
            :placeholder="t('users.rolePlaceholder')"
            @update:model-value="(v: RsSelectModelValue) => (editForm.member_role = asString(v))"
          />
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="editOpen = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" @click="saveMember">{{ t('common.save') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.dlg-label {
  margin-bottom: 0.35rem;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--shell-muted);
}

.dlg-name {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 550;
  color: var(--shell-title);
}
</style>
