/**
 * Mensajes en inglés — idioma por defecto y `fallbackLocale`.
 *
 * `en.ts` es la FUENTE DE VERDAD del esquema: el tipo `MessageSchema` derivado
 * de este objeto se usa en `es.ts` para que TypeScript marque cualquier clave
 * que falte o sobre en la traducción al español.
 */
const en = {
  common: {
    brand: 'Go Data Lab',
    projectName: 'Go Concurrent Data Processing Lab',
  },
  nav: {
    dashboard: {
      title: 'Dashboard',
      description: 'Laboratory overview',
    },
    customers: {
      title: 'Customers',
      description: 'Customer catalog (lab_source)',
    },
    products: {
      title: 'Products',
      description: 'Product catalog (lab_source)',
    },
    orders: {
      title: 'Orders',
      description: 'Orders and items (lab_source)',
    },
    target: {
      title: 'Target / Analytics',
      description: 'Processed data and analytics (lab_target)',
    },
    jobs: {
      title: 'Jobs',
      description: 'Real-time job control and progress',
    },
    scheduler: {
      title: 'Scheduler',
      description: 'Cron task scheduling',
    },
    adminLogs: {
      title: 'Admin logs',
      description: 'Backend error logs (admin only)',
    },
    sections: 'Sections',
  },
  auth: {
    title: 'Sign in',
    subtitle: 'Access the Go Concurrent Data Processing Lab',
    email: 'Email',
    password: 'Password',
    emailRequired: 'Email is required',
    emailInvalid: 'Email format is not valid',
    passwordRequired: 'Password is required',
    submitting: 'Signing in…',
    submit: 'Sign in',
  },
  dashboard: {
    title: 'Dashboard',
    greeting: 'Hi, {name}. This is the concurrent data processing lab.',
    defaultUser: 'user',
  },
  placeholder: {
    upcoming: 'Coming soon',
    fallbackTitle: 'Section',
    body: 'This section is planned for Issues #20–#24 (data tables, job monitoring, scheduler, admin and analytics). Meanwhile, the rest of the dashboard is already available.',
  },
  publicHeader: {
    signIn: 'Sign in',
    goToDashboard: 'Go to dashboard',
  },
  footer: {
    project: 'Go Concurrent Data Processing Lab — synthetic e-commerce data platform.',
    developedBy: 'Developed by Jose Antonio Bianco',
    licensedUnder: 'Licensed under',
  },
  theme: {
    label: 'Theme',
    light: 'Light',
    dark: 'Dark',
    system: 'System',
  },
  language: {
    label: 'Language',
    es: 'ES',
    en: 'EN',
    esFull: 'Español',
    enFull: 'English',
  },
  landing: {
    tagline: 'Go Concurrent Data Processing Lab',
    hero: 'Concurrent data processing, massive simulation and real-time analytics over a synthetic e-commerce dataset — customers, products, orders and order items, from ingestion to insight.',
    overview: 'Overview',
    whatIsThisTitle: 'What is this?',
    whatIsThisP1:
      'Go Data Lab is a laboratory for exploring concurrent data processing in Go over a synthetic e-commerce model — customers, products, orders and order items. Every dataset is generated on demand inside a real PostgreSQL database, at whatever volume the experiment requires.',
    whatIsThisP2:
      'Its purpose is to simulate massive volumes while keeping the whole pipeline visible, from ingestion to analytics, so concurrency strategies, batching and resumability can be measured end to end on data you control.',
    pipelineTitle: 'How the pipeline works',
    pipelineSubtitle:
      'Two independent PostgreSQL schemas, so a failed run never touches the source data.',
    pipeline: {
      ingestTitle: 'Ingest',
      ingestDetail:
        'Raw schema. Generated records land here untouched — the ground truth for every run.',
      transformTitle: 'Transform',
      transformDetail:
        'Checkpoint-based, idempotent migration in batches with panic recovery. Interrupted runs resume from their last checkpoint.',
      serveTitle: 'Serve',
      serveDetail: 'Processed records plus the aggregates that feed real-time analytics.',
    },
    capabilitiesTitle: 'What can you do with it?',
    capabilitiesSubtitle: 'Three entry points into the lab.',
    capabilities: {
      simulateTitle: 'Run massive simulations',
      simulateDetail:
        'Worker pools process batches of 3,000–4,000 records concurrently and report progress as they go.',
      scheduleTitle: 'Schedule recurring work',
      scheduleDetail: 'Cron definitions are persisted in PostgreSQL and executed by robfig/cron.',
      exploreTitle: 'Explore the results',
      exploreDetail:
        'A REST API under /api/v1, documented in Swagger and guarded by RBAC roles (admin, operator, viewer).',
    },
    featuresAriaLabel: 'Features',
    features: {
      workerPoolTitle: 'Concurrent worker pool',
      workerPoolDetail:
        'Goroutine worker pools with panic recovery, live metrics and batched job control.',
      schedulerTitle: 'Cron scheduler',
      schedulerDetail: 'Schedules persisted in PostgreSQL and executed with robfig/cron.',
      etlTitle: 'Resumable ETL',
      etlDetail:
        'Checkpoint-based, idempotent migration between the source and target schemas.',
      rbacTitle: 'Role-based access control',
      rbacDetail:
        'RBAC roles — admin, operator and viewer — guarding every read and write endpoint.',
      apiTitle: 'REST API + Swagger',
      apiDetail: 'A documented OpenAPI surface under /api/v1 with stable envelopes.',
      opaqueIdsTitle: 'Encrypted opaque IDs',
      opaqueIdsDetail:
        'AES-256-GCM tokens keep internal database identifiers out of every response.',
    },
  },
  roles: {
    admin: 'Administrator',
    operator: 'Operator',
    viewer: 'Viewer',
  },
  layout: {
    openMenu: 'Open menu',
    closeMenu: 'Close menu',
    logout: 'Sign out',
  },
  table: {
    loading: 'Loading…',
    empty: 'No results',
    error: 'Could not load the data',
    retry: 'Retry',
    perPage: 'Rows per page',
    previous: 'Previous page',
    next: 'Next page',
    pageOf: 'Page {page} of {total}',
    results: '{count} result | {count} results',
    sortBy: 'Sort by {column}',
  },
  filters: {
    search: 'Search',
    searchPlaceholder: 'Search…',
    country: 'Country',
    category: 'Category',
    status: 'Status',
    dateFrom: 'From',
    dateTo: 'To',
    all: 'All',
    clear: 'Clear',
    apply: 'Apply',
  },
  data: {
    rowId: 'ID',
    source: 'Source (lab_source)',
    target: 'Target (lab_target)',
    customers: {
      title: 'Customers',
      firstName: 'First name',
      lastName: 'Last name',
      email: 'Email',
      country: 'Country',
      city: 'City',
      state: 'State',
      createdAt: 'Created',
    },
    products: {
      title: 'Products',
      sku: 'SKU',
      name: 'Name',
      category: 'Category',
      price: 'Price',
      stock: 'Stock',
      status: 'Status',
      createdAt: 'Created',
    },
    orders: {
      title: 'Orders',
      orderNumber: 'Order',
      customer: 'Customer',
      status: 'Status',
      total: 'Total',
      currency: 'Currency',
      orderDate: 'Date',
      items: 'Items',
      viewDetail: 'View',
      detailTitle: 'Order {number}',
      back: 'Back to orders',
      shippingAddress: 'Shipping address',
      itemsTitle: 'Order items',
      product: 'Product',
      quantity: 'Quantity',
      unitPrice: 'Unit price',
      subtotal: 'Subtotal',
      notFound: 'Order not found',
    },
    orderItems: {
      title: 'Order items',
      order: 'Order',
      product: 'Product',
      quantity: 'Quantity',
      unitPrice: 'Unit price',
      subtotal: 'Subtotal',
    },
  },
}

/** Esquema de mensajes (fuente de verdad: inglés). */
export type MessageSchema = typeof en

export default en
