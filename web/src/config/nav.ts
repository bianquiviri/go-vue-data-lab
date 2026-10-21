import {
  BarChart3,
  CalendarClock,
  Cpu,
  LayoutDashboard,
  Package,
  ShieldAlert,
  ShoppingCart,
  Users,
  type LucideIcon,
} from 'lucide-vue-next'

import type { UserRole } from '@/types/auth'

/**
 * Item de navegación. `titleKey`/`descriptionKey` son claves i18n (no texto):
 * así `nav.ts` es datos puros sin depender de la instancia de vue-i18n y los
 * consumidores resuelven con `t()` en tiempo de render (reactivo al idioma).
 */
export interface NavItem {
  titleKey: string
  /** Nombre de la ruta (usado por el router y los links activos). */
  routeName: string
  to: string
  icon: LucideIcon
  descriptionKey: string
  /** Roles que pueden ver el item; vacío = todos los roles autenticados. */
  roles?: UserRole[]
}

/**
 * Navegación del layout. Las secciones de datos/monitoreo son placeholders
 * hasta los Issues #20-#24.
 */
export const NAV_ITEMS: NavItem[] = [
  {
    titleKey: 'nav.dashboard.title',
    routeName: 'dashboard',
    to: '/dashboard',
    icon: LayoutDashboard,
    descriptionKey: 'nav.dashboard.description',
  },
  {
    titleKey: 'nav.customers.title',
    routeName: 'customers',
    to: '/customers',
    icon: Users,
    descriptionKey: 'nav.customers.description',
  },
  {
    titleKey: 'nav.products.title',
    routeName: 'products',
    to: '/products',
    icon: Package,
    descriptionKey: 'nav.products.description',
  },
  {
    titleKey: 'nav.orders.title',
    routeName: 'orders',
    to: '/orders',
    icon: ShoppingCart,
    descriptionKey: 'nav.orders.description',
  },
  {
    titleKey: 'nav.target.title',
    routeName: 'target',
    to: '/target',
    icon: BarChart3,
    descriptionKey: 'nav.target.description',
  },
  {
    titleKey: 'nav.jobs.title',
    routeName: 'jobs',
    to: '/jobs',
    icon: Cpu,
    descriptionKey: 'nav.jobs.description',
  },
  {
    titleKey: 'nav.scheduler.title',
    routeName: 'scheduler',
    to: '/scheduler',
    icon: CalendarClock,
    descriptionKey: 'nav.scheduler.description',
  },
  {
    titleKey: 'nav.adminLogs.title',
    routeName: 'admin-logs',
    to: '/admin/logs',
    icon: ShieldAlert,
    descriptionKey: 'nav.adminLogs.description',
    roles: ['admin'],
  },
]

/** Filtra los items de navegación visibles para un rol. */
export function visibleNavItems(role: UserRole | null): NavItem[] {
  return NAV_ITEMS.filter((item) => !item.roles || (role !== null && item.roles.includes(role)))
}
