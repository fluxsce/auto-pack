import { onBeforeUnmount, ref, watch, type Ref } from 'vue'

export type DialogResizeDirection = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'

export const DIALOG_RESIZE_HANDLES: DialogResizeDirection[] = [
  'n',
  's',
  'e',
  'w',
  'ne',
  'nw',
  'se',
  'sw',
]

export interface UseDialogResizeOptions {
  enabled: Ref<boolean>
  fullscreen: Ref<boolean>
  visible: Ref<boolean>
  initialWidth: Ref<string | number>
  initialHeight?: Ref<string | number | undefined>
  getDialogEl: () => HTMLElement | null
  minWidth?: number
  minHeight?: number
}

function parseWidth(value: string | number): number {
  if (typeof value === 'number') return value
  const n = Number.parseFloat(value)
  return Number.isFinite(n) ? n : 520
}

function parseHeight(value: string | number): number {
  if (typeof value === 'number') return value
  const raw = String(value).trim().toLowerCase()
  const n = Number.parseFloat(raw)
  if (!Number.isFinite(n)) return 280
  if (raw.endsWith('vh') && typeof globalThis.innerHeight === 'number') {
    return (globalThis.innerHeight * n) / 100
  }
  if (raw.endsWith('%') && typeof globalThis.innerHeight === 'number') {
    return (globalThis.innerHeight * n) / 100
  }
  return n
}

function readMinSize(name: string, fallback: number): number {
  if (typeof document === 'undefined') return fallback
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  const n = Number.parseFloat(raw)
  return Number.isFinite(n) ? n : fallback
}

function parseTranslate(style: CSSStyleDeclaration): { x: number; y: number } {
  const t = style.transform
  if (!t || t === 'none') return { x: 0, y: 0 }

  const matrix = /^matrix\((.+)\)$/.exec(t)
  if (matrix) {
    const parts = matrix[1].split(',').map((s) => Number.parseFloat(s.trim()))
    if (parts.length === 6) return { x: parts[4] ?? 0, y: parts[5] ?? 0 }
  }

  const matrix3d = /^matrix3d\((.+)\)$/.exec(t)
  if (matrix3d) {
    const parts = matrix3d[1].split(',').map((s) => Number.parseFloat(s.trim()))
    return { x: parts[12] ?? 0, y: parts[13] ?? 0 }
  }

  const translate = /translate(?:3d)?\(([^)]+)\)/.exec(t)
  if (translate) {
    const parts = translate[1].split(',').map((s) => Number.parseFloat(s.trim()))
    return { x: parts[0] ?? 0, y: parts[1] ?? 0 }
  }

  return { x: 0, y: 0 }
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

function affectsWidth(direction: DialogResizeDirection): boolean {
  return direction.includes('e') || direction.includes('w')
}

function affectsHeight(direction: DialogResizeDirection): boolean {
  return direction.includes('n') || direction.includes('s')
}

