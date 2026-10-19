/** Roles de usuario del backend (models.UserRole). */
export type UserRole = 'admin' | 'operator' | 'viewer'

/** Usuario público (GET /api/v1/auth/me y login; nunca expone hashes). */
export interface User {
  id: string
  email: string
  role: UserRole
  active: boolean
  created_at: string
  updated_at: string
}

/** Tokens emitidos por login/refresh. */
export interface Tokens {
  access_token: string
  refresh_token: string
  token_type: string
  /** Segundos de vida del access token. */
  expires_in: number
}

/** Respuesta de POST /auth/login y /auth/refresh (payload plano, sin envelope data). */
export interface AuthSession {
  user: User
  tokens: Tokens
}

export interface LoginPayload {
  email: string
  password: string
}

export interface ChangePasswordPayload {
  old_password: string
  new_password: string
}
