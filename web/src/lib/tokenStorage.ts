/**
 * Capa de persistencia del refresh token, aislada del resto de la app para
 * poder sustituirla (cookies httpOnly, memoria, backend) sin tocar el store.
 *
 * El access token NUNCA se persiste aquí: vive solo en memoria.
 */

export const REFRESH_TOKEN_STORAGE_KEY = 'gdl.refresh_token'

export interface TokenStorage {
  getRefreshToken(): string | null
  setRefreshToken(token: string): void
  clear(): void
}

/** Implementación sobre localStorage, tolerante a cuotas/bloqueo (modo privado). */
export function createLocalStorageTokenStorage(key: string = REFRESH_TOKEN_STORAGE_KEY): TokenStorage {
  return {
    getRefreshToken() {
      try {
        return window.localStorage.getItem(key)
      } catch {
        return null
      }
    },
    setRefreshToken(token: string) {
      try {
        window.localStorage.setItem(key, token)
      } catch {
        // localStorage no disponible: la sesión será de un solo uso en memoria.
      }
    },
    clear() {
      try {
        window.localStorage.removeItem(key)
      } catch {
        // ignorar
      }
    },
  }
}

/** Implementación en memoria (tests y contextos sin storage). */
export function createMemoryTokenStorage(): TokenStorage {
  let token: string | null = null
  return {
    getRefreshToken: () => token,
    setRefreshToken: (value: string) => {
      token = value
    },
    clear: () => {
      token = null
    },
  }
}

/** Instancia por defecto usada por el store de auth. */
export const tokenStorage: TokenStorage = createLocalStorageTokenStorage()
