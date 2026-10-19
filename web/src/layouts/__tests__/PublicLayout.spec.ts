import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import PublicLayout from '../PublicLayout.vue'

/** Monta PublicLayout dentro de un router mínimo con la landing y /login. */
async function mountPublicLayout() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/',
        component: PublicLayout,
        children: [{ path: '', name: 'landing', component: { template: '<div data-test="slot-content">Hero</div>' } }],
      },
      { path: '/login', name: 'login', component: { template: '<div />' } },
    ],
  })
  router.push('/')
  await router.isReady()

  const wrapper = mount(PublicLayout, {
    global: { plugins: [router] },
    slots: { default: '<div data-test="slot-content">Hero</div>' },
  })
  return { wrapper }
}

describe('layouts/PublicLayout', () => {
  it('muestra el nombre del proyecto en el header', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.get('[data-test="public-brand"]').text()).toContain('Go Data Lab')
  })

  it('el botón Sign in enlaza a /login', async () => {
    const { wrapper } = await mountPublicLayout()
    const signIn = wrapper.get('[data-test="public-signin"]')
    expect(signIn.attributes('href')).toBe('/login')
    expect(signIn.text()).toContain('Sign in')
  })

  it('renderiza un footer simple', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('[data-test="public-footer"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Concurrent Data Processing')
  })

  it('renderiza el contenido del slot', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.get('[data-test="slot-content"]').text()).toBe('Hero')
  })

  it('no asume sesión: no muestra email, rol ni logout', async () => {
    const { wrapper } = await mountPublicLayout()
    expect(wrapper.find('[data-test="user-email"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="logout"]').exists()).toBe(false)
  })
})
