.PHONY: help dev-up services-up migrate seed down test lint build clean

# Default target
help:
	@echo "go-saas-api — Makefile"
	@echo ""
	@echo "  make dev-up          Levantar infraestructura (Postgres, Redis, NATS)"
	@echo "  make services-up     Levantar todos los microservicios"
	@echo "  make up              Levantar todo (infra + servicios)"
	@echo "  make down            Detener todo"
	@echo "  make migrate         Ejecutar migrations"
	@echo "  make seed            Insertar datos de ejemplo"
	@echo "  make test            Ejecutar tests"
	@echo "  make lint            Ejecutar linter"
	@echo "  make build           Compilar todos los binarios"
	@echo "  make clean           Limpiar binarios y volúmenes"

# Infrastructure only
dev-up:
	docker-compose up -d postgres redis nats
	@echo "⏳ Esperando que Postgres esté listo..."
	@sleep 3
	@echo "✅ Infraestructura lista"

# All services
services-up:
	docker-compose up -d --build

# Everything
up: dev-up services-up
	@echo "✅ Todo levantado"

# Stop everything
down:
	docker-compose down

# Stop and remove volumes
clean:
	docker-compose down -v
	docker system prune -f

# Run migrations (placeholder — will use golang-migrate)
migrate:
	@echo "🔄 Ejecutando migrations..."
	@echo "TODO: Implementar con golang-migrate"
	@# migrate -path ./migrations -database "postgresql://postgres:postgres@localhost:5432/saas_db?sslmode=disable" up

# Seed data
seed:
	@echo "🌱 Insertando datos de ejemplo..."
	@echo "TODO: Implementar seed script"

# Tests
test:
	@echo "🧪 Ejecutando tests..."
	go test ./...

# Lint
lint:
	@echo "🔍 Ejecutando linter..."
	golangci-lint run ./...

# Build all binaries
build:
	@echo "🔨 Compilando binarios..."
	CGO_ENABLED=0 go build -o bin/api-gateway ./cmd/api-gateway
	CGO_ENABLED=0 go build -o bin/agent-service ./cmd/agent-service
	CGO_ENABLED=0 go build -o bin/sandbox-service ./cmd/sandbox-service
	CGO_ENABLED=0 go build -o bin/auth-service ./cmd/auth-service
	CGO_ENABLED=0 go build -o bin/billing-service ./cmd/billing-service
	CGO_ENABLED=0 go build -o bin/usage-service ./cmd/usage-service
	@echo "✅ Binarios en ./bin/"

# Health check
health:
	@echo "🏥 Health checks:"
	@curl -s http://localhost:3001/health | jq . || echo "API Gateway no responde"
	@curl -s http://localhost:3002/health | jq . || echo "Agent Service no responde"
	@curl -s http://localhost:3003/health | jq . || echo "Auth Service no responde"
	@curl -s http://localhost:3004/health | jq . || echo "Billing Service no responde"
	@curl -s http://localhost:3005/health | jq . || echo "Usage Service no responde"
