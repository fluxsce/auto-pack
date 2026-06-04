<script setup lang="ts">
import { computed, nextTick, ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { Close, FullScreen, ScaleToOriginal } from '@element-plus/icons-vue'
import type { DialogInstance } from 'element-plus'
import type { AppDialogProps } from './types'
import {
  DIALOG_RESIZE_HANDLES,
  type DialogResizeDirection,
  useDialogResize,
} from './useDialogResize'

import { APP_DIALOG_Z_INDEX } from './constants'

const visible = defineModel<boolean>({ default: false })
const fullscreen = defineModel<boolean>('fullscreen', { default: false })

const props = withDefaults(defineProps<AppDialogProps>(), {
  title: '',
  width: '520px',
  showFullscreenToggle: true,
  showFooter: true,
  confirmLoading: false,
  confirmDisabled: false,
  showClose: true,
  modal: false,
  closeOnClickModal: false,
  closeOnPressEscape: true,
  destroyOnClose: true,
  alignCenter: true,
  draggable: true,
  resizable: true,
  initialHeight: '76vh',
})

const emit = defineEmits<{
  confirm: []
  cancel: []
  closed: []
}>()

const { t } = useI18n()

const elDialogRef = ref<DialogInstance>()

const cancelLabel = computed(() => props.cancelText ?? t('cancel'))
const confirmLabel = computed(() => props.confirmText ?? t('save'))
const fullscreenLabel = computed(() =>
  fullscreen.value ? t('dialogExitFullscreen') : t('dialogFullscreen'),
)

const resizeEnabled = computed(() => props.resizable && !fullscreen.value)

function resolveDialogEl(): HTMLElement | null {
  const content = elDialogRef.value?.dialogContentRef
  const el = content?.$el
  return el instanceof HTMLElement ? el : null
}

const { dialogWidth, mountEl, bindMount, applyInitialHeight, onResizeStart } = useDialogResize({
  enabled: resizeEnabled,
  fullscreen,
  visible,
  initialWidth: toRef(props, 'width'),
  initialHeight: toRef(props, 'initialHeight'),
  getDialogEl: resolveDialogEl,
  minWidth: props.minWidth,
  minHeight: props.minHeight,
})

function isAutoInitialHeight(value: string | number | 'auto' | undefined): boolean {
  if (value == null) return false
  if (value === 'auto') return true
  return typeof value === 'string' && value.trim().toLowerCase() === 'auto'
}

const initialHeightCss = computed(() => {
  if (fullscreen.value || isAutoInitialHeight(props.initialHeight)) return undefined
  const v = props.initialHeight
  return typeof v === 'number' ? `${v}px` : String(v)
})

const dialogStyle = computed(() => {
  const h = initialHeightCss.value
  if (!h) return undefined
  return { '--app-dialog-initial-height': h }
})

const dialogClass = computed(() => [
  'app-dialog',
  fullscreen.value ? 'app-dialog--fullscreen' : 'app-dialog--windowed',
  resizeEnabled.value ? 'app-dialog--resizable' : '',
  initialHeightCss.value ? 'app-dialog--initial-height' : '',
])

const overlayClass = computed(() =>
  props.modal ? 'app-dialog__overlay' : 'app-dialog__overlay app-dialog__overlay--plain',
)

function toggleFullscreen(): void {
  fullscreen.value = !fullscreen.value
}

function onCancel(): void {
  visible.value = false
  emit('cancel')
}

function onConfirm(): void {
  emit('confirm')
}

function onClosed(): void {
  emit('closed')
}

async function onOpened(): Promise<void> {
  await nextTick()
  if (!resizeEnabled.value && !initialHeightCss.value) return
  bindMount()
  applyInitialHeight()
}

function handleResizeStart(direction: DialogResizeDirection, event: MouseEvent): void {
  onResizeStart(direction, event)
}
</script>

<template>
  <el-dialog
    ref="elDialogRef"
    v-model="visible"
    append-to-body
    :z-index="APP_DIALOG_Z_INDEX"
    :width="fullscreen ? undefined : dialogWidth"
    :fullscreen="fullscreen"
    :modal="modal"
    :show-close="false"
    :close-on-click-modal="closeOnClickModal"
    :close-on-press-escape="closeOnPressEscape"
    :destroy-on-close="destroyOnClose"
    :align-center="alignCenter"
    :draggable="draggable"
    :class="dialogClass"
    :style="dialogStyle"
    :modal-class="overlayClass"
    header-class="app-dialog__header"
    body-class="app-dialog__body"
    footer-class="app-dialog__footer-wrap"
    @opened="onOpened"
    @closed="onClosed"
  >
    <template v-if="$slots.header || title" #header>
      <slot name="header">
        <div class="app-dialog__header-inner">
          <h2 class="app-dialog__title">{{ title }}</h2>
          <div class="app-dialog__actions">
            <el-tooltip v-if="showFullscreenToggle" :content="fullscreenLabel" placement="bottom">
              <button
                type="button"
                class="app-dialog__icon-btn"
                :aria-label="fullscreenLabel"
                @click="toggleFullscreen"
              >
                <el-icon :size="16">
                  <ScaleToOriginal v-if="fullscreen" />
                  <FullScreen v-else />
                </el-icon>
              </button>
            </el-tooltip>
            <button
              v-if="showClose"
              type="button"
              class="app-dialog__icon-btn app-dialog__icon-btn--close"
              :aria-label="t('dialogClose')"
              @click="onCancel"
            >
              <el-icon :size="16">
                <Close />
              </el-icon>
            </button>
          </div>
        </div>
      </slot>
    </template>

    <div class="app-dialog__content">
      <slot />
    </div>

    <template v-if="showFooter" #footer>
      <slot name="footer">
        <div class="app-dialog__footer">
          <el-button size="small" class="app-dialog__btn-cancel" @click="onCancel">
            {{ cancelLabel }}
          </el-button>
          <el-button
            size="small"
            type="primary"
            class="app-dialog__btn-confirm"
            :loading="confirmLoading"
            :disabled="confirmDisabled"
            @click="onConfirm"
          >
            {{ confirmLabel }}
          </el-button>
        </div>
      </slot>
    </template>
  </el-dialog>

  <Teleport v-if="resizeEnabled && mountEl" :to="mountEl">
    <div class="app-dialog__resize-handles" aria-hidden="true">
      <div
        v-for="dir in DIALOG_RESIZE_HANDLES"
        :key="dir"
        class="app-dialog__resize-handle"
        :class="`app-dialog__resize-handle--${dir}`"
        @mousedown="handleResizeStart(dir, $event)"
      />
    </div>
  </Teleport>
</template>
