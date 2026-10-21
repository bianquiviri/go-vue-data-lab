import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { NAV_ITEMS } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'
import type { UserRole } from '@/types/auth'

/**
 * Secciones que YA tienen vista propia (Issue #20). El router NO genera
 * placeholders para ellas: se declaran explícitamente para controlar el orden
 * (p. ej. `/orders/:id` no debe taparse con nada) y el componente/slot.
 *
 * A medida que los Issues #21-#24 lleguen, las entradas se moverán de los
 * placeholders generados a esta lista.
 */
const IMPLEMENTED_ROUTES = new Set(['customers', 'products', 'orders', 'target'])

/** Placeholder del dashboard: es la única ruta que no sale de NAV_ITEMS. */
const PLACEHOLDER_ITEMS = NAV_ITEMS.filter((item) => !IMPLEMENTED_ROUTES.has(item.routeName))

const titleKeyOf = (name: string): string | undefined =>
  NAV_ITEMS.find((item) => item.routeName === name)?.titleKey

export const APP_ROUTES: RouteRecordRaw[] = [
  {
    // Landing pública (Issue #39): SIEMPRE visible en `/`, con o sin sesión.
    path: '/',
    component: () => import('@/layouts/PublicLayout.vue'),
    meta: { public: true },
    children: [
      {
        path: '',
        name: 'landing',
        component: () => import('@/views/LandingView.vue'),
        // `titleKey`: clave i18n (no texto) para que el título sea reactivo al idioma.
        meta: { public: true, titleKey: 'landing.tagline' },
      },
    ],
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, titleKey: 'auth.title' },
  },
  {
    // Panel protegido. El padre vive en `/` para que las secciones conserven sus
    // URLs (/customers, /jobs, ...); el dashboard ocupa la subruta `dashboard`.
    // `/` en sí no tiene hijo directo ⇒ resuelve en la landing pública de arriba.
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { titleKey: 'nav.dashboard.title' },
      },

      // --- Vistas reales de datos (Issue #20, solo lectura) ---
      // `scope`: esquema del backend; el `id` de la ruta es el token opaco.
      {
        path: 'customers',
        name: 'customers',
        component: () => import('@/views/CustomersView.vue'),
        props: { scope: 'source' },
        meta: { titleKey: titleKeyOf('customers') },
      },
      {
        path: 'products',
        name: 'products',
        component: () => import('@/views/ProductsView.vue'),
        props: { scope: 'source' },
        meta: { titleKey: titleKeyOf('products') },
      },
      {
        path: 'orders',
        name: 'orders',
        component: () => import('@/views/OrdersView.vue'),
        props: { scope: 'source' },
        meta: { titleKey: titleKeyOf('orders') },
      },
      {
        // `/orders/:id` es más específica y se registra junto al listado; el
        // orden dentro del mismo padre no importa (vue-router prioriza por
        // especificidad del path, no por orden de declaración).
        path: 'orders/:id',
        name: 'order-detail',
        component: () => import('@/views/OrderDetailView.vue'),
        props: { scope: 'source' },
        meta: { titleKey: titleKeyOf('orders') },
      },

      // --- Target: clientes/productos/órdenes procesados (Issue #20) ---
      {
        path: 'target',
        name: 'target',
        component: () => import('@/views/TargetView.vue'),
        meta: { titleKey: titleKeyOf('target') },
      },
      {
        // Detalle de orden de lab_target: mantiene el usuario en el esquema
        // procesado (el listado de target enlaza aquí).
        path: 'target/orders/:id',
        name: 'target-order-detail',
        component: () => import('@/views/OrderDetailView.vue'),
        props: { scope: 'target' },
        meta: { titleKey: titleKeyOf('target') },
      },

      // --- Secciones aún placeholder (Issues #21-#24) ---
      // Se generan desde NAV_ITEMS (excepto dashboard y las ya implementadas).
      ...PLACEHOLDER_ITEMS.map((item) => ({
        path: item.to.replace(/^\//, ''),
        name: item.routeName,
        component: () => import('@/views/PlaceholderView.vue'),
        meta: {
          titleKey: item.titleKey,
          descriptionKey: item.descriptionKey,
          ...(item.roles ? { roles: item.roles satisfies UserRole[] } : {}),
        },
      })),
    ],
  },
  {
    // Catch-all: redirige a `/`, que es la landing pública (exista o no sesión).
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

export function createAppRouter() {
  const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes: APP_ROUTES,
  })

  router.beforeEach(async (to) => {
    const auth = useAuthStore()
    if (!auth.initialized) {
      await auth.initialize()
    }

    const requiresAuth = to.matched.some((record) => record.meta.requiresAuth === true)
    if (requiresAuth && !auth.isAuthenticated) {
      return { name: 'login', query: { redirect: to.fullPath } }
    }

    // La landing se sirve siempre en `/` (con o sin sesión): sin redirect.
    if (to.name === 'login' && auth.isAuthenticated) {
      return typeof to.query.redirect === 'string' ? to.query.redirect : { name: 'dashboard' }
    }

    const roles = to.meta.roles as UserRole[] | undefined
    if (roles && !roles.includes(auth.role ?? ('viewer' as UserRole))) {
      return { name: 'dashboard' }
    }
  })

  return router
}

export default createAppRouter()
