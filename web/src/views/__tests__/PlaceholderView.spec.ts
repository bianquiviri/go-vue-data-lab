import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import PlaceholderView from '../PlaceholderView.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'

async function mountPlaceholder(path: string) {
  const auth = useAuthStore()
  auth.user = makeUser()
  auth.accessToken = 'access-token'
  auth.initialized = true

  const router = createAppRouter()
  await router.push(path)
  await router.isReady()

  const wrapper = mount(PlaceholderView, {
    global: { plugins: [router] },
  })
  return { wrapper, router }
}

describe('views/PlaceholderView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('parametriza el título y la descripción desde meta de la ruta', async () => {
    const { wrapper, router } = await mountPlaceholder('/customers')
    expect(router.currentRoute.value.name).toBe('customers')
    expect(wrapper.text()).toContain('Customers')
    expect(wrapper.text()).toContain('Catálogo de clientes (lab_source)')
  })

  it('marca la sección como próximamente', async () => {
    const { wrapper } = await mountPlaceholder('/jobs')
    expect(wrapper.text()).toContain('Jobs')
    expect(wrapper.text()).toContain('Próximamente')
    expect(wrapper.text()).toContain('Issues #20')
  })

  it('usa valores por defecto si la ruta no trae meta de nav', async () => {
    // Cubre el fallback: se monta la vista con un route simulado sin meta.
    const { router } = await mountPlaceholder('/scheduler')
    router.currentRoute.value.meta.title = undefined
    const wrapper = mount(PlaceholderView, { global: { plugins: [router] } })
    // name 'scheduler' sí está en NAV_ITEMS ⇒ cae al icono/título del item.
    expect(wrapper.text()).toContain('Scheduler')
  })
})
