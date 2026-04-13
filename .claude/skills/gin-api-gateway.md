# Gin API Gateway Skill

## Overview
Building a robust API Gateway with Gin framework for microservices architecture.

## Project Setup

### Basic Structure
```
api-gateway/
├── cmd/
│   └── main.go
├── internal/
│   ├── handlers/
│   ├── middleware/
│   ├── router/
│   └── config/
└── pkg/
    └── utils/
```

### Main.go Template
```go
package main

import (
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    r := gin.New()
    r.Use(gin.Recovery())
    
    setupRoutes(r, logger)
    
    if err := r.Run(":" + getEnv("PORT", "3001")); err != nil {
        logger.Fatal("failed to start server", zap.Error(err))
    }
}
```

## Routing Patterns

### Group Routes
```go
v1 := r.Group("/api/v1")
{
    auth := v1.Group("/auth")
    {
        auth.POST("/login", authHandler.Login)
        auth.POST("/register", authHandler.Register)
    }
    
    chat := v1.Group("/chat")
    chat.Use(authMiddleware())
    {
        chat.POST("/completions", chatHandler.Completions)
        chat.POST("/stream", chatHandler.Stream)
    }
}
```

## Middleware

### Authentication Middleware
```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        
        // Validate JWT
        claims, err := jwt.Validate(token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
            return
        }
        
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
```

### Rate Limiter
```go
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
            return
        }
        c.Next()
    }
}
```

### CORS Configuration
```go
import "github.com/gin-contrib/cors"

config := cors.Config{
    AllowOrigins:     []string{"https://yourdomain.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    ExposeHeaders:    []string{"Content-Length"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}

r.Use(cors.New(config))
```

## Error Handling

### Custom Error Response
```go
type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

func HandleError(c *gin.Context, err error) {
    var status int
    var message string
    
    switch {
    case errors.Is(err, ErrNotFound):
        status = 404
        message = "resource not found"
    case errors.Is(err, ErrUnauthorized):
        status = 401
        message = "unauthorized"
    default:
        status = 500
        message = "internal server error"
    }
    
    c.JSON(status, ErrorResponse{
        Code:    status,
        Message: message,
    })
}
```

## Request Validation

### Struct Validation
```go
type ChatRequest struct {
    Model    string    `json:"model" binding:"required"`
    Messages []Message `json:"messages" binding:"required,min=1"`
    Stream   bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role" binding:"required,oneof=user assistant system"`
    Content string `json:"content" binding:"required,max=8000"`
}

func ChatHandler(c *gin.Context) {
    var req ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // Process request...
}
```

## Proxy to Services

### Reverse Proxy
```go
import "net/http/httputil"

func ProxyToService(targetURL string) gin.HandlerFunc {
    url, _ := url.Parse(targetURL)
    proxy := httputil.NewSingleHostReverseProxy(url)
    
    return func(c *gin.Context) {
        proxy.ServeHTTP(c.Writer, c.Request)
    }
}

// Usage
r.POST("/api/v1/chat", ProxyToService("http://chat-service:3002"))
```

## Graceful Shutdown

```go
func main() {
    r := setupRouter()
    
    srv := &http.Server{
        Addr:    ":3001",
        Handler: r,
    }
    
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %s\n", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal("Server forced to shutdown:", err)
    }
    
    log.Println("Server exiting")
}
```

## Health Check

```go
func HealthHandler(c *gin.Context) {
    // Check dependencies
    checks := map[string]string{
        "database": checkDatabase(),
        "redis":    checkRedis(),
        "nats":     checkNATS(),
    }
    
    status := "healthy"
    for _, v := range checks {
        if v != "ok" {
            status = "unhealthy"
            break
        }
    }
    
    c.JSON(200, gin.H{
        "status":    status,
        "checks":    checks,
        "timestamp": time.Now().Unix(),
    })
}
```

## Testing Handlers

```go
func TestChatHandler(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    r := gin.New()
    r.POST("/chat", ChatHandler)
    
    body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
    req := httptest.NewRequest("POST", "/chat", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
}
```
