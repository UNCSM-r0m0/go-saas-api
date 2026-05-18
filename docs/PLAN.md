# go-saas-api — Plan de Arquitectura & Roadmap

> **Estado:** En desarrollo | **Prioridad:** Alta | **Objetivo:** Backend de producción en Go, reemplazo optimizado del NestJS actual

---

## 🎯 Visión

Backend SaaS multi-tenant para chat AI con:
- **Streaming ultra-rápido** (WebSocket + SSE con Go routines)
- **Providers escalables** (modelos y API keys en BD, no .env)
- **Arquitectura limpia** (hexagonal / clean architecture)
- **Stripe reutilizado** (migrar lógica desde NestJS)
- **Docker Compose** para desarrollo, **Kubernetes-ready** para producción

---

## 📊 Estado Actual (Análisis Abril 2026)

| Servicio | Estado | Notas |
|----------|--------|-------|
| api-gateway | 🟡 Parcial | reverse proxy stub |
| agent-service | 🟡 Implementado | SSE streaming, tool calls, artifacts |
| sandbox-service | 🟡 Implementado | code execution |
| auth-service | 🔴 Vacío | stub only |
| billing-service | 🔴 Vacío | stub only |
| usage-service | 🔴 Vacío | stub only |

**Problemas críticos detectados:**
1. API Gateway: proxy es stub (retorna JSON placeholder)
2. Agent Service: SSE streaming, tool calls y artifacts implementados (conecta vía LLM client)
3. Docker Compose: URLs malformadas (`redis://redis:***@postgres:5432`)
4. 5 de 8 servicios son **solo carpetas vacías**
5. `engram/`, `engram-memory/`, `enram-memory/` — código experimental sin usar

---

## 🏗️ Arquitectura Target

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Gateway                              │
│                    (Go + Gin + Reverse Proxy)                    │
│                         Puerto 3001                              │
│  Responsabilidades:                                              │
│  • Routing a servicios                                           │
│  • Auth middleware (JWT validation)                              │
│  • Rate limiting (Redis-based)                                 │
│  • CORS, request logging, request ID propagation                 │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌──────────────┐   ┌──────────────┐
│ Agent Service │   │ Auth Service │   │ Billing Svc  │
│    :3002      │   │   :3003      │   │   :3004      │
│               │   │              │   │              │
│ • Streaming   │   │ • JWT/OAuth  │   │ • Stripe     │
│ • Tool calls  │   │ • Users      │   │ • Webhooks   │
│ • Artifacts   │   │ • Sessions   │   │ • Plans      │
└───────┬───────┘   └──────────────┘   └──────────────┘
        │
        ▼
┌───────────────┐
│Sandbox Service│
│    :3006      │
│               │
│ • Code exec   │
└───────┬───────┘
        │
        └─────────────────────┬─────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │  Usage Service    │
                    │    :3005          │
                    │                   │
                    │ • Tracking        │
                    │ • Quotas          │
                    │ • Analytics       │
                    └───────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  PostgreSQL  │    │    Redis     │    │    NATS      │
│    :5432     │    │    :6379     │    │   :4222      │
│              │    │              │    │              │
│ • Users      │    │ • Sessions   │    │ • Events     │
│ • Messages   │    │ • Rate limit │    │ • Async      │
│ • Providers  │    │ • Cache      │    │ • Pub/Sub    │
│ • Billing    │    │ • Pub/Sub    │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

---

## 🗄️ Schema de Base de Datos (PostgreSQL)

### Enums
```sql
CREATE TYPE user_role AS ENUM ('super_admin', 'admin', 'user');
CREATE TYPE auth_provider AS ENUM ('local', 'google', 'github');
CREATE TYPE subscription_tier AS ENUM ('free', 'registered', 'premium');
CREATE TYPE subscription_status AS ENUM ('active', 'canceled', 'expired', 'trialing');
CREATE TYPE message_role AS ENUM ('user', 'assistant', 'system');
CREATE TYPE provider_type AS ENUM ('ollama', 'openai', 'gemini', 'deepseek', 'anthropic', 'custom');
```

### Tablas Core

#### `users`
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255), -- NULL para OAuth
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar_url TEXT,
    role user_role DEFAULT 'user',
    provider auth_provider DEFAULT 'local',
    provider_id VARCHAR(255), -- ID del proveedor OAuth
    is_active BOOLEAN DEFAULT true,
    email_verified BOOLEAN DEFAULT false,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_tenant ON users(tenant_id);
```

#### `tenants` (Multitenancy)
```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_tenants_slug ON tenants(slug);
```

#### `subscriptions`
```sql
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    tier subscription_tier DEFAULT 'registered',
    status subscription_status DEFAULT 'active',
    stripe_customer_id VARCHAR(255) UNIQUE,
    stripe_subscription_id VARCHAR(255) UNIQUE,
    stripe_price_id VARCHAR(255),
    stripe_current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_stripe_customer ON subscriptions(stripe_customer_id);
