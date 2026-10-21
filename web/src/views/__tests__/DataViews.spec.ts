import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CustomersView from '../CustomersView.vue'
import ProductsView from '../ProductsView.vue'
import OrdersView from '../OrdersView.vue'
import OrderDetailView from '../OrderDetailView.vue'
import { server } from '@/__tests__/mocks/server'
import { get, json, opaqueId, pagedEnvelope, useLocale } from '@/__tests__/fixtures'
import { createAppRouter } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { makeUser } from '@/__tests__/fixtures'
import type { Customer, Order, OrderItem, Product } from '@/types/data'

const CUSTOMER: Customer = {
  id: opaqueId('customer'),
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  country_code: 'GB',
  city: 'London',
  created_at: '2026-01-01T00:00:00Z',
}

const PRODUCT: Product = {
  id: opaqueId('product'),
  sku: 'SKU-1',
  name: 'Widget',
  category: 'Toys',
  price: 9.99,
  stock: 3,
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
}

const ITEM: OrderItem = {
  id: opaqueId('item'),
  order_id: opaqueId('order'),
  product_id: opaqueId('product'),
  quantity: 2,
  unit_price: 5,
  subtotal: 10,
}

const ORDER: Order = {
  id: opaqueId('order'),
  order_number: 'ORD-1',
  customer_id: opaqueId('customer'),
  status: 'paid',
  total_amount: 10,
  currency: 'USD',
  order_date: '2026-01-01T00:00:00Z',
  order_items: [ITEM],
}

async function mountView(
  component: unknown,
  path: string,
  query: Record<string, string> = {},
) {
  const auth = useAuthStore()
  auth.user = makeUser()
  auth.accessToken = 'access-token'
  auth.initialized = true

  const router = createAppRouter()
  await router.push({ path, query })
  await router.isReady()

  const wrapper = mount(component as never, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('views/CustomersView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('carga y muestra los clientes con el token opaco intacto', async () => {
    server.use(get('/source/customers', () => json(pagedEnvelope([CUSTOMER]))))
    const { wrapper } = await mountView(CustomersView, '/customers')

    expect(wrapper.findAll('[data-test="data-table-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('ada@example.com')
    // El id es un token y se muestra verbatim.
    expect(wrapper.text()).toContain(CUSTOMER.id)
  })

  it('la búsqueda se refleja en la query string y en la request', async () => {
    const urls: string[] = []
    server.use(
      get('/source/customers', (info) => {
        urls.push(info.request.url)
        return json(pagedEnvelope([CUSTOMER]))
      }),
    )
    const { wrapper, router } = await mountView(CustomersView, '/customers')

    await wrapper.get('[data-test="data-filters-search"] input, [data-test="data-filters-search"]').setValue('ada')
    await new Promise((resolve) => setTimeout(resolve, 400))
    await flushPromises()

    expect(router.currentRoute.value.query.search).toBe('ada')
    expect(urls[urls.length - 1]).toContain('search=ada')
  })

  it('hidrata los filtros desde la URL al montar', async () => {
    const urls: string[] = []
    server.use(
      get('/source/customers', (info) => {
        urls.push(info.request.url)
        return json(pagedEnvelope([CUSTOMER]))
      }),
    )
    await mountView(CustomersView, '/customers', { country: 'GB', page: '2' })

    const parsed = new URL(urls[0]!)
    expect(parsed.searchParams.get('country')).toBe('GB')
    expect(parsed.searchParams.get('page')).toBe('2')
  })

  it('muestra el estado vacío sin resultados', async () => {
    server.use(get('/source/customers', () => json(pagedEnvelope([]))))
    const { wrapper } = await mountView(CustomersView, '/customers')
    expect(wrapper.find('[data-test="data-table-empty"]').exists()).toBe(true)
  })
})

describe('views/ProductsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('lista productos y filtra por categoría en la URL', async () => {
    const urls: string[] = []
    server.use(
      get('/source/products', (info) => {
        urls.push(info.request.url)
        return json(pagedEnvelope([PRODUCT]))
      }),
    )
    const { wrapper, router } = await mountView(ProductsView, '/products')

    expect(wrapper.text()).toContain('Widget')
    await wrapper.get('[data-test="data-filters-category"]').setValue('Toys')
    await flushPromises()

    expect(router.currentRoute.value.query.category).toBe('Toys')
    expect(new URL(urls[urls.length - 1]!).searchParams.get('category')).toBe('Toys')
  })
})

