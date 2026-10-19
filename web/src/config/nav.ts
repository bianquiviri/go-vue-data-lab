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

export interface NavItem {
  title: string
  /** Nombre de la ruta (usado por el router y los links activos). */
  routeName: string
  to: string
  icon: LucideIcon
  description: string
  /** Roles que pueden ver el item; vacío = todos los roles autenticados. */
  roles?: UserRole[]
}

/**
 * Navegación del layout. Las secciones de datos/monitoreo son placeholders
 * hasta los Issues #20-#24.
 */
export const NAV_ITEMS: NavItem[] = [
  {
    title: 'Dashboard',
    routeName: 'dashboard',
    to: '/dashboard',
    icon: LayoutDashboard,
    description: 'Resumen general del laboratorio',
  },
  {
    title: 'Customers',
    routeName: 'customers',
    to: '/customers',
    icon: Users,
    description: 'Catálogo de clientes (lab_source)',
  },
  {
    title: 'Products',
    routeName: 'products',
    to: '/products',
    icon: Package,
    description: 'Catálogo de productos (lab_source)',
  },
  {
    title: 'Orders',
    routeName: 'orders',
    to: '/orders',
    icon: ShoppingCart,
    description: 'Órdenes e items (lab_source)',
  },
  {
    title: 'Target / Analytics',
    routeName: 'target',
    to: '/target',
    icon: BarChart3,
    description: 'Datos procesados y analítica (lab_target)',
  },
  {
    title: 'Jobs',
    routeName: 'jobs',
    to: '/jobs',
    icon: Cpu,
    description: 'Control y progreso de jobs en tiempo real',
  },
  {
    title: 'Scheduler',
    routeName: 'scheduler',
    to: '/scheduler',
    icon: CalendarClock,
    description: 'Programación de tareas cron',
  },
  {
    title: 'Admin logs',
    routeName: 'admin-logs',
    to: '/admin/logs',
    icon: ShieldAlert,
    description: 'Logs de error del backend (solo admin)',
    roles: ['admin'],
  },
]

/** Filtra los items de navegación visibles para un rol. */
export function visibleNavItems(role: UserRole | null): NavItem[] {
  return NAV_ITEMS.filter((item) => !item.roles || (role !== null && item.roles.includes(role)))
}
