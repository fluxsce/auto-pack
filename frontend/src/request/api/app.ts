import http from '../utils/http'

export interface AppHealth {
  status: string
  version: string
}

export function fetchAppHealth(): Promise<AppHealth> {
  return http.get('/health')
}
