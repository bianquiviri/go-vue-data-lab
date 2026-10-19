import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { NAV_ITEMS } from '@/config/nav'
import { useAuthStore } from '@/stores/auth'
import type { UserRole } from '@/types/auth'

export const APP_ROUTES: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, title: 'Iniciar sesión' },
  },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { title: 'Dashboard' },
      },
      // Secciones placeholder (Issues #20-#24); reutilizan PlaceholderView.
      ...NAV_ITEMS.filter((item) => item.to !== '/').map((item) => ({
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