```

#### `conversations`
```sql
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) DEFAULT 'Nueva conversación',
    model VARCHAR(100) DEFAULT 'deepseek-r1:7b',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_conversations_user ON conversations(user_id);
CREATE INDEX idx_conversations_created ON conversations(created_at);
```

#### `messages`
```sql
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    role message_role NOT NULL,
    content TEXT NOT NULL,
    model VARCHAR(100) DEFAULT 'deepseek-r1:7b',
    tokens_used INT DEFAULT 0,
    attachments TEXT[], -- URLs de archivos
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_messages_conversation ON messages(conversation_id);
CREATE INDEX idx_messages_user ON messages(user_id);
CREATE INDEX idx_messages_created ON messages(created_at);
```

#### `usage_records` (Rate limiting diario)
```sql
CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    anonymous_id VARCHAR(255), -- Para usuarios sin login
    date DATE NOT NULL,
    message_count INT DEFAULT 0,
    tokens_used INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, date),
    UNIQUE(anonymous_id, date)
);
CREATE INDEX idx_usage_user_date ON usage_records(user_id, date);
CREATE INDEX idx_usage_anon_date ON usage_records(anonymous_id, date);
```

#### `ai_providers` ⭐ NUEVO — Escalable, no .env
```sql
CREATE TABLE ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL, -- "Ollama Local", "OpenAI GPT-4", etc.
    type provider_type NOT NULL,
    base_url TEXT NOT NULL, -- http://host.docker.internal:11434, https://api.openai.com
    api_key_encrypted TEXT, -- Encriptado con AES-256
    api_key_hash VARCHAR(64), -- SHA-256 para búsqueda rápida sin desencriptar
    is_active BOOLEAN DEFAULT true,
    is_public BOOLEAN DEFAULT true, -- Visible para todos los usuarios
    priority INT DEFAULT 0, -- Orden de fallback
    config JSONB DEFAULT '{}', -- Config extra por provider (temperature default, etc)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_providers_type ON ai_providers(type);
