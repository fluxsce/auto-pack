<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Delete, Download, Upload, View } from '@element-plus/icons-vue'

defineProps<{
  isDir: boolean
  downloadLoading?: boolean
  deleteLoading?: boolean
  remotePushLoading?: boolean
  showDelete?: boolean
  showRemotePush?: boolean
  /** When set, delete requires confirmation via popconfirm. */
  deleteConfirmTitle?: string
}>()

const emit = defineEmits<{
  view: []
  download: []
  delete: []
  remotePush: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="file-entry-actions" @click.stop>
    <div class="file-entry-actions__slot">
      <el-button
        v-if="!isDir"
        size="small"
        link
        type="primary"
        class="file-entry-actions__btn"
        :aria-label="t('fileView')"
        @click="emit('view')"
      >
        <el-icon class="file-entry-actions__icon"><View /></el-icon>
        <span>{{ t('fileView') }}</span>
      </el-button>
    </div>
    <div class="file-entry-actions__slot">
      <el-button
        size="small"
        link
        type="primary"
        class="file-entry-actions__btn"
        :aria-label="t('fileDownload')"
        :loading="downloadLoading"
        @click="emit('download')"
      >
        <el-icon v-if="!downloadLoading" class="file-entry-actions__icon"><Download /></el-icon>
        <span>{{ t('fileDownload') }}</span>
      </el-button>
    </div>
    <div v-if="showRemotePush" class="file-entry-actions__slot">
      <el-button
        size="small"
        link
        type="primary"
        class="file-entry-actions__btn"
        :aria-label="t('projectFtpPushAction')"
        :loading="remotePushLoading"
        @click="emit('remotePush')"
      >
        <el-icon v-if="!remotePushLoading" class="file-entry-actions__icon"><Upload /></el-icon>
        <span>{{ t('projectFtpPushAction') }}</span>
      </el-button>
    </div>
    <div v-if="showDelete" class="file-entry-actions__slot">
      <AppPopconfirm
        v-if="deleteConfirmTitle"
        :title="deleteConfirmTitle"
        @confirm="emit('delete')"
      >
        <template #reference>
          <el-button
            size="small"
            link
            type="danger"
            class="file-entry-actions__btn file-entry-actions__btn--danger"
            :aria-label="t('fileDelete')"
            :loading="deleteLoading"
          >
            <el-icon v-if="!deleteLoading" class="file-entry-actions__icon"><Delete /></el-icon>
            <span>{{ t('fileDelete') }}</span>
          </el-button>
        </template>
      </AppPopconfirm>
      <el-button
        v-else
        size="small"
        link
        type="danger"
        class="file-entry-actions__btn file-entry-actions__btn--danger"
        :aria-label="t('fileDelete')"
        :loading="deleteLoading"
        @click="emit('delete')"
      >
        <el-icon v-if="!deleteLoading" class="file-entry-actions__icon"><Delete /></el-icon>
        <span>{{ t('fileDelete') }}</span>
      </el-button>
    </div>
  </div>
</template>

<style scoped lang="scss" src="./FileEntryActions.scss"></style>
