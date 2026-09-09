import AppDialog from './AppDialog.vue'
import { DialogPlugin } from './plugin'

export { AppDialog, DialogPlugin }
export type { AppDialogProps, AppDialogWidth } from './types'
export {
  APP_DIALOG_Z_INDEX,
  APP_DIALOG_NESTED_Z_INDEX,
  APP_DIALOG_POPPER_CLASS,
  APP_DIALOG_POPPER_Z_INDEX,
} from './constants'

export default DialogPlugin
