import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import LandingView from '../LandingView.vue'
import { useAuthStore } from '@/stores/auth'
import { makeUser, useLocale } from '@/__tests__/fixtures'

/**
 * Monta la landing con un router mínimo (no depende de los guards reales) y un
 * pinia propio: el CTA depende de `useAuthStore().isAuthenticated`.
 */
async function mountLanding({ authenticated = false } = {}) {
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
      { path: '/', name: 'landing', component: LandingView },
      { path: '/login', name: 'login', component: { template: '<div />' } },
      { path: '/dashboard', name: 'dashboard', component: { template: '<div />' } },
    ],
  })
  router.push('/')
  await router.isReady()

  const wrapper = mount(LandingView, { global: { plugins: [pinia, router] } })
  return { wrapper, router }
}

describe('views/LandingView', () => {
  beforeEach(() => {
    localStorage.clear()
    // Inglés determinista: la landing es la vista que estaba en inglés y sus
    // aserciones se escribieron contra ese idioma.
    useLocale('en')
  })

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

  // --- CTA según sesión ---

  it('sin sesión el CTA invita a iniciar sesión', async () => {
    const { wrapper } = await mountLanding()
    const cta = wrapper.get('[data-test="landing-cta"]')
    expect(cta.text()).toContain('Sign in')
    expect(cta.attributes('href')).toBe('/login')
  })

  it('con sesión el CTA lleva al dashboard', async () => {
    const { wrapper } = await mountLanding({ authenticated: true })
    const cta = wrapper.get('[data-test="landing-cta"]')
    expect(cta.text()).toContain('Go to dashboard')
    expect(cta.attributes('href')).toBe('/dashboard')
  })

  // --- Sección "What is this?" (Issue #39) ---

  it('explica qué es el sistema en la sección "What is this?"', async () => {
    const { wrapper } = await mountLanding()
    const section = wrapper.get('[data-test="what-is-this"]')
    const text = section.text()

    expect(section.get('[data-test="what-is-this-title"]').text()).toBe('What is this?')
    expect(text).toContain('synthetic e-commerce model')
    expect(text).toContain('customers, products, orders and order items')
    // Para qué sirve: simular volúmenes masivos y ver el pipeline completo.
    expect(text).toContain('simulate massive volumes')
    expect(text).toContain('from ingestion to analytics')
  })

  it('describe el pipeline lab_source → ETL → lab_target', async () => {
    const { wrapper } = await mountLanding()
    const steps = wrapper.findAll('[data-test="pipeline-step"]')
    expect(steps).toHaveLength(3)

    const text = wrapper.get('[data-test="what-is-this"]').text()
    expect(text).toContain('lab_source')
    expect(text).toContain('lab_target')
    expect(text).toContain('resumable ETL')
    expect(text.toLowerCase()).toContain('checkpoint')
    // Dos esquemas PostgreSQL independientes.
    expect(text).toContain('Two independent PostgreSQL schemas')
  })

  it('resume qué se puede hacer con el laboratorio (simulaciones, scheduler, API)', async () => {
    const { wrapper } = await mountLanding()
    const text = wrapper.get('[data-test="capabilities"]').text()

    expect(text).toContain('3,000–4,000 records')
    expect(text.toLowerCase()).toContain('cron')
    expect(text).toContain('/api/v1')
    // API documentada en Swagger y protegida con RBAC.
    expect(text).toContain('Swagger')
    expect(text).toContain('RBAC')
    expect(text).toContain('admin, operator, viewer')
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

  it('el copy de la landing cambia de idioma de forma reactiva (EN → ES)', async () => {
    const { wrapper } = await mountLanding()
    expect(wrapper.text()).toContain('Sign in')
    expect(wrapper.get('[data-test="what-is-this-title"]').text()).toBe('What is this?')

    useLocale('es')
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Iniciar sesión')
    expect(wrapper.text()).not.toContain('Sign in')
    expect(wrapper.get('[data-test="what-is-this-title"]').text()).toBe('¿Qué es esto?')
    expect(wrapper.text()).toContain('clientes, productos, órdenes e items')
    // La estructura (claves/schema) se mantiene estable entre idiomas.
    expect(wrapper.text()).toContain('lab_source')
    expect(wrapper.text()).toContain('lab_target')
  })

  it('en español el CTA sin sesión invita a iniciar sesión', async () => {
    useLocale('es')
    const { wrapper } = await mountLanding()
    const cta = wrapper.get('[data-test="landing-cta"]')
    expect(cta.text()).toContain('Iniciar sesión')
    expect(cta.attributes('href')).toBe('/login')
  })
})
