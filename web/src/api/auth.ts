import { api, withoutRefresh } from './client'
import type { AuthSession, ChangePasswordPayload, LoginPayload, User } from '@/types/auth'

/**
 * Endpoints de autenticación. El backend devuelve payloads planos
 * ({"user":...,"tokens":...}) en login/refresh/me, no el envelope {"data":...}.
 */

export async function loginRequest(payload: LoginPayload): Promise<AuthSession> {
  // Sin refresh: un 401 aquí significa credenciales inválidas, no token caducado.
  const { data } = await api.post<AuthSession>('/auth/login', payload, withoutRefresh())
  return data
}

export async function refreshRequest(refreshToken: string): Promise<AuthSession> {
  const { data } = await api.post<AuthSession>(
    '/auth/refresh',
    { refresh_token: refreshToken },
    withoutRefresh(),
  )
  return data
}

export async function meRequest(): Promise<User> {
  const { data } = await api.get<{ user: User }>('/auth/me')
  return data.user
}

export async function changePasswordRequest(payload: ChangePasswordPayload): Promise<string> {
  const { data } = await api.post<{ message: string }>('/auth/change-password', payload)
  return data.message
}

export async function logoutRequest(): Promise<string> {
  const { data } = await api.post<{ message: string }>('/auth/logout', {}, withoutRefresh())
  return data.message
}
