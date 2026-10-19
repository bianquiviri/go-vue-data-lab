/** Envelope de error del backend: {"error":{code,message,details}}. */
export interface ApiErrorBody {
  code: string
  message: string
  details?: Record<string, unknown>
}

/** Códigos de error del envelope (internal/handler/response.go). */
export const API_ERROR_CODES = {
  validationFailed: 'validation_failed',
  badRequest: 'bad_request',
  notFound: 'not_found',
  conflict: 'conflict',
  internal: 'internal_error',
  /** No viene del backend: falta de red/CORS/DNS. */
  network: 'network_error',
  /**401 sin cuerpo reconocible. */
  unauthorized: 'unauthorized',
} as const

export type ApiErrorCode = string

/** Metadatos de paginación del envelope: {"data":...,"meta":{page,...}}. */
export interface ApiMeta {
  page?: number
  per_page?: number
  total?: number
  total_pages?: number
  [key: string]: unknown
}

/** Envelope de éxito paginado del backend. */
export interface ApiEnvelope<T> {
  data: T
  meta?: ApiMeta
}
