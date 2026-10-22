# MEMORY — Go Concurrent Data Processing Lab

Memoria viva del proyecto. Se carga en cada sesión de opencode.
Mantener concisa. Actualizar con `/remember` al cierre de cada bloque de trabajo.

## Estado actual
- ✅ **Issue #21 (Job Monitoring & Control) COMPLETADO @ `295a780`+`c56ed97` (22-oct)**: SSE en vivo
  + panel de control. **Backend**: `jobs.Manager` gana un **broker pub/sub** (`internal/jobs/subscribe.go`)
  — canales buffered por suscriptor (cap 8), broadcast **no bloqueante** (un cliente lento descarta el
  evento, nunca frena el job), **snapshot al suscribirse**, evento terminal + **cierre al terminar el job**;
  `handler.JobsHandler.Events` (`internal/handler/jobs_sse.go`) escribe frames `event: progress` con el
  payload **sellado por el codec** (`job_id` = token `v1.*`) + heartbeat `: ping` cada 20s;
  `middleware.AuthRequiredSSE` valida el **access token por query param `?token=`** (EventSource no
  manda headers; los refresh se rechazan con 401); la ruta SSE se monta **fuera** del grupo `/jobs`.
  **Frontend**: `JobsView.vue` (`/jobs`) con diálogo de parámetros (generate/migrate), stop, barra de
  progreso + tabla por entidad en vivo, e historial paginado; `api/jobs.ts` (cliente tipado +
  `buildJobEventsUrl` con el token codificado), `composables/useJobEvents.ts` (ciclo del EventSource +
  fallback REST), `components/IdChip.vue` (chip de IDs opacos: **solo acorta la presentación**, el
  token íntegro va al `title`/portapapeles), `components/TruncatedCell.vue`, `ui/progress/**`.
  **4 bugs reales encontrados y corregidos en la revisión del orquestador** (details en "Pendientes/bugs"):
  (1) 🔴 la ruta SSE heredaba el guard bearer del grupo ⇒ **siempre 401** en producción;
  (2) 🔴 race *send on closed channel* en el broker (2 instancias) ⇒ panic del proceso;
  (3) 🟠 los `Input type="number"` emiten **números** (Vue castea) ⇒ `value.trim()` reventaba y los jobs
  no arrancaban con parámetros numéricos; (4) 🟡 label `entities` hardcodeado (regla i18n).
  Verificado: backend `go build`/`go vet`/`gofmt`/`go test -race` (**21 pkgs ok**)/`golangci-lint`
  (**0 findings**); frontend **304 tests (30 archivos)**, cobertura **94.72% stmts / 85.97% branches /
  95.01% funcs / 97% lines**, `type-check`/`lint`/`build` ✅. **E2E en vivo** (Caddy `:8443` → Vite
  `:5174` → API `:8080`): `?token=` sin header ⇒ **200** `text/event-stream`; sin token/garbage/refresh ⇒
  **401**; frames de progreso cada **~200ms** (el `encode zstd gzip` de Caddy **no** bufferiza) y evento
  terminal `completed` con cierre del stream.
- ✅ **i18n ES/EN con `vue-i18n` @ WIP (21-oct)**: doble idioma real. `web/src/i18n/`
  (`index.ts` con `createI18n({legacy:false})`, `detectLocale()` = `gdl.locale` → `navigator.language`
  (`es*` ⇒ es) → default **`en`**; `fallbackLocale: 'en'`, `SUPPORTED_LOCALES`),
  `i18n/locales/{en,es}.ts` (**EN es la fuente de verdad**: `MessageSchema = typeof en` tipa `es`
  ⇒ TS detecta claves faltantes/sobrantes), `stores/locale.ts` (patrón de `stores/theme.ts`:
  `init`/`setLocale`/`apply`; escribe `i18n.global.locale` + `<html lang>`), `components/LanguageToggle.vue`
  (grupo ES/EN con `aria-pressed`). Toggle en las **3 superficies** junto al `ThemeToggle`.
  Anti-flash de idioma en `index.html`. **Diseño**: `nav.ts` y `router` guardan **claves**
  (`titleKey`/`descriptionKey`, `meta.titleKey`) y los consumidores resuelven con `t()` ⇒ reactivo
  al cambio de idioma sin re-montar; `route.meta.title` literal eliminado. **No se traducen**:
  nombre del autor y código `CC BY-NC-ND 4.0`; `data-test` intactos. Tests: `setup.ts` registra
  i18n globalmente + resetea locale en `afterEach`, helper `useLocale()` en fixtures para idioma
  determinista. Verificado (subagente `frontend-vue` + revisión del orquestador):
  **158 tests** (17 archivos), cobertura **98.62% stmts / 95.52% branches**, `type-check`/`lint`/`build` ✅;
  E2E propio con vitest: detección, persistencia tras reload, cambio inmediato de textos
  (`auth.title`/`layout.logout` ES↔EN), `<html lang>`, y paridad de claves ES=EN (112=112).
- ✅ **Footer CC + toggle de tema en todas las vistas @ WIP (21-oct)**: componentes compartidos
  `components/AppFooter.vue` (proyecto + **"Developed by Jose Antonio Bianco"** + licencia
  **CC BY-NC-ND 4.0** — la más restrictiva de Creative Commons, enlace con `rel="noopener"`)
  y `components/ThemeToggle.vue` (Claro/Oscuro/Sistema, extraído del header de AppLayout).
  Ambos en las **3 superficies**: `PublicLayout` (footer propio reemplazado + toggle en el
  header junto al CTA), `LoginView` (layout a columna: header con toggle, main centrado,
  footer al pie) y `AppLayout` (footer tras `<main>`; el bloque de tema inline se eliminó).
  TDD rojo→verde. Verificado: **123 tests**, cobertura **98.36% stmts / 95.3% branches**,
  `type-check`/`lint`/`build` ✅.
