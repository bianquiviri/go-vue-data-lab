import { deleteUser, listUsers } from '@/api/admin'
import { useDataList } from '@/composables/useDataList'
import type { AdminUser, AdminUserListParams } from '@/types/admin'

/**
 * Listado de usuarios del panel de admin (Issue #23).
 *
 * Envuelve `useDataList` (patrón del Issue #20) para que búsqueda (`q`), filtros
 * (`role`, `active`) y paginación vivan en la **query string** de `/admin`:
 * la URL es compartible y sobrevive a un reload.
 *
 * El backend ordena de forma fija por `created_at DESC`, así que no hay orden
 * client-side: las filas se renderizan en el orden recibido.
 */
export function useAdminUsers() {
  const list = useDataList<AdminUser>(
    (params) => listUsers(params as AdminUserListParams),
    // `q` (no `search`): es el nombre del parámetro de GET /admin/users.
    { filterKeys: ['q', 'role', 'active'] },
  )

  /**
   * Borra un usuario y recarga la página actual.
   *
   * No se hace borrado optimista: los guards del backend (auto-borrado, último
   * admin) pueden rechazar la operación con 409 y la tabla debe seguir mostrando
   * la verdad del servidor. El llamador traduce el error.
   */
  async function remove(id: string): Promise<void> {
    await deleteUser(id)
    await list.refresh()
  }

  return { ...list, remove }
}
