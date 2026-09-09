import type { Project } from '../api'
import { parseRecipe } from './project-defaults'

export function isGitProject(row: Project): boolean {
  return parseRecipe(row.recipe_json, row.project_type).scm_mode !== 'local'
}

export function isMavenProject(row: Project): boolean {
  return row.project_type === 'maven'
}
