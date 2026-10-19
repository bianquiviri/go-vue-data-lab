import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { changePasswordRequest, loginRequest, logoutRequest, meRequest, refreshRequest } from '../auth'
import { configureAuthHooks, resetAuthHooks } from '../client'
import { server } from '@/__tests__/mocks/server'
import { flatError, get, json, makeSession, makeUser, post } from '@/__tests__/fixtures'
import type { AuthSession, User } from '@/types/auth'

describe('api/auth', () => {
  beforeEach(() => {
    resetAuthHooks()
  })

  afterEach(() => {
    resetAuthHooks()
  })

  it('loginRequest envía email/password y devuelve la sesión', async () => {
    const session = makeSession()
    let body: unknown = null
    server.use(post('/auth/login', async (info) => {
      body = await info.request.json()
      return json(session)
    }))

    const result = await loginRequest({ email: 'root@gmail.com', password: 'secret-123' })
    expect(result).toEqual(session)
    expect(body).toEqual({ email: 'root@gmail.com', password: 'secret-123' })
  })

  it('refreshRequest envía refresh_token en snake_case y no dispara el refresh del interceptor', async () => {
    const session = makeSession()
    let body: unknown = null
    server.use(post('/auth/refresh', async (info) => {
      body = await info.request.json()
      return json(session)
    }))

    const result = await refreshRequest('rt-1')
    expect(result).toEqual(session)
    expect(body).toEqual({ refresh_token: 'rt-1' })
  })

  it('refreshRequest no entra en bucle con el interceptor ante un 401', async () => {
    const refresh = vi.fn<() => Promise<void>>()
    configureAuthHooks({ getAccessToken: () => 'stale', refresh })
    server.use(post('/auth/refresh', () => json(flatError('invalid or expired refresh token'), 401)))

    await expect(refreshRequest('bad-token')).rejects.toMatchObject({ status: 401 })
    expect(refresh).not.toHaveBeenCalled()
  })

  it('meRequest desenvuelve {"user":...}', async () => {
    const user = makeUser()
    server.use(get('/auth/me', () => json({ user })))

    const result: User = await meRequest()
    expect(result).toEqual(user)
  })

  it('changePasswordRequest envía old/new password y devuelve el mensaje', async () => {
    let body: unknown = null
    server.use(post('/auth/change-password', async (info) => {
      body = await info.request.json()
      return json({ message: 'password updated' })
    }))

    const result = await changePasswordRequest({ old_password: 'old-pass-1', new_password: 'new-pass-2' })
    expect(result).toBe('password updated')
    expect(body).toEqual({ old_password: 'old-pass-1', new_password: 'new-pass-2' })
  })

  it('logoutRequest llama a POST /auth/logout y devuelve el mensaje', async () => {
    let called = false
    server.use(post('/auth/logout', () => {
      called = true
      return json({ message: 'logged out' })
    }))

    const result = await logoutRequest()
    expect(called).toBe(true)
    expect(result).toBe('logged out')
  })

  it('propaga los ApiError del backend', async () => {
    server.use(post('/auth/login', () => json(flatError('invalid email or password'), 401)))
    await expect(loginRequest({ email: 'a@b.c', password: 'nope' })).rejects.toMatchObject({
      status: 401,
      message: 'invalid email or password',
    })
  })

  it('el tipo AuthSession refleja el contrato del backend', () => {
    const session: AuthSession = makeSession()
    expect(session.tokens.token_type).toBe('Bearer')
    expect(typeof session.tokens.expires_in).toBe('number')
  })
})