- **Landing siempre visible + fix `PublicLayout` @ WIP (20-oct)**: se resolvió la
  decisión pendiente — `/` **no** redirige a `/dashboard` con sesión; el CTA se adapta
  (`Sign in` → `/login`, con sesión `Go to dashboard` → `/dashboard`) en `LandingView` y en el
  header de `PublicLayout`. Guard sin el branch `landing && authenticated`; catch-all sigue a `/`.
  Contenido nuevo en la landing: *What is this?* (propósito, 2 párrafos), *pipeline*
  (`lab_source` → ETL resumable → `lab_target`, 3 cards con iconos) y *What can you do with it?*
  (3 capacidades). **Bug real encontrado por el test de router**: `PublicLayout` usaba
  `<slot />` en vez de `<RouterView />` ⇒ la ruta hija `landing` **nunca se renderizaba** en la
  app real (solo header+footer); corregido y su spec ahora monta `RouterView` raíz en vez de
  pasar slot. También se borraron `tsconfig.*.tsbuildinfo` obsoletos (arrastraban un
  `__debug__.spec.ts` ya eliminado y rompían `vue-tsc --build`). Verificado: **110 tests**,
  cobertura **98.35% stmts / 95.34% branches**, `type-check`/`lint`/`build` ✅.
- ✅ **PRs mergeados**: #40 (`ec11a7e`), **#41** (`c47ddfc`, = `origin/main` actual). El #41 llevó
  secureid T2 + lint + #38 (paleta) + #39 (landing) + footer/theme-toggle + i18n ES/EN.
  **PR #42 abierto** (`feature/data-tables-views` → `main`, commit `8c4f937` = Issue #20). `gh release`
  sigue denegado.
- **main**: PRs #32-#36 mergeados. `make build`, `go test -race ./...`
  y `go vet ./...` en verde.
- **Hosts locales nuevos** (decisión del usuario, `/etc/hosts` ya actualizado):
  `127.0.0.1 apidatago.josebianco.local` (API) y `127.0.0.1 frontgodata.josebianco.local` (web).
  Reemplazan a `godata.josebianco.local`. `deploy/Caddyfile` con dos sitios (Caddy →
  `host.docker.internal:8080` y `:5173`), certs mkcert por host en `deploy/certs/`,
  `gen-certs.sh` regenera ambos, Makefile comprueba ambos `.pem`. CORS/APP_URL por defecto
  apuntan a los nuevos hosts (config + middleware + tests). `vite.config.ts` con
  `host: true` + `allowedHosts` (Vite 6+ devuelve 403 con Host desconocido) + proxy `/api`→8080.
  README raíz y `web/README.md` reescritos con el flujo completo (hosts, certs, SECURE_ID_KEY,
  frontend dev) y `AGENTS.md` actualizado.
- **Proyecto levantado en local (19-oct)**: `lab_postgres` + `lab_caddy` (make docker-up),
  migraciones aplicadas, backend `make run` en `:8080`, Vite `npm run dev` en `:5174`.
  **Puertos del lab** (taskflow local ocupa 80/443/5173): Caddy `8443` (HTTPS) + `8081`,
  Vite `5174` con `strictPort: true`. Caddyfile corregido: `frontgodata` proxyaba al 5173
  (¡caía en el Vite de taskflow y devolvía 403!). URL front `https://frontgodata.josebianco.local:8443`,
  API `https://apidatago.josebianco.local:8443`. Fix nuevo: `config.loadDotEnv()` sube desde el
  cwd hasta encontrar el `.env` raíz (los `make` corren desde `backend/` y no lo veían ⇒
  `SECURE_ID_KEY` fallaba). E2E verificado: login admin, `/source/customers`, `/jobs/status`,
  `/target/analytics/summary`, CORS preflight, swagger, cert mkcert trusted. Commit `f09b4df`.
- **secureid @ 69a591d (T1, commiteado)**: paquete `backend/internal/secureid` — IDs opacos
  cifrados AES-256-GCM determinista (nonce = HMAC-SHA256 con subclave derivada del master key),
  formato `v1.<base64url(nonce||ct||tag)>`, `SealUUID/OpenUUID/SealInt64/OpenInt64`, sentinelas
  `ErrInvalidKey/ErrInvalidToken`. Config `SecureIDKey` desde env `SECURE_ID_KEY` (base64 o hex,
  32 bytes exactos; falta ⇒ arranque falla). Decisión: determinismo obligatorio para que las
  referencias cruzadas en JSON produzcan el mismo token.
