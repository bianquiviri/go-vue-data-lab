.PHONY: build run test lint migrate-up migrate-down migrate-status docker-up docker-down docker-logs docker-reset certs swagger up start restart-containers docker-restart stop down-all

# Docker Compose interface. Prefer the Compose v2 plugin (`docker compose`).
# When the repo-root .env exists it is passed to Compose for variable
# interpolation, so a single .env drives both the backend and the containers.
# Otherwise Compose falls back to deploy/.env (git-ignored) or safe defaults.
COMPOSE := cd deploy && docker compose $(if $(wildcard .env),--env-file ../.env,)

# Paths and runtime helpers
PID_DIR := .pid
LOG_DIR := logs
API_PID := $(PID_DIR)/api.pid
WEB_PID := $(PID_DIR)/web.pid

# Load .env if present for use in shell commands
ifneq ($(wildcard .env),)
  include .env
  export
endif

# Defaults
SERVER_PORT ?= 8080
VITE_PORT   ?= 5174
FRONTEND_DIR := web
BACKEND_DIR  := backend

build:
	cd backend && go build -o ../bin/api ./cmd/api

run:
	cd backend && go run ./cmd/api

test:
	cd backend && go test -race -v ./...

lint:
	cd backend && golangci-lint run

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down

# Show applied/pending migrations for both databases.
migrate-status:
	cd backend && go run ./cmd/migrate status

# Generate locally-trusted TLS certs for the local lab hosts
# (apidatago.josebianco.local + frontgodata.josebianco.local)
certs:
	./deploy/gen-certs.sh

docker-up:
	@if [ ! -f deploy/certs/apidatago.josebianco.local.pem ] || [ ! -f deploy/certs/frontgodata.josebianco.local.pem ]; then \
		echo "error: TLS certificates not found."; \
		echo "run 'make certs' first (requires mkcert)."; \
		exit 1; \
	fi
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f --tail=100

# Destroy containers AND named volumes (wipes the database).
docker-reset:
	$(COMPOSE) down -v

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs/swagger

# Wait until Postgres in Docker is healthy
.PHONY: wait-db
wait-db:
	@mkdir -p $(LOG_DIR)
	@echo "Waiting for Postgres (lab_postgres) to be healthy..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do \
		status=$$($(COMPOSE) ps --format '{{.State}}' postgres 2>/dev/null | tr -d '\n'); \
		health=$$($(COMPOSE) ps --format '{{.Health}}' postgres 2>/dev/null | tr -d '\n'); \
		if echo "$$health" | grep -q healthy || echo "$$status" | grep -q healthy; then \
			echo "Postgres is healthy."; \
			exit 0; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "Timed out waiting for Postgres."; \
			$(COMPOSE) ps; \
			exit 1; \
		fi; \
		sleep 2; \
	done

# Start all containers (postgres + caddy)
.PHONY: infra-up
infra-up: docker-up wait-db

# Restart only Docker containers (postgres + caddy)
restart-containers: docker-restart
docker-restart:
	@echo "Restarting Docker containers (postgres + caddy)..."
	$(COMPOSE) restart
	@$(MAKE) wait-db
	@echo "Containers restarted."

# Stop all background processes started by make up/start
stop:
	@mkdir -p $(PID_DIR)
	@if [ -f $(API_PID) ]; then \
		pid=$$(cat $(API_PID)); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "Stopping API (pid $$pid)..."; \
			kill $$pid 2>/dev/null || true; \
			sleep 1; \
			kill -9 $$pid 2>/dev/null || true; \
		fi; \
		rm -f $(API_PID); \
	fi
	@if [ -f $(WEB_PID) ]; then \
		pid=$$(cat $(WEB_PID)); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "Stopping frontend (pid $$pid)..."; \
			kill $$pid 2>/dev/null || true; \
			sleep 1; \
			kill -9 $$pid 2>/dev/null || true; \
		fi; \
		rm -f $(WEB_PID); \
	fi
	@echo "Stopped background processes."

