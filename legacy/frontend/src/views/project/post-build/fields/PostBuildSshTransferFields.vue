<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import type { EnvType } from '@/types'
import type { PostBuildAction } from '@/types/post-build'
import { DEPLOY_ENV_TYPES } from '../../project-meta'
import { defaultTransferSet } from '../post-build-defaults'
import type { PostBuildFieldsProps } from '../post-build-fields-props'

const props = defineProps<PostBuildFieldsProps>()

const action = defineModel<PostBuildAction>({ required: true })

const emit = defineEmits<{
  reloadEnvs: []
}>()

const { t } = useI18n()

const remoteEnvTypes = DEPLOY_ENV_TYPES

function onEnvSelectVisible(visible: boolean): void {
  if (visible) emit('reloadEnvs')
}

function envTypeLabel(type: EnvType): string {
  return t(`envType_${type}`)
}

function addTransferSet(): void {
  if (!action.value.transfer_sets) action.value.transfer_sets = []
  action.value.transfer_sets.push(defaultTransferSet())
}

function removeTransferSet(index: number): void {
  if (!action.value.transfer_sets || action.value.transfer_sets.length <= 1) return
  action.value.transfer_sets = action.value.transfer_sets.filter((_, i) => i !== index)
}
</script>

<template>
  <div class="post-build-fields">
    <el-form-item :label="$t('projectPostBuildRemoteEnv')" required>
      <el-select
        v-model="action.env_rec_id"
        clearable
        filterable
        class="post-build-fields__full"
        :placeholder="$t('projectDeployPlaceholder')"
        @visible-change="onEnvSelectVisible"
      >
        <el-option-group v-for="tp in remoteEnvTypes" :key="tp" :label="envTypeLabel(tp)">
          <el-option
            v-for="e in props.envOptions(tp)"
            :key="e.rec_id"
            :label="e.env_name"
            :value="e.rec_id"
          />
        </el-option-group>
      </el-select>
    </el-form-item>

    <div class="post-build-fields__transfers">
      <div class="post-build-fields__transfers-head">
        <span>{{ $t('projectPostBuildTransfers') }}</span>
        <el-button size="small" :icon="Plus" @click="addTransferSet">
          {{ $t('projectPostBuildAddTransferSet') }}
        </el-button>
      </div>

      <div
        v-for="(set, setIndex) in action.transfer_sets"
        :key="setIndex"
        class="post-build-fields__transfer-set"
      >
        <div class="post-build-fields__transfer-set-head">
          <span>{{ $t('projectPostBuildTransferSet') }} {{ setIndex + 1 }}</span>
          <el-button
            v-if="(action.transfer_sets?.length ?? 0) > 1"
            text
            type="danger"
            size="small"
            @click="removeTransferSet(setIndex)"
          >
            {{ $t('delete') }}
          </el-button>
        </div>
        <el-form-item :label="$t('projectPostBuildSourceFiles')">
          <el-input
            v-model="set.source_files"
            :placeholder="$t('projectPostBuildSourceFilesPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="$t('projectPostBuildRemovePrefix')">
          <el-input v-model="set.remove_prefix" placeholder="target" />
        </el-form-item>
        <el-form-item :label="$t('projectPostBuildRemoteDir')">
          <el-input v-model="set.remote_dir" placeholder="/data/appinstall/develop" />
        </el-form-item>
        <el-form-item :label="$t('projectPostBuildExecCommand')">
          <el-input
            v-model="set.exec_command"
            type="textarea"
            :rows="3"
            :placeholder="$t('projectPostBuildTransferExecPlaceholder')"
          />
        </el-form-item>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss" src="./post-build-fields.scss"></style>
