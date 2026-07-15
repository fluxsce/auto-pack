import type { Component } from 'vue'
import { Clock, Link, Monitor, Promotion, Setting, Tools } from '@element-plus/icons-vue'

export type ProjectEditorSection =
  | 'general'
  | 'scm'
  | 'environment'
  | 'build'
  | 'triggers'
  | 'deploy'

export interface ProjectEditorNavItem {
  id: ProjectEditorSection
  labelKey: string
  icon: Component
}

export const PROJECT_EDITOR_NAV: ProjectEditorNavItem[] = [
  { id: 'general', labelKey: 'projectNavGeneral', icon: Setting },
  { id: 'scm', labelKey: 'projectNavScm', icon: Link },
  { id: 'environment', labelKey: 'projectNavEnvironment', icon: Monitor },
  { id: 'build', labelKey: 'projectNavBuild', icon: Tools },
  { id: 'triggers', labelKey: 'projectNavTriggers', icon: Clock },
  { id: 'deploy', labelKey: 'projectNavDeploy', icon: Promotion },
]

export type ScmMode = 'local' | 'git'
