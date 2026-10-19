import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import LandingView from '../LandingView.vue'

/** Monta la landing con un router mínimo (no depende de los guards reales). */
async function mountLanding() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'landing', component: LandingView },
      { path: '/login', name: 'login', component: { template: '<div />' } },
    ],
  })
  router.push('/')
  await router.isReady()

  const wrapper = mount(LandingView, { global: { plugins: [router] } })
  return { wrapper }
}

describe('views/LandingView', () => {
  it('muestra el nombre del proyecto y su tagline', async () => {
    const { wrapper } = await mountLanding()
    expect(wrapper.get('[data-test="landing-title"]').text()).toContain('Go Data Lab')
    expect(wrapper.text()).toContain('Concurrent Data Processing')
  })

  it('describe el propósito del laboratorio', async () => {
    const { wrapper } = await mountLanding()
    const text = wrapper.text().toLowerCase()
    expect(text).toContain('concurrent')
    expect(text).toContain('synthetic')
    expect(text).toContain('analytics')
  })

  it('el CTA principal enlaza a /login', async () => {
    const { wrapper } = await mountLanding()
    const cta = wrapper.get('[data-test="cta-login"]')
    expect(cta.attributes('href')).toBe('/login')
    expect(cta.text()).toContain('Sign in')
  })

  it('lista las seis features del laboratorio', async () => {
    const { wrapper } = await mountLanding()
    const features = wrapper.findAll('[data-test="feature"]')
    expect(features).toHaveLength(6)

    const text = wrapper.text()
    expect(text).toContain('worker pool')
    expect(text.toLowerCase()).toContain('scheduler')
    expect(text.toLowerCase()).toContain('etl')
    expect(text).toContain('RBAC')
    expect(text).toContain('Swagger')
    expect(text.toLowerCase()).toContain('opaque')
  })

  it('el copy de la landing está en inglés', async () => {
    const { wrapper } = await mountLanding()
    const text = wrapper.text()
    expect(text).not.toContain('Iniciar sesión')
    expect(text).not.toContain('Bienvenido')
    expect(text).toContain('Sign in')
  })
})
