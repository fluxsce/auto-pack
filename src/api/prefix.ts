export const webBase = '/autopack'
export const apiRoot = '/autopack/api'

export function joinPath(base: string, path = ''): string {
  if (!path) return base
  return `${base.replace(/\/$/, '')}/${path.replace(/^\//, '')}`
}

export function apiPath(...parts: string[]): string {
  return parts.reduce((acc, p) => joinPath(acc, p), apiRoot)
}
