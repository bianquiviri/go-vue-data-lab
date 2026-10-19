import axios, { AxiosError, AxiosHeaders, type AxiosInstance, type AxiosRequestConfig } from 'axios'

import { API_ERROR_CODES } from '@/types/api'

/** Error tipado del API: siempre se propaga un ApiError, nunca un AxiosError. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(init: { status: number; code: string; message: string; details?: Record<string, unknown> }) {
    super(init.message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.details = init.details
  }
}

const STATUS_CODES: Record<number, string> = {
  400: API_ERROR_CODES.badRequest,
  401: API_ERROR_CODES.unauthorized,
  404: API_ERROR_CODES.notFound,
  409: API_ERROR_CODES.conflict,
  500: API_ERROR_CODES.internal,
}

/**
 * Normaliza cualquier fallo a un ApiError, entendiendo ambos formatos del
 * backend: el envelope {"error":{code,message,details}} (endpointes de datos)
 * y el plano {"error":"mensaje"} (endpointes de auth).
 */
export function parseErrorPayload(data: unknown, status: number, fallbackMessage = 'Error inesperado'): ApiError {
  if (status === 0) {
    return new ApiError({
      status: 0,
      code: API_ERROR_CODES.network,
      message: 'No se pudo conectar con el servidor',
    })
  }

  const fallbackCode = STATUS_CODES[status] ?? API_ERROR_CODES.internal

  if (data !== null && typeof data === 'object' && 'error' in data) {
    const { error } = data as { error: unknown }

    if (typeof error === 'string') {
      return new ApiError({ status, code: fallbackCode, message: error })
    }

    if (error !== null && typeof error === 'object') {
      const body = error as { code?: unknown; message?: unknown; details?: unknown }
      return new ApiError({
        status,
        code: typeof body.code === 'string' ? body.code : fallbackCode,
        message: typeof body.message === 'string' ? body.message : fallbackMessage,
        details:
          body.details !== null && typeof body.details === 'object'
            ? (body.details as Record<string, unknown>)
            : undefined,
      })
    }
  }

  return new ApiError({ status, code: fallbackCode, message: fallbackMessage })
}

/** AxiosRequestConfig con flags internos del interceptor. */
export type RetriableRequestConfig = AxiosRequestConfig & {
  /** Ya se reintentó una vez tras un 401 (evita bucles infinitos). */
  _retry?: boolean
  /** El caller pidió explícitamente no intentar refresh ante un 401. */
  _skipRefresh?: boolean
}

/** Marca una request para que el interceptor NO intente refresh en un 401. */
export function withoutRefresh(config: AxiosRequestConfig = {}): RetriableRequestConfig {
  return { ...config, _skipRefresh: true }
}

/** Hooks de sesión que el store de auth registra en el cliente. */
export interface AuthHooks {
  getAccessToken(): string | null
  refresh(): Promise<void>
  onAuthFailure?(): void
}

let authHooks: AuthHooks | null = null

export function configureAuthHooks(hooks: AuthHooks): void {
  authHooks = hooks
}

export function resetAuthHooks(): void {
  authHooks = null
}

export const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  headers: { 'Content-Type': 'application/json' },
  timeout: 30_000,
})

// Inyecta el access token (en memoria) en cada request autenticada.
api.interceptors.request.use((config) => {
  const token = authHooks?.getAccessToken()
  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`)
  }
  return config
})

// Normaliza errores y, ante un 401, refresca el token y reintenta la request original.
api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    if (!(error instanceof AxiosError)) {
      return Promise.reject(error)
    }

    const config = error.config as RetriableRequestConfig | undefined
    const status = error.response?.status ?? 0

    if (status === 401 && config && !config._retry && !config._skipRefresh && authHooks) {
      config._retry = true
      try {
        await authHooks.refresh()
        const token = authHooks.getAccessToken()
        if (token) {
          ;(config.headers as AxiosHeaders).set('Authorization', `Bearer ${token}`)
        }
        return api.request(config)
      } catch {
        authHooks.onAuthFailure?.()
        // Se propaga el error original de la request (más útil para la UI);
        // el fallo del refresh solo sirve para limpiar la sesión local.
        return Promise.reject(parseErrorPayload(error.response?.data, status, error.message))
      }
    }

    return Promise.reject(parseErrorPayload(error.response?.data, status, error.message))
  },
)
