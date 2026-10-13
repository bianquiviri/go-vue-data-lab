# AGENTS.md — Go Concurrent Data Processing Lab

Contexto persistente del proyecto. Se carga en **cada sesión** de opencode.

## Qué es
Plataforma de procesamiento concurrente de datos, simulación masiva y analítica
en tiempo real para un modelo de **e-commerce sintético** (customers, products,
orders, order_items).

## Stack
- **Backend**: Go, Gin, GORM + SQL Raw. Arquitectura limpia, DI, errores con `%w`.
- **DB**: PostgreSQL con dos esquemas independientes: `lab_source` y `lab_target`.
  Migraciones SQL versionadas UP/DOWN (sin depender de AutoMigrate).
- **Concurrencia**: goroutines, channels, `context`, recuperación de pánicos, lotes 3,000–4,000.
- **Scheduler**: `robfig/cron/v3` persistido en PostgreSQL.
- **Observabilidad**: `go.uber.org/zap` + `lumberjack`; endpoint `GET /api/v1/admin/logs/errors`.
- **Auth**: Argon2id/bcrypt, bootstrap `root@gmail.com` desde env, JWT, RBAC (admin/operator/viewer).
- **API docs**: OpenAPI 3.x / Swagger UI en `/swagger/index.html`.
- **Frontend**: Vue 3 (`<script setup>`), TypeScript, Vite, Tailwind, shadcn-vue, pinia, vue-router.
  Temas Claro/Oscuro/Sistema persistidos en el navegador.
- **HTTPS local**: `https://godata.josebianco.local` con Caddy + mkcert.

## Estructura
- `backend/cmd/api` — entrypoint (pendiente).
- `backend/internal/` — `config`, `domain/models`, y capas por crear.
- `backend/migrations/{source,target}` — migraciones SQL.
- `web/` — frontend (fuente única; `frontend/` está vacío y no se usa).
- `deploy/` — docker-compose, Caddyfile, init-db.sh.
- `docs/ISSUES.md` — backlog por fases. `docs/MEMORY.md` — memoria viva.

## Comandos
```bash
make build   # backend build
make run     # backend run
make test    # go test -race -v ./...
make lint    # golangci-lint
make migrate-up / migrate-down
make docker-up / docker-down
make swagger
```

## Convenciones
- TDD (Red → Green → Refactor). Cobertura ≥80% (Go y TS/Vue).
- Errores Go: `fmt.Errorf("contexto: %w", err)`.
- Sin secretos en el código. `.env` ignorado; usar `.env.example`.

## Reglas de agentes
- Equipo definido en `.opencode/agent/`: `orchestrator` (primary), `backend-go`,
  `frontend-vue`, `qa-testing`, `devops`.
- **Publicación**: solo el usuario autoriza `git push` / PRs / releases. Los agentes
  tienen `git push` denegado por permisos.
- Al cerrar un bloque de trabajo, actualizar `docs/MEMORY.md` (o usar `/remember`).
