# Go SaaS API

Backend SaaS multi-tenant para AI Agents — Monolito Modular en Go 1.24.

## 🏗️ Arquitectura

```
┌─────────────────────────────────────────────────────────┐
│                    API Gateway :3001                     │
│              (Routing, Auth, Rate Limiting)              │
└─────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Agent :3002  │  │ Auth :3003   │  │ Billing :3004│
│ Agent Runtime│  │ JWT + OAuth  │  │ Stripe       │
│ Tools + SSE  │  │ Users        │  │ Subscriptions│
└──────────────┘  └──────────────┘  └──────────────┘
        │
        ▼
┌──────────────┐
│ Sandbox :3006│
│ Code Exec    │
└──────────────┘
                            │
                    ┌───────┴───────┐
                    │ Usage :3005   │
                    │ Tracking      │
                    │ Rate Limits   │
                    └───────────────┘
```

### Agent Runtime (dentro de Agent Service)

```
┌────────────────────────────────────────┐
│           Agent Service :3002          │
│  ┌─────────┐  ┌─────────┐  ┌────────┐ │
│  │Classifier│→│Dispatcher│→│  LLM   │ │
│  │(keywords)│  │(prompts) │  │(Ollama)│ │
│  └─────────┘  └─────────┘  └────────┘ │
│       ↓              ↓                 │
│  ┌─────────────────────────────────┐   │
│  │        Tool Registry            │   │
│  │  file_write │ code_execute │ ... │   │
│  └─────────────────────────────────┘   │
│       ↓                                │
│  ┌─────────────┐    ┌──────────────┐   │
│  │  Postgres   │    │    Redis     │   │
│  │  (RLS)      │    │  (sessions)  │   │
│  └─────────────┘    └──────────────┘   │
└────────────────────────────────────────┘
```

## 🚀 Quick Start

```bash
# 1. Clonar
cd ~/Workspace/GO/go-saas-api

# 2. Configurar
cp .env.example .env
# Editar .env con tus credenciales

# 3. Levantar infraestructura
make dev-up          # Postgres + Redis + NATS

# 4. Levantar servicios
make services-up     # Todos los microservicios

# 5. Verificar
make health
```

## 📦 Stack

| Capa | Tecnología |
|------|-----------|
| Lenguaje | Go 1.24 |
| HTTP Framework | Gin |
| Base de Datos | PostgreSQL 16 (RLS multi-tenant) |
| Cache | Redis 7 |
| Mensajería | NATS 2.10 |
| Auth | JWT + OAuth 2.0 |
| Pagos | Stripe |
| AI Local | Ollama |
| Container | Docker + Docker Compose |

## 📁 Estructura

```
cmd/
  agent-service/        # Agent Runtime (orquestador + tools + SSE)
  api-gateway/          # API Gateway
  auth-service/         # Auth Service
  billing-service/      # Billing Service
  sandbox-service/      # Sandboxed code execution
  usage-service/        # Usage Tracking
internal/
  agent/
    model/              # Domain types (Conversation, Message, Agent, Artifact)
    repository/         # Repository interfaces
    runtime/            # Classifier, Dispatcher, Orchestrator, SessionManager
    tools/              # Tool registry, file_write, code_execute
    prompts/            # System prompts (coder, conversational)
    store/              # Postgres implementations with RLS
    memory/             # 3-layer memory (stub)
    subagent/           # Sub-agent spawner (stub)
    artifact/           # Artifact versioning (stub)
  platform/
    config/             # Environment configuration
    logger/             # Zap wrapper
    middleware/         # RequestID, Logger, CORS
    postgres/           # pgxpool helper
    redis/              # go-redis helper
    nats/               # NATS connection helper
pkg/
  llm/                  # LLM client interface + Ollama implementation
migrations/             # SQL migrations (base + agent platform + RLS)
deployments/
  docker/               # Dockerfiles
```

## 🛠️ Comandos

```bash
make help          # Ver todos los comandos
make up            # Levantar todo
make down          # Detener todo
make build         # Compilar binarios
make test          # Ejecutar tests
make lint          # Ejecutar linter
make migrate       # Aplicar migrations
make seed          # Insertar datos de ejemplo
make health        # Verificar salud de servicios
```

## 🔧 Variables de Entorno

Ver `.env.example` para todas las opciones.

| Variable | Descripción | Default |
|----------|-------------|---------|
| `PORT` | Puerto del servicio | 3001-3006 |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `NATS_URL` | NATS connection string | - |
| `JWT_SECRET` | Secret para JWT | - |
| `STRIPE_SECRET_KEY` | Stripe API key | - |
| `OLLAMA_URL` | Ollama endpoint | http://localhost:11434 |
| `SANDBOX_SERVICE_URL` | Sandbox service URL | http://localhost:3006 |

## 📚 API Endpoints

### API Gateway (3001)
- `GET /health` — Health check
- `/api/v1/auth/*` → Auth Service
- `/api/v1/agent/*` → Agent Service
- `/api/v1/billing/*` → Billing Service
- `/api/v1/usage/*` → Usage Service

### Agent Service (3002)
- `GET /health` — Health check
- `GET /chat/models` — Listar modelos disponibles
- `POST /agent/chat` — Chat con SSE streaming (requiere `X-Tenant-ID`, `X-User-ID`)
- `POST /artifacts` — Crear artifact generado
- `GET /artifacts/:id/preview` — Ver contenido de artifact

### Sandbox Service (3006)
- `GET /health` — Health check
- `POST /execute` — Ejecutar código (JSON: `code`, `language`)

### Auth Service (3003)
- `POST /auth/register` — Registro
- `POST /auth/login` — Login
- `POST /auth/refresh` — Refresh token
- `GET /auth/me` — Perfil

## 🧪 Testing

```bash
# Tests unitarios
go test ./internal/agent/...

# Tests de integración HTTP
go test ./cmd/agent-service/...

# Todos los tests
go test ./...
```

| Paquete | Tests |
|---------|-------|
| `internal/agent/tools` | 6 |
| `internal/agent/prompts` | 2 |
| `internal/agent/runtime` | 6 |
| `cmd/agent-service` | 6 |

## 📝 Roadmap

Ver [PLAN.md](PLAN.md) para el plan histórico de desarrollo.

El proyecto actualmente implementa:
- ✅ Monolito modular con Agent Runtime
- ✅ Tool registry (`file_write`, `code_execute`)
- ✅ Keyword-based classifier (coder, researcher, copywriter, assistant)
- ✅ SSE streaming con Ollama
- ✅ Multi-tenancy vía PostgreSQL RLS
- ✅ Artifact storage con versioning
- ✅ Sandbox service stub (HTTP)

## 📄 Licencia

MIT — R0LM0
