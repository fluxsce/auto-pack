<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { APP_DIALOG_POPPER_Z_INDEX } from '../dialog/constants'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    /** Popper z-index; default above AppDialog */
    zIndex?: number
    /** Teleport popper to body (default on) */
    teleported?: boolean
  }>(),
  {
    teleported: true,
    zIndex: APP_DIALOG_POPPER_Z_INDEX,
  },
)

const emit = defineEmits<{
  confirm: [e: MouseEvent]
  cancel: [e: MouseEvent]
}>()

const attrs = useAttrs()

const forwarded = computed(() => {
  const { onConfirm, onCancel, ...rest } = attrs
  return {
    ...rest,
    teleported: props.teleported,
    zIndex: props.zIndex,
  }
})
</script>

<template>
  <el-popconfirm
    v-bind="forwarded"
    @confirm="emit('confirm', $event)"
    @cancel="emit('cancel', $event)"
  >
    <template v-for="(_, name) in $slots" #[name]="scope">
      <slot :name="name" v-bind="scope ?? {}" />
    </template>
  </el-popconfirm>
</template>
