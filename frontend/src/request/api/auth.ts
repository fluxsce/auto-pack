import type { ChangePasswordPayload, ChangeUsernamePayload, LoginResult, UserView } from '@/types'
import { clearToken, http, setToken } from '../utils'

export function login(data: { username: string; password: string }): Promise<LoginResult> {
  return http.post('/auth/login', data).then((result: LoginResult) => {
    setToken(result.token)
    return result
  })
}

export function logout(): void {
  clearToken()
}

export function fetchMe(): Promise<UserView> {
  return http.get('/auth/me')
}

export function changePassword(data: ChangePasswordPayload): Promise<void> {
  return http.put('/auth/password', data)
}

export function changeUsername(data: ChangeUsernamePayload): Promise<UserView> {
  return http.put('/auth/username', data)
}
