import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { REFRESH_TOKEN_STORAGE_KEY } from '@/lib/tokenStorage'
import { useAuthStore } from '../auth'
import { server } from '@/__tests__/mocks/server'
import { errorEnvelope, flatError, get, json, makeSession, makeTokens, makeUser, post } from '@/__tests__/fixtures'

describe('stores/auth', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('arranca sin sesión', () => {
    const store = useAuthStore()
    expect(store.isAuthenticated).toBe(false)
    expect(store.user).toBeNull()
    expect(store.accessToken).toBeNull()
    expect(store.initialized).toBe(false)
    expect(store.role).toBeNull()
    expect(store.isAdmin).toBe(false)
  })

  it('login guarda el access token en memoria, el refresh en storage y el usuario', async () => {
    const session = makeSession(makeUser({ role: 'operator', email: 'op@x.io' }))
    server.use(post('/auth/login', () => json(session)))
    const store = useAuthStore()

    const user = await store.login({ email: 'op@x.io', password: 'pw' })
    expect(user).toEqual(session.user)
    expect(store.isAuthenticated).toBe(true)
    expect(store.user?.email).toBe('op@x.io')
    expect(store.role).toBe('operator')
    expect(store.accessToken).toBe('access-token-1')
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBe('refresh-token-1')
  })

  it('login con credenciales inválidas expone el mensaje del backend', async () => {
    server.use(post('/auth/login', () => json(flatError('invalid email or password'), 401)))
    const store = useAuthStore()

    await expect(store.login({ email: 'a@b.c', password: 'bad' })).rejects.toMatchObject({ status: 401 })
    expect(store.error).toBe('invalid email or password')
    expect(store.isAuthenticated).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('refleja el estado de loading durante el login', async () => {
    let release: (value: Response) => void = () => {}
    let markReceived: () => void = () => {}
    const received = new Promise<void>((resolve) => {
      markReceived = resolve
    })
    server.use(
      post(
        '/auth/login',
        () =>
          new Promise<Response>((resolve) => {
            release = resolve
            markReceived()
          }),
      ),
    )
    const store = useAuthStore()
    const promise = store.login({ email: 'a@b.c', password: 'x' })
    expect(store.loading).toBe(true)
    await received
    release(json(makeSession()))
    await promise
    expect(store.loading).toBe(false)
  })

  it('initialize sin refresh token marca initialized sin llamar a la API', async () => {
    const store = useAuthStore()
    await store.initialize()
    expect(store.initialized).toBe(true)
    expect(store.isAuthenticated).toBe(false)
  })

  it('initialize refresca y carga el usuario usando el refresh token guardado', async () => {
    localStorage.setItem(REFRESH_TOKEN_STORAGE_KEY, 'stored-rt')
    const user = makeUser()
    let refreshBody: unknown = null
    server.use(
      post('/auth/refresh', async (info) => {
        refreshBody = await info.request.json()
        return json(makeSession(user))
      }),
      get('/auth/me', () => json({ user })),
    )
    const store = useAuthStore()

    await store.initialize()
    expect(store.isAuthenticated).toBe(true)
    expect(store.user).toEqual(user)
    expect(refreshBody).toEqual({ refresh_token: 'stored-rt' })
  })

  it('initialize es idempotente (solo refresca una vez)', async () => {
    localStorage.setItem(REFRESH_TOKEN_STORAGE_KEY, 'stored-rt')
    let refreshCalls = 0
    server.use(
      post('/auth/refresh', () => {
        refreshCalls += 1
        return json(makeSession())
      }),
      get('/auth/me', () => json({ user: makeUser() })),
    )
    const store = useAuthStore()

    await store.initialize()
    await store.initialize()
    expect(refreshCalls).toBe(1)
  })

  it('initialize limpia la sesión si el refresh falla', async () => {
    localStorage.setItem(REFRESH_TOKEN_STORAGE_KEY, 'expired-rt')
    server.use(post('/auth/refresh', () => json(flatError('invalid or expired refresh token'), 401)))
    const store = useAuthStore()

    await store.initialize()
    expect(store.initialized).toBe(true)
    expect(store.isAuthenticated).toBe(false)
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('un 401 en una request autenticada dispara refresh y reintenta la original', async () => {
    const user = makeUser()
    server.use(post('/auth/login', () => json(makeSession(user))))
    const store = useAuthStore()
    await store.login({ email: user.email, password: 'pw' })

    let meCalls = 0
    server.use(
      get('/auth/me', () => {
        meCalls += 1
        return meCalls === 1 ? json(flatError('unauthorized'), 401) : json({ user })
      }),
      post('/auth/refresh', () => json(makeSession(user, makeTokens({ access_token: 'renewed' })))),
    )

    await store.fetchMe()
    expect(meCalls).toBe(2)
    expect(store.accessToken).toBe('renewed')
    expect(store.user).toEqual(user)
  })

  it('logout llama al backend, limpia el estado y borra el storage', async () => {
    const store = useAuthStore()
    server.use(post('/auth/login', () => json(makeSession())))
    await store.login({ email: 'a@b.c', password: 'pw' })

    let logoutCalled = false
    server.use(
      post('/auth/logout', () => {
        logoutCalled = true
        return json({ message: 'logged out' })
      }),
    )
    await store.logout()
    expect(logoutCalled).toBe(true)
    expect(store.isAuthenticated).toBe(false)
    expect(store.user).toBeNull()
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('logout es best-effort: si el backend falla, la sesión local se limpia igual', async () => {
    const store = useAuthStore()
    server.use(post('/auth/login', () => json(makeSession())))
    await store.login({ email: 'a@b.c', password: 'pw' })

    server.use(
      post('/auth/logout', () => json(flatError('unauthorized'), 401)),
      post('/auth/refresh', () => json(flatError('invalid or expired refresh token'), 401)),
    )
    await expect(store.logout()).resolves.toBeUndefined()
    expect(store.isAuthenticated).toBe(false)
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('logout sin sesión no lanza ni llama al backend', async () => {
    const store = useAuthStore()
    await expect(store.logout()).resolves.toBeUndefined()
  })

  it('changePassword envía las credenciales y devuelve el mensaje', async () => {
    const store = useAuthStore()
    let body: unknown = null
    server.use(
      post('/auth/change-password', async (info) => {
        body = await info.request.json()
        return json({ message: 'password updated' })
      }),
    )

    await expect(store.changePassword('old-pass-1', 'new-pass-2')).resolves.toBe('password updated')
    expect(body).toEqual({ old_password: 'old-pass-1', new_password: 'new-pass-2' })
  })

  it('changePassword propaga el error del backend', async () => {
    const store = useAuthStore()
    server.use(post('/auth/change-password', () => json(flatError('current password is incorrect'), 401)))

    await expect(store.changePassword('wrong', 'new-pass-2')).rejects.toMatchObject({
      status: 401,
      message: 'current password is incorrect',
    })
  })

  it('fetchMe propaga errores del backend sin romper el store', async () => {
    const store = useAuthStore()
    server.use(post('/auth/login', () => json(makeSession())), get('/auth/me', () => json(errorEnvelope('internal_error', 'boom'), 500)))
    await store.login({ email: 'a@b.c', password: 'pw' })

    await expect(store.fetchMe()).rejects.toMatchObject({ status: 500 })
    expect(store.isAuthenticated).toBe(true)
  })

  it('agenda un refresh automático antes de que expire el access token', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    server.use(post('/auth/login', () => json(makeSession(makeUser(), makeTokens({ expires_in: 3600 })))))
    const store = useAuthStore()
    await store.login({ email: 'a@b.c', password: 'pw' })

    const spy = vi.spyOn(store, 'refreshTokens').mockResolvedValue(makeSession())
    // El refresh se agenda con 60 s de margen antes de la expiración (3600 s).
    await vi.advanceTimersByTimeAsync(3600 * 1000 - 60_000 - 1)
    expect(spy).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(51)
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('logout cancela el refresh automático programado', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    server.use(post('/auth/login', () => json(makeSession(makeUser(), makeTokens({ expires_in: 3600 })))))
    const store = useAuthStore()
    await store.login({ email: 'a@b.c', password: 'pw' })
    server.use(post('/auth/logout', () => json({ message: 'logged out' })))

    const spy = vi.spyOn(store, 'refreshTokens').mockResolvedValue(makeSession())
    await store.logout()
    await vi.advanceTimersByTimeAsync(3600 * 1000)
    expect(spy).not.toHaveBeenCalled()
  })
})
