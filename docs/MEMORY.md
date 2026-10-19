# MEMORY — Go Concurrent Data Processing Lab

Memoria viva del proyecto. Se carga en cada sesión de opencode.
Mantener concisa. Actualizar con `/remember` al cierre de cada bloque de trabajo.

## Estado actual
- **PR #40 abierto** (`feature/jobs-scheduler-api` → `main`) — *un solo PR* (decisión del
  usuario), 7 commits, 98 archivos / +15.3k líneas: **secureid T1** + **frontend Issues #17-#19**
  + **infra hosts/puertos**. Push + `gh pr create` autorizados por el usuario; `gh release`
  sigue denegado. Verificado antes de subir: `go vet`/`go build`/`go test -race` ✅, y en web
  88 tests, **cobertura 98.07% stmts / 94.87% branches**, `type-check`, `lint`, `build` ✅.
- **main**: PRs #32-#35 y **#36** mergeados (`2ae4699`). `make build`, `go test -race ./...`
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
  es gratis. Pendiente: TDD de los 5 bloques + `make swagger`.
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
- **Issue #38 (GitHub)**: dar **color** al tema — la paleta es gris puro (`oklch(x 0 0)`, croma 0
  en todos los tokens, `chart-1..5` también grises ⇒ analítica ilegible). Alcance mínimo:
  `--primary/--ring/--accent/--sidebar-primary/--chart-1..5` con matiz y contraste AA medido.
  Extensión opcional: selector de acento (store `accent.ts` aparte, `gdl.accent`, anti-flash).
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
- **T2 secureid EN CURSO** (subagente backend-go, 19-oct): inyección via
  `router.Dependencies.Codec` + context value, walk outbound en `RespondJSON`/`RespondData`/
  `RespondPaginated`, inbound token-only (`parseUUIDParam`/`parseOptionalUUID`), bodies
  (`data_write.go`), `make swagger`, TDD. Al terminar: revisar su entrega, `make build/test/lint`
  y commit del orquestador.
- Frontend: **#37** (inglés) → **#38** (color). El #39 ya está hecho; el #38 la landing es
  buen banco de pruebas.
- **Rama**: todo el trabajo sigue en `feature/jobs-scheduler-api` (misma rama del PR #40,
  decisión del orquestador: no se puede ramificar de `main` porque el frontend solo existe
  aquí, y había un subagente concurrente en `backend/`). **Ojo**: al subir, los commits nuevos
  (#39 y T2) se cuelan en el PR #40 — decidir con el usuario si se parte en PR apilado.
- Luego Issues #20-#24: tablas CRUD, jobs/scheduler, admin logs, analytics.
- Considerar rate limiting / auditoría de escrituras y logout stateful (denylist de tokens).
