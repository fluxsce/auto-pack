import axios, { AxiosError } from 'axios'
import { API_BASE } from '@/constants/app'
import { getToken } from './token'

interface ErrorBody {
  error?: string
}

/** 后端鉴权中间件 / 需登录接口返回的 token 失效类错误（不含登录失败、改密旧密码错误等） */
const TOKEN_AUTH_ERRORS = new Set([
  'invalid or expired token',
  'missing authorization header',
  'invalid authorization header',
  'unauthorized',
])

const http = axios.create({
  baseURL: API_BASE,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response.data,
  (error: AxiosError<ErrorBody>) => {
    const status = error.response?.status
    const errorMessage = error.response?.data?.error
    const requestUrl = error.config?.url ?? ''
    const isLoginRequest = requestUrl.includes('/auth/login')

    if (
      status === 401 &&
      !isLoginRequest &&
      errorMessage != null &&
      TOKEN_AUTH_ERRORS.has(errorMessage)
    ) {
      void import('@/stores/session').then(({ useSessionStore }) => {
        void useSessionStore().forceReLogin()
      })
    }

    const message =
      errorMessage ??
      error.message ??
      `Request failed (${error.response?.status ?? 'network'})`
    return Promise.reject(new Error(message))
  },
)

export default http
