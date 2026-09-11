import type { Project } from '../api'
import { defaultBuild, parseRecipe, type ProjectRecipe } from './project-defaults'

export type CommandFamily = 'maven' | 'node'

const packGoals = new Set(['package', 'install', 'deploy'])

export type CommandPatchDraft = {
  family: CommandFamily
  keepPhases: boolean
  applyMeta: boolean
  recipe: ProjectRecipe
}

export function familyProjectType(family: CommandFamily): string {
  return family === 'maven' ? 'maven' : 'vue'
}

export function matchCommandFamily(projectType: string, family: CommandFamily): boolean {
  const code = projectType.trim().toLowerCase()
  if (family === 'maven') return code === 'maven' || code === 'java'
  return code === 'npm' || code === 'node' || code === 'nodejs' || code === 'vue' || code === 'frontend'
}

export function recipeOf(row: Project): ProjectRecipe {
  return parseRecipe(row.recipe_json, row.project_type)
}

export function lastPackGoal(command: string): string {
  let last = ''
  for (const part of command.split(/\s+/).filter(Boolean)) {
    if (packGoals.has(part)) last = part
  }
  return last
}

export function rewriteMavenGoal(command: string, goal: string): string | null {
  const want = goal.trim().toLowerCase()
  if (!packGoals.has(want)) return null
  const parts = command.split(/\s+/).filter(Boolean)
  let last = -1
  for (let i = 0; i < parts.length; i++) {
    if (packGoals.has(parts[i])) last = i
  }
  if (last < 0 || parts[last] === want) return null
  parts[last] = want
  return parts.join(' ')
}

export function commandWithGoal(type: string, goal: string): string {
  const base = defaultBuild(type).command || ''
  return rewriteMavenGoal(base, goal) || base.replace(/\b(package|install|deploy)\b/, goal) || base
}

export function previewCommand(row: Project, draft: CommandPatchDraft): string | null {
  const current = recipeOf(row).command
  if (draft.family === 'maven' && draft.keepPhases) {
    const goal = lastPackGoal(draft.recipe.command) || 'install'
    return rewriteMavenGoal(current, goal)
  }
  const next = draft.recipe.command.trim()
  if (!next || next === current.trim()) return null
  return next
}

export function projectsForPatch(rows: Project[], draft: CommandPatchDraft): Project[] {
  return rows.filter((row) => {
    if (!matchCommandFamily(row.project_type, draft.family)) return false
    if (previewCommand(row, draft)) return true
    if (!draft.applyMeta) return false
    const rec = recipeOf(row)
    const next = draft.recipe
    return (
      (next.workdir || '') !== (rec.workdir || '') ||
      (next.output_dir || '') !== (rec.output_dir || '') ||
      (next.artifact_pattern || '') !== (rec.artifact_pattern || '') ||
      (next.log_retention_days ?? 0) !== (rec.log_retention_days ?? 0)
    )
  })
}
