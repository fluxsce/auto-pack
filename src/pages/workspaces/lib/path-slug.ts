const slugRe = /^[A-Za-z0-9](?:[A-Za-z0-9._-]{0,62}[A-Za-z0-9])?$/
const unsafeNameRe = /[\x00-\x1f\\/:*?"<>|]/

/** 给 i18n 插值用，勿写进文案原文：`|` `<` `>` 会被 vue-i18n 当成复数 / HTML。 */
export const UNSAFE_NAME_CHARS = '\\ / : * ? " < > |'
const reserved = new Set([
  'CON',
  'PRN',
  'AUX',
  'NUL',
  'COM1',
  'COM2',
  'COM3',
  'COM4',
  'COM5',
  'COM6',
  'COM7',
  'COM8',
  'COM9',
  'LPT1',
  'LPT2',
  'LPT3',
  'LPT4',
  'LPT5',
  'LPT6',
  'LPT7',
  'LPT8',
  'LPT9',
])

/** 克隆工程用的新 slug：`name-copy` / `name-copy2`，与后端 uniqueCopyName 一致。 */
export function uniqueCopyName(name: string, taken: Iterable<string>): string {
  const used = taken instanceof Set ? taken : new Set(taken)
  let base = name.trim() || 'project'
  if (base.length > 57) {
    base = base.slice(0, 57).replace(/[-_]+$/, '')
  }
  const ok = (n: string) => isPathSlug(n) && !used.has(n)
  const first = `${base}-copy`
  if (ok(first)) return first
  for (let i = 2; i < 100; i++) {
    const next = `${base}-copy${i}`
    if (ok(next)) return next
  }
  return `${base}-copy${Date.now()}`
}

/** 会进磁盘目录的名称：允许点号，便于 V9.0.0-P09 当空间编码或工程名。 */
export function isPathSlug(value: string): boolean {
  const name = value.trim()
  return slugRe.test(name) && !name.includes('..') && !reserved.has(name.toUpperCase())
}

/** 空间编码与路径 slug 相同。 */
export function isWorkspaceCode(value: string): boolean {
  return isPathSlug(value)
}

/** 显示名称：非空，且不含路径分隔或通配符。 */
export function isSafeDisplayName(value: string): boolean {
  const name = value.trim()
  return name.length > 0 && [...name].length <= 80 && !unsafeNameRe.test(name)
}

/** 工程相对子路径：空合法；每一段都必须是路径 slug。 */
export function isRelPath(value: string): boolean {
  const rel = value.trim().replace(/\\/g, '/')
  if (!rel) return true
  if (rel.startsWith('/') || rel.includes('..')) return false
  return rel.split('/').filter(Boolean).every(isPathSlug)
}

function isSourceVarSegment(segment: string): boolean {
  return /^\$\{AP_[A-Z0-9_]+\}$/.test(segment) || /^\$AP_[A-Z0-9_]+$/.test(segment) || /^%AP_[A-Z0-9_]+%$/.test(segment)
}

/** 本地源码路径：相对 slug 或本机绝对路径（v1 导入常带 /home/...）；段内可含 AP_* 占位符。 */
export function isLocalSourcePath(value: string): boolean {
  const path = value.trim().replace(/\\/g, '/')
  if (!path) return true
  if (path.includes('..')) return false
  if (path.startsWith('/') || /^[A-Za-z]:\//.test(path)) return true
  return path.split('/').filter(Boolean).every((seg) => isPathSlug(seg) || isSourceVarSegment(seg))
}

/** Git 检出地址：http(s) / ssh / git / file。 */
export function isGitURL(value: string): boolean {
  return /^(https?:\/\/|git@|ssh:\/\/|git:\/\/|file:\/\/)/i.test(value.trim())
}

/** Maven / POM 版本号：字母数字开头，可含点、下划线、连字符。 */
export function isPomVersion(value: string): boolean {
  const ver = value.trim()
  return ver.length > 0 && ver.length <= 80 && /^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(ver)
}

/** Git 分支名：单段或多段路径，禁止空格、.. 与通配符。 */
export function isGitBranch(value: string): boolean {
  const name = value.trim()
  if (!name || name.length > 200) return false
  if (name.includes('..') || name.includes('//') || name.startsWith('/') || name.endsWith('/')) return false
  if (name.split('/').some((part) => !part || part.startsWith('.') || part.endsWith('.lock'))) return false
  return !/[\x00-\x20\x7f\\:~^?*\[@{]/.test(name)
}
