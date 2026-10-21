import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'

import PublicLayout from '../PublicLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { makeUser, useLocale } from '@/__tests__/fixtures'

/**
 * Monta la ruta pública (`/` ⇒ PublicLayout + hijo) dentro de un router
 * mínimo y devuelve el wrapper del RouterView raíz. `authenticated` siembra
 * el auth store: el acceso del header depende de él.
 */
async function mountPublicLayout({ authenticated = false } = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)

  if (authenticated) {
    const auth = useAuthStore()
    auth.user = makeUser()
    auth.accessToken = 'access-token'
    auth.initialized = true
  }

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/',
        component: PublicLayout,
        children: [
          {
            path: '',
            name: 'landing',
            component: { template: '<div data-test="slot-content">Hero</div>' },
          },
        ],
      },
      { path: '/login', name: 'login', component: { template: '<div />' } },
      { path: '/dashboard', name: 'dashboard', component: { template: '<div />' } },
    ],
  })
  router.push('/')
  await router.isReady()

  const wrapper = mount(RouterView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('layouts/PublicLayout', () => {
  beforeEach(() => {
    localStorage.clear()
    // Inglés determinista para las cadenas del CTA.
    useLocale('en')
  })

  it('muestra el nombre del proyecto en el header', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.get('[data-test="public-brand"]').text()).toContain('Go Data Lab')
  })

  it('sin sesión el botón del header invita a iniciar sesión', async () => {
    const { wrapper } = await mountPublicLayout()
    const cta = wrapper.get('[data-test="public-cta"]')
    expect(cta.text()).toContain('Sign in')
    expect(cta.attributes('href')).toBe('/login')
  })

  it('con sesión el botón del header lleva al dashboard', async () => {
    const { wrapper } = await mountPublicLayout({ authenticated: true })
    const cta = wrapper.get('[data-test="public-cta"]')
    expect(cta.text()).toContain('Go to dashboard')
    expect(cta.attributes('href')).toBe('/dashboard')
  })

  it('renderiza el footer con licencia CC y autoría', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('footer[data-test="app-footer"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Concurrent Data Processing')
    // Footer compartido: licencia + autor presentes en TODAS las vistas
    expect(wrapper.get('[data-test="footer-license"]').text()).toContain('CC BY-NC-ND 4.0')
    expect(wrapper.get('[data-test="footer-author"]').text()).toContain('Jose Antonio Bianco')
  })

  it('ofrece el toggle de tema (Claro/Oscuro/Sistema) en el header público', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('[data-test="theme-light"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-dark"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-system"]').exists()).toBe(true)
  })

  it('ofrece el selector de idioma ES/EN en el header público', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('[data-test="language-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-es"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-en"]').exists()).toBe(true)
  })

  it('cambiar a ES traduce el CTA y el footer al instante', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.get('[data-test="public-cta"]').text()).toContain('Sign in')

    await wrapper.get('[data-test="locale-es"]').trigger('click')

    expect(wrapper.get('[data-test="public-cta"]').text()).toContain('Iniciar sesión')
    expect(wrapper.get('[data-test="footer-project"]').text()).toContain('plataforma sintética')
  })

  it('renderiza el contenido de la ruta hija', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.get('[data-test="slot-content"]').text()).toBe('Hero')
  })

  it('no asume sesión: no muestra email, rol ni logout', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('[data-test="user-email"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="logout"]').exists()).toBe(false)
  })
})
