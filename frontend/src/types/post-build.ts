/** When to run a post-build action relative to the build result. */
export type PostBuildWhen = 'success' | 'failure' | 'always'

/** shell_exec (local) | ssh_exec | ssh_transfer | email_notify */
export type PostBuildActionType = 'shell_exec' | 'ssh_exec' | 'ssh_transfer' | 'email_notify'

/** Jenkins-style transfer set (source → remote + optional exec). */
export interface PostBuildTransferSet {
  source_files: string
  remove_prefix: string
  remote_dir: string
  exec_command: string
}

export interface PostBuildAction {
  id: string
  name: string
  type: PostBuildActionType
  enabled: boolean
  when: PostBuildWhen
  /** Remote env (ssh / sftp / ftp) or smtp env for email. */
  env_rec_id: string
  command?: string
  transfer_sets?: PostBuildTransferSet[]
  recipients?: string
  subject?: string
  body?: string
}

export const POST_BUILD_WHEN_OPTIONS: PostBuildWhen[] = ['success', 'failure', 'always']