- **T2 secureid — decisiones de diseño tomadas (usuario, 19-oct)**: alcance completo = URLs +
  bodies de request + respuestas. Tres acuerdos:
  1. **Outbound** → interceptar en `RespondJSON`/`RespondData`/`RespondPaginated` con un walk
     recursivo del JSON que selle toda key `id`/`*_id` que parsee como UUID o int64. *No* se
     crea capa DTO (los handlers serializan modelos GORM directos hoy).
  2. **Inbound** → **solo token**: `parseUUIDParam` y `parseOptionalUUID` rechazan con 400
     cualquier UUID crudo (formato interno no expuesto).
  3. **Alcance** → paths `:id` (17 rutas), filtros query (`customer_id`/`order_id`/`product_id`),
     bodies de escritura (`data_write.go` con `uuid.UUID`+binding ⇒ pasar a `string` + abrir) y
     todos los IDs de responses (incl. `order_items.id` **int64**, `CronSchedule.schedule_id`
     **nullable** y `auth` `/me`).
  Inyección vía `router.Dependencies.Codec` + context value (los respondedores son funciones
  libres). El `user_id` interno del JWT **no** se toca (va firmado, no es JSON de respuesta).
  Momento óptimo: el frontend aún no consume IDs (#20-#24 son placeholders) ⇒ romper contrato
  es gratis.
- ✅ **T2 secureid COMPLETADO @ 3ac309f (19-oct, revisado por el orquestador)**: `handler/secureid.go`
  nuevo (`WithCodec`/`codecFrom`, walk `sealJSONValue`/`sealInto`/`sealIDInto` con `json.Decoder`
  preservando orden de claves y escaping, `isOpaqueIDKey` = `id` exacto o sufijo `_id`,
  `openInboundID` token-only, `invalidIDHint`, `respondInvalidID`). DTOs de borde en `handler`
  (`OrderCreateRequest`/`OrderItemRequest`, ids `string`); el servicio mantiene `uuid.UUID`.
  `RespondJSON` sella, `RespondData` delega, `RespondPaginated` intacto (meta nunca sellada).
  Login/Refresh/Me pasan de `c.JSON` a `RespondJSON`. Swagger regenerado (25 menciones
  "opaque token"). **Fail-closed confirmado**: `config.Load()` → error si falta `SECURE_ID_KEY`,
  `secureid.New` → error si la clave es inválida, `main.go` propaga con `%w`; sin codec no arranca.
  **E2E verificado en vivo** (binario nuevo en `:8080`): id sellado en lista y detalle, determinismo
  list↔detail, `GET /customers/{token}` 200, UUID crudo → 400 `must be an opaque id`, token corrupto
  → 400, filtro query por token 200 / por UUID crudo 400, `order_items.id` (int64) → token string,
  `/auth/me` y `/auth/refresh` con id sellado, swagger 200, todo vía Caddy 200.
  Cobertura: `handler` 87.3%→**88.7%**, `router` 99.1%, `secureid` 91.0%. `make build`/`go vet`/
  `go test -race` (21 paquetes ok) limpios.
- ✅ **`make lint` VERDE (19-oct)**: saneados los 8 hallazgos preexistentes — `rand.Read` sin
  comprobar en `config_test.go`/`config_cors_test.go`/`secureid_test.go` (helpers ahora reciben
  `t` y usan `t.Fatalf`), rama vacía `SA9003` en `secureid/secureid.go` (**código muerto
  redundante**: la validación real de longitud ya existe al final de `open()`, no era agujero de
  seguridad), tipo `fileRef` sin usar en `migrate.go` y `p.Submit` sin comprobar en
  `worker_test.go` ×3. Verificado: `make lint` exit 0, `go test -race` 21 paquetes ok, `gofmt` limpio.
- **Frontend Bloque 1 completo @ 0a56550 (Issues #17-#19)**: scaffold `web/` terminado tras
  una sesión previa interrumpida — `LoginView`, `AppLayout` (sidebar RBAC por rol, header con
  email/rol, toggle tema Claro/Oscuro/Sistema), `DashboardView` (tarjetas por sección), `PlaceholderView`
  parametrizada por meta del router; script anti-flash de tema en `index.html` (clave `gdl.theme`);
  borrado HelloWorld del template. Verificado: 84 tests, cobertura 97.75% stmts / 92.3% branches,
  `vue-tsc`, lint y `build` en verde.
- **Rama `feature/jobs-scheduler-api` @ 2379820**: merge de `wip/jobs-scheduler-api` (trabajo
  recuperado de otra sesión, ~21k líneas) sobre el main actual **cerrado y commiteado**
  (`320c2cd` merge + `c7c5e20` tidy + `2379820` memory). Conflictos resueltos uniendo ambos lados: `cmd/api/main.go`
  (auth + jobs/scheduler/CORS, con `AuthRequired` enchufado vía `Dependencies.Middleware`),
  `config.go` (TTLs JWT + CORS), `config_test.go` (split en `config_test.go` white-box +
  `config_cors_test.go` black-box) y `docs/MEMORY.md`. `router.go` se auto-mergeó conservando auth
  + grupos source/target/jobs/scheduler. Conflicto semántico extra: `repository.ErrNotFound`
  duplicado en `repository.go` y `auth.go` ⇒ se conservó el de `repository.go`. Verificado:
  `make build`, `go vet ./...`, `go test -race ./...` en verde (golangci-lint no instalado).
- **CRUD lab_source + RBAC + admin logs @ d8816e8 (commiteado)**: escrituras
  customers/products/orders en `repository/{customer,product,order}.go` (Order en transacción con
  items; customer/product `Update` con `Select(...)` para persistir valores cero), sentinelas y
  `mapPgError` en `repository/errors.go` (23505→`ErrConflict`, 23503→`ErrInvalidReference`),
  DTOs+métodos write en `service/data_write.go` (normaliza email/country/status/currency, recomputa
  subtotales/total, `wrapWriteErr` mapea a `service.Err{NotFound,Conflict,InvalidReference}`),
  handlers en `handler/source_write.go` (POST 201 / PUT 200 / DELETE 204; 400/404/409/500).
  RBAC: `Dependencies.WriteMiddleware` (aplica a rutas mutantes de source/jobs/scheduler) y
  `Dependencies.AdminMiddleware` + `Dependencies.Admin` (grupo `/api/v1/admin`, solo se registra si
  **ambos** están seteados). Endpoint `GET /api/v1/admin/logs/errors`:
  `service.ErrorLogs`/`FileErrorLogs` (`service/errorlog.go`, lee la cola ≤4 MiB del log JSON,
  filtra ERROR/FATAL/PANIC/DPANIC, newest-first, paginado) + `handler/admin.go`. En `main.go`:
  `WriteMiddleware=[RBAC(admin,operator)]`, `AdminMiddleware=[AuthRequired,RBAC(admin)]`,
  `Admin=NewFileErrorLogs(cfg.LogPath)`. Tests nuevos: `data_write_test.go`, `errorlog_test.go`,
  `source_write_test.go`, `admin_test.go`, `errors_unit_test.go` y casos en `router_test.go`
  (writes, WriteMiddleware por grupo, admin guard, mapa RBAC). Cobertura: handler 87%, service 84%,
  router 99%. `go build`/`go vet`/`go test -race ./...` en verde. Swagger regenerado
  (`swag` instalado vía `go install github.com/swaggo/swag/cmd/swag@v1.16.6`; tag
  `swaggertype:"object"` añadido a los `ShippingAddress json.RawMessage` de los DTOs de orden).
- **Issue #1 (infra local) completa**: `deploy/docker-compose.yml` (postgres:15-alpine + caddy:2-alpine),
  `deploy/Caddyfile` (TLS mkcert → host-gateway:8080), `deploy/init-db.sh` idempotente, `deploy/gen-certs.sh`.
- **Issue #6 (auth) completa**: Argon2id + JWT access/refresh HS256 (`internal/auth`), `UserRepository`
  (`repository/auth.go`), `BootstrapAdmin` (`service/bootstrap.go`), `AuthRequired` + `RBAC`
  (`middleware/auth.go`), endpoints `/api/v1/auth/{login,refresh,me,change-password,logout}`.
  Bootstrap admin `root@gmail.com` desde env. Tests unitarios con mocks.
- **Issue #7 (data generator)**: `internal/generator` (+ `generator/postgres`, store con inserts
  batcheados), `cmd/generate` CLI, migración `source/003_generator_seeds` (tabla `generator_seeds`).
- **Issue #8 (worker pool)**: `internal/worker` — pool con `context`, recuperación de pánicos,
  métricas, agregación de errores y `SubmitBatch`/`SubmitStream`.
- **Migration runner**: `backend/internal/migrate/` + `backend/cmd/migrate/` (subcomandos
  `up`/`down`/`status`, flag `-db source|target|all`, `-dir`, `-steps`). `make migrate-up`/
  `migrate-down`/`migrate-status` funcionan. Verificado con Postgres 15 real.
- **Issue #9 (ETL worker)**: `backend/internal/etl/` (+ adaptadores `internal/etl/postgres/`,
  CLI `cmd/migrate-data/`): extract keyset, transform/validate, upsert idempotente SQL Raw,
  checkpoint reanudable, métricas y logging zap. Verificado E2E con Postgres 15 real.
