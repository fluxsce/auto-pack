import type { UserAccount, UserAccountPayload, UserResetPasswordPayload } from '@/types'
import http from '../utils/http'

export function fetchUsers(): Promise<{ items: UserAccount[] }> {
  return http.get('/users')
}

export function createUser(data: UserAccountPayload & { password: string }): Promise<UserAccount> {
  return http.post('/users', data)
}

export function updateUser(recId: string, data: UserAccountPayload): Promise<UserAccount> {
  return http.put(`/users/${recId}`, data)
}

export function deleteUser(recId: string): Promise<{ message: string }> {
  return http.delete(`/users/${recId}`)
}

export function resetUserPassword(
  recId: string,
  data: UserResetPasswordPayload,
): Promise<{ message: string }> {
  return http.post(`/users/${recId}/reset-password`, data)
}
