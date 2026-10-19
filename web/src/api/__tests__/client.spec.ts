import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { http, HttpResponse } from 'msw'

import {
  ApiError,
  api,
  configureAuthHooks,
  parseErrorPayload,
  resetAuthHooks,
  withoutRefresh,
} from '../client'
import { server } from '@/__tests__/mocks/server'
import {
  errorEnvelope,
  flatError,
  get,
  json,
  makeSession,
  makeTokens,
  makeUser,
  post,
} from '@/__tests__/fixtures'

describe('api client', () => {
  beforeEach(() => {
    resetAuthHooks()
  })

  afterEach(() => {
    resetAuthHooks()
    vi.restoreAllMocks()
  })

  it('usa /api/v1 como baseURL por defecto', () => {
    expect(api.defaults.baseURL).toBe('/api/v1')
  })

  it('adjunta el Authorization Bearer cuando hay access token en memoria', async () => {
    const seen = { value: null as string | null }
    configureAuthHooks({ getAccessToken: () => 'access-xyz', refresh: vi.fn<() => Promise<void>>() })
    server.use(get('/me', (info) => {
      seen.value = info.request.headers.get('Authorization')
      return json({ user: makeUser() })
    }))

    await api.get('/me')
    expect(seen.value).toBe('Bearer access-xyz')
  })

  it('no manda Authorization cuando no hay access token', async () => {
    const seen = { value: 'sentinel' as string | null }
    configureAuthHooks({ getAccessToken: () => null, refresh: vi.fn<() => Promise<void>>() })
    server.use(get('/me', (info) => {
      seen.value = info.request.headers.get('Authorization')
      return json({ user: makeUser() })
    }))

    await api.get('/me')
    expect(seen.value).toBeNull()
  })

  describe('parseErrorPayload', () => {
    it('parsea el envelope {"error":{code,message,details}}', () => {
      const err = parseErrorPayload(errorEnvelope('not_found', 'customer not found', { id: 7 }), 404)
      expect(err).toBeInstanceOf(ApiError)
      expect(err.status).toBe(404)
      expect(err.code).toBe('not_found')
      expect(err.message).toBe('customer not found')
      expect(err.details).toEqual({ id: 7 })
    })

    it('parsea el error plano {"error":"mensaje"} de los endpoints de auth', () => {
      const err = parseErrorPayload(flatError('invalid email or password'), 401)
      expect(err.code).toBe('unauthorized')
      expect(err.message).toBe('invalid email or password')
    })

    it('usa el estado HTTP como fallback de code/message', () => {
      const err = parseErrorPayload({ unexpected: true }, 500)
      expect(err.code).toBe('internal_error')
      expect(err.message.length).toBeGreaterThan(0)
    })

    it('marca network_error cuando no hay respuesta', () => {
      const err = parseErrorPayload(undefined, 0)
      expect(err.code).toBe('network_error')
      expect(err.status).toBe(0)
    })
  })

  it('convierte el envelope de error del backend en ApiError', async () => {
    server.use(get('/customers/999', () => json(errorEnvelope('not_found', 'customer not found'), 404)))

    const promise = api.get('/customers/999')
    await expect(promise).rejects.toBeInstanceOf(ApiError)
    await expect(api.get('/customers/999').catch((e: ApiError) => e)).resolves.toMatchObject({
      status: 404,
      code: 'not_found',
      message: 'customer not found',
    })
  })

  it('convierte el error plano de auth en ApiError', async () => {
    server.use(post('/auth/login', () => json(flatError('invalid email or password'), 401)))

    const err = (await api.post('/auth/login', {}).catch((e: ApiError) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(401)
    expect(err.message).toBe('invalid email or password')
  })

  it('reporta network_error si el backend no responde', async () => {
    server.use(http.get('*/api/v1/me', () => HttpResponse.error()))

    const err = (await api.get('/me').catch((e: ApiError) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('network_error')
  })

  it('reintenta la request original tras un 401 si el refresh funciona', async () => {
    let token = 'old-access'
    let meCalls = 0
    server.use(
      post('/auth/refresh', () => json(makeSession(makeUser(), makeTokens({ access_token: 'new-access' })))),
      get('/me', () => {
        meCalls += 1
        return meCalls === 1 ? json(flatError('unauthorized'), 401) : json({ user: makeUser() })
      }),
    )
    const refresh = vi.fn<() => Promise<void>>(async () => {
      token = 'new-access'
    })
    const onAuthFailure = vi.fn<() => void>()
    configureAuthHooks({ getAccessToken: () => token, refresh, onAuthFailure })

    const res = await api.get('/me')
    expect(res.status).toBe(200)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(onAuthFailure).not.toHaveBeenCalled()
  })

  it('no reintenta más de una vez si el retry vuelve a devolver 401', async () => {
    server.use(get('/me', () => json(flatError('unauthorized'), 401)))
    const refresh = vi.fn<() => Promise<void>>().mockResolvedValue(undefined)
    const onAuthFailure = vi.fn<() => void>()
    configureAuthHooks({ getAccessToken: () => 'access', refresh, onAuthFailure })

    const err = await api.get('/me').catch((e: ApiError) => e)
    expect(err.status).toBe(401)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(onAuthFailure).not.toHaveBeenCalled()
  })

  it('si el refresh falla, limpia la sesión y propaga el error', async () => {
    server.use(
      post('/auth/refresh', () => json(flatError('invalid or expired refresh token'), 401)),
      get('/me', () => json(flatError('unauthorized'), 401)),
    )
    const onAuthFailure = vi.fn<() => void>()
    const refresh = vi.fn<() => Promise<void>>().mockRejectedValue(
      new ApiError({ status: 401, code: 'unauthorized', message: 'invalid or expired refresh token' }),
    )
    configureAuthHooks({ getAccessToken: () => 'stale', refresh, onAuthFailure })

    const err = await api.get('/me').catch((e: ApiError) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(401)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(onAuthFailure).toHaveBeenCalledTimes(1)
  })

  it('las requests marcadas con withoutRefresh no disparan el refresh', async () => {
    const refresh = vi.fn<() => Promise<void>>()
    configureAuthHooks({ getAccessToken: () => 'stale', refresh, onAuthFailure: vi.fn<() => void>() })
    server.use(post('/auth/refresh', () => json(flatError('invalid or expired refresh token'), 401)))

    const err = await api
      .post('/auth/refresh', { refresh_token: 'x' }, withoutRefresh())
      .catch((e: ApiError) => e)
    expect(err.status).toBe(401)
    expect(refresh).not.toHaveBeenCalled()
  })

  it('sin hooks de auth configurados, un 401 se propaga tal cual', async () => {
    server.use(get('/me', () => json(flatError('unauthorized'), 401)))
    const err = await api.get('/me').catch((e: ApiError) => e)
    expect(err.status).toBe(401)
  })
})