CREATE INDEX idx_providers_active ON ai_providers(is_active);
```

#### `ai_models` ⭐ NUEVO — Catálogo de modelos
```sql
CREATE TABLE ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES ai_providers(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL, -- "qwen2.5-coder:7b", "gpt-4", etc.
    display_name VARCHAR(255) NOT NULL, -- "Qwen 2.5 Coder 7B"
    description TEXT,
    max_tokens INT DEFAULT 4096,
    context_window INT DEFAULT 8192,
    supports_streaming BOOLEAN DEFAULT true,
    supports_images BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_public BOOLEAN DEFAULT true, -- free tier puede usar
    is_premium BOOLEAN DEFAULT false, -- solo premium
    config JSONB DEFAULT '{}', -- { temperature: 0.7, top_p: 0.9, etc }
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_models_provider ON ai_models(provider_id);
CREATE INDEX idx_models_active ON ai_models(is_active);
CREATE INDEX idx_models_public ON ai_models(is_public);
```

#### `api_keys` ⭐ NUEVO — API keys de usuarios (para API pública)
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL, -- SHA-256 del key (no almacenar plaintext)
    key_prefix VARCHAR(8) NOT NULL, -- "sk_live_" o "sk_test_"
    permissions JSONB DEFAULT '["chat:read", "chat:write"]',
    rate_limit INT DEFAULT 60, -- requests per minute
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
```

---

## ⚡ Optimizaciones de Rendimiento (Go)

### 1. Streaming con WebSocket + SSE

```go
// Agent Service — WebSocket para streaming bidireccional
// SSE para clientes que prefieren HTTP puro

type StreamManager struct {
    clients    map[string]*Client      // userID -> Client
    broadcast  chan Message            // Canal para broadcast
    register   chan *Client            // Registro de nuevos clientes
    unregister chan *Client            // Desregistro
    mu         sync.RWMutex            // Protección de clients map
}

// Go routine por conexión + pool de workers para procesar chunks
type ChunkWorker struct {
    id        int
    jobs      chan ChunkJob
    wg        sync.WaitGroup
}

// ChunkJob representa un chunk de streaming a procesar
type ChunkJob struct {
    clientID    string
    messageID   string
    content     string
    done        bool
    provider    string
}
```

### 2. Pool de conexiones HTTP para providers

```go
type ProviderClient struct {
    client      *http.Client
    baseURL     string
    apiKey      string
    maxConns    int
    timeout     time.Duration
}

// Transport optimizado con connection pooling
func NewProviderClient(baseURL, apiKey string) *ProviderClient {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        DisableCompression:  false,
    }
    
    return &ProviderClient{
        client: &http.Client{
            Transport: transport,
            Timeout:   30 * time.Second,
        },
        baseURL:  baseURL,
        apiKey:   apiKey,
        maxConns: 10,
        timeout:  30 * time.Second,
    }
}
```

### 3. Pipeline de streaming con canales

```go
// Pipeline: Provider -> ChunkProcessor -> SSE/WS Emitter

func (s *AgentService) StreamMessage(ctx context.Context, req ChatRequest, stream chan<- StreamChunk) error {
    // 1. Obtener provider de BD (no .env)
    provider, err := s.providerRepo.GetByModel(ctx, req.Model)
    if err != nil {
        return err
    }
    
    // 2. Crear cliente HTTP con pool
    client := s.getClient(provider)
    
    // 3. Iniciar request al provider con context
    resp, err := client.StreamCompletion(ctx, req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    // 4. Leer chunks en Go routine, enviar al canal
    go func() {
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            chunk := parseChunk(scanner.Bytes())
            select {
            case stream <- chunk:
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return nil
}
```

### 4. Rate limiting distribuido (Redis)

```go
// Sliding window rate limiter con Redis

func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
    pipe := rl.redis.Pipeline()
    now := time.Now().Unix()
    windowStart := now - int64(window.Seconds())
    
    // ZREMRANGEBYSCORE para limpiar entradas viejas
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
    
    // ZCARD para contar entradas actuales
    count := pipe.ZCard(ctx, key)
    
    // ZADD para agregar entrada actual
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
    
    // EXPIRE para auto-limpieza
    pipe.Expire(ctx, key, window)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }
    
    currentCount, err := count.Result()
    if err != nil {
        return false, err
    }
    
    return currentCount < int64(limit), nil
}
```

---

## 📦 Estructura de Directorios (Clean Architecture)

```
Workspace/GO/go-saas-api/
├── README.md
├── docker-compose.yml
├── docker-compose.prod.yml
├── Makefile
├── .env.example
├── go.work                    # Go workspace (Go 1.24+)
│
├── cmd/
│   ├── api-gateway/
│   │   └── main.go
│   ├── agent-service/
│   │   └── main.go
│   ├── sandbox-service/
│   │   └── main.go
│   ├── auth-service/
│   │   └── main.go
│   ├── billing-service/
│   │   └── main.go
│   └── usage-service/
│       └── main.go
│
├── internal/
│   ├── shared/               # Código compartido entre servicios
│   │   ├── config/
│   │   ├── logger/
│   │   ├── middleware/
│   │   ├── models/
│   │   ├── errors/
│   │   └── utils/
│   │
│   ├── api-gateway/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   ├── router/
│   │   └── service/
│   │
│   ├── agent/
│   │   ├── domain/           # Entidades de negocio
│   │   ├── repository/       # Interfaces de BD
│   │   ├── usecase/          # Lógica de negocio
│   │   ├── delivery/         # HTTP handlers + WS handlers
│   │   ├── providers/        # Implementaciones por provider
│   │   │   ├── ollama.go
│   │   │   ├── openai.go
│   │   │   ├── gemini.go
│   │   │   └── deepseek.go
│   │   └── infrastructure/   # DB, Redis, NATS implementations
│   │
│   ├── auth/
│   │   ├── domain/
│   │   ├── repository/
│   │   ├── usecase/
│   │   ├── delivery/
│   │   └── infrastructure/
│   │
│   ├── billing/
│   │   ├── domain/
│   │   ├── repository/
│   │   ├── usecase/
│   │   ├── delivery/
│   │   └── infrastructure/
│   │
│   └── usage/
│       ├── domain/
│       ├── repository/
│       ├── usecase/
│       ├── delivery/
│       └── infrastructure/
│
├── pkg/
│   ├── encryption/           # AES-256 para API keys
│   ├── jwt/                  # JWT utils
│   ├── validation/           # Input validation
│   └── streaming/            # SSE/WS helpers
│
├── migrations/
│   ├── 001_initial_schema.sql
│   ├── 002_add_providers.sql
│   └── 003_add_api_keys.sql
│
├── deployments/
│   ├── docker/
│   │   └── Dockerfile.*      # Un Dockerfile por servicio
│   └── k8s/                  # Kubernetes manifests (futuro)
│
└── scripts/
    ├── dev.sh                # Levantar stack local
    ├── migrate.sh            # Run migrations
    └── seed.sh               # Seed data (providers, models)
```

---

## 🔧 Migración desde NestJS

### Qué migrar (lógica de negocio)

| Componente NestJS | Estado | Acción en Go |
|-------------------|--------|--------------|
| Prisma Schema | ✅ Completo | Migrar a SQL + migrations |
| Auth (JWT + OAuth) | ✅ Funcional | Reimplementar en Go |
| Stripe (subscriptions) | ✅ Funcional | **Reutilizar lógica**, adaptar a Go |
| Agent service (providers) | ✅ Funcional | Reimplementar con Go routines |
| Rate limiting | ✅ Funcional | Reimplementar con Redis |
| Usage tracking | ✅ Funcional | Reimplementar |
| Ollama provider | ✅ Funcional | Reimplementar con streaming |
| OpenAI provider | ✅ Funcional | Reimplementar |
| Gemini provider | ✅ Funcional | Reimplementar |
| DeepSeek provider | ✅ Funcional | Reimplementar |

### Qué NO migrar
- NestJS DI container → Go usa interfaces + factories
- Prisma Client → sqlx/pgx + migrations SQL
- NestJS guards/interceptors → Gin middleware

---

## 📋 Issues a Crear (GitHub)

### Milestone 1: Fundamentos (Sprint 1-2)
- [x] **#1** Setup workspace + Go modules + Docker Compose fixes
- [x] **#2** Implementar schema SQL completo + migrations
- [x] **#3** Crear `internal/platform` (antes `shared`) con logger, config, errors
- [ ] **#4** Implementar API Gateway real (reverse proxy)

### Milestone 2: Core Services (Sprint 3-4)
- [ ] **#5** Implementar Auth Service (JWT + OAuth Google/GitHub)
- [x] **#6** Implementar Agent Service con SSE streaming real
- [x] **#7** Implementar provider Ollama con Go routines + chunks (parcial, vía LLM client)
- [ ] **#8** Implementar provider OpenAI con streaming

### Milestone 3: Providers Escalables (Sprint 5)
- [ ] **#9** Crear tablas `ai_providers` + `ai_models`
- [ ] **#10** Implementar CRUD de providers (admin API)
- [ ] **#11** Implementar provider Gemini
- [ ] **#12** Implementar provider DeepSeek
- [ ] **#13** Sistema de fallback entre providers

### Milestone 4: Billing & Usage (Sprint 6)
- [ ] **#14** Migrar lógica Stripe desde NestJS
- [ ] **#15** Implementar Usage Service con Redis
- [ ] **#16** Webhooks Stripe (subscriptions)
- [ ] **#17** Rate limiting por tier (free/registered/premium)

### Milestone 5: Optimización (Sprint 7)
- [ ] **#18** WebSocket support para streaming bidireccional
- [ ] **#19** Connection pooling HTTP para providers
- [ ] **#20** Caché de modelos/config en Redis
- [ ] **#21** Health checks + graceful shutdown
- [ ] **#22** Tests unitarios + integración

### Milestone 6: Producción (Sprint 8)
- [ ] **#23** Docker Compose producción
- [ ] **#24** Kubernetes manifests
- [ ] **#25** Observabilidad (metrics, tracing)
- [ ] **#26** Documentación API (OpenAPI/Swagger)

---

## 🚀 Quick Start (Target)

```bash
# 1. Clonar y entrar
cd ~/Workspace/GO/go-saas-api

# 2. Configurar
cp .env.example .env
# Editar .env con credenciales

# 3. Levantar infraestructura
make dev-up          # Postgres + Redis + NATS

# 4. Run migrations
make migrate

# 5. Seed providers
make seed

# 6. Levantar servicios
make services-up     # Todos los microservicios

# 7. Test
curl http://localhost:3001/health
curl http://localhost:3001/api/v1/agent/models
curl -N http://localhost:3001/api/v1/agent/stream \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5-coder:7b","messages":[{"role":"user","content":"Hola"}]}'
```

---

## 📁 Acción Inmediata: Organizar Workspace

```bash
# Mover go-saas-api al workspace
mv ~/go-saas-api ~/Workspace/GO/go-saas-api

# Eliminar código experimental no usado
rm -rf ~/Workspace/GO/go-saas-api/engram
rm -rf ~/Workspace/GO/go-saas-api/engram-memory
rm -rf ~/Workspace/GO/go-saas-api/enram-memory
rm -rf ~/Workspace/GO/go-saas-api/.claude
rm -rf ~/Workspace/GO/go-saas-api/.gentle-ai

# Eliminar proyectos abandonados
rm -rf ~/kimi-projects

# Eliminar duplicados frontend (Vercel ya los tiene)
rm -rf ~/r3-chat-fix
rm -rf ~/r3-chat-cookies

# Limpieza de backups OpenClaw (ya migrado a Hermes)
rm -rf ~/.openclaw.old.20260401_140808
rm -rf ~/.openclaw-backup-*.tar.gz
rm -rf ~/backups/openclaw

# Archivar backups grandes
mv ~/backups ~/backups-archive  # o mantener en ~/backups
```

---

*Documento creado: Abril 2026 | Próximo paso: Crear issues en GitHub y empezar Milestone 1*
