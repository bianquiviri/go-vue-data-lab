# Go Concurrent Data Processing Lab

A high-performance platform for concurrent data processing, massive simulation, and real-time analytics for synthetic e-commerce models.

## Tech Stack

- **Backend**: Go (Golang) with Gin, GORM, SQL Raw
- **Database**: PostgreSQL (two schemas: lab_source, lab_target)
- **Frontend**: Vue 3 + TypeScript + Vite + Tailwind CSS + shadcn-vue
- **Concurrency**: Goroutines, channels, worker pools, context cancellation
- **Scheduler**: cron/v3 with PostgreSQL persistence
- **Observability**: zap logging with lumberjack rotation
- **Security**: Argon2id/bcrypt for password hashing, RBAC (admin/operator/viewer)
- **Docs**: OpenAPI 3.x / Swagger UI

## Local Environment Setup

Docker Compose runs **PostgreSQL** and **Caddy**; the Go backend runs on the
**host** via `make run` (port `8080`). Caddy terminates TLS for
`https://godata.josebianco.local` and proxies to the host backend.

### Prerequisites

| Tool           | Notes                                              |
| -------------- | -------------------------------------------------- |
| Docker         | Docker Desktop or Docker Engine + Compose plugin   |
| mkcert         | `brew install mkcert nss` (nss needed for Firefox) |
| Go             | 1.22+ (to run the backend)                         |

### 1. Environment file

```bash
cp .env.example .env    # then edit values; .env is git-ignored
```

The repo-root `.env` is loaded by **both** the Go backend (via `godotenv`) and
Docker Compose: `make docker-up` passes it to Compose with `--env-file`. Keep
`POSTGRES_USER`/`POSTGRES_PASSWORD` in sync with `DB_USER`/`DB_PASSWORD` so the
database and the backend agree. Alternatively, place a `deploy/.env` file (also
git-ignored) to configure Compose only. Never commit either file.

### 2. Generate local TLS certificates

```bash
make certs              # runs deploy/gen-certs.sh -> mkcert -install
```

This installs a locally-trusted root CA and writes
`deploy/certs/godata.josebianco.local.pem` + `...-key.pem` (git-ignored).
`mkcert -install` may prompt for your password / Touch ID.

**TLS approach:** the `Caddyfile` loads the mounted mkcert certificates
(`tls /certs/godata.josebianco.local.pem /certs/godata.josebianco.local-key.pem`),
so browsers show a valid padlock. The certificates must exist before
`make docker-up` (the Makefile enforces this). If you cannot install mkcert, an
alternative is Caddy's own local CA: replace the `tls` line with `tls internal`
and import the generated root CA from the `caddy_data` volume (the browser will
warn until you trust it). mkcert is preferred because its CA is trusted
automatically on the host.

### 3. Add the local hostname

```bash
# /etc/hosts
127.0.0.1   godata.josebianco.local
```

On macOS apply it with:

```bash
sudo sh -c 'grep -q "godata.josebianco.local" /etc/hosts || echo "127.0.0.1   godata.josebianco.local" >> /etc/hosts'
```

### 4. Start the infrastructure

```bash
make docker-up          # starts postgres + caddy (detached)
```

On first boot PostgreSQL creates the two databases `lab_source` and `lab_target`
(via `deploy/init-db.sh`). Verify with `make docker-logs`.

### 5. Run the backend and reach the app

```bash
make run                # backend on http://localhost:8080
```

Then open: **https://godata.josebianco.local**

The request flow is: browser → Caddy (`:443`, TLS) → `host.docker.internal:8080`
→ Go backend. The database is reachable only on the host loopback
(`127.0.0.1:5432`) and is not exposed to your LAN.

### Useful commands

```bash
make docker-down        # stop containers (keeps data)
make docker-reset       # stop and WIPE volumes/database
make docker-logs        # follow container logs
```

See `docs/` for detailed project documentation.
