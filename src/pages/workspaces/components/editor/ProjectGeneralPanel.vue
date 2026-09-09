<script setup lang="ts">
import { RsFormItem, RsInput, RsSelect, RsSwitch, type RsSelectModelValue } from '@/ui'

defineProps<{
  typeOptions: { value: string; label: string }[]
  checkoutPath: string
  nameValidator: (value: string) => true | string
}>()

const form = defineModel<{
  project_name: string
  project_type: string
  is_enabled: number
  remark: string
}>('form', { required: true })

const advancedOpen = defineModel<boolean>('advanced', { default: false })

const emit = defineEmits<{
  typeChange: [value: RsSelectModelValue]
}>()
</script>

<template>
  <section class="panel">
    <header class="panel-head">
      <h3>{{ $t('workspaces.nav.general') }}</h3>
      <p>{{ $t('workspaces.sectionGeneralDesc') }}</p>
    </header>
    <RsFormItem :label="$t('workspaces.projectName')" :tooltip="$t('workspaces.slugHint')" required>
      <RsInput
        :model-value="form.project_name"
        :placeholder="$t('workspaces.projectNamePh')"
        required
        :validator="nameValidator"
        validate-trigger="both"
        @update:model-value="form.project_name = $event"
      />
    </RsFormItem>
    <RsFormItem :label="$t('workspaces.projectType')" required>
      <RsSelect :model-value="form.project_type" :options="typeOptions" required @update:model-value="emit('typeChange', $event)" />
    </RsFormItem>
    <RsFormItem :label="$t('envs.enabled')">
      <RsSwitch v-model="form.is_enabled" :checked-value="1" :unchecked-value="0" />
    </RsFormItem>
    <p v-if="form.is_enabled === 0" class="hint">{{ $t('workspaces.enabledHint') }}</p>
    <div class="path">
      <span>{{ $t('workspaces.saveDir') }}</span>
      <code>{{ checkoutPath }}</code>
    </div>
    <button type="button" class="more" @click="advancedOpen = !advancedOpen">{{ $t('workspaces.advanced') }}</button>
    <RsInput v-if="advancedOpen" v-model="form.remark" :label="$t('workspaces.remark')" :placeholder="$t('workspaces.remark')" />
  </section>
</template>

<style scoped src="./panel.css"></style>