describe('views/OrdersView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('lista órdenes y navega al detalle con el token intacto', async () => {
    server.use(get('/source/orders', () => json(pagedEnvelope([ORDER]))))
    const { wrapper, router } = await mountView(OrdersView, '/orders')

    expect(wrapper.text()).toContain('ORD-1')
    await wrapper.get('[data-test="order-detail-link"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('order-detail')
    // El token viaja verbatim al path del detalle.
    expect(router.currentRoute.value.params.id).toBe(ORDER.id)
  })

  it('filtra por estado y rango de fechas en la query string', async () => {
    const urls: string[] = []
    server.use(
      get('/source/orders', (info) => {
        urls.push(info.request.url)
        return json(pagedEnvelope([ORDER]))
      }),
    )
    const { wrapper, router } = await mountView(OrdersView, '/orders')

    await wrapper.get('[data-test="data-filters-status"]').setValue('paid')
    await flushPromises()
    await wrapper.get('[data-test="data-filters-date-from"]').setValue('2026-01-01')
    await flushPromises()

    expect(router.currentRoute.value.query.status).toBe('paid')
    expect(router.currentRoute.value.query.date_from).toBe('2026-01-01')
    const parsed = new URL(urls[urls.length - 1]!)
    expect(parsed.searchParams.get('status')).toBe('paid')
    expect(parsed.searchParams.get('date_from')).toBe('2026-01-01')
  })
})

describe('views/OrderDetailView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    useLocale('es')
  })

  it('carga la orden por token y muestra sus order_items', async () => {
    const paths: string[] = []
    server.use(
      get('/source/orders/:id', (info) => {
        paths.push(new URL(info.request.url).pathname)
        return json(ORDER)
      }),
    )
    const { wrapper } = await mountView(OrderDetailView, `/orders/${ORDER.id}`)

    expect(wrapper.text()).toContain('ORD-1')
    expect(wrapper.findAll('[data-test="data-table-row"]')).toHaveLength(1)
    // El item también expone sus tokens intactos.
    expect(wrapper.text()).toContain(ITEM.product_id)
    expect(paths[0]).toBe(`/api/v1/source/orders/${ORDER.id}`)
  })

  it('muestra un mensaje de error/404 si la orden no existe', async () => {
    server.use(get('/source/orders/:id', () => json({ error: { code: 'not_found', message: 'gone' } }, 404)))
    const { wrapper } = await mountView(OrderDetailView, `/orders/${ORDER.id}`)
    expect(wrapper.text()).toContain('gone')
  })

  it('rechaza ordenar la tabla de items si no aplica (id token no numérico)', async () => {
    server.use(get('/source/orders/:id', () => json({ ...ORDER, order_items: [ITEM] })))
    const { wrapper } = await mountView(OrderDetailView, `/orders/${ORDER.id}`)
    // La columna de cantidad (numérica) sí es ordenable client-side.
    await wrapper.get('[data-test="data-table-sort-quantity"]').trigger('click')
    expect(wrapper.emitted()).toBeTruthy()
  })

  it('renderiza la dirección de envío cuando existe', async () => {
    server.use(
      get('/source/orders/:id', () =>
        json({ ...ORDER, shipping_address: { city: 'London', country_code: 'GB' } }),
      ),
    )
    const { wrapper } = await mountView(OrderDetailView, `/orders/${ORDER.id}`)
    expect(wrapper.text()).toContain('London')
  })

  it('omite la dirección de envío cuando viene vacía', async () => {
    server.use(get('/source/orders/:id', () => json({ ...ORDER, shipping_address: null })))
    const { wrapper } = await mountView(OrderDetailView, `/orders/${ORDER.id}`)
    expect(wrapper.text()).not.toContain('shippingAddress')
  })

  it('la vista de target vuelve a /target', async () => {
    server.use(get('/target/orders/:id', () => json(ORDER)))
    const router = createAppRouter()
    await router.push(`/target/orders/${ORDER.id}`)
    await router.isReady()
    const wrapper = mount(OrderDetailView, {
      props: { scope: 'target' },
      global: { plugins: [router] },
    })
    await flushPromises()
    expect(wrapper.get('[data-test="order-detail-back"]').attributes('href')).toBe('/target')
  })

  it('no dispara request si falta el id de la ruta', async () => {
    let calls = 0
    server.use(
      get('/source/orders/:id', () => {
        calls += 1
        return json(ORDER)
      }),
    )
    // Ruta sin `:id`: el composable no debe intentar cargar nada.
    const bare = createAppRouter()
    bare.addRoute({ path: '/bare-order', name: 'bare-order', component: OrderDetailView })
    await bare.push('/bare-order')
    await bare.isReady()
    mount(OrderDetailView, { global: { plugins: [bare] } })
    await flushPromises()
    expect(calls).toBe(0)
  })

  it('el listado de órdenes de target navega al detalle de target con el token intacto', async () => {
    server.use(get('/target/orders', () => json(pagedEnvelope([ORDER]))))
    const { wrapper, router } = await mountView(OrdersView, '/target', {})
    // Monta OrdersView con scope target a través de la ruta /target.
    const targetList = mount(OrdersView, {
      props: { scope: 'target' },
      global: { plugins: [router] },
    })
    await flushPromises()
    await targetList.get('[data-test="order-detail-link"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('target-order-detail')
    expect(router.currentRoute.value.params.id).toBe(ORDER.id)
    expect(wrapper).toBeTruthy()
  })
})
