# MEMORY — Go Concurrent Data Processing Lab

Memoria viva del proyecto. Se carga en cada sesión de opencode.
Mantener concisa. Actualizar con `/remember` al cierre de cada bloque de trabajo.

## Estado actual
- **main @ 9e814e4**: los 4 PRs están mergeados (#32 generator+worker, #33 infra polish,
  #34 web icons, #35 auth). `make build`, `go test -race ./...` y `go vet ./...` en verde.
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
- **PR #34**: `web/public/` con iconos (SVG + PNGs) y `manifest.webmanifest`; `web/scripts/generate-icons.sh`.
- `frontend/` sigue vacío (no trackeado por git); no usar.

## Branch local recuperado (WIP de otra sesión)
- **`wip/jobs-scheduler-api` (local, sin push)**: ~20k líneas de trabajo no commiteado en otra sesión
  que se perdió al hacer `git stash drop` y se recuperó del commit del stash. Contiene: `internal/jobs`
  (manager + runners generate/migrate), `internal/scheduler` (cron persistido en lab_target), `internal/etl`
  (source→target con checkpoint y métricas), `internal/migrate` (runner de migraciones con tracker),
  `cmd/migrate` y `cmd/migrate-data`, endpoints source/target/analytics, CORS middleware, swagger docs
  (`docs/swagger`), y repositorios (customer, product, order, cron_schedule, job_execution, analytics).
  **Está basado en el main anterior (7733502)**, no en el actual (9e814e4). Requiere rebase/merge con
  los cambios de auth y generator antes de continuar.

## Decisiones
- Frontend va en `web/` (no en `frontend/`).
- Sin AutoMigrate: migraciones SQL versionadas.
- TLS local: certificados **mkcert montados** en `/certs` (no `tls internal`); `make docker-up` falla si faltan.
- Un único `.env` raíz alimenta backend (godotenv) y Compose (`--env-file ../.env` desde el Makefile).
- Los agentes tienen `git push` denegado; publicación solo por el usuario.
- Auth: JWT HS256 con claim `token_type` (access/refresh) para impedir replay cruzado; TTLs por env
  `JWT_ACCESS_TTL` (15m) y `JWT_REFRESH_TTL` (168h). `JWTManager` inyecta reloj (`now`) para tests.
- `AuthRequired`/`RBAC` son opcionales en `router.Dependencies`: si faltan, las rutas auth no se registran
  (mantiene compatibilidad con tests de router existentes).
- Logout es stateless por ahora (sin denylist); se volverá stateful con token store/versionado.

## Pendientes / bugs
- `002_seeders.up.sql` y `003_admin_bootstrap.up.sql` son placeholders `SELECT 1` **por diseño**
  (seeders vía generator service; hash del admin en código).
- `internal/repository` sin tests de integración con DB (solo `mapError`/constructor); requiere Postgres de test.
- Completar Issues #2+ (migraciones de negocio, CRUD customers/products/orders, frontend).

## Siguiente paso
- Implementar CRUD de customers/products/orders (repository/service/handler) con tests y RBAC por rol.
