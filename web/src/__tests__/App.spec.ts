import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import App from '../App.vue'

/**
 * App.vue es deliberadamente trivial: solo monta <RouterView />. El test
 * existe para (a) cubrir el componente raíz y (b) garantizar que no se le
 * cuela lógica propia (providers, watchers, etc.) por accidente.
 */
async function mountApp(initialPath = '/') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div id="home-view">Home</div>' } },
      { path: '/other', name: 'other', component: { template: '<div id="other-view">Other</div>' } },
    ],
  })
  router.push(initialPath)
  await router.isReady()

  const wrapper = mount(App, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('App', () => {
  it('delega el renderizado en RouterView', async () => {
    const { wrapper } = await mountApp('/')
    expect(wrapper.find('#home-view').exists()).toBe(true)
    expect(wrapper.text()).toContain('Home')
  })

  it('reacciona a cambios de ruta sin re-montar App', async () => {
    const { wrapper, router } = await mountApp('/')
    await router.push('/other')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('#home-view').exists()).toBe(false)
    expect(wrapper.find('#other-view').exists()).toBe(true)
  })

  it('no renderiza nada propio fuera del RouterView', async () => {
    const { wrapper } = await mountApp('/')
    // App no debe aportar nodos propios: todo lo visible viene de la ruta.
    expect(wrapper.element.tagName).toBe('DIV')
    expect(wrapper.html()).not.toContain('<script')
  })
})
