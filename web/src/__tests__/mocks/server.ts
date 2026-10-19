import { setupServer } from 'msw/node'

/**
 * Servidor MSW compartido por los tests. Cada spec añade sus handlers con
 * `server.use(...)`; se limpian automáticamente tras cada test (ver setup.ts).
 */
export const server = setupServer()
