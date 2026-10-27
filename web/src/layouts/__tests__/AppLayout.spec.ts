import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AppLayout from '../AppLayout.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { THEME_STORAGE_KEY, useThemeStore } from '@/stores/theme'
import { installMatchMedia, json, makeUser, post, useLocale } from '@/__tests__/fixtures'
import { server } from '@/__tests__/mocks/server'

/** Seed del store + router autenticado para montar el layout. */
async function mountLayout(role: 'admin' | 'operator' | 'viewer' = 'admin') {
  const auth = useAuthStore()
  auth.user = makeUser({ role })
  auth.accessToken = 'access-token'
  auth.initialized = true

  const router = createAppRouter()
  await router.push('/dashboard')
  await router.isReady()

  const wrapper = mount(AppLayout, {
    global: { plugins: [router] },
  })
  return { wrapper, router, auth }
}

describe('layouts/AppLayout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    installMatchMedia(false)
    // Español determinista: los títulos de nav y el rol se afirman en ES.
    useLocale('es')
  })

  it('muestra el footer compartido con licencia CC y autoría', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.find('footer[data-test="app-footer"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="footer-license"]').text()).toContain('CC BY-NC-ND 4.0')
    expect(wrapper.get('[data-test="footer-author"]').text()).toContain('Jose Antonio Bianco')
  })

  afterEach(() => {
    document.documentElement.classList.remove('dark')
    localStorage.clear()
  })

  it('muestra la navegación visible para el rol', async () => {
    const { wrapper } = await mountLayout('viewer')
    const links = wrapper.findAll('[data-test="sidebar"] a')
    const titles = links.map((link) => link.text())

    expect(titles).toContain('Customers')
    expect(titles).toContain('Jobs')
    // admin solo es visible para admin (RBAC en nav.ts)
    expect(titles).not.toContain('Admin logs')  })

  it('un admin ve también Admin logs', async () => {
    const { wrapper } = await mountLayout('admin')
    const titles = wrapper.findAll('[data-test="sidebar"] a').map((link) => link.text())
    expect(titles).toContain('Admin')
  })

  it('muestra el email y el rol del usuario en el header', async () => {
    const { wrapper } = await mountLayout('viewer')
    expect(wrapper.get('[data-test="user-email"]').text()).toBe('root@gmail.com')
    expect(wrapper.get('[data-test="user-role"]').text()).toBe('Observador')
  })

  it('ofrece el selector de idioma junto al de tema en el header', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.find('[data-test="language-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-es"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-en"]').exists()).toBe(true)
  })

  it('cambiar de idioma traduce la navegación y el rol de inmediato', async () => {
    const { wrapper } = await mountLayout('viewer')

    await wrapper.get('[data-test="locale-en"]').trigger('click')

    expect(wrapper.get('[data-test="user-role"]').text()).toBe('Viewer')
    const menuButton = wrapper.findAll('button').find((b) => b.text().includes('Open menu'))
    expect(menuButton).toBeDefined()
  })

  it('el toggle de tema aplica Oscuro y lo persiste', async () => {
    const { wrapper } = await mountLayout()

    await wrapper.get('[data-test="theme-dark"]').trigger('click')

    const theme = useThemeStore()
    expect(theme.mode).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('el toggle de tema vuelve a Sistema', async () => {
    const { wrapper } = await mountLayout()
    const theme = useThemeStore()

    await wrapper.get('[data-test="theme-dark"]').trigger('click')
    await wrapper.get('[data-test="theme-system"]').trigger('click')

    expect(theme.mode).toBe('system')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('logout limpia la sesión y navega a /login', async () => {
    server.use(post('/auth/logout', () => json({ message: 'sesión cerrada' })))
    const { wrapper, router, auth } = await mountLayout()

    await wrapper.get('[data-test="logout"]').trigger('click')

    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('login'))
    expect(auth.isAuthenticated).toBe(false)
    expect(auth.user).toBeNull()
  })

  it('en móvil el botón de menú abre y cierra el sidebar', async () => {
    const { wrapper } = await mountLayout()
    const menuButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Abrir menú'))
    expect(menuButton).toBeDefined()

    await menuButton!.trigger('click')
    expect(wrapper.find('[data-test="sidebar-overlay"]').exists()).toBe(true)

    await wrapper.get('[data-test="sidebar-overlay"]').trigger('click')
    expect(wrapper.find('[data-test="sidebar-overlay"]').exists()).toBe(false)
  })
})