- **Issue #10 (REST API Core) + #11 (Source Data Endpoints)**: envelope de error/paginación
  (`internal/handler/response.go`, `pagination.go`), CORS hand-rolled (`internal/middleware/cors.go`),
  repos GORM de `lab_source` (`internal/repository/{customer,product,order,order_item}.go` + interfaces),
  `DataService` (`internal/service/data.go`, puerto `service.SourceData`), handlers+swagger
  (`internal/handler/source.go`), rutas `/api/v1/source/*` y `/swagger/*any`. Verificado E2E.
- **Issue #12 (Target Data + Analytics)**: `TargetRepositories` (alias de `SourceRepositories`),
  `AnalyticsRepository` + `GormAnalyticsRepository` (SQL Raw: summary, top-products, top-customers),
  `service.TargetData` (embebe `SourceData` + analytics) y `TargetDataService`, handlers
  `internal/handler/target.go`, rutas `/api/v1/target/*` y `/api/v1/target/analytics/*`. Verificado E2E.
- **Issue #13 (Worker & Job Control) + #14 (Scheduler/Cron)**: `internal/jobs/` (Manager con un solo
  job activo, runners generate/migrate, `JobRepository` sobre `job_executions`), `internal/scheduler/`
  (cron `robfig/cron/v3` persistido en `cron_schedules`, `ScheduleRepository`, `JobStarter`), puertos
  `service.JobControl`/`service.SchedulerControl`, handlers `internal/handler/{jobs,scheduler}.go`,
  rutas `/api/v1/jobs/*` y `/api/v1/scheduler/*`, cableado en `cmd/api/main.go`. Swagger regenerado.
- **PR #34**: `web/public/` con iconos (SVG + PNGs) y `manifest.webmanifest`; `web/scripts/generate-icons.sh`.
- `frontend/` sigue vacío (no trackeado por git); no usar.

