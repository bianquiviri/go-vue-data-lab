# MEMORY — Go Concurrent Data Processing Lab

Memoria viva del proyecto. Se carga en cada sesión de opencode.
Mantener concisa. Actualizar con `/remember` al cierre de cada bloque de trabajo.

## Estado actual
- **Issue #1 (infra local) completa y validada**: `deploy/docker-compose.yml` (postgres:15-alpine + caddy:2-alpine,
  healthcheck `pg_isready`, `extra_hosts` host-gateway, red `lab_network`), `deploy/Caddyfile`
  (TLS mkcert montado → `host.docker.internal:8080`), `deploy/init-db.sh` idempotente (lab_source/lab_target),
  `deploy/gen-certs.sh`, `deploy/certs/` git-ignorado.
- Backend ejecutable: `backend/cmd/api/main.go`, config, router, handler health, DB (lab_source/lab_target),
  logger zap+lumberjack, middleware. Migraciones SQL source/target UP/DOWN.
- **Issue #6 (auth) completa**: `internal/auth` (Argon2id `Hash`/`Verify` + `JWTManager` access/refresh HS256),
  `internal/repository/auth.go` (`UserRepository` GORM + `EnsureUser` transaccional), `internal/service/auth.go`
  (login/refresh/me/change-password/logout) y `bootstrap.go` (`BootstrapAdmin` idempotente),
  `internal/middleware/auth.go` (`AuthRequired` + `RBAC`), `internal/handler/auth.go` y rutas `/api/v1/auth/*`.
  Bootstrap admin `root@gmail.com` desde env en el arranque. Tests unitarios con mocks (auth 90%, service 90%,
  handler 90%, middleware 89%, router 96%).
- `web/` y `frontend/` siguen vacíos.

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
