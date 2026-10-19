import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'

async function mountDashboard(role: 'admin' | 'viewer' = 'admin') {
  const auth = useAuthStore()
  auth.user = makeUser({ role })
  auth.accessToken = 'access-token'
  auth.initialized = true

  const router = createAppRouter()
  await router.push('/dashboard')
  await router.isReady()

  const wrapper = mount(DashboardView, {
    global: { plugins: [router] },
  })
  return { wrapper }
}

describe('views/DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('muestra el saludo con el email del usuario', async () => {
    const { wrapper } = await mountDashboard()
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('root')
  })

  it('renderiza una tarjeta por sección visible y enlaza a cada una', async () => {
    const { wrapper } = await mountDashboard('admin')
    const links = wrapper.findAll('a')
    // NAV_ITEMS sin el dashboard: customers, products, orders, target, jobs, scheduler, admin-logs
    expect(links).toHaveLength(7)
    const hrefs = links.map((link) => link.attributes('href'))
    expect(hrefs).toContain('/customers')
    expect(hrefs).toContain('/jobs')
    expect(hrefs).toContain('/admin/logs')
  })

  it('para un viewer oculta la tarjeta de Admin logs', async () => {
    const { wrapper } = await mountDashboard('viewer')
    const hrefs = wrapper.findAll('a').map((link) => link.attributes('href'))
    expect(hrefs).not.toContain('/admin/logs')
  })
})
