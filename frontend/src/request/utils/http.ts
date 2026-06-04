import axios, { AxiosError } from 'axios'
import { API_BASE } from '@/constants/app'
import { getToken } from './token'

interface ErrorBody {
  error?: string
}

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
    const message =
      error.response?.data?.error ??
      error.message ??
      `Request failed (${error.response?.status ?? 'network'})`
    return Promise.reject(new Error(message))
  },
)

export default http
