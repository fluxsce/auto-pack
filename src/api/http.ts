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
  return config
})

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn
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
