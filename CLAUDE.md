# CLAUDE.md - Go SaaS API

This is a Go microservices architecture project for building a SaaS AI platform with chat capabilities.

## Project Overview

**Stack:** Go + Gin + Microservices + Docker
**Services:** API Gateway, Chat Service, Auth Service, Billing Service, Usage Service
**Infrastructure:** PostgreSQL, Redis, NATS, Docker Compose

## Quick Commands

```bash
# Start all services
docker-compose up --build

# Start only infrastructure
docker-compose up -d postgres redis nats

# Test a service
cd chat-service && go run cmd/main.go

# Health check
curl http://localhost:3001/health
```

## Architecture

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│  API Gateway │──────▶│ Chat Service │──────▶│   Ollama    │
│   :3001      │      │   :3002      │      │  :11434     │
└──────────────┘      └──────────────┘      └──────────────┘
       │
       ├──▶ Auth Service :3003
       ├──▶ Billing Service :3004
       └──▶ Usage Service :3005
```

## Service Responsibilities

- **API Gateway:** Routing, auth middleware, rate limiting
- **Chat Service:** AI chat with SSE streaming to Ollama
- **Auth Service:** JWT authentication, OAuth (Google/GitHub)
- **Billing Service:** Stripe integration for subscriptions
- **Usage Service:** Token tracking, rate limits

## Key Technologies

- **Go 1.24** - Language
- **Gin** - HTTP framework
- **NATS** - Message queue
- **PostgreSQL** - Database
- **Redis** - Cache
- **Docker** - Containerization

## Important Patterns

### SSE Streaming (Chat)
```go
c.Header("Content-Type", "text/event-stream")
c.Stream(func(w io.Writer) bool {
    fmt.Fprintf(w, "data: %s\n\n", chunk)
    return true
})
```

### Graceful Shutdown
```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
srv.Shutdown(ctx)
```

### Service Communication
- Synchronous: HTTP/REST between services
- Asynchronous: NATS for events
- Streaming: SSE for real-time chat

## Environment Variables

See `.env.example` for all configuration options.

Key variables:
- `PORT` - Service port
- `DATABASE_URL` - PostgreSQL connection
- `REDIS_URL` - Redis connection
- `NATS_URL` - NATS connection
- `OLLAMA_URL` - Ollama endpoint

## Development Guidelines

1. Use `internal/` for private code
2. Use `pkg/` for shared libraries
3. One binary per `cmd/main.go`
4. Interface-based design for testability
5. Structured logging with Zap
6. Context propagation for timeouts
7. Graceful shutdown handling

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires Docker)
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## Deployment

```bash
# Build all images
docker-compose build

# Push to registry
docker-compose push

# Deploy to Kubernetes
kubectl apply -f k8s/
```

## Skills Available

See `.claude/skills/` directory:
- `go-microservices-architecture.md` - Overall architecture
- `gin-api-gateway.md` - API Gateway patterns
- `go-sse-streaming.md` - Server-Sent Events

## Troubleshooting

**Port already in use:**
```bash
lsof -i :3001
kill -9 <PID>
```

**Database connection failed:**
```bash
docker-compose up -d postgres
```

**NATS connection refused:**
```bash
docker-compose up -d nats
```

## Resources

- [Gin Documentation](https://gin-gonic.com/docs/)
- [Go Best Practices](https://go.dev/doc/effective_go)
- [NATS Documentation](https://docs.nats.io/)
- [Docker Compose](https://docs.docker.com/compose/)

## Notes

- This project follows 12-factor app methodology
- All services are stateless
- Configuration via environment variables
- Horizontal scaling supported via Docker Compose/K8s
