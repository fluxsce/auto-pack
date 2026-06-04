import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  createUser,
  deleteUser,
  fetchUsers,
  resetUserPassword,
  updateUser,
} from '@/request/api/users'
import type { UserAccount, UserAccountPayload } from '@/types'
import { useSessionStore } from '@/stores'

export type UserPanelMode = 'none' | 'create' | 'edit'

export function useUsers() {
  const { t } = useI18n()
  const message = useMessage()
  const session = useSessionStore()

  const items = ref<UserAccount[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const panelMode = ref<UserPanelMode>('none')
  const editingId = ref<string | null>(null)
  const resetPassword = ref('')

  const form = reactive<UserAccountPayload & { password: string }>({
    login_name: '',
    password: '',
    is_enabled: 1,
    remark: '',
  })

  const editorOpen = computed(() => panelMode.value !== 'none')
  const editorTitle = computed(() =>
    panelMode.value === 'create' ? t('userCreateTitle') : t('userEditTitle'),
  )
  const isCreateMode = computed(() => panelMode.value === 'create')
  const canDeleteEditing = computed(() => {
    const row = editingId.value ? items.value.find((u) => u.rec_id === editingId.value) : null
    return !!row && row.rec_id !== session.currentUser?.rec_id
  })

  function resetForm(): void {
    panelMode.value = 'none'
    editingId.value = null
    resetPassword.value = ''
    form.login_name = ''
    form.password = ''
    form.is_enabled = 1
    form.remark = ''
  }

  function openCreate(): void {
    resetForm()
    panelMode.value = 'create'
  }

  function openEdit(row: UserAccount): void {
    panelMode.value = 'edit'
    editingId.value = row.rec_id
    form.login_name = row.login_name
    form.password = ''
    form.is_enabled = row.is_enabled
    form.remark = row.remark
    resetPassword.value = ''
  }

  async function loadList(): Promise<void> {
    loading.value = true
    try {
      const res = await fetchUsers()
      items.value = Array.isArray(res.items) ? res.items : []
    } catch (e) {
      message.error((e as Error).message)
      items.value = []
    } finally {
      loading.value = false
    }
  }

  async function onSave(): Promise<void> {
    if (!form.login_name.trim()) {
      message.warning(t('userLoginNameRequired'))
      return
    }
    if (isCreateMode.value && (form.password?.length ?? 0) < 6) {
      message.warning(t('userPasswordRequired'))
      return
    }
    saving.value = true
    try {
      if (isCreateMode.value) {
        await createUser({
          login_name: form.login_name.trim(),
          password: form.password,
          is_enabled: form.is_enabled,
          remark: form.remark,
        })
        message.success(t('userCreateSuccess'))
      } else if (editingId.value) {
        await updateUser(editingId.value, {
          login_name: form.login_name.trim(),
          is_enabled: form.is_enabled,
          remark: form.remark,
        })
        message.success(t('userSaveSuccess'))
      }
      await loadList()
      resetForm()
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      saving.value = false
    }
  }

  async function onResetPassword(): Promise<void> {
    if (!editingId.value) return
    if (resetPassword.value.length < 6) {
      message.warning(t('userPasswordRequired'))
      return
    }
    saving.value = true
    try {
      await resetUserPassword(editingId.value, { new_password: resetPassword.value })
      message.success(t('userResetPasswordSuccess'))
      resetPassword.value = ''
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      saving.value = false
    }
  }

  async function onDelete(row: UserAccount): Promise<void> {
    try {
      await deleteUser(row.rec_id)
      message.success(t('userDeleteSuccess'))
      if (editingId.value === row.rec_id) resetForm()
      await loadList()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  onMounted(() => {
    void loadList()
  })

  async function onDeleteEditing(): Promise<void> {
    const row = items.value.find((u) => u.rec_id === editingId.value)
    if (row) await onDelete(row)
  }

  return {
    items,
    loading,
    saving,
    editorOpen,
    editorTitle,
    isCreateMode,
    canDeleteEditing,
    editingId,
    resetPassword,
    form,
    openCreate,
    openEdit,
    closeEditor: resetForm,
    onSave,
    onResetPassword,
    onDelete,
    onDeleteEditing,
    loadList,
  }
}
