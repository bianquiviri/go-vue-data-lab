.PHONY: build run test lint migrate-up migrate-down migrate-status docker-up docker-down docker-logs docker-reset certs swagger

# Docker Compose interface. Prefer the Compose v2 plugin (`docker compose`).
# When the repo-root .env exists it is passed to Compose for variable
# interpolation, so a single .env drives both the backend and the containers.
# Otherwise Compose falls back to deploy/.env (git-ignored) or safe defaults.
COMPOSE := cd deploy && docker compose $(if $(wildcard .env),--env-file ../.env,)

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
