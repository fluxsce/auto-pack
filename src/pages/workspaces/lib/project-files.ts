import type { ProjectFileEntry } from '../api'

export function fileExt(name: string): string {
  const base = name.trim()
  const i = base.lastIndexOf('.')
  if (i <= 0 || i === base.length - 1) return ''
  return base.slice(i + 1).toLowerCase()
}

export function formatBytes(n?: number, empty = '—'): string {
  if (n == null || !Number.isFinite(n) || n < 0) return empty
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = n / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`
}

export function keepDirsFirst(left: ProjectFileEntry, right: ProjectFileEntry): number {
  if (left.is_dir !== right.is_dir) return left.is_dir ? -1 : 1
  return left.name.localeCompare(right.name, undefined, { sensitivity: 'base' })
}

/** matchProjectFile 按文件名或类型文案做包含匹配，大小写不敏感。 */
export function matchProjectFile(row: ProjectFileEntry, query: string, kind: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  return row.name.toLowerCase().includes(q) || kind.toLowerCase().includes(q)
}

const EXT_LANG: Record<string, string> = {
  xml: 'xml',
  xsd: 'xml',
  xsl: 'xml',
  svg: 'xml',
  java: 'java',
  js: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  vue: 'vue',
  json: 'json',
  jsonc: 'json',
  yml: 'yaml',
  yaml: 'yaml',
  md: 'markdown',
  markdown: 'markdown',
  go: 'go',
  py: 'python',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  cmd: 'shell',
  bat: 'shell',
  html: 'html',
  htm: 'html',
  css: 'css',
  scss: 'css',
  less: 'css',
  sql: 'sql',
  toml: 'toml',
  rs: 'rust',
  c: 'c',
  h: 'c',
  cc: 'cpp',
  cpp: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
}

/** previewLang 按文件名推断 RsCodeBlock 语言。 */
export function previewLang(name: string): string {
  const base = name.trim().toLowerCase()
  if (base === 'pom.xml' || base.endsWith('.pom')) return 'xml'
  if (base === 'dockerfile') return 'shell'
  return EXT_LANG[fileExt(name)] || 'plaintext'
}

export function fileBaseName(path: string): string {
  const parts = path.replace(/\\/g, '/').split('/').filter(Boolean)
  return parts[parts.length - 1] || path
}