## Decisiones
- **Numbering de issues**: el numbering de **GitHub lleva un offset (+1)** respecto al interno de
  `docs/ISSUES.md` (existe un "test issue" #1 en GitHub). Mapeo: interno **#16 (SSE)** → GitHub
  **#17** · interno **#17/#18/#19 (frontend setup/auth UI/dashboard)** → GitHub **#18/#19/#20** ·
  interno **#20 (Data Tables)** → GitHub **#21** · interno **#21 (Job Monitoring)** → GitHub **#22** ·
  interno **#22/#23/#24 (scheduler/admin/analytics UI)** → GitHub **#23/#24/#25**.
  ⚠️ Al escribir `Closes #N` en un PR, usar **el número de GitHub**, no el interno.
- **SSE de jobs (Issue #21)**: el pub/sub vive **en el `jobs.Manager`** (`Subscribe(ctx) <-chan Progress`,
  sin paquete nuevo: el Manager ya tenía el mutex y es quien aplica el progreso). Broadcast no
  bloqueante + buffer 8 ⇒ un cliente lento nunca frena el job (descarta eventos). El stream entrega
  un **snapshot** al suscribirse y se **cierra al terminar el job** (evento terminal primero).
- **SSE + auth**: `EventSource` nativo no permite cabeceras ⇒ el access token viaja por **query param
  `?token=`** (`middleware.AuthRequiredSSE`); los refresh tokens se rechazan con 401. Trade-off
  documentado: el token puede quedar en logs/URLs. El resto del grupo `/jobs` sigue con `Bearer`.
- **La ruta SSE NO hereda el middleware del grupo `/jobs`**: en gin un subgrupo hereda el middleware del
  padre y este corre **primero**, así que el guard bearer abortaba con 401 antes de que corriera el
  guard del query param. Por eso `/jobs/events` se registra en un **grupo propio bajo `/api/v1`**
  (`registerJobEventsRoute`) y la ruta es **fail-closed**: solo se registra si hay guard
  (`SSEMiddleware`, o `Middleware` como fallback bearer; sin ninguno ⇒ **404**, nunca abierta).
- **Canal del suscriptor**: se escribe **y** se cierra siempre bajo `m.subs.mu` ⇒ nunca hay
  *send on closed channel* (el bug existía en `broadcast` vs `close` y en `Subscribe` vs `broadcastTerminal`).
  `m.mu` (estado del job) nunca se anida dentro de `m.subs.mu`.
- **Frontend**: `Input type="number"` de shadcn-vue usa `useVModel` **passive** ⇒ Vue castea el valor a
  **número**, no cadena. Todo helper que parsea campos numéricos debe aceptar `string | number`.
- Frontend va en `web/` (no en `frontend/`).
- Hosts lab: API en `apidatago.josebianco.local`, web en `frontgodata.josebianco.local`
  (el viejo `godata.josebianco.local` quedó obsoleto). TLS con mkcert por host vía Caddy.
- IDs en URLs y JSON: tokens opacos AES-256-GCM **deterministas** (`internal/secureid`),
  porque el mismo ID debe producir el mismo token en todas las referencias cruzadas
  del JSON. Clave en env `SECURE_ID_KEY` (nunca en código).
- Sin AutoMigrate: migraciones SQL versionadas.
- TLS local: certificados **mkcert montados** en `/certs` (no `tls internal`); `make docker-up` falla si faltan.
- Un único `.env` raíz alimenta backend (godotenv) y Compose (`--env-file ../.env` desde el Makefile).
- Los agentes pueden hacer `git push`/PRs solo si el usuario lo autoriza; `gh release` denegado.
- Auth: JWT HS256 con claim `token_type` (access/refresh) para impedir replay cruzado; TTLs por env
  `JWT_ACCESS_TTL` (15m) y `JWT_REFRESH_TTL` (168h). `JWTManager` inyecta reloj (`now`) para tests.
- `AuthRequired`/`RBAC` son opcionales en `router.Dependencies`: si faltan, las rutas auth no se registran
  (mantiene compatibilidad con tests de router existentes).
- Logout es stateless por ahora (sin denylist); se volverá stateful con token store/versionado.
- Migraciones: tabla de tracking `schema_migrations (version PK, name, applied_at)` por BD.
  Orden lexicográfico/núm. ascendente; `up` idempotente; `down` revierte las últimas N
  (nuevas primero) en transacción por migración. `-dir` autodetecta `migrations/` subiendo
  desde el cwd (`backend/migrations` desde la raíz).
- ETL: upsert multi-fila `INSERT ... ON CONFLICT (id) DO UPDATE ... RETURNING (xmax = 0) AS inserted`
  (inserted/updated derivados del booleano); conflicto siempre por PK (IDs de origen copiados
  verbatim ⇒ idempotente). Colisiones de claves únicas secundarias (email/SKU/order_number con
  PK distinto) abortan el lote a propósito. Cada lote en `db.Transaction`; sub-chunks de 1000
  filas/statement. Reanudación por keyset (`WHERE id > ? ORDER BY id LIMIT ?`) + `batchTracker`
  que solo avanza el checkpoint sobre una racha contigua de lotes OK. Checkpoint en
  `MemoryStore` (default) o `FileStore` JSON atómico (`logs/etl-checkpoint.json` en el CLI).
- ETL transform: normaliza (email lower, country/currency upper, status lower), recomputa
  `orders.total_amount = round2(sum(subtotals))` y `order_items.subtotal = round2(qty*unit_price)`;
  filas inválidas → `SkipRecord` (motivo) y se cuentan en métricas.
- API: envelope estable `{"error":{code,message,details}}` (códigos validation_failed/bad_request/
  not_found/internal_error) y `{"data":...,"meta":{page,per_page,total,total_pages}}` (defaults
  page=1/per_page=20, máx 100; `page=0`/`per_page=0` equivalen a omitido). Fechas: RFC3339 o
  `YYYY-MM-DD` (el bound de fin de rango se extiende al último instante del día).
- API: capas handler→service→repository; puertos `service.SourceData` y `repository.*Repository`
  permiten fakes (sin BD en tests). Repos: búsqueda `ILIKE` con wildcards escapados, orden
  `created_at/order_date DESC, id DESC` (`order_items` por id ASC), `GetOrder` preload de items.
- API: `router.Dependencies` extendido de forma retrocompatible con `CORS`, `Data` (nil ⇒ grupo
  `/api/v1/source` no registrado), `Target` (nil ⇒ grupo `/api/v1/target` no registrado),
  `Jobs`/`Scheduler` (nil ⇒ grupo no registrado) y `Middleware []gin.HandlerFunc` (hook opcional
  aplicado a los grupos source, target, jobs y scheduler: por dónde se enchufa auth sin importar
  `internal/auth`). En `cmd/api/main.go` se pasa `middleware.AuthRequired(tokenManager)`.
- API target/analytics: `repository.TargetRepositories` es alias de `SourceRepositories` (mismo
  esquema ⇒ mismos repos GORM contra `lab_target`). `service.TargetData` embebe `SourceData` y
  añade `Summary`/`TopProducts`/`TopCustomers`; `TargetDataService` delega entidades en un
  `SourceData` y analytics en `repository.AnalyticsRepository` (SQL Raw). Top-N: `limit` default
  10, máx 100 (`service.NormalizeAnalyticsLimit`); respuesta `{"data":[...],"meta":{"limit":N}}`.
  `summary` devuelve el objeto plano. `models.AnalyticsSummary.OrdersByStatus` lleva `gorm:"-"`
  para que GORM no lo interprete como relación al escanear el SQL Raw.
- Swagger: swag v1 (`swag init -g cmd/api/main.go -o docs/swagger` ⇒ `backend/docs/swagger`,
  package `swagger`, importado en blanco por `cmd/api`). Genera spec **Swagger 2.0**, no OAS 3.x
  (el CLI v2 con OAS 3.1 no es compatible con gin-swagger v1.6.1). Tag `swaggertype:"object"` en
  `models.Order.ShippingAddress` para que swag resuelva `json.RawMessage`.
- Jobs: `jobs.Manager` mantiene **un solo job activo** (slot reservado en `Start`; segundo intento
  ⇒ `ErrJobRunning`). `RunnerFunc func(params any) (JobFunc, error)` construye el trabajo;
  `JobFunc func(ctx, report func(EntityProgress)) (map[string]any, error)` (el report recibe
  `EntityProgress`, no `Progress` completo; el Manager compone `Progress`). Estados
  pending/running/completed/failed/stopped + `idle`; sentinelas `ErrJobRunning`, `ErrNoRunningJob`,
  `ErrUnknownJobType`, `ErrInvalidParams`. `Stop()` cancela el ctx del job (no error si idle).
  `History` pagina sobre `job_executions` (defaults 1/20, máx 100). Runners: `NewGenerateRunner`
  (generator.Store) y `NewMigrateRunner` (etl.Source + etl.Loader).
- Scheduler: `scheduler.Scheduler` envuelve `robfig/cron/v3` (5 campos, `cron.ParseStandard` para
  validar y calcular `next_run`). `ScheduleRepository` (List/Get/Create/Update/Delete) sobre
  `cron_schedules`; `JobStarter` = `jobManager.Start`. Operaciones List/Create/Update/Delete/
  Enable/Disable/RunNow; `Enabled *bool` opcional en Create/Update (tri-estado). Sentinelas
  `ErrNotFound`, `ErrInvalidCron`, `ErrInvalidParams`, `ErrUnknownCommand`; `WithValidCommands`
  restringe `command` a generate/migrate. `Start(ctx)` carga y registra los schedules habilitados.
- API jobs/scheduler: contrato start ⇒ `202` `{"data":{job_id,type,status}}`; stop ⇒ `200`
  `{"data":{"stopped":true}}`; status/progress ⇒ `200` `{"data":...}`; history ⇒ envelope paginado;
  create schedule ⇒ `201`; delete ⇒ `204` sin body; run ⇒ `200` `{"data":{"job_id":...}}`.
  Código de error nuevo `conflict` (409) en `handler/response.go` (`RespondConflict`), además de
  `RespondData`.
- CRUD: solo sobre `lab_source` (`lab_target` queda solo lectura). Contrato: POST ⇒ `201`
  `{"data":<row>}`, PUT ⇒ `200` `{"data":<row>}`, DELETE ⇒ `204` sin body; errores
  not_found 404 / conflict 409 / invalid_reference 400 / internal 500. `Update` de customer/product
  usa `Select(...)` (persiste ceros/nil); el de order usa `Updates(struct)` (omite ceros ⇒ solo
  escribe los campos provistos por el DTO con punteros). Normalización: email lower, country/currency
  upper, status lower; `order_number` autogenerado `ORD-<unixnano>` si falta; `total_amount` y
  `subtotal` recomputados con `round2`. `UpdateOrder` sin cambios no llama al repo (evita falso 404)
  y devuelve la orden completa con items vía `GetByID`.
- RBAC (mapa aprobado): lecturas (GET source/target/jobs status/scheduler list) = cualquier rol
  (`AuthRequired`); escrituras (POST/PUT/DELETE de source, jobs control y scheduler) =
  `RBAC(admin,operator)` vía `Dependencies.WriteMiddleware`; `/api/v1/admin/*` =
  `RBAC(admin)` vía `Dependencies.AdminMiddleware` (+ `AuthRequired`). El grupo admin solo se
  registra si `Admin != nil && len(AdminMiddleware) > 0` (nunca abierto por defecto). Roles:
  `models.UserRole` admin/operator/viewer.
- Admin logs: `service.FileErrorLogs` lee **solo la cola ≤4 MiB** del log JSON rotado (evita escanear
  100 MB), parsea una línea zap por registro (`level`,`ts`,`msg`,`caller` + extras en `fields`),
  conserva niveles de error (ERROR/FATAL/PANIC/DPANIC, descarta registros sin `msg`), los devuelve
  **newest-first** y pagina. Archivo inexistente ⇒ vacío sin error. `ts` acepta ISO8601 de zap,
  RFC3339 y número unix.

## Dependencias nuevas (API)
- `github.com/swaggo/gin-swagger`, `github.com/swaggo/files`, `github.com/swaggo/swag` v1.16.6.
- `github.com/go-playground/validator/v10` promovida de indirecta a directa (ya estaba vía Gin).
- `github.com/jackc/pgx/v5` promovida de indirecta a directa (para `pgconn`/códigos de error en
  `repository/errors.go`).
- `github.com/rogpeppe/go-internal v1.12.0` fijada como indirecta solo para que `go mod tidy`
  no suba el toolchain (`go 1.24.0` intacto; swag arrastra ese test-dep transitivo).

## Pendientes / bugs
- 🟠 **El percent global puede retroceder** (semántica **preexistente** del Issue #13, ahora visible en la
  UI): `cur.percent = percentOf(sum(done), sum(total))` sobre las **entidades conocidas hasta ese
  momento** ⇒ cuando aparece una entidad nueva a mitad de job el denominador crece y la barra baja
  (medido en vivo: 100% → 97% → 90% → 100% con customers/products/orders/order_items). El clamp
  monótono **no** sirve (se quedaría en 100% tras terminar la 1ª entidad). El fix correcto exige que el
  runner **declare el plan de entidades con sus totales antes de empezar** (cambio de contrato en
  `jobs.JobFunc`/runners) para que el agregado sea monótono desde el inicio ⇒ **decisión pendiente del
  usuario** (no se aplicó en el Issue #21 por ser un cambio transversal).
- 🟡 **Reconexión del SSE tras el evento terminal**: por diseño el servidor **cierra** el stream cuando el
  job termina ⇒ el `EventSource` del navegador reconecta solo (cada ~3s, el default) ⇒ con la
  `JobsView` abierta hay un bucle de reconexión cada 3s (cada reconnect recibe el snapshot del job
  terminal y se le cierra). El progreso del **siguiente** job se captura en la reconexión (con un hueco
  de hasta ~3s, mitigado por el snapshot). Opciones: (a) el frontend cierra el `EventSource` al detectar
  estado terminal y reconecta bajo demanda; (b) el backend mantiene el stream abierto (heartbeat) en vez
  de cerrarlo (contradice la decisión actual de "cierre al terminar el job").
- 🟡 **Cobertura frontend bajó** respecto al baseline del #20 (97.92%/92.52% → 94.72%/85.97%): los puntos
  débiles son las ramas de `composables/useJobEvents.ts` (56.66%) y `api/jobs.ts` (53.57%) — candidatos
  a tests de seguimiento. Umbral 80% cumplido en todas las métricas.
- `002_seeders.up.sql` (source) y `003_admin_bootstrap.up.sql` (target) son placeholders (`SELECT 1`)
  **por diseño** (seeders vía generator service; hash del admin en código).
- `internal/repository` sin tests de integración con DB (los repos GORM requieren Postgres de test;
  ya hay unit tests de `mapPgError` en `errors_unit_test.go`).
- `docs/ISSUES.md` es un backlog estático: no se marcan checkboxes (ni siquiera tareas ya hechas);
  el estado real vive en este MEMORY.
- **Issue #37 (GitHub)**: pasar la UI del frontend a **inglés** — cadenas en español en
  `config/nav.ts`, `router/index.ts`, `LoginView.vue`, `AppLayout.vue`, `PlaceholderView.vue`
  e `index.html` (`lang="es"`), + tests que las afirman. Decisión anotada: inglés hardcodeado,
  sin librería i18n (solo hay un idioma).
- ✅ **Issue #38 (GitHub) COMPLETADO @ WIP (20-oct, sin commitear)**: color al tema — paleta
  **azul** (preset blue de shadcn) en `web/src/assets/main.css` **solo con variables**: `:root`
  y `.dark` → `--primary`/`--ring`/`--sidebar-primary` = Tailwind **blue-600**
  (`oklch(0.546 0.245 262.881)`, oscuro usa blue-500 para ring/sidebar-primary),
  `--accent` con tinte azul (blue-100 claro / blue-900 oscuro), `--chart-1..5` multicolor
  (blue/green/amber/violet/rose; 600 en claro, 400 en oscuro). Fondos/superficies neutros
  sin cambios. **Contraste AA medido** con script one-off OKLCH→WCAG: **26/26 pares PASS**
  (texto ≥4.5, iconos/gráficos ≥3, ambos modos). Sin cambios en componentes ni tests:
  110 tests, cobertura **98.35% / 95.34%**, `type-check`/`lint`/`build` ✅.
  Extensión opcional futura: selector de acento (store `accent.ts` aparte, `gdl.accent`,
  anti-flash).
- ✅ **Issue #39 (GitHub) COMPLETADO @ 259124d (19-oct)**: `LandingView` + `PublicLayout`
  nuevos, Dashboard movido a `/dashboard`. Copy de la landing en inglés (adelanto del #37).
  **Trampa resuelta con 3 puntos de acoplamiento** (el issue solo anticipaba 2): `nav.ts`
  (`to: '/'`→`'/dashboard'`), filtro del router (`item.to !== '/'` → `item.routeName !== 'dashboard'`,
  inmune a cambios de ruta) y **`DashboardView`** (tenía el mismo filtro por `to`). Arquitectura
  de rutas: el padre `AppLayout` sigue en `/` con `requiresAuth` y el dashboard es hijo
  `path: 'dashboard'` — así `/` sin hijo resuelve en la landing y las URLs de secciones
  (`/customers`, `/jobs`…) **no** se anidan bajo `/dashboard`. Guard: autenticado en `/` ⇒
  redirect a `/dashboard`; catch-all → `/` y el guard resuelve. `LoginView` redirige tras
  login a `{ name: 'dashboard' }`. Verificado: **104 tests, 98.23% stmts / 95.3% branches**
  (subió), `type-check`/`lint`/`build` ✅. Decisiones del subagente a revisar: `meta.public`
  documental (el guard no lo lee), `PublicLayout` sin toggle de tema, catch-all con doble
  redirect.
- Completar Issues #2+ (migraciones de negocio, frontend).

## Siguiente paso
- ✅ **Issue #21 publicado**: el **PR #42** (`feature/data-tables-views` → `main`) se **actualizó con
  los 4 commits** del #21 (fast-forward `8c4f937..1b5ec88`, +4.4k líneas) ⇒ el PR lleva
  **#20 (tablas) + #21 (monitoreo/SSE) + #16 (SSE backend)** en un solo PR. Título actualizado y body
  reescrito con el mapeo de issues, los 4 bugs corregidos, la verificación y los pendientes.
  Cierra en GitHub **#17**, **#21** y **#22** al mergear. Estado: **OPEN**, `MERGEABLE`, `mergeStateStatus: CLEAN`,
  65 archivos / +7.724 / -60. Push autorizado por el usuario (`git push origin feature/data-tables-views`).
  `gh release` sigue denegado.
- 🟡 **Issues de GitHub que quedaron abiertos por error de numbering** (ya implementados y mergeados en
  PRs anteriores, pero nunca cerrados): **#18** (frontend setup), **#19** (authentication UI),
  **#20** (dashboard & layout) ⇒ cerrar cuando el usuario lo confirme.
- 🟠 **Decisión pendiente del usuario**: el **percent global retrocede** (100% → 97% → 90%). Fix correcto
  = que el runner declare el plan de entidades con sus totales antes de empezar (cambio de contrato
  transversal en `jobs.JobFunc`/runners). Abordarlo en #22 o dejarlo documentado.
- ✅ **Issue #21 cerrado** (ver "Estado actual"): backend SSE + panel de control en el frontend, con
  4 bugs corregidos y E2E en vivo verificado.
- 🚧 **Issue #20 (Data Tables & Views) IMPLEMENTADO @ `8c4f937` (21-oct)** — decisiones del usuario:
  **solo lectura** (sin CRUD de escritura) y tabla con el **`Table` de shadcn-vue** (sin TanStack).
  Nuevos: `types/data.ts` (IDs como `string`), `api/data.ts` (source+target, omite filtros vacíos),
  `composables/useDataList.ts` (paginación+filtros sincronizados con la **query string**, tokens
  verbatim), `components/{DataTable,DataFilters,StatusBadge}.vue`, `ui/table/**`, y vistas
  `CustomersView`/`ProductsView`/`OrdersView`/`OrderDetailView`/`TargetView`. Router: `IMPLEMENTED_ROUTES`
  hace que solo **scheduler** y **admin-logs** sigan en `PlaceholderView` (jobs ya implementado en #21).
  **Orden client-side** sobre la página actual (el backend ordena fijo ⇒ no hay endpoint de orden).
  `OrderItemsView` no se creó: el detalle de orden ya tabla sus items.
  **Revisión del orquestador**: detecté 2 strings hardcodeados en inglés (`useDataList.ts` y
  `OrderDetailView.vue`) que rompían la regla de i18n ⇒ test rojo + fix usando `table.error`
  existente (ES/EN). Verificado: **249 tests** (23 archivos), cobertura **97.92% stmts / 92.52%
  branches**, `type-check`/`lint`/`build` ✅; E2E contra el backend vivo: `source/customers` con
  meta y tokens `v1.*`, `GET /customers/{token}` ⇒ 200.
- ✅ **PR #41 mergeado en `main`** (`c47ddfc`). Issues #37 (i18n ES/EN), #38 (paleta) y #39 (landing)
  **cerrados en GitHub**. PR **#42** (`feature/data-tables-views` → `main`) **abierto** con el #20.
- Luego Issues **#22** (scheduler), **#23** (admin logs) y **#24** (analytics), y #25-#31
  (testing/infra/docs). (landing siempre visible, Issue #38 paleta azul, footer CC + ThemeToggle).
  `gh release` sigue denegado. **Ojo**: el PR #40 ya había sido **mergeado** (`ec11a7e`), así que la rama
  siguió creciendo post-merge ⇒ se abrió el **PR #41** (`feature/jobs-scheduler-api` → `main`)
  con los 10 commits acumulados (secureid T2, lint, #38, #39, landing, footer/toggle).
- ✅ **Issue #37 (i18n) RESUELTO @ `797b493` (21-oct)**: el usuario eligió **doble idioma ES/EN**
  (no inglés hardcodeado) y delegó la ejecución en `frontend-vue`. Implementado con
  `vue-i18n@11.4.16`: infra `i18n/` + `stores/locale.ts` + `LanguageToggle.vue` en las 3
  superficies; todas las cadenas de UI pasan por `t()` (vistas, layout, nav, router meta,
  footer, toggles). Fuera de alcance por decisión: `document.title` dinámico, i18n de
  fechas/números, mensajes de error del backend.
- **Rama**: el trabajo vive en `feature/jobs-monitoring-sse` (4 commits por delante de `origin/main`;
  contiene a `feature/data-tables-views`, la base del PR #42).
- Considerar rate limiting / auditoría de escrituras y logout stateful (denylist de tokens).
