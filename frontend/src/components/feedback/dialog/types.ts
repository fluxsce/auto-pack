export type AppDialogWidth = string | number

export interface AppDialogProps {
  /** Dialog title; omit when using the header slot */
  title?: string
  /** Effective when fullscreen is false */
  width?: AppDialogWidth
  /** Fullscreen layout (default off); set true for full viewport */
  fullscreen?: boolean
  /** Show toggle in header to switch fullscreen / windowed */
  showFullscreenToggle?: boolean
  /** Show default footer with cancel / confirm */
  showFooter?: boolean
  cancelText?: string
  confirmText?: string
  confirmLoading?: boolean
  confirmDisabled?: boolean
  showClose?: boolean
  /** Dimmed backdrop mask (default off) */
  modal?: boolean
  closeOnClickModal?: boolean
  closeOnPressEscape?: boolean
  destroyOnClose?: boolean
  alignCenter?: boolean
  /** Drag by header (disabled while fullscreen) */
  draggable?: boolean
  /** Resize by border handles (disabled while fullscreen) */
  resizable?: boolean
  minWidth?: number
  minHeight?: number
  /**
   * Windowed height (px number or e.g. `76vh`). Default `76vh`.
   * Pass `auto` for content-driven height (no fixed viewport ratio).
   */
  initialHeight?: string | number | 'auto'
}
