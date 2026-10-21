import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import PlaceholderView from '../PlaceholderView.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser, useLocale } from '@/__tests__/fixtures'

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
    // Español determinista: descripción, badge y fallbacks se afirman en ES.
    useLocale('es')
  })

  it('parametriza el título y la descripción desde meta de la ruta', async () => {
    const { wrapper, router } = await mountPlaceholder('/customers')
    expect(router.currentRoute.value.name).toBe('customers')
    expect(wrapper.text()).toContain('Customers')
    expect(wrapper.text()).toContain('Catálogo de clientes (lab_source)')
  })

  it('en inglés traduce el badge y el párrafo de forma reactiva', async () => {
    const { wrapper } = await mountPlaceholder('/customers')
    expect(wrapper.text()).toContain('Próximamente')

    // Cambia el idioma y comprueba que el texto se re-renderiza sin re-montar.
    useLocale('en')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Coming soon')
    expect(wrapper.text()).toContain('Issues #20–#24')
    // El nombre de la sección es igual en ambos idiomas.
    expect(wrapper.text()).toContain('Customers')
  })

  it('marca la sección como próximamente', async () => {
    const { wrapper } = await mountPlaceholder('/jobs')
    expect(wrapper.text()).toContain('Jobs')
    expect(wrapper.text()).toContain('Próximamente')
    expect(wrapper.text()).toContain('Issues #20')
  })

  it('cae al item de NAV_ITEMS si la ruta no trae claves de meta', async () => {
    // Ruta con un name que SÍ está en NAV_ITEMS pero sin meta.titleKey.
    const auth = useAuthStore()
    auth.user = makeUser()
    auth.accessToken = 'access-token'
    auth.initialized = true

    const router = createAppRouter()
    router.addRoute({
      path: '/bare-scheduler',
      name: 'scheduler',
      component: PlaceholderView,
    })
    await router.push('/bare-scheduler')
    await router.isReady()

    const wrapper = mount(PlaceholderView, { global: { plugins: [router] } })
    // Título y descripción salen del item de NAV_ITEMS (claves i18n).
    expect(wrapper.text()).toContain('Scheduler')
    expect(wrapper.text()).toContain('Programación de tareas cron')
  })

  it('usa el fallback absoluto cuando no hay meta ni entrada en NAV_ITEMS', async () => {
    // Ruta huérfana: sin meta y con un name que no existe en NAV_ITEMS.
    const auth = useAuthStore()
    auth.user = makeUser()
    auth.accessToken = 'access-token'
    auth.initialized = true

    const router = createAppRouter()
    router.addRoute({
      path: '/orphan-section',
      name: 'orphan-section',
      component: PlaceholderView,
    })
    await router.push('/orphan-section')
    await router.isReady()

    const wrapper = mount(PlaceholderView, { global: { plugins: [router] } })

    expect(wrapper.text()).toContain('Sección')
    // Sin descripción ⇒ el párrafo existe pero vacío.
    expect(wrapper.find('p').exists()).toBe(true)
  })
})
