.PHONY: build run test lint migrate-up migrate-down docker-up docker-down docker-logs docker-reset certs swagger

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

# Generate locally-trusted TLS certs for https://godata.josebianco.local
certs:
	./deploy/gen-certs.sh

docker-up:
	@if [ ! -f deploy/certs/godata.josebianco.local.pem ]; then \
		echo "error: TLS certificates not found."; \
		echo "run 'make certs' first (requires mkcert)."; \
		exit 1; \
	fi
	cd deploy && docker-compose up -d

docker-down:
	cd deploy && docker-compose down

docker-logs:
	cd deploy && docker-compose logs -f --tail=100

# Destroy containers AND named volumes (wipes the database).
docker-reset:
	cd deploy && docker-compose down -v

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs/swagger
