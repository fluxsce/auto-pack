/** Teleported AppDialog stack (above app header ~100) */
export const APP_DIALOG_Z_INDEX = 2500

/** Nested AppDialog opened above another AppDialog */
export const APP_DIALOG_NESTED_Z_INDEX = APP_DIALOG_Z_INDEX + 50

/** Popconfirm / popover / select dropdown opened inside AppDialog */
export const APP_DIALOG_POPPER_Z_INDEX = APP_DIALOG_NESTED_Z_INDEX + 100

/** Element Plus `popper-class` for selects inside AppDialog */
export const APP_DIALOG_POPPER_CLASS = 'app-dialog-popper'
