import type { Component } from 'vue'
import type { PostBuildActionType } from '@/types/post-build'
import PostBuildEmailNotifyFields from './fields/PostBuildEmailNotifyFields.vue'
import PostBuildShellExecFields from './fields/PostBuildShellExecFields.vue'
import PostBuildSshExecFields from './fields/PostBuildSshExecFields.vue'
import PostBuildSshTransferFields from './fields/PostBuildSshTransferFields.vue'

export interface PostBuildTypeDescriptor {
  type: PostBuildActionType
  /** i18n key: projectPostBuildType_{type} */
  labelKey: `projectPostBuildType_${PostBuildActionType}`
  component: Component
}

/**
 * Register new post-build action types here (order = add-action dropdown & type select).
 * Also extend PostBuildActionType, defaultPostBuildAction, backend runner, and i18n.
 */
export const POST_BUILD_TYPE_REGISTRY: readonly PostBuildTypeDescriptor[] = [
  {
    type: 'shell_exec',
    labelKey: 'projectPostBuildType_shell_exec',
    component: PostBuildShellExecFields,
  },
  {
    type: 'ssh_exec',
    labelKey: 'projectPostBuildType_ssh_exec',
    component: PostBuildSshExecFields,
  },
  {
    type: 'ssh_transfer',
    labelKey: 'projectPostBuildType_ssh_transfer',
    component: PostBuildSshTransferFields,
  },
  {
    type: 'email_notify',
    labelKey: 'projectPostBuildType_email_notify',
    component: PostBuildEmailNotifyFields,
  },
] as const

export const POST_BUILD_TYPE_OPTIONS: PostBuildActionType[] = POST_BUILD_TYPE_REGISTRY.map(
  (d) => d.type,
)

const fieldByType = Object.fromEntries(
  POST_BUILD_TYPE_REGISTRY.map((d) => [d.type, d.component]),
) as Record<PostBuildActionType, Component>

export function postBuildFieldComponent(type: PostBuildActionType): Component {
  return fieldByType[type]
}

export function isRegisteredPostBuildType(v: unknown): v is PostBuildActionType {
  return typeof v === 'string' && v in fieldByType
}
