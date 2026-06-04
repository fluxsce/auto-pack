import type { OperationLogEntry } from '@/types'
import http from '../utils/http'

export interface OperationLogQuery {
  module?: string
  action?: string
  user_rec_id?: string
  keyword?: string
  limit?: number
  offset?: number
}

export interface OperationLogListResult {
  items: OperationLogEntry[]
  total: number
  limit: number
  offset: number
}

export function fetchOperationLogs(query: OperationLogQuery = {}): Promise<OperationLogListResult> {
  return http.get('/operation-logs', { params: query })
}

export function fetchOperationLogModules(): Promise<{ items: string[] }> {
  return http.get('/operation-log/modules')
}

export function fetchOperationLogActions(): Promise<{ items: string[] }> {
  return http.get('/operation-log/actions')
}
