# Go Microservices Architecture Skill

## Project Structure

```
go-saas-api/
├── api-gateway/          # API Gateway (Entry Point)
│   ├── cmd/
│   │   └── main.go      # Application entry point
│   ├── internal/
│   │   ├── handlers/    # HTTP handlers
│   │   ├── middleware/  # Auth, logging, rate limiting
│   │   └── router.go    # Route definitions
│   ├── pkg/
│   │   └── config/      # Configuration management
│   └── Dockerfile
├── chat-service/         # Chat AI with Streaming
├── auth-service/         # Authentication & Authorization
├── billing-service/      # Payments (Stripe)
├── usage-service/        # Usage Tracking
└── docker-compose.yml
```

## Go Best Practices

### 1. Package Structure
- Use `internal/` for private code
- Use `pkg/` for shared libraries
- One binary per `cmd/` subdirectory

### 2. Dependency Injection
- Use interfaces for testability
- Inject dependencies through constructors
- Avoid global state

### 3. Error Handling
```go
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### 4. Context Usage
- Always accept `context.Context` as first parameter
- Pass context through the call chain
- Use timeouts for external calls

### 5. Concurrency
- Use goroutines for I/O bound operations
- Use channels for communication
- Never share memory without synchronization

## Microservices Patterns

### Communication
- **Synchronous**: REST/HTTP for client-facing APIs
- **Asynchronous**: NATS/Message Queue for internal communication
- **Streaming**: SSE (Server-Sent Events) for real-time updates

### Service Discovery
- Use Docker Compose for local development
- Use Kubernetes DNS for production
- Health checks on `/health` endpoint

### Configuration
- Use environment variables
- 12-factor app methodology
- Never commit secrets to git

## Gin Framework Patterns

### Middleware Stack
```go
r := gin.New()
r.Use(gin.Recovery())
r.Use(cors.Default())
r.Use(loggerMiddleware())
r.Use(rateLimiter())
```

### Handler Structure
```go
func (h *Handler) GetUser(c *gin.Context) {
    id := c.Param("id")
    
    user, err := h.service.GetUser(c.Request.Context(), id)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, user)
}
```

## Database Patterns

### PostgreSQL with GORM
```go
type User struct {
    ID        uuid.UUID `gorm:"type:uuid;primary_key"`
    Email     string    `gorm:"uniqueIndex"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Connection Pooling
```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
sqlDB, err := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

## Testing

### Unit Tests
```go
func TestService(t *testing.T) {
    // Arrange
    mockRepo := &MockRepository{}
    service := NewService(mockRepo)
    
    // Act
    result, err := service.DoSomething()
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Integration Tests
- Use testcontainers for databases
- Spin up services in Docker
- Test full request/response cycles

## Docker Best Practices

### Multi-stage Builds
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main ./cmd/main.go

FROM alpine:latest
COPY --from=builder /app/main .
CMD ["./main"]
```

### Health Checks
```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:3001/health"]
  interval: 10s
  timeout: 5s
  retries: 5
```

## Deployment

### Docker Compose
```yaml
version: '3.8'
services:
  api-gateway:
    build: ./api-gateway
    ports:
      - "3001:3001"
    environment:
      - PORT=3001
```

### Environment Variables
- Use `.env` file for local development
- Use Docker secrets or K8s secrets for production
- Validate required env vars at startup

## Monitoring & Logging

### Structured Logging (Zap)
```go
logger, _ := zap.NewProduction()
logger.Info("request processed",
    zap.String("method", c.Request.Method),
    zap.String("path", c.Request.URL.Path),
    zap.Duration("duration", time.Since(start)),
)
```

### Metrics
- Request count and latency
- Database connection pool stats
- External service call latency
- Error rates

## Security

### Input Validation
- Validate all user input
- Use struct tags for validation
- Sanitize data before storing

### Authentication
- JWT tokens with short expiry
- Refresh token rotation
- HTTPS only in production

### Secrets Management
- Never hardcode secrets
- Use environment variables
- Rotate secrets regularly
