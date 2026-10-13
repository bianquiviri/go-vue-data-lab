.PHONY: build run test lint migrate-up migrate-down docker-up docker-down swagger

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

docker-up:
	cd deploy && docker-compose up -d

docker-down:
	cd deploy && docker-compose down

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs/swagger
