export const projectSections = ['general', 'scm', 'environment', 'build', 'trigger', 'deploy'] as const

export type ProjectSection = (typeof projectSections)[number]

export const projectSectionIcons: Record<ProjectSection, string> = {
  general: 'settings',
  scm: 'git-branch',
  environment: 'monitor',
  build: 'hammer',
  trigger: 'clock',
  deploy: 'rocket',
}

export function asProjectSection(value: unknown): ProjectSection {
  return projectSections.includes(value as ProjectSection) ? (value as ProjectSection) : 'general'
}
