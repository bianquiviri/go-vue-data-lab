import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import LoginView from '../LoginView.vue'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { errorEnvelope, flatError, json, makeSession, post, useLocale } from '@/__tests__/fixtures'
import { server } from '@/__tests__/mocks/server'

/** Monta LoginView dentro del router real (para query.redirect y guards). */
async function mountLogin(query: Record<string, string> = {}) {
  const router = createAppRouter()
  await router.push({ path: '/login', query })
  await router.isReady()
  const wrapper = mount(LoginView, {
    global: { plugins: [router] },
  })
  return { wrapper, router }
}

describe('views/LoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    // Español determinista: las aserciones de errores/etiquetas van en ES.
    useLocale('es')
  })

  it('renderiza el formulario de email y contraseña', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.find('input#email').exists()).toBe(true)
    expect(wrapper.find('input#password').exists()).toBe(true)
    expect(wrapper.find('button[type="submit"]').exists()).toBe(true)
  })

  it('ofrece el selector de idioma junto al de tema en el login', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.find('[data-test="language-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-es"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="locale-en"]').exists()).toBe(true)
  })

  it('cambiar a inglés traduce el formulario de inmediato', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.text()).toContain('Iniciar sesión')

    await wrapper.get('[data-test="locale-en"]').trigger('click')

    expect(wrapper.text()).toContain('Sign in')
    expect(wrapper.get('button[type="submit"]').text()).toContain('Sign in')
  })

  it('muestra el footer compartido con licencia CC y autoría', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.find('footer[data-test="app-footer"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="footer-license"]').text()).toContain('CC BY-NC-ND 4.0')
    expect(wrapper.get('[data-test="footer-author"]').text()).toContain('Jose Antonio Bianco')
  })

  it('ofrece el toggle de tema (Claro/Oscuro/Sistema) en el login', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.find('[data-test="theme-light"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-dark"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="theme-system"]').exists()).toBe(true)
  })

  it('no envía el formulario si faltan campos y muestra errores', async () => {
    const loginSpy = vi.spyOn(useAuthStore(), 'login')
    const { wrapper } = await mountLogin()

    await wrapper.find('form').trigger('submit')

    expect(loginSpy).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="email-error"]').text()).toContain('obligatorio')
    expect(wrapper.get('[data-test="password-error"]').text()).toContain('obligatoria')
  })

  it('valida el formato del email', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.find('input#email').setValue('no-es-email')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')

    expect(wrapper.get('[data-test="email-error"]').text()).toContain('formato')
  })

  it('con credenciales válidas inicia sesión y redirige a query.redirect', async () => {
    server.use(post('/auth/login', () => json(makeSession())))
    const { wrapper, router } = await mountLogin({ redirect: '/jobs' })

    await wrapper.find('input#email').setValue('root@gmail.com')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('jobs'))

    const auth = useAuthStore()
    expect(auth.isAuthenticated).toBe(true)
    expect(auth.user?.email).toBe('root@gmail.com')
  })

  it('sin query.redirect vuelve al dashboard', async () => {
    server.use(post('/auth/login', () => json(makeSession())))
    const { wrapper, router } = await mountLogin()

    await wrapper.find('input#email').setValue('root@gmail.com')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')
    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('dashboard'))
  })

  it('muestra el mensaje del envelope de error con credenciales inválidas', async () => {
    server.use(post('/auth/login', () => json(flatError('credenciales inválidas'), 401)))
    const { wrapper } = await mountLogin()

    await wrapper.find('input#email').setValue('root@gmail.com')
    await wrapper.find('input#password').setValue('mala')
    await wrapper.find('form').trigger('submit')

    await vi.waitFor(() =>
      expect(wrapper.get('[data-test="server-error"]').text()).toBe('credenciales inválidas'),
    )
    expect(useAuthStore().isAuthenticated).toBe(false)
  })

  it('muestra el mensaje del envelope estructurado {"error":{...}}', async () => {
    server.use(
      post('/auth/login', () =>
        json(errorEnvelope('internal_error', 'algo salió mal'), 500),
      ),
    )
    const { wrapper } = await mountLogin()

    await wrapper.find('input#email').setValue('root@gmail.com')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')

    await vi.waitFor(() =>
      expect(wrapper.get('[data-test="server-error"]').text()).toBe('algo salió mal'),
    )
  })

  it('deshabilita el botón mientras la petición está en curso', async () => {
    let release: (() => void) | undefined
    server.use(
      post(
        '/auth/login',
        () =>
          new Promise<Response>((resolve) => {
            release = () => resolve(json(makeSession()))
          }),
      ),
    )
    const { wrapper } = await mountLogin()

    await wrapper.find('input#email').setValue('root@gmail.com')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')

    await vi.waitFor(() => {
      const button = wrapper.get<HTMLButtonElement>('button[type="submit"]')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.text()).toContain('Entrando')
    })
    release?.()
  })
})
