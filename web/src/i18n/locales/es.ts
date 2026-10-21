import type { MessageSchema } from './en'

/**
 * Mensajes en español. Tipados con `MessageSchema` (derivado de `en.ts`) para
 * que TypeScript detecte de inmediato cualquier clave que falte o que sobre.
 *
 * No se traducen: el nombre del autor y el código de licencia CC BY-NC-ND 4.0.
 */
const es: MessageSchema = {
  common: {
    brand: 'Go Data Lab',
    projectName: 'Go Concurrent Data Processing Lab',
  },
  nav: {
    dashboard: {
      title: 'Dashboard',
      description: 'Resumen general del laboratorio',
    },
    customers: {
      title: 'Customers',
      description: 'Catálogo de clientes (lab_source)',
    },
    products: {
      title: 'Products',
      description: 'Catálogo de productos (lab_source)',
    },
    orders: {
      title: 'Orders',
      description: 'Órdenes e items (lab_source)',
    },
    target: {
      title: 'Target / Analytics',
      description: 'Datos procesados y analítica (lab_target)',
    },
    jobs: {
      title: 'Jobs',
      description: 'Control y progreso de jobs en tiempo real',
    },
    scheduler: {
      title: 'Scheduler',
      description: 'Programación de tareas cron',
    },
    adminLogs: {
      title: 'Admin logs',
      description: 'Logs de error del backend (solo admin)',
    },
    sections: 'Secciones',
  },
  auth: {
    title: 'Iniciar sesión',
    subtitle: 'Acceso al Go Concurrent Data Processing Lab',
    email: 'Email',
    password: 'Contraseña',
    emailRequired: 'El email es obligatorio',
    emailInvalid: 'El formato del email no es válido',
    passwordRequired: 'La contraseña es obligatoria',
    submitting: 'Entrando…',
    submit: 'Entrar',
  },
  dashboard: {
    title: 'Dashboard',
    greeting: 'Hola, {name}. Este es el laboratorio de procesamiento concurrente de datos.',
    defaultUser: 'usuario',
  },
  placeholder: {
    upcoming: 'Próximamente',
    fallbackTitle: 'Sección',
    body: 'Esta sección está planificada en los Issues #20–#24 (tablas de datos, monitoreo de jobs, scheduler, admin y analítica). Mientras tanto, el resto del panel ya está disponible.',
  },
  publicHeader: {
    signIn: 'Iniciar sesión',
    goToDashboard: 'Ir al panel',
  },
  footer: {
    project: 'Go Concurrent Data Processing Lab — plataforma sintética de datos de e-commerce.',
    developedBy: 'Developed by Jose Antonio Bianco',
    licensedUnder: 'Licensed under',
  },
  theme: {
    label: 'Tema',
    light: 'Claro',
    dark: 'Oscuro',
    system: 'Sistema',
  },
  language: {
    label: 'Idioma',
    es: 'ES',
    en: 'EN',
    esFull: 'Español',
    enFull: 'English',
  },
  landing: {
    tagline: 'Go Concurrent Data Processing Lab',
    hero: 'Procesamiento concurrente de datos, simulación masiva y analítica en tiempo real sobre un dataset sintético de e-commerce — clientes, productos, órdenes e items, desde la ingesta hasta el análisis.',
    overview: 'Visión general',
    whatIsThisTitle: '¿Qué es esto?',
    whatIsThisP1:
      'Go Data Lab es un laboratorio para explorar el procesamiento concurrente de datos en Go sobre un modelo sintético de e-commerce — clientes, productos, órdenes e items. Cada dataset se genera bajo demanda dentro de una base de datos PostgreSQL real, con el volumen que requiera el experimento.',
    whatIsThisP2:
      'Su propósito es simular volúmenes masivos manteniendo todo el pipeline visible, desde la ingesta hasta la analítica, para poder medir de extremo a extremo sobre datos que controlas las estrategias de concurrencia, el batching y la reanudación.',
    pipelineTitle: 'Cómo funciona el pipeline',
    pipelineSubtitle:
      'Dos esquemas PostgreSQL independientes, para que una corrida fallida nunca toque los datos de origen.',
    pipeline: {
      ingestTitle: 'Ingesta',
      ingestDetail:
        'Esquema crudo. Los registros generados llegan aquí intactos — la verdad de referencia de cada corrida.',
      transformTitle: 'Transformación',
      transformDetail:
        'Migración idempotente por lotes con checkpoints y recuperación de pánicos. Las corridas interrumpidas se reanudan desde su último checkpoint.',
      serveTitle: 'Servicio',
      serveDetail: 'Registros procesados más los agregados que alimentan la analítica en tiempo real.',
    },
    capabilitiesTitle: '¿Qué puedes hacer con esto?',
    capabilitiesSubtitle: 'Tres puntos de entrada al laboratorio.',
    capabilities: {
      simulateTitle: 'Correr simulaciones masivas',
      simulateDetail:
        'Los worker pools procesan lotes de 3.000–4.000 registros en paralelo e informan el progreso sobre la marcha.',
      scheduleTitle: 'Programar trabajo recurrente',
      scheduleDetail:
        'Las definiciones cron se persisten en PostgreSQL y se ejecutan con robfig/cron.',
      exploreTitle: 'Explorar los resultados',
      exploreDetail:
        'Una API REST bajo /api/v1, documentada en Swagger y protegida con roles RBAC (admin, operator, viewer).',
    },
    featuresAriaLabel: 'Características',
    features: {
      workerPoolTitle: 'Worker pool concurrente',
      workerPoolDetail:
        'Pools de goroutines con recuperación de pánicos, métricas en vivo y control de jobs por lotes.',
      schedulerTitle: 'Scheduler cron',
      schedulerDetail: 'Programaciones persistidas en PostgreSQL y ejecutadas con robfig/cron.',
      etlTitle: 'ETL reanudable',
      etlDetail:
        'Migración idempotente con checkpoints entre los esquemas de origen y destino.',
      rbacTitle: 'Control de acceso por roles',
      rbacDetail:
        'Roles RBAC — admin, operator y viewer — protegen cada endpoint de lectura y escritura.',
      apiTitle: 'API REST + Swagger',
      apiDetail: 'Una superficie OpenAPI documentada bajo /api/v1 con envelopes estables.',
      opaqueIdsTitle: 'IDs opacos cifrados',
      opaqueIdsDetail:
        'Tokens AES-256-GCM mantienen los identificadores internos de la base fuera de cada respuesta.',
    },
  },
  roles: {
    admin: 'Administrador',
    operator: 'Operador',
    viewer: 'Observador',
  },
  layout: {
    openMenu: 'Abrir menú',
    closeMenu: 'Cerrar menú',
    logout: 'Salir',
  },
  table: {
    loading: 'Cargando…',
    empty: 'Sin resultados',
    error: 'No se pudieron cargar los datos',
    retry: 'Reintentar',
    perPage: 'Filas por página',
    previous: 'Página anterior',
    next: 'Página siguiente',
    pageOf: 'Página {page} de {total}',
    results: '{count} resultado | {count} resultados',
    sortBy: 'Ordenar por {column}',
  },
  filters: {
    search: 'Buscar',
    searchPlaceholder: 'Buscar…',
    country: 'País',
    category: 'Categoría',
    status: 'Estado',
    dateFrom: 'Desde',
    dateTo: 'Hasta',
    all: 'Todos',
    clear: 'Limpiar',
    apply: 'Aplicar',
  },
  data: {
    rowId: 'ID',
    source: 'Origen (lab_source)',
    target: 'Destino (lab_target)',
    customers: {
      title: 'Clientes',
      firstName: 'Nombre',
      lastName: 'Apellido',
      email: 'Email',
      country: 'País',
      city: 'Ciudad',
      state: 'Provincia',
      createdAt: 'Creado',
    },
    products: {
      title: 'Productos',
      sku: 'SKU',
      name: 'Nombre',
      category: 'Categoría',
      price: 'Precio',
      stock: 'Stock',
      status: 'Estado',
      createdAt: 'Creado',
    },
    orders: {
      title: 'Órdenes',
      orderNumber: 'Orden',
      customer: 'Cliente',
      status: 'Estado',
      total: 'Total',
      currency: 'Moneda',
      orderDate: 'Fecha',
      items: 'Items',
      viewDetail: 'Ver',
      detailTitle: 'Orden {number}',
      back: 'Volver a órdenes',
      shippingAddress: 'Dirección de envío',
      itemsTitle: 'Items de la orden',
      product: 'Producto',
      quantity: 'Cantidad',
      unitPrice: 'Precio unitario',
      subtotal: 'Subtotal',
      notFound: 'Orden no encontrada',
    },
    orderItems: {
      title: 'Items de orden',
      order: 'Orden',
      product: 'Producto',
      quantity: 'Cantidad',
      unitPrice: 'Precio unitario',
      subtotal: 'Subtotal',
    },
  },
}

export default es
