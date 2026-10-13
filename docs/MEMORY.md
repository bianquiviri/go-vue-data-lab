# MEMORY — Go Concurrent Data Processing Lab

Memoria viva del proyecto. Se carga en cada sesión de opencode.
Mantener concisa. Actualizar con `/remember` al cierre de cada bloque de trabajo.

## Estado actual
- Solo existe **scaffolding** (~15-20%). 1 commit (`Initial commit`); el resto sin commitear.
- Presente: `deploy/*` (compose, Caddyfile, init-db), `.env.example`, `Makefile`,
  `backend/internal/config/config.go`, 7 modelos GORM, migraciones source/target UP/DOWN,
  `README.md`, `docs/ISSUES.md`.
- **No hay código ejecutable**: `backend/cmd/api/` está vacío. `web/` y `frontend/` vacíos.
- `go.mod` solo con `uuid` + `godotenv`.

## Decisiones
- Frontend va en `web/` (no en `frontend/`).
- Sin AutoMigrate: migraciones SQL versionadas.
- Los agentes tienen `git push` denegado; publicación solo por el usuario.

## Pendientes / bugs
- BUG: `backend/migrations/target/002_control_tables.up.sql:29` → falta comilla: `DEFAULT 'viewer'`.
- `go.mod`: `godotenv` marcado `// indirect` aunque es dependencia directa.
- `002_seeders.up.sql` y `003_admin_bootstrap.up.sql` son placeholders (`SELECT 1`).
- `docs/ISSUES.md`: todas las tareas siguen sin marcar.

## Siguiente paso
- Crear `backend/cmd/api/main.go`: conexión a `lab_source`/`lab_target`, logging zap+lumberjack y health check.
