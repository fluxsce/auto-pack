<script setup lang="ts">
import { InfoFilled } from '@element-plus/icons-vue'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'

defineProps<{
  text?: string
  tip?: string
  popperClass?: string
}>()
</script>

<template>
  <span class="form-label-tip">
    <slot>{{ text }}</slot>
    <el-tooltip
      v-if="tip"
      :content="tip"
      placement="top-start"
      :show-after="200"
      :popper-class="[APP_DIALOG_POPPER_CLASS, popperClass, 'form-label-tip__popper'].filter(Boolean).join(' ')"
    >
      <button type="button" class="form-label-tip__trigger" :aria-label="tip">
        <el-icon class="form-label-tip__icon" :size="14">
          <InfoFilled />
        </el-icon>
      </button>
    </el-tooltip>
  </span>
</template>

<style scoped lang="scss">
.form-label-tip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  line-height: 1.4;
  vertical-align: middle;

  &__trigger {
    display: inline-flex;
    align-items: center;
    flex-shrink: 0;
    padding: 0;
    border: none;
    background: transparent;
    outline: none;
    border-radius: 4px;
    cursor: help;

    &:focus-visible {
      box-shadow: 0 0 0 2px var(--el-color-primary-light-7);
    }
  }

  &__icon {
    color: var(--el-text-color-secondary);
  }
}
</style>

<style lang="scss">
.form-label-tip__popper {
  max-width: 360px;
  white-space: pre-line;
  line-height: 1.55;
}
</style>
