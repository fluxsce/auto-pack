export type BuildLogLineKind =
  | 'default'
  | 'command'
  | 'section'
  | 'error'
  | 'warning'
  | 'info'
  | 'success'

export type LogViewerVariant = 'build' | 'release'

const ERROR_PATTERN =
  /\[ERROR\]|^ERROR:|Finished: FAILURE|BUILD FAILURE|FATAL ERROR|\[FATAL\]|^\[失败\]|^\[FAIL\]/i
const WARNING_PATTERN =
  /\[WARNING\]|^WARN(?:ING)?:|\bWARNING\b|^\[跳过\]|^\[SKIP\]|^\[…\]/i
const INFO_PATTERN = /^\[INFO\]|^  · /i
const SUCCESS_PATTERN =
  /Finished: SUCCESS|BUILD SUCCESS|\[SUCCESS\]|^\[成功\]|^\[OK\]|汇总 —|Summary —/i
const SECTION_PATTERN = /^\[[A-Za-z][^\]]*\]$/
const COMMAND_PREFIX = ' > '

const RELEASE_ERROR_DETAIL = /^  → /

export function splitBuildLogLines(content: string): string[] {
  if (!content) return []
  return content.split('\n')
}

export function classifyBuildLogLine(line: string): BuildLogLineKind {
  return classifyLogLine(line, 'build')
}

export function classifyLogLine(line: string, variant: LogViewerVariant = 'build'): BuildLogLineKind {
  const trimmed = line.trim()
  if (!trimmed) return 'default'
  if (variant === 'release') {
    if (/^\[成功\]|^\[OK\]/i.test(trimmed)) return 'success'
    if (/^\[失败\]|^\[FAIL\]/i.test(trimmed)) return 'error'
    if (/^\[跳过\]|^\[SKIP\]/i.test(trimmed)) return 'warning'
    if (/^\[…\]|发版结束|Release finished/i.test(trimmed)) return 'info'
    if (/^  · /.test(line)) return 'info'
    if (RELEASE_ERROR_DETAIL.test(line)) return 'error'
    if (/^  → /.test(line)) return 'warning'
    if (/^\[\d{4}-\d{2}-\d{2}/.test(trimmed)) return 'section'
    if (/^发版分支:|^Branch:/i.test(trimmed)) return 'default'
    if (/^汇总 —|^Summary —/i.test(trimmed)) return 'success'
    return 'default'
  }
  if (trimmed.startsWith(COMMAND_PREFIX)) return 'command'
  if (SECTION_PATTERN.test(trimmed)) return 'section'
  if (SUCCESS_PATTERN.test(trimmed)) return 'success'
  if (ERROR_PATTERN.test(trimmed)) return 'error'
  if (WARNING_PATTERN.test(trimmed)) return 'warning'
  if (INFO_PATTERN.test(trimmed)) return 'info'
  return 'default'
}

export function buildLogLineClass(kind: BuildLogLineKind): string {
  return `build-log-viewer__line--${kind}`
}
