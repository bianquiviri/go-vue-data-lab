# Go Concurrent Data Processing Lab

A high-performance platform for concurrent data processing, massive simulation, and real-time analytics for synthetic e-commerce models.

## Tech Stack

- **Backend**: Go (Golang) with Gin, GORM, SQL Raw
- **Database**: PostgreSQL (two schemas: lab_source, lab_target)
- **Frontend**: Vue 3 + TypeScript + Vite + Tailwind CSS + shadcn-vue
- **Concurrency**: Goroutines, channels, worker pools, context cancellation
- **Scheduler**: cron/v3 with PostgreSQL persistence
- **Observability**: zap logging with lumberjack rotation
- **Security**: Argon2id/bcrypt for password hashing, RBAC (admin/operator/viewer),
  opaque encrypted IDs in URLs/JSON (AES-256-GCM via `SECURE_ID_KEY`)
- **Docs**: OpenAPI 3.x / Swagger UI

## Local Environment Setup

Docker Compose runs **PostgreSQL** and **Caddy**; the Go backend runs on the
**host** via `make run` (port `8080`) and the frontend runs on the **host** via
`npm run dev` (port `5174`).

**Port note:** this machine's `taskflow` stack already owns `80`/`443`/`5173`,
so the lab uses **`8443` (HTTPS)**, `8081` (HTTP) and `5174` (Vite). Caddy
terminates TLS for two local hosts:

| Host                                          | Proxies to                          |
| --------------------------------------------- | ----------------------------------- |
| `https://apidatago.josebianco.local:8443`     | `host.docker.internal:8080` (Go API) |
| `https://frontgodata.josebianco.local:8443`   | `host.docker.internal:5174` (Vite dev server) |

The browser talks only to `frontgodata...:8443` (same origin); Vite's dev proxy
forwards `/api` and `/swagger` to the backend on `localhost:8080`, so no CORS
is needed on that path. `CORS_ALLOWED_ORIGINS` still lists both lab origins
(with port) for direct API calls (e.g. Swagger UI served from
`apidatago...:8443`).

### Prerequisites

| Tool           | Notes                                              |
| -------------- | -------------------------------------------------- |
| Docker         | Docker Desktop or Docker Engine + Compose plugin   |
| mkcert         | `brew install mkcert nss` (nss needed for Firefox) |
| Go             | 1.24+ (to run the backend)                         |
| Node           | 20+ / npm (to run the frontend)                    |

### 1. Environment file

```bash
cp .env.example .env    # then edit values; .env is git-ignored
```

The repo-root `.env` is loaded by **both** the Go backend (via `godotenv`) and
Docker Compose: `make docker-up` passes it to Compose with `--env-file`. Keep
`POSTGRES_USER`/`POSTGRES_PASSWORD` in sync with `DB_USER`/`DB_PASSWORD` so the
database and the backend agree. Alternatively, place a `deploy/.env` file (also
git-ignored) to configure Compose only. Never commit either file.

**Required key:** `SECURE_ID_KEY` — 32 bytes, base64 or hex. The backend
refuses to start without it (it encrypts every entity ID exposed in URLs and
JSON responses). Generate one with:

```bash
openssl rand -base64 32     # or: openssl rand -hex 32
```

### 2. Generate local TLS certificates

```bash
make certs              # runs deploy/gen-certs.sh -> mkcert -install
```

This installs a locally-trusted root CA and writes, for **each** lab host
(`apidatago...` and `frontgodata...`), a `deploy/certs/<host>.pem` +
`deploy/certs/<host>-key.pem` pair (git-ignored). `mkcert -install` may prompt
for your password / Touch ID.

**TLS approach:** the `Caddyfile` loads the mounted mkcert certificates
(`tls /certs/<host>.pem /certs/<host>-key.pem`), so browsers show a valid
padlock. The certificates must exist before `make docker-up` (the Makefile
enforces this). If you cannot install mkcert, an alternative is Caddy's own
local CA: replace the `tls` lines with `tls internal` and import the generated
root CA from the `caddy_data` volume (the browser will warn until you trust
it). mkcert is preferred because its CA is trusted automatically on the host.

### 3. Add the local hostnames

```bash
# /etc/hosts
127.0.0.1   apidatago.josebianco.local
127.0.0.1   frontgodata.josebianco.local
```

On macOS apply it with:

```bash
sudo sh -c 'grep -q "apidatago.josebianco.local" /etc/hosts || echo "127.0.0.1   apidatago.josebianco.local" >> /etc/hosts'
sudo sh -c 'grep -q "frontgodata.josebianco.local" /etc/hosts || echo "127.0.0.1   frontgodata.josebianco.local" >> /etc/hosts'
```

### 4. Start the infrastructure

```bash
make docker-up          # starts postgres + caddy (detached)
```

On first boot PostgreSQL creates the two databases `lab_source` and `lab_target`
(via `deploy/init-db.sh`). Verify with `make docker-logs`.

### 5. Run the backend

```bash
make migrate-up         # apply SQL migrations (first run / after changes)
make run                # backend on http://localhost:8080
```

API check: **https://apidatago.josebianco.local:8443/api/v1/health**
Swagger UI: **https://apidatago.josebianco.local:8443/swagger/index.html**

### 6. Run the frontend

```bash
cd web
npm install             # first run only
npm run dev             # Vite on http://localhost:5174
```

Then open: **https://frontgodata.josebianco.local:8443**

The full request flow is: browser → Caddy (`:8443`, TLS) →
`host.docker.internal:5174` → Vite → (`/api` proxy) →
`host.docker.internal:8080` → Go backend → PostgreSQL. The database is
reachable only on the host loopback (`127.0.0.1:5433`, chosen because other
local projects commonly own 5432) and is not exposed to your LAN. `make
docker-up` publishes it there; keep `DB_PORT` in `.env` in sync.

#### Port conflicts with other local projects

The lab expects `8080` for the API, but this machine already runs other stacks.
If `make run` fails with *address already in use*:

```bash
SERVER_PORT=18080 make run                          # API on :18080
cd web && VITE_BACKEND_TARGET=http://localhost:18080 npm run dev
```

Vite then proxies `/api` to `:18080`. Caddy keeps pointing at `:8080`
(`deploy/Caddyfile`), so the `https://…:8443` entrypoint only reflects the API
when the backend is actually listening on `8080`.

### Useful commands

```bash
make test              # go test -race -v ./...
make lint              # golangci-lint run
make swagger           # regenerate Swagger docs
make docker-down       # stop containers (keeps data)
make docker-reset      # stop and WIPE volumes/database
make docker-logs       # follow container logs
```

Frontend (`web/`): `npm run test` (vitest), `npm run type-check`,
`npm run lint`, `npm run build`.

See `docs/` for detailed project documentation.
