export type Project = {
  rec_id: string
  workspace_rec_id?: string
  project_name: string
  project_type: string
  repo_url?: string
  repo_branch?: string
  work_rel_path?: string
  recipe_json: string
  cron_expr?: string
  is_enabled: number
  remark?: string
  created_at?: string
  updated_at?: string
  last_job_status?: string
  last_job_at?: string
}

export type WorkspaceMode = 'create' | 'edit' | 'clone'

export type WorkspaceWrite = {
  rec_id?: string
  source_rec_id?: string
  workspace_code: string
  workspace_name: string
  remark?: string
}

export type ProjectWrite = {
  rec_id?: string
  workspace_rec_id: string
  project_name: string
  project_type: string
  repo_url?: string
  repo_branch?: string
  work_rel_path?: string
  recipe_json: string
  cron_expr?: string
  is_enabled: number
  remark?: string
}

export function errText(e: unknown): string {
  if (e instanceof Error) return e.message
  if (typeof e === 'string') return e
  return 'request failed'
}

export function asSelectString(value: unknown): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return ''
  return String(typeof raw === 'object' && raw && 'value' in raw ? (raw as { value: unknown }).value : raw)
}

export function asSelectStrings(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => asSelectString(item)).filter(Boolean)
  }
  const one = asSelectString(value)
  return one ? [one] : []
}
