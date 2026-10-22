import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import TargetView from '../TargetView.vue'
import { server } from '@/__tests__/mocks/server'
import { get, json, pagedEnvelope, useLocale } from '@/__tests__/fixtures'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'
import type { Customer } from '@/types/data'

const CUSTOMER: Customer = {
  id: 'v1.customer-opaque-token',
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  country_code: 'GB',
  city: 'London',
  created_at: '2026-01-01T00:00:00Z',
}

async function mountTarget() {
  const auth = useAuthStore()
  auth.user = makeUser()
  auth.accessToken = 'access-token'
  auth.initialized = true

  const router = createAppRouter()
  await router.push('/target')
  await router.isReady()

  const wrapper = mount(TargetView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper }
}

describe('views/TargetView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('arranca en la pestaña de clientes y consulta /target/customers', async () => {
    const paths: string[] = []
    server.use(
      get('/target/customers', (info) => {
        paths.push(new URL(info.request.url).pathname)
        return json(pagedEnvelope([CUSTOMER]))
      }),
    )
    const { wrapper } = await mountTarget()

    expect(paths[0]).toBe('/api/v1/target/customers')
    expect(wrapper.text()).toContain('ada@example.com')
  })

  it('cambia a productos y consulta /target/products', async () => {
    const paths: string[] = []
    server.use(
      get('/target/customers', () => json(pagedEnvelope([CUSTOMER]))),
      get('/target/products', (info) => {
        paths.push(new URL(info.request.url).pathname)
        return json(pagedEnvelope([]))
      }),
    )
    const { wrapper } = await mountTarget()

    await wrapper.get('[data-test="target-tab-products"]').trigger('click')
    await flushPromises()

    expect(paths).toContain('/api/v1/target/products')
  })

  it('cambia a órdenes y consulta /target/orders', async () => {
    const paths: string[] = []
    server.use(
      get('/target/customers', () => json(pagedEnvelope([CUSTOMER]))),
      get('/target/orders', (info) => {
        paths.push(new URL(info.request.url).pathname)
        return json(pagedEnvelope([]))
      }),
    )
    const { wrapper } = await mountTarget()

    await wrapper.get('[data-test="target-tab-orders"]').trigger('click')
    await flushPromises()

    expect(paths).toContain('/api/v1/target/orders')
  })

  it('marca la pestaña activa con aria-selected', async () => {
    server.use(get('/target/customers', () => json(pagedEnvelope([]))))
    const { wrapper } = await mountTarget()

    expect(wrapper.get('[data-test="target-tab-customers"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[data-test="target-tab-products"]').attributes('aria-selected')).toBe('false')
  })
})
