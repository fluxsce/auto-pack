const descriptors = new Set(['@yearly', '@annually', '@monthly', '@weekly', '@daily', '@midnight', '@hourly'])

export type CronValidity = 'empty' | 'ok' | 'bad'

/** 与后端 robfig 5 字段 / 描述符对齐的前端预检。最终以保存接口为准。 */
export function cronValidity(expr: string): CronValidity {
  const value = expr.trim()
  if (!value) return 'empty'
  if (value.startsWith('@')) {
    return descriptors.has(value.toLowerCase()) ? 'ok' : 'bad'
  }
  const parts = value.split(/\s+/)
  return parts.length === 5 ? 'ok' : 'bad'
}
