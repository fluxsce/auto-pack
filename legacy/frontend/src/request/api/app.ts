import http from '../utils/http'

export interface AppHealth {
  status: string
  version: string
  /** Process start time; changes when the backend restarts. */
  started_at?: string
}

export function fetchAppHealth(): Promise<AppHealth> {
  return http.get('/health')
}
