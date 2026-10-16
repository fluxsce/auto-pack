import axios, { type AxiosRequestConfig } from 'axios'

const http = axios.create({
  baseURL: '',
  timeout: 60000,
  headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
})

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('ap_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  // FormData 须由浏览器带 boundary；默认 application/json 会破坏 multipart。
  if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
    if (typeof config.headers?.set === 'function') {
      config.headers.set('Content-Type', false as unknown as string)
    } else if (config.headers) {
      delete (config.headers as Record<string, unknown>)['Content-Type']
    }
  }
  return config
})

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn
}

export function notifyUnauthorized(): void {
  onUnauthorized?.()
}

http.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err?.response?.status === 401) {
      onUnauthorized?.()
    }
    const msg = err?.response?.data?.error || err?.message || 'request failed'
    return Promise.reject(new Error(msg))
  },
)

export function setRequestTimeout(sec: number): void {
  const n = Number(sec)
  if (!Number.isFinite(n)) return
  const clamped = Math.min(600, Math.max(5, n))
  http.defaults.timeout = clamped * 1000
}

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const res = await http.request<T>(config)
  return res.data
}

export async function downloadBlob(config: AxiosRequestConfig, fallbackName: string): Promise<void> {
  const res = await http.request<Blob>({ ...config, responseType: 'blob' })
  const type = String(res.headers['content-type'] || '')
  if (type.includes('application/json')) {
    const text = await res.data.text()
    let msg = 'request failed'
    try {
      const parsed = JSON.parse(text) as { error?: string }
      if (parsed.error) msg = parsed.error
    } catch {
      /* keep fallback */
    }
    throw new Error(msg)
  }
  const raw = String(res.headers['content-disposition'] || '')
  const star = /filename\*=UTF-8''([^;]+)/i.exec(raw)
  const quoted = /filename="([^"]+)"/i.exec(raw)
  const name = (star ? decodeURIComponent(star[1]) : quoted?.[1]) || fallbackName
  const url = URL.createObjectURL(res.data)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
}