export function useDialogResize(options: UseDialogResizeOptions) {
  const widthPx = ref(parseWidth(options.initialWidth.value))
  const heightPx = ref<number | null>(null)
  const heightLocked = ref(false)
  const mountEl = ref<HTMLElement | null>(null)

  let stopResize: (() => void) | null = null

  function getLimits() {
    const minW = options.minWidth ?? readMinSize('--app-dialog-min-width', 400)
    const minH = options.minHeight ?? readMinSize('--app-dialog-min-height', 280)
    const margin = 32
    return {
      minW,
      minH,
      maxW: Math.max(minW, globalThis.innerWidth - margin),
      maxH: Math.max(minH, globalThis.innerHeight - margin),
    }
  }

  function applyTransform(el: HTMLElement, tx: number, ty: number): void {
    el.style.transform = tx || ty ? `translate(${Math.round(tx)}px, ${Math.round(ty)}px)` : ''
  }

  function applyDimensions(
    el: HTMLElement,
    w: number,
    h: number,
    tx: number,
    ty: number,
    setHeight: boolean,
  ): void {
    el.style.width = `${w}px`
    el.style.setProperty('--el-dialog-width', `${w}px`)
    if (setHeight) {
      el.style.height = `${h}px`
      heightLocked.value = true
      heightPx.value = Math.round(h)
    } else if (heightLocked.value && heightPx.value != null) {
      el.style.height = `${heightPx.value}px`
    } else {
      el.style.height = ''
    }
    applyTransform(el, tx, ty)
  }

  function clearInlineSize(el: HTMLElement | null): void {
    if (!el) return
    el.style.width = ''
    el.style.height = ''
    el.style.transform = ''
    el.style.removeProperty('--el-dialog-width')
  }

  function resetSize(): void {
    widthPx.value = parseWidth(options.initialWidth.value)
    heightPx.value = null
    heightLocked.value = false
    clearInlineSize(options.getDialogEl())
  }

  function bindMount(): void {
    mountEl.value = options.getDialogEl()
  }

  /** Sync resize state from layout; height comes from CSS (--app-dialog-initial-height). */
  function isAutoHeight(raw: string | number | undefined): boolean {
    return typeof raw === 'string' && raw.trim().toLowerCase() === 'auto'
  }

  function applyInitialHeight(): void {
    const raw = options.initialHeight?.value
    if (raw == null || isAutoHeight(raw) || options.fullscreen.value) return
    const el = options.getDialogEl()
    if (!el) return
    const { minH, maxH } = getLimits()
    const targetH = clamp(parseHeight(raw), minH, maxH)
    const rectH = el.getBoundingClientRect().height
    const h = rectH > 0 ? Math.round(clamp(rectH, minH, maxH)) : Math.round(targetH)
    const w = widthPx.value
    heightPx.value = h
    heightLocked.value = true
    el.style.width = `${w}px`
    el.style.setProperty('--el-dialog-width', `${w}px`)
    // Do not set inline height — avoids shrinking after open animation.
  }

  function onResizeStart(direction: DialogResizeDirection, event: MouseEvent): void {
    if (!options.enabled.value || options.fullscreen.value) return
    const el = options.getDialogEl()
    if (!el) return

    event.preventDefault()
    event.stopPropagation()

    const resizeWidth = affectsWidth(direction)
    const resizeHeight = affectsHeight(direction)
    if (!resizeWidth && !resizeHeight) return

    const { minW, minH, maxW, maxH } = getLimits()
    const rect = el.getBoundingClientRect()
    const { x: tx0, y: ty0 } = parseTranslate(getComputedStyle(el))
    const startW = rect.width
    const startH = rect.height
    const startX = event.clientX
    const startY = event.clientY

    const onMove = (e: MouseEvent): void => {
      const dx = e.clientX - startX
      const dy = e.clientY - startY
      let w = startW
      let h = startH
      let tx = tx0
      let ty = ty0

      if (resizeWidth && direction.includes('e')) w = startW + dx
      if (resizeWidth && direction.includes('w')) {
        w = startW - dx
        tx = tx0 + (startW - w)
      }
      if (resizeHeight && direction.includes('s')) h = startH + dy
      if (resizeHeight && direction.includes('n')) {
        h = startH - dy
        ty = ty0 + (startH - h)
      }

      if (resizeWidth) w = clamp(w, minW, maxW)
      if (resizeHeight) h = clamp(h, minH, maxH)

      if (resizeWidth && direction.includes('w')) tx = tx0 + (startW - w)
      if (resizeHeight && direction.includes('n')) ty = ty0 + (startH - h)

      widthPx.value = Math.round(w)
      applyDimensions(el, w, h, tx, ty, resizeHeight)
    }

    const onUp = (): void => {
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
      document.body.style.removeProperty('user-select')
      document.body.style.removeProperty('cursor')
      stopResize = null
    }

    document.body.style.userSelect = 'none'
    document.body.style.cursor = `${direction}-resize`
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
    stopResize = onUp
  }

  const dialogWidth = ref<string | number>(options.initialWidth.value)

  watch(widthPx, (w) => {
    dialogWidth.value = `${w}px`
  })

  watch(
    () => options.initialWidth.value,
    (v) => {
      if (!options.visible.value) widthPx.value = parseWidth(v)
    },
  )

  watch(
    () => options.visible.value,
    (v) => {
      if (!v) {
        resetSize()
        mountEl.value = null
      }
    },
  )

  watch(
    () => options.fullscreen.value,
    (fs) => {
      if (fs) {
        clearInlineSize(options.getDialogEl())
        mountEl.value = null
      } else if (options.visible.value) {
        queueMicrotask(() => {
          bindMount()
          applyInitialHeight()
        })
      }
    },
  )

  onBeforeUnmount(() => {
    stopResize?.()
    clearInlineSize(options.getDialogEl())
  })

  return {
    widthPx,
    heightPx,
    dialogWidth,
    mountEl,
    bindMount,
    applyInitialHeight,
    resetSize,
    onResizeStart,
  }
}
