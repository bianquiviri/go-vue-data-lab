import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { NAV_ITEMS } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'
import type { UserRole } from '@/types/auth'

export const APP_ROUTES: RouteRecordRaw[] = [
  {
    // Landing pública (Issue #39): SIEMPRE visible en `/`, con o sin sesión.
    // El guard no la redirige; el CTA de la vista se adapta a la sesión.
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
        meta: { titleKey: 'nav.dashboard.title' },
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