down-all: stop docker-down
	@echo "All stopped (infra + background processes)."

# Lift everything: infra (postgres+caddy) + backend + frontend
up: start
start:
	@mkdir -p $(PID_DIR) $(LOG_DIR)
	@echo "==> Starting full lab stack"
	# Check TLS certs for Caddy
	@if [ ! -f deploy/certs/apidatago.josebianco.local.pem ] || [ ! -f deploy/certs/frontgodata.josebianco.local.pem ]; then \
		echo "error: TLS certificates not found."; \
		echo "run 'make certs' first (requires mkcert)."; \
		exit 1; \
	fi
	# Check if ports are in use
	@if lsof -ti:$(SERVER_PORT) >/dev/null 2>&1; then \
		echo "Port $(SERVER_PORT) is already in use."; \
		if [ "$(SERVER_PORT)" = "8080" ]; then \
			echo "Falling back to SERVER_PORT=18080..."; \
			export SERVER_PORT=18080; \
			echo "SERVER_PORT set to $$SERVER_PORT"; \
		else \
			echo "Please free port $(SERVER_PORT) or change SERVER_PORT in .env."; \
			exit 1; \
		fi; \
	fi
	# Ensure infra is up
	@$(MAKE) infra-up
	# Determine backend port and target
	@BACKEND_PORT=$${SERVER_PORT:-8080}; \
	if lsof -ti:$$BACKEND_PORT >/dev/null 2>&1 && [ "$$BACKEND_PORT" = "8080" ]; then \
		BACKEND_PORT=18080; \
	fi; \
	echo "Backend will run on port $$BACKEND_PORT"; \
	echo "Starting backend in background..."; \
	cd $(BACKEND_DIR) && SERVER_PORT=$$BACKEND_PORT nohup go run ./cmd/api > ../$(LOG_DIR)/api.log 2>&1 & echo $$! > ../$(API_PID); \
	sleep 3; \
	if [ -f $(API_PID) ] && kill -0 $$(cat $(API_PID)) 2>/dev/null; then \
		echo "Backend started (pid $$(cat $(API_PID))) - http://localhost:$$BACKEND_PORT"; \
	else \
		echo "Backend may have failed to start. Check $(LOG_DIR)/api.log"; \
	fi; \
	# Frontend
	VITE_BACKEND_TARGET="http://localhost:$$BACKEND_PORT"; \
	echo "Frontend VITE_BACKEND_TARGET=$$VITE_BACKEND_TARGET (port $(VITE_PORT))"; \
	echo "Starting frontend dev server in background..."; \
	cd $(FRONTEND_DIR) && VITE_BACKEND_TARGET=$$VITE_BACKEND_TARGET nohup npm run dev -- --port $(VITE_PORT) --strictPort > ../$(LOG_DIR)/web.log 2>&1 & echo $$! > ../$(WEB_PID); \
	sleep 4; \
	if [ -f $(WEB_PID) ] && kill -0 $$(cat $(WEB_PID)) 2>/dev/null; then \
		echo "Frontend started (pid $$(cat $(WEB_PID))) - http://localhost:$(VITE_PORT)"; \
	else \
		echo "Frontend may have failed to start. Check $(LOG_DIR)/web.log"; \
	fi; \
	echo ""; \
	echo "==> Stack started"; \
	echo "Infra:"; \
	echo "  Postgres: localhost:5433 (lab_postgres)"; \
	echo "  Caddy:    https://apidatago.josebianco.local:8443 (API)"; \
	echo "            https://frontgodata.josebianco.local:8443 (Web via Caddy)"; \
	echo "Local dev:"; \
	echo "  Backend:  http://localhost:$$BACKEND_PORT"; \
	echo "  Frontend: http://localhost:$(VITE_PORT)"; \
	echo "Logs:"; \
	echo "  $(LOG_DIR)/api.log, $(LOG_DIR)/web.log"; \
	echo "PIDs:"; \
	echo "  $(API_PID), $(WEB_PID)"; \
	echo "Stop:     make stop / make down-all"
