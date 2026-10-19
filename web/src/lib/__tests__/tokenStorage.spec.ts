import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  REFRESH_TOKEN_STORAGE_KEY,
  createLocalStorageTokenStorage,
  createMemoryTokenStorage,
  tokenStorage,
} from '../tokenStorage'

describe('tokenStorage (localStorage)', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('guarda y recupera el refresh token', () => {
    const storage = createLocalStorageTokenStorage()
    expect(storage.getRefreshToken()).toBeNull()

    storage.setRefreshToken('rt-123')
    expect(storage.getRefreshToken()).toBe('rt-123')
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBe('rt-123')
  })

  it('borra el refresh token', () => {
    const storage = createLocalStorageTokenStorage()
    storage.setRefreshToken('rt-123')
    storage.clear()
    expect(storage.getRefreshToken()).toBeNull()
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('respeta la clave personalizada', () => {
    const storage = createLocalStorageTokenStorage('custom.key')
    storage.setRefreshToken('rt-x')
    expect(localStorage.getItem('custom.key')).toBe('rt-x')
    expect(localStorage.getItem(REFRESH_TOKEN_STORAGE_KEY)).toBeNull()
  })

  it('no lanza si localStorage falla (modo privado)', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceeded')
    })
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })
    const removeItem = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })

    const storage = createLocalStorageTokenStorage()
    expect(() => storage.setRefreshToken('rt')).not.toThrow()
    expect(storage.getRefreshToken()).toBeNull()
    expect(() => storage.clear()).not.toThrow()

    setItem.mockRestore()
    getItem.mockRestore()
    removeItem.mockRestore()
  })

  it('expone una instancia por defecto basada en localStorage', () => {
    tokenStorage.setRefreshToken('default-rt')
    expect(tokenStorage.getRefreshToken()).toBe('default-rt')
    tokenStorage.clear()
  })
})

describe('tokenStorage (en memoria)', () => {
  it('funciona sin persistir', () => {
    const storage = createMemoryTokenStorage()
    expect(storage.getRefreshToken()).toBeNull()
    storage.setRefreshToken('rt-mem')
    expect(storage.getRefreshToken()).toBe('rt-mem')
    storage.clear()
    expect(storage.getRefreshToken()).toBeNull()
  })
})
