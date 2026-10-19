# web — Go Concurrent Data Processing Lab (frontend)

Frontend Vue 3 del lab. Fuente única del frontend (`frontend/` está vacío y no
se usa).

## Stack

- Vue 3 (`<script setup>`) + TypeScript + Vite
- Tailwind CSS + shadcn-vue (reka-ui) + lucide icons
- pinia (stores) + vue-router
- Vitest + @vue/test-utils (cobertura ≥80%)

## Desarrollo

```sh
npm install
npm run dev        # Vite en http://localhost:5173
```

El dev server está en **modo `host: true`** con `allowedHosts` para que
`https://frontgodata.josebianco.local` (Caddy → `host.docker.internal:5173`)
pueda proxyar hacia él. Si añades otro hostname lab, agrégalo a
`allowedHosts` en `vite.config.ts`.

### Proxy a la API

`vite.config.ts` proxya `/api` y `/swagger` hacia el backend Go
(`http://localhost:8080` por defecto, configurable con
`VITE_BACKEND_TARGET`). Por eso el cliente API usa `baseURL = '/api/v1'`
(`src/api/client.ts`) y todo es same-origin desde el navegador.

También puedes apuntar directo a la API con un `.env.local`:

```sh
VITE_API_BASE_URL=https://apidatago.josebianco.local/api/v1
```

En ese caso el backend debe permitir el origen en `CORS_ALLOWED_ORIGINS`.

## Comandos

```sh
npm run test         # vitest run (unit + component)
npm run test:watch   # watch
npm run test:coverage# cobertura (≥80%)
npm run type-check   # vue-tsc
npm run lint         # oxlint + eslint
npm run build        # type-check + build de producción
```

## Estructura

```
src/
  api/        cliente Axios (envelope de errores, refresh JWT, hooks)
  components/ UI shadcn-vue y componentes propios
  config/     navegación / menú
  layouts/    layout autenticado (sidebar + header + tema)
  router/     rutas + guards de sesión
  stores/     pinia (auth, theme)
  types/      tipos compartidos de API
  views/      vistas por ruta
  lib/        utilidades (cn, etc.)
```

## Temas

Claro / Oscuro / Sistema, persistido en `localStorage` y aplicado sin flash
inicial (script en `index.html`). Modo Sistema respeta
`prefers-color-scheme`.

## Iconos

`public/` contiene los iconos del PWA (SVG + PNG + `manifest.webmanifest`).
Regenerados con `scripts/generate-icons.sh`. Ver `README-icons.md`.
