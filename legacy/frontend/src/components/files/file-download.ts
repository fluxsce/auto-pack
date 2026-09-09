import { API_BASE } from '@/constants/app'
import { getToken } from '@/request/utils/token'

export function buildDownloadUrl(path: string, params?: Record<string, string>): string {
  const url = path.startsWith('http') ? path : `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`
  if (!params || !Object.keys(params).length) return url
  const qs = new URLSearchParams(params)
  return `${url}?${qs.toString()}`
}

export async function downloadAuthenticated(
  url: string,
  filename: string,
): Promise<void> {
  const token = getToken()
  const res = await fetch(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) {
    let message = `Download failed (${res.status})`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      /* ignore non-json body */
    }
    throw new Error(message)
  }
  const blob = await res.blob()
  triggerBlobDownload(blob, filename)
}

export function triggerBlobDownload(blob: Blob, filename: string): void {
  const href = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = filename
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(href)
}

export function projectWorkspaceDownloadUrl(projectRecId: string, entryPath: string): string {
  return buildDownloadUrl(`/projects/${projectRecId}/workspace/download`, { path: entryPath })
}
