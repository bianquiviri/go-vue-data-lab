import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { NAV_ITEMS } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'
import type { UserRole } from '@/types/auth'

export const APP_ROUTES: RouteRecordRaw[] = [
  {
    // Landing pública (Issue #39): solo para visitantes. Un usuario con
    // sesión que abra `/` es redirigido al dashboard en el guard.
    path: '/',
    component: () => import('@/layouts/PublicLayout.vue'),
    meta: { public: true },
    children: [
      {
        path: '',
        name: 'landing',
        component: () => import('@/views/LandingView.vue'),
        meta: { public: true, title: 'Go Data Lab — Concurrent Data Processing' },
      },
    ],
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, title: 'Iniciar sesión' },
  },
  {
    // Panel protegido. El padre vive en `/` para que las secciones placeholder
    // conserven sus URLs (/customers, /jobs, ...); el dashboard ocupa la
    // subruta `dashboard` (/dashboard). `/` en sí no tiene hijo directo ⇒
    // resuelve en la landing pública de arriba.
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { title: 'Dashboard' },
      },
      // Secciones placeholder (Issues #20-#24); reutilizan PlaceholderView.
      // El item Dashboard de NAV_ITEMS NO genera ruta propia: la ruta declarada
      // arriba es la única. Se filtra por routeName (no por `to`) para que el
      // filtro siga siendo correcto aunque cambie la ruta del dashboard.
      ...NAV_ITEMS.filter((item) => item.routeName !== 'dashboard').map((item) => ({
        path: item.to.replace(/^\//, ''),
        name: item.routeName,
        component: () => import('@/views/PlaceholderView.vue'),
        meta: {
          title: item.title,
          description: item.description,
          ...(item.roles ? { roles: item.roles satisfies UserRole[] } : {}),
        },
      })),
    ],
  },
  {
    // Catch-all: redirige a `/` y el guard resuelve según sesión
    // (sin sesión ⇒ landing; con sesión ⇒ /dashboard).
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

    // La landing es solo para visitantes: con sesión, `/` va al dashboard.
    if (to.name === 'landing' && auth.isAuthenticated) {
      return { name: 'dashboard' }
    }

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
