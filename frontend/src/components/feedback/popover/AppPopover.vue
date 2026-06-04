<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { APP_DIALOG_POPPER_Z_INDEX } from '../dialog/constants'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    zIndex?: number
    teleported?: boolean
    popperClass?: string
  }>(),
  {
    teleported: true,
    zIndex: APP_DIALOG_POPPER_Z_INDEX,
    popperClass: 'app-dialog-popper',
  },
)

const attrs = useAttrs()

const forwarded = computed(() => ({
  ...attrs,
  teleported: props.teleported,
  zIndex: props.zIndex,
  popperClass: props.popperClass,
}))
</script>

<template>
  <el-popover v-bind="forwarded">
    <template v-for="(_, name) in $slots" #[name]="scope">
      <slot :name="name" v-bind="scope ?? {}" />
    </template>
  </el-popover>
</template>
